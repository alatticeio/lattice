package repository

import (
	"context"
	"fmt"
	"reflect"

	"gorm.io/gorm"
)

// internal/repository/base.go
type BaseRepository[T any] struct {
	db *gorm.DB
}

func NewBaseRepository[T any](db *gorm.DB) *BaseRepository[T] {
	return &BaseRepository[T]{db: db}
}

// Find automatically returns []*T, no type conversion needed
func (r *BaseRepository[T]) Find(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]*T, error) {
	var results []*T
	err := r.db.WithContext(ctx).Scopes(scopes...).Find(&results).Error
	return results, err
}

// First returns a single object; passes through gorm.ErrRecordNotFound if not found.
func (r *BaseRepository[T]) First(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) (*T, error) {
	var result T
	if err := r.db.WithContext(ctx).Scopes(scopes...).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

// DB returns the underlying *gorm.DB for subclasses that need Preload/Joins and other advanced operations.
func (r *BaseRepository[T]) DB() *gorm.DB {
	return r.db
}

func (r *BaseRepository[T]) Count(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) (int64, error) {
	var total int64
	var model T
	err := r.db.WithContext(ctx).Model(&model).Scopes(scopes...).Count(&total).Error
	return total, err
}

// WithTransaction wraps the transaction
func (r *BaseRepository[T]) WithTransaction(fn func(txRepo *BaseRepository[T]) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Create a temporary repository instance with the transactional DB
		txRepo := &BaseRepository[T]{db: tx}
		return fn(txRepo)
	})
}

// Create creates a record
func (r *BaseRepository[T]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

// GetByID retrieves a single record by ID (with field selection)
func (r *BaseRepository[T]) GetByID(ctx context.Context, id interface{}, preloads ...string) (*T, error) {
	var result T
	db := r.db.WithContext(ctx)
	for _, p := range preloads {
		db = db.Preload(p)
	}
	if err := db.Where("id = ?", id).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete deletes records in batches
func (r *BaseRepository[T]) Delete(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) error {
	var model T
	return r.db.WithContext(ctx).Scopes(scopes...).Delete(&model).Error
}

func (r *BaseRepository[T]) Update(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Updates(entity).Error
}

// getID helper: retrieves the primary key ID of a generic entity via reflection
// nolint:unused
func (r *BaseRepository[T]) getID(entity *T) any {
	val := reflect.ValueOf(entity).Elem()
	return val.FieldByName("ID").Interface()
}

func (r *BaseRepository[T]) Upsert(ctx context.Context, attrs T, values T) error {
	var model T
	return r.db.WithContext(ctx).Where(attrs).Assign(values).FirstOrCreate(&model).Error
}

// WithKeyword defines a generic Keyword scope
func WithKeyword(keyword string, columns ...string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if keyword == "" || len(columns) == 0 {
			return db
		}

		// Build the first condition
		subQuery := db.Where(fmt.Sprintf("%s LIKE ?", columns[0]), "%"+keyword+"%")

		// If there are multiple fields, cycle through and add Or conditions
		for i := 1; i < len(columns); i++ {
			subQuery = subQuery.Or(fmt.Sprintf("%s LIKE ?", columns[i]), "%"+keyword+"%")
		}

		return db.Where(subQuery)
	}
}
