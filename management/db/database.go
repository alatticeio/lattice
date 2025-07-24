package db

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DatabaseConfig struct {
	Host     string `yaml:"dsn,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Name     string `yaml:"MetricName,omitempty"`
	User     string `yaml:"user,omitempty"`
	Password string `yaml:"password,omitempty"`
}

var db *gorm.DB
var once sync.Once
var lock sync.Mutex

func GetDB(cfg *DatabaseConfig) *gorm.DB {
	lock.Lock()
	defer lock.Unlock()
	if db != nil {
		return db
	}
	// Use sync.Once to ensure that the database connection is only initialized once
	// regardless of how many times GetDB is called concurrently
	once.Do(func() {
		var err error
		// Initialize the database connection
		db, err = connect(cfg)
		if err != nil {
			panic(err)
		}

	})

	return db
}

func connect(cfg *DatabaseConfig) (*gorm.DB, error) {

	newLogger := logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold: time.Second,
		LogLevel:      logger.Info,
		Colorful:      true,
	})

	dsn :=
		fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", cfg.User, cfg.Password, fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), cfg.Name)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}
