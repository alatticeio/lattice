package alert

import (
	"linkany/monitor/collector"
	"log"
	"time"
)

// ThresholdAlerter 基于阈值的告警器
type ThresholdAlerter struct {
	rules     []AlertRule
	notifiers []Notifier
}

type AlertRule struct {
	MetricName string
	Operator   string // >, <, ==, etc.
	Threshold  float64
	Duration   time.Duration
	Severity   string
}

func (a *ThresholdAlerter) Evaluate(metrics []collector.Metric) ([]collector.Alert, error) {
	// 根据规则评估指标
	// ...
	return nil, nil
}

func (a *ThresholdAlerter) Send(alerts []collector.Alert) error {
	// 通过各种渠道发送告警
	for _, notifier := range a.notifiers {
		if err := notifier.Notify(alerts); err != nil {
			log.Printf("Error sending alerts via %s: %v", notifier.Name(), err)
		}
	}
	return nil
}
