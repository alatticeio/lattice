package models

// DashboardResponse 全域视角返回数据
type DashboardResponse struct {
	GlobalStats    []GlobalStatItem    `json:"global_stats"`
	WorkspaceUsage []WorkspaceUsageRow `json:"workspace_usage"`
	GlobalEvents   []GlobalEventItem   `json:"global_events"`
}

type GlobalStatItem struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	Unit     string `json:"unit"`
	Trend    string `json:"trend"`
	Color    string `json:"color"`
	BarWidth string `json:"barWidth"`
	TrendUp  bool   `json:"trendUp"`
}

type WorkspaceUsageRow struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Nodes   int    `json:"nodes"`
	Traffic string `json:"traffic"`
	Health  int    `json:"health"`
	Status  string `json:"status"`
}

type GlobalEventItem struct {
	Time    string `json:"time"`
	WS      string `json:"ws"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Tone    string `json:"tone"` // 映射前端色值类
}
