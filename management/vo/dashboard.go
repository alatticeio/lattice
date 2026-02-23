package vo

// const stats = [
//  { label: '在线节点', value: '12', trend: '+2', trendUp: true, color: 'text-emerald-500', barWidth: 'w-2/3' },
//  { label: '编排策略', value: '08', trend: 'Active', trendUp: true, color: 'text-blue-500', barWidth: 'w-1/2' },
//  { label: '活跃隧道', value: '23', trend: 'Live', trendUp: true, color: 'text-amber-500', barWidth: 'w-3/4' },
//  { label: '系统告警', value: '00', trend: 'Healthy', trendUp: false, color: 'text-slate-400', barWidth: 'w-0' },
//]

// OverviewStats 对应前端 Stats 卡片
type OverviewStats struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	Trend    string `json:"trend"`
	TrendUp  bool   `json:"trend_up"` // 改为 bool，前端逻辑判断更方便
	Color    string `json:"color"`    // 返回标识符（如 emerald, blue），前端映射颜色
	Progress int    `json:"progress"` // 返回 0-100 的数值，对应 barWidth
}

// DashboardVo 整体返回结构
type DashboardVo struct {
	Stats        []OverviewStats `json:"stats"`         // 对应顶部的四个卡片
	SystemHealth float64         `json:"system_health"` // 对应右上角 Health 状态
}
