package database

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/minjieguo/infra/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// defaultSQLiteDataDir 为 sqlite 数据目录的默认值。
const defaultSQLiteDataDir = "./data"

// Config 数据库配置
type Config struct {
	Type    string
	DSN     string // sqlite 为数据库文件名, postgres 为完整 DSN
	DataDir string // sqlite 数据目录, 为空时使用默认值 ./data; postgres 忽略该字段
	Debug   bool
	Logger  logger.Logger
}

// Client 数据库客户端。
type Client struct {
	db *gorm.DB
}

func New(cfg Config) (*Client, error) {
	var dialector gorm.Dialector
	switch cfg.Type {
	case "sqlite":
		if cfg.DSN == "" {
			return nil, fmt.Errorf("sqlite dsn is null")
		}
		dataDir := cfg.DataDir
		if dataDir == "" {
			dataDir = defaultSQLiteDataDir
		}
		// sqlite 驱动不会自动创建父目录, 目录不存在时打开数据库会失败, 因此先确保目录存在。
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
		dialector = sqlite.Open(filepath.Join(dataDir, cfg.DSN))
	case "postgres":
		if cfg.DSN == "" {
			return nil, fmt.Errorf("postgres dns is null")
		}
		dialector = postgres.Open(cfg.DSN)
	default:
		return nil, fmt.Errorf("undefined db type:%s", cfg.Type)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy:                           schema.NamingStrategy{SingularTable: true},
		Logger:                                   newLogger(cfg.Logger, cfg.Debug),
		NowFunc:                                  func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	return &Client{db: db}, nil
}

func (c *Client) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	db, err := c.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

func (c *Client) DB() *gorm.DB {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Session(&gorm.Session{NewDB: true})
}
