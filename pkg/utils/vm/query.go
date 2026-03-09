package vm

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// QuerySingleValue 封装官方 API，直接返回浮点数
func QuerySingleValue(ctx context.Context, api v1.API, query string) (float64, error) {
	// 执行查询，注意这里返回的是 model.Value 接口
	result, _, err := api.Query(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}

	// 转换结果类型：Prometheus 瞬时查询通常返回 Vector
	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		// 如果没查到数据，返回 0
		return 0, nil
	}

	// 取 Vector 中的第一个样本值
	return float64(vector[0].Value), nil
}
