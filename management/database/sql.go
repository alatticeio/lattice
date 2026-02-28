package database

import (
	"log"
	"time"
	"wireflow/management/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(dsn string) {
	var err error

	// 建议增加重试机制，因为 K8s 启动时数据库可能还没准备好
	for i := 0; i < 5; i++ {
		DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info), // 打印 SQL 日志，方便调试 403 问题
		})
		if err == nil {
			break
		}
		log.Printf("数据库连接失败，正在重试... (%d/5)", i+1)
		time.Sleep(5 * time.Second)
	}

	if err != nil {
		log.Fatal("无法连接到 MariaDB:", err)
	}

	// 设置连接池
	sqlDB, _ := DB.DB()
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移表结构
	err = DB.AutoMigrate(&model.User{}, &model.Token{}, &model.Workspace{}, &model.WorkspaceMember{})
	if err != nil {
		log.Printf("自动迁移失败: %v", err)
	}
}
