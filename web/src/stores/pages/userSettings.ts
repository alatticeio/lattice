import {defineStore} from 'pinia'
import {reactive, ref} from 'vue'
import {getMe, updateMe} from '@/api/user' // 假设的 API

export const useUserSettingsStore = defineStore('userSettings', () => {
    // =========================================================
    // 1. DATA (State)
    // =========================================================
    const activeTab = ref('profile')
    const isLoading = ref(false)
    const isSaving = ref(false)

    // 原始表单数据
    const form = reactive({
        name: 'Admin',
        email: 'admin@wireflow.local',
        title: 'Platform Architect', // 体现你架构师的角色
        company: 'Wireflow Cluster',
        bio: 'Infrastructure as Code. Networking as a Service. 🚀', // 更有格调的简介
        timezone: 'Asia/Shanghai',
        language: 'zh-CN',
        emailNotify: true,
        // 默认给一个彩色占位图或你的 Logo，避免页面刷新时头像区域“秃”一块
        avatarUrl: 'https://img.daisyui.com/images/stock/photo-1534528741775-53994a69daeb.webp',

        // 建议增加一些不可编辑的系统字段（如果是 Store-Driven）
        metadata: {
            lastLogin: '2026-03-09 18:00',
            version: 'v0.1.2', // 对应你最近发布的版本号
            status: 'Online'
        }
    })

    // =========================================================
    // 2. ACTIONS (Logic)
    // =========================================================
    const actions = {
        // 从后端加载初始数据
        async loadSettings() {
            isLoading.value = true
            try {
                const {data} = await getMe()
                Object.assign(form, data)
            } finally {
                isLoading.value = false
            }
        },

        // 提交更改
        async submitChanges() {
            isSaving.value = true
            try {
                await updateMe(form)
                // 可以在这里触发全局 Toast 提示
            } finally {
                isSaving.value = false
            }
        },

        setTab(tabId: string) {
            activeTab.value = tabId
        },

        handleAvatarUpload() {
            // 接入你的文件上传逻辑
            console.log('Pick avatar triggered')
        },
        // 核心：处理头像选择
        onPickAvatar() {
            const input = document.createElement('input')
            input.type = 'file'
            input.accept = 'image/*' // 仅限图片

            input.onchange = async (e: Event) => {
                const file = (e.target as HTMLInputElement).files?.[0]
                if (!file) return

                // 1. (可选) 本地预览：提升用户体验，不需要等服务器返回
                const reader = new FileReader()
                reader.onload = (event) => {
                    form.avatarUrl = event.target?.result as string
                }
                reader.readAsDataURL(file)

                // 2. 调用上传接口
                try {
                    const formData = new FormData()
                    formData.append('file', file)
                    const {data} = await uploadAvatar(formData)

                    // 3. 用服务器返回的真实 URL 覆盖预览图
                    form.avatarUrl = data.url
                    console.log('头像同步成功')
                } catch (err) {
                    console.error('上传失败', err)
                    // 如果失败，可以考虑把预览图还原或报错
                }
            }

            input.click() // 模拟点击
        }
    }

    return {
        activeTab,
        form,
        isLoading,
        isSaving,
        actions
    }
})