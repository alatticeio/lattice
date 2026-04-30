// Package db 提供数据库 Store 的工厂函数。
// 根据 DatabaseConfig.Driver 自动选择底层驱动：
//
//   - "sqlite"（或空值）→ SQLite，默认文件路径 lattice.db，DSN 即文件路径。
//     适合开源自部署场景，零额外依赖，开箱即用。
//   - "mysql" / "mariadb"    → MySQL/MariaDB，DSN 为标准连接字符串。
//     适合生产环境，支持高并发与集群部署。
//
// 两种实现共用同一套 GORM CRUD 逻辑（internal/db/gormstore），
// 切换数据库只需修改配置，无需改动业务代码。
package db

import (
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/db/gormstore"
	"log"
	"time"

	gormsqlite "github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewStore 根据 cfg.Database.Driver 创建对应的 Store 实现。
// MySQL/MariaDB 模式下内置重试机制（最多 5 次，间隔 5 秒），
// 以应对 K8s 环境中数据库容器尚未就绪的情况。
func NewStore(cfg *config.Config) (store.Store, error) {
	driver := cfg.Database.Driver
	dsn := cfg.Database.DSN

	var db *gorm.DB
	var err error

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	switch driver {
	case "mysql", "mariadb":
		db, err = openWithRetry(func() (*gorm.DB, error) {
			return gorm.Open(mysql.Open(dsn), gormCfg)
		})
		if err != nil {
			return nil, fmt.Errorf("db: connect mysql/mariadb failed: %w", err)
		}
		if sqlDB, e := db.DB(); e == nil {
			sqlDB.SetMaxIdleConns(10)
			sqlDB.SetMaxOpenConns(100)
			sqlDB.SetConnMaxLifetime(time.Hour)
		}

	default:
		// 开源默认：SQLite。DSN 为文件路径，空时使用 lattice.db。
		if dsn == "" {
			dsn = "lattice.db"
		}
		db, err = gorm.Open(gormsqlite.Open(dsn), gormCfg)
		if err != nil {
			return nil, fmt.Errorf("db: open sqlite failed (path=%s): %w", dsn, err)
		}
		// SQLite 不支持并发写，限制为单连接。
		if sqlDB, e := db.DB(); e == nil {
			sqlDB.SetMaxOpenConns(1)
		}
	}

	return gormstore.New(db)
}

// openWithRetry 尝试最多 5 次打开数据库连接，每次间隔 5 秒。
func openWithRetry(open func() (*gorm.DB, error)) (*gorm.DB, error) {
	var (
		db  *gorm.DB
		err error
	)
	for i := 1; i <= 5; i++ {
		db, err = open()
		if err == nil {
			return db, nil
		}
		log.Printf("[db] 连接失败，重试 %d/5: %v", i, err)
		time.Sleep(5 * time.Second)
	}
	return nil, err
}
