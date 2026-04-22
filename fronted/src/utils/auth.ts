const TOKEN_KEY = 'wf_token'

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