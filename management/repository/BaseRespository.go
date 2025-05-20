package repository

import (
	"context"
	"gorm.io/gorm"
	"linkany/management/utils"
	"linkany/pkg/log"
)

type BaseRepository[T any] interface {
	WithTx(tx *gorm.DB) BaseRepository[T]
	FindByCondition(ctx context.Context, condition *utils.QueryConditions) ([]T, error)
}

// nodeBaseRepository 是一个通用的基础仓库实现
// 它实现了 BaseRepository 接口，并提供了 FindByCondition 方法
// 该方法可以根据传入的查询条件从数据库中查找数据
// 该实现使用了 GORM 作为 ORM 框架
// 该实现是线程安全的，可以在多个 goroutine 中共享
// 该实现使用了泛型，可以处理任意类型的数据
// 该实现使用了日志记录器，可以记录查询操作的日志
// 该实现使用了上下文，可以在查询操作中传递请求范围的信息
// 该实现使用了错误处理，可以在查询操作中返回错误信息

var (
	_ BaseRepository[any] = (*nodeBaseRepository[any])(nil)
)

type nodeBaseRepository[T any] struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewNodeBaseRepository[T any](db *gorm.DB) BaseRepository[T] {
	return &nodeBaseRepository[T]{
		db:     db,
		logger: log.NewLogger(log.Loglevel, "base-repository"),
	}
}

func (r *nodeBaseRepository[T]) WithTx(tx *gorm.DB) BaseRepository[T] {
	return &nodeBaseRepository[T]{
		db:     tx,
		logger: r.logger,
	}
}

func (r *nodeBaseRepository[T]) FindByCondition(ctx context.Context, condition *utils.QueryConditions) ([]T, error) {
	var (
		items []T
		err   error
	)
	query := condition.BuildQuery(r.db.WithContext(ctx))

	if err = query.Find(&items).Error; err != nil {
		return nil, err
	}

	return items, nil
}
