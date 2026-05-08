// Package db provides the factory function for the database Store.
// It automatically selects the underlying driver based on DatabaseConfig.Driver:
//
//   - "sqlite" (or empty)  → SQLite, default file path lattice.db, DSN is the file path.
//     Suitable for open-source self-deployment, zero extra dependencies, ready to use.
//   - "mysql" / "mariadb"  → MySQL/MariaDB, DSN is the standard connection string.
//     Suitable for production environments, supports high concurrency and cluster deployment.
//
// Both implementations share the same GORM CRUD logic (internal/db/gormstore).
// Switching databases only requires changing the configuration, no business code changes needed.
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

// NewStore creates the corresponding Store implementation based on cfg.Database.Driver.
// MySQL/MariaDB mode has a built-in retry mechanism (up to 5 attempts, 5-second interval)
// to handle the case where the database container is not yet ready in a K8s environment.
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
		// Open-source default: SQLite. DSN is the file path, defaults to lattice.db when empty.
		if dsn == "" {
			dsn = "lattice.db"
		}
		db, err = gorm.Open(gormsqlite.Open(dsn), gormCfg)
		if err != nil {
			return nil, fmt.Errorf("db: open sqlite failed (path=%s): %w", dsn, err)
		}
		// SQLite does not support concurrent writes, limited to a single connection.
		if sqlDB, e := db.DB(); e == nil {
			sqlDB.SetMaxOpenConns(1)
		}
	}

	return gormstore.New(db)
}

// openWithRetry attempts to open the database connection up to 5 times, with a 5-second interval between attempts.
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
		log.Printf("[db] connection failed, retry %d/5: %v", i, err)
		time.Sleep(5 * time.Second)
	}
	return nil, err
}
