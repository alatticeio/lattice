package notifier

import "time"

// Notifier 告警通知接口
type Notifier interface {
	// Name 返回通知器名称
	Name() string

	// Notify 发送告警通知
	Notify(alerts []Alert) error

	// IsAvailable 检查通知服务是否可用
	IsAvailable() bool
}

// Alert 告警信息结构
type Alert struct {
	ID          string
	Name        string
	Description string
	MetricName  string
	MetricValue interface{}
	Threshold   float64
	Severity    string     // 严重程度: info, warning, error, critical
	Status      string     // 状态: firing, resolved
	StartTime   time.Time  // 告警开始时间
	EndTime     *time.Time // 告警结束时间，如果仍在告警中则为nil
	Labels      map[string]string
}
