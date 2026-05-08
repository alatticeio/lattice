package repository

import "gorm.io/gorm"

func WithID(id string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if id == "" {
			return db
		}
		return db.Where("id = ?", id)
	}
}

func WithUserID(userID string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if userID == "" {
			return db
		}
		return db.Where("user_id = ?", userID)
	}
}

func WithNamespace(namespace string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if namespace == "" {
			return db
		}
		return db.Where("namespace = ?", namespace)
	}
}

func WithUsername(username string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if username == "" {
			return db
		}
		return db.Where("username = ?", username)
	}
}

func WithWorkspaceID(wsID string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if wsID == "" {
			return db
		}
		return db.Where("workspace_id = ?", wsID)
	}
}

func WithIdentity(role string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("role = ?", role)
	}
}

// Paginate is a generic pagination Scope
func Paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		// 1. Handle defaults: if page <= 0, default to page 1
		if page <= 0 {
			page = 1
		}

		// 2. Handle pageSize: set defaults and upper limit to prevent pulling too much data and exhausting memory
		switch {
		case pageSize > 100:
			pageSize = 100
		case pageSize <= 0:
			pageSize = 10
		}

		// 3. Calculate offset
		offset := (page - 1) * pageSize
		return db.Offset(offset).Limit(pageSize)
	}
}
