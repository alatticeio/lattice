package database

import (
	"log"
	"wireflow/management/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(dbPath string) {
	// dbPath 建议从环境变量获取，K8s 部署时指向挂载的 PV 路径
	// 例如：/data/wireflow.db
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("无法连接数据库: %v", err)
	}

	// 自动迁移表结构
	err = DB.AutoMigrate(&model.User{}, &model.Token{})
	if err != nil {
		log.Printf("自动迁移失败: %v", err)
	}

	log.Println("SQLite 数据库初始化成功")
}
