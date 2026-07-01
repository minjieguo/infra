package database

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var client *gorm.DB

// Config 数据库配置
type Config struct {
	Type   string
	DSN    string
	Debug  bool
	Logger gormlogger.Interface
}

func New(cfg Config) error {
	var dialector gorm.Dialector
	switch cfg.Type {
	case "sqlite":
		dialector = sqlite.Open("./data/" + cfg.DSN)
	case "postgres":
		if cfg.DSN == "" {
			return fmt.Errorf("postgres dns is null")
		}
		dialector = postgres.Open(cfg.DSN)
	default:
		return fmt.Errorf("undefined db type:%s", cfg.Type)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy:                           schema.NamingStrategy{SingularTable: true},
		Logger:                                   cfg.Logger,
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	client = db
	return nil
}

func Close() error {
	if client == nil {
		return nil
	}
	db, err := client.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

func DB() *gorm.DB {
	return client.Session(&gorm.Session{NewDB: true})
}
