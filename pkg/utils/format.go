package utils

import (
	"fmt"
	"strings"
)

// AutoFormat using for monitor
func AutoFormat(metricName string, val float64) string {
	// 逻辑：根据指标名的后缀，自动匹配转换函数
	switch {
	case strings.HasSuffix(metricName, "_percent"):
		return fmt.Sprintf("%.1f%%", val)

	case strings.HasSuffix(metricName, "_bytes"):
		// 自动转换 B, KB, MB, GB
		return formatBytes(val)

	case strings.HasSuffix(metricName, "_seconds"):
		// 自动转换 1h 20m 3s
		return formatDuration(int64(val))

	case strings.HasSuffix(metricName, "_count"):
		// 比如重连次数，直接转整数
		return fmt.Sprintf("%d", int64(val))

	default:
		// 兜底：保留两位小数
		return fmt.Sprintf("%.2f", val)
	}
}

// formatBytes 将字节数转换为可读的单位 (GB, MB, KB)
func formatBytes(b float64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%.2f B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", b/float64(div), "KMGTPE"[exp])
}

// formatDuration 将秒数转换为 1d 2h 3m 这种格式
func formatDuration(seconds int64) string {
	if seconds <= 0 {
		return "Starting..."
	}

	d := seconds / 86400
	h := (seconds % 86400) / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60

	if d > 0 {
		return fmt.Sprintf("%dd %dh", d, h)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
