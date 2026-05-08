package utils

import (
	"fmt"
	"strings"
)

// AutoFormat using for monitor
func AutoFormat(metricName string, val float64) string {
	// Logic: auto-match conversion function based on the metric name suffix
	switch {
	case strings.HasSuffix(metricName, "_percent"):
		return fmt.Sprintf("%.1f%%", val)

	case strings.HasSuffix(metricName, "_bytes"):
		// Auto-convert B, KB, MB, GB
		return formatBytes(val)

	case strings.HasSuffix(metricName, "_seconds"):
		// Auto-convert to 1h 20m 3s format
		return formatDuration(int64(val))

	case strings.HasSuffix(metricName, "_count"):
		// For example, reconnection count — convert directly to integer
		return fmt.Sprintf("%d", int64(val))

	default:
		// Fallback: keep two decimal places
		return fmt.Sprintf("%.2f", val)
	}
}

// formatBytes converts bytes to human-readable units (GB, MB, KB)
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

// formatDuration converts seconds to format like 1d 2h 3m
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
