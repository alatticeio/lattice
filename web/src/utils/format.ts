/**
 * 格式化字节或速率 (如 1024 -> 1.00 KB 或 1.00 KB/s)
 */
export function formatBytes(bytes?:any, decimals = 2, isRate = false) {
    if (bytes === 0) return isRate ? '0 B/s' : '0 B'

    const k = 1024
    const dm = decimals < 0 ? 0 : decimals
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

    const i = Math.floor(Math.log(bytes) / Math.log(k))
    const value = parseFloat((bytes / Math.pow(k, i)).toFixed(dm))

    return `${value} ${sizes[i]}${isRate ? '/s' : ''}`
}