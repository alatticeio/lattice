// utils/color.ts

// 对应你 CSS 中的 --color-chart-1 到 --color-chart-5
const WS_COLOR_CLASSES = [
    'text-chart-1',
    'text-chart-2',
    'text-chart-3',
    'text-chart-4',
    'text-chart-5',
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