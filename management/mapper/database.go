package mapper

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"sync"
	"time"
)

type DatabaseConfig struct {
	Host     string `yaml:"dsn,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Name     string `yaml:"Name,omitempty"`
	User     string `yaml:"user,omitempty"`
	Password string `yaml:"password,omitempty"`
}

var dataBaseService *DatabaseService
var once sync.Once

type DatabaseService struct {
	*gorm.DB
	cfg *DatabaseConfig
}

func NewDatabaseService(cfg *DatabaseConfig) *DatabaseService {
	once.Do(func() {
		db, err := connect(cfg)
		if err != nil {
			panic(err)
		}

		dataBaseService = &DatabaseService{
			DB: db,
		}
	})

	return dataBaseService
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

//func (d *DatabaseService) GetDB() *gorm.DB {
//	return d.DB
//}
