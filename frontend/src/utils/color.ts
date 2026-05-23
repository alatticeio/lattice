// utils/color.ts

// Workspace avatar 背景色类（实心色 + 白色文字，与 Members 风格一致）
const WS_COLOR_CLASSES = [
    'bg-blue-500 text-white',
    'bg-violet-500 text-white',
    'bg-emerald-500 text-white',
    'bg-orange-500 text-white',
    'bg-rose-500 text-white',
    'bg-cyan-500 text-white',
    'bg-indigo-500 text-white',
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