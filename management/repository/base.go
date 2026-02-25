package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// internal/repository/base.go
type BaseRepository[T any] struct {
	db *gorm.DB
}

func NewBaseRepository[T any](db *gorm.DB) *BaseRepository[T] {
	return &BaseRepository[T]{db: db}
}

// Find 自动返回 [] *T，不需要类型转换
func (r *BaseRepository[T]) Find(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) ([]*T, error) {
	var results []*T
	err := r.db.WithContext(ctx).Scopes(scopes...).Find(&results).Error
	return results, err
}

// First 返回单个对象
// 如果找不到，gorm 会返回 ErrRecordNotFound
func (r *BaseRepository[T]) First(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) (*T, error) {
	var result T
	err := r.db.WithContext(ctx).Scopes(scopes...).First(&result).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 或者返回自定义的 NotFound 错误
		}
		return nil, err
	}
	return &result, nil
}

func (r *BaseRepository[T]) Count(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) (int64, error) {
	var total int64
	var model T
	err := r.db.WithContext(ctx).Model(&model).Scopes(scopes...).Count(&total).Error
	return total, err
}

// 把事务封装进来
func (r *BaseRepository[T]) WithTransaction(fn func(txRepo *BaseRepository[T]) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 创建一个带有事务 DB 的临时仓库实例
		txRepo := &BaseRepository[T]{db: tx}
		return fn(txRepo)
	})
}

// 创建记录
func (r *BaseRepository[T]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

// 根据 ID 获取单个（带字段选择）
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

// 批量删除
func (r *BaseRepository[T]) Delete(ctx context.Context, scopes ...func(*gorm.DB) *gorm.DB) error {
	var model T
	return r.db.WithContext(ctx).Scopes(scopes...).Delete(&model).Error
}

func (r *BaseRepository[T]) Upsert(ctx context.Context, attrs T, values T) error {
	var model T
	return r.db.WithContext(ctx).Where(attrs).Assign(values).FirstOrCreate(&model).Error
}

// 定义通用 Keyword Scope
func WithKeyword(keyword string, columns ...string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if keyword == "" || len(columns) == 0 {
			return db
		}

		// 构造第一个条件
		subQuery := db.Where(fmt.Sprintf("%s LIKE ?", columns[0]), "%"+keyword+"%")

		// 如果有多个字段，循环添加 Or
		for i := 1; i < len(columns); i++ {
			subQuery = subQuery.Or(fmt.Sprintf("%s LIKE ?", columns[i]), "%"+keyword+"%")
		}

		return db.Where(subQuery)
	}
}
