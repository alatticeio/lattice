// utils/color.ts

// Workspace avatar 背景色类（带背景和文字颜色）
const WS_COLOR_CLASSES = [
    'bg-chart-1/10 text-chart-1 ring-1 ring-chart-1/20',
    'bg-chart-2/10 text-chart-2 ring-1 ring-chart-2/20',
    'bg-chart-3/10 text-chart-3 ring-1 ring-chart-3/20',
    'bg-chart-4/10 text-chart-4 ring-1 ring-chart-4/20',
    'bg-chart-5/10 text-chart-5 ring-1 ring-chart-5/20',
]

export function getWsColor(name: string): string {
    if (!name) return WS_COLOR_CLASSES[0]

    let h = 0
    // 经典的 Hash 算法，确保同一个名字永远得到同一个索引
    for (const c of name) {
        h = (h * 31 + c.charCodeAt(0)) & 0xff
    }

    // 返回对应的 Tailwind 类名
    return WS_COLOR_CLASSES[h % WS_COLOR_CLASSES.length]
}