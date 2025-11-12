// Copyright 2025 Wireflow.io, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// QueryConditions GORM查询条件封装
type QueryConditions struct {
	// 查询条件
	Where   map[string]interface{}    // 精确匹配条件
	Like    map[string]interface{}    // 模糊查询条件
	In      map[string][]interface{}  // IN查询条件
	Between map[string][2]interface{} // 范围查询条件
	Or      []map[string]interface{}  // OR条件

	// 排序和分页
	OrderBy  []string // 排序条件
	Page     int      // 页码
	PageSize int      // 每页数量

	// 高级查询
	Preload []string // 预加载关联
	Select  []string // 选择特定字段
	GroupBy []string // 分组条件
	Having  string   // Having条件
}

// NewQueryConditions 创建查询条件实例
func NewQueryConditions() *QueryConditions {
	return &QueryConditions{
		Where:    make(map[string]interface{}),
		Like:     make(map[string]interface{}),
		In:       make(map[string][]interface{}),
		Between:  make(map[string][2]interface{}),
		Or:       make([]map[string]interface{}, 0),
		OrderBy:  make([]string, 0),
		Preload:  make([]string, 0),
		Select:   make([]string, 0),
		GroupBy:  make([]string, 0),
		Page:     1,
		PageSize: 10,
	}
}

// BuildQuery 构建GORM查询
func (q *QueryConditions) BuildQuery(db *gorm.DB) *gorm.DB {
	query := db

	// 设置Select
	if len(q.Select) > 0 {
		query = query.Select(q.Select)
	}

	// 添加Where条件
	if len(q.Where) > 0 {
		query = query.Where(q.Where)
	}

	// 添加Like条件
	for field, value := range q.Like {
		query = query.Where(field+" LIKE ?", fmt.Sprintf("%%%v%%", value))
	}

	// 添加In条件
	for field, values := range q.In {
		query = query.Where(field+" IN ?", values)
	}

	// 添加Between条件
	for field, values := range q.Between {
		query = query.Where(field+" BETWEEN ? AND ?", values[0], values[1])
	}

	// 添加Or条件
	for _, condition := range q.Or {
		query = query.Or(condition)
	}

	// 添加GroupBy
	if len(q.GroupBy) > 0 {
		query = query.Group(strings.Join(q.GroupBy, ", "))
	}

	// 添加Having
	if q.Having != "" {
		query = query.Having(q.Having)
	}

	// 添加预加载
	for _, preload := range q.Preload {
		query = query.Preload(preload)
	}

	// 添加排序
	for _, order := range q.OrderBy {
		query = query.Order(order)
	}

	// 添加分页
	if q.PageSize > 0 {
		offset := (q.Page - 1) * q.PageSize
		query = query.Offset(offset).Limit(q.PageSize)
	}

	return query
}

// AddWhere 添加精确匹配条件
func (q *QueryConditions) AddWhere(field string, value interface{}) *QueryConditions {
	q.Where[field] = value
	return q
}

// AddLike 添加模糊查询条件
func (q *QueryConditions) AddLike(field string, value interface{}) *QueryConditions {
	q.Like[field] = value
	return q
}

// AddIn 添加IN查询条件
func (q *QueryConditions) AddIn(field string, values []interface{}) *QueryConditions {
	q.In[field] = values
	return q
}

// AddBetween 添加范围查询条件
func (q *QueryConditions) AddBetween(field string, start, end interface{}) *QueryConditions {
	q.Between[field] = [2]interface{}{start, end}
	return q
}

// AddOr 添加OR条件
func (q *QueryConditions) AddOr(condition map[string]interface{}) *QueryConditions {
	q.Or = append(q.Or, condition)
	return q
}

// AddOrderBy 添加排序条件
func (q *QueryConditions) AddOrderBy(field string, isDesc bool) *QueryConditions {
	order := field
	if isDesc {
		order += " DESC"
	}
	q.OrderBy = append(q.OrderBy, order)
	return q
}

// AddPreload 添加预加载关联
func (q *QueryConditions) AddPreload(preload string) *QueryConditions {
	q.Preload = append(q.Preload, preload)
	return q
}

// SetPage 设置分页
func (q *QueryConditions) SetPage(page, pageSize int) *QueryConditions {
	q.Page = page
	q.PageSize = pageSize
	return q
}

// AddSelect 添加要查询的字段
func (q *QueryConditions) AddSelect(fields ...string) *QueryConditions {
	q.Select = append(q.Select, fields...)
	return q
}

// AddGroupBy 添加分组条件
func (q *QueryConditions) AddGroupBy(fields ...string) *QueryConditions {
	q.GroupBy = append(q.GroupBy, fields...)
	return q
}

// SetHaving 设置Having条件
func (q *QueryConditions) SetHaving(having string) *QueryConditions {
	q.Having = having
	return q
}
