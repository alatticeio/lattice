package vo

// const stats = [
//  { label: 'Online Nodes', value: '12', trend: '+2', trendUp: true, color: 'text-emerald-500', barWidth: 'w-2/3' },
//  { label: 'Orchestration Policies', value: '08', trend: 'Active', trendUp: true, color: 'text-blue-500', barWidth: 'w-1/2' },
//  { label: 'Active Tunnels', value: '23', trend: 'Live', trendUp: true, color: 'text-amber-500', barWidth: 'w-3/4' },
//  { label: 'System Alerts', value: '00', trend: 'Healthy', trendUp: false, color: 'text-slate-400', barWidth: 'w-0' },
//]

// OverviewStats corresponds to frontend Stats cards
type OverviewStats struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	Trend    string `json:"trend"`
	TrendUp  bool   `json:"trend_up"` // Changed to bool for easier frontend logic
	Color    string `json:"color"`    // Returns an identifier (e.g. emerald, blue); frontend maps to colors
	Progress int    `json:"progress"` // Returns a value from 0-100, corresponds to barWidth
}

// DashboardVo overall return structure
type DashboardVo struct {
	Stats        []OverviewStats `json:"stats"`         // Corresponds to the four cards at the top
	SystemHealth float64         `json:"system_health"` // Corresponds to the Health status in the top-right
}
