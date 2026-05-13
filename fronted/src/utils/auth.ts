const TOKEN_KEY = 'wf_token'
const REFRESH_TOKEN_KEY = 'wf_refresh_token'

/**
 * 存入 Token
 */
export const setToken = (token: string) => {
    localStorage.setItem(TOKEN_KEY, token)
}

/**
 * 读取 Token
 */
export const getToken = () => {
    return localStorage.getItem(TOKEN_KEY)
}

/**
 * 删除 Token
 */
export const removeToken = () => {
    localStorage.removeItem(TOKEN_KEY)
}

/**
 * 快速判断是否有 Token
 */
export const hasToken = () => {
    const token = getToken()
    // 额外增加对 'undefined' 字符串的过滤，防止程序报错
    return !!token && token !== 'undefined' && token !== 'null'
}

/**
 * 存入 Refresh Token
 */
export const setRefreshToken = (token: string) => {
    localStorage.setItem(REFRESH_TOKEN_KEY, token)
}

/**
 * 读取 Refresh Token
 */
export const getRefreshToken = (): string | null => {
    return localStorage.getItem(REFRESH_TOKEN_KEY)
}

/**
 * 删除 Refresh Token
 */
export const removeRefreshToken = () => {
    localStorage.removeItem(REFRESH_TOKEN_KEY)
}

/**
 * 清除所有认证信息
 */
export const clearAuth = () => {
    removeToken()
    removeRefreshToken()
}