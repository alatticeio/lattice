package service

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type DatabaseService struct {
	db *gorm.DB
}

func NewDatabaseService() *DatabaseService {
	db, err := connect()
	if err != nil {
		panic(err)
	}

	return &DatabaseService{
		db: db,
	}
}

func connect() (*gorm.DB, error) {
	// refer https://github.com/go-sql-driver/mysql#dsn-data-source-name for details
	dsn := "user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (d *DatabaseService) GetDB() *gorm.DB {
	return d.db
}
