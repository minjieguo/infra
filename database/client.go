package database

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Config 数据库配置
type Config struct {
	Type   string
	DSN    string
	Debug  bool
	Logger LogWriter
}

// Client 数据库客户端。
type Client struct {
	db *gorm.DB
}

func New(cfg Config) (*Client, error) {
	var dialector gorm.Dialector
	switch cfg.Type {
	case "sqlite":
		dialector = sqlite.Open("./data/" + cfg.DSN)
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
