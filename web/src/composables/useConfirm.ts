// composables/useConfirm.js
import { h, render } from 'vue'
import ConfirmModal from '@/components/ConfirmModal.vue'

export const useConfirm = () => {
    const confirm = (options = {}) => {
        return new Promise((resolve) => {
            // 1. 创建挂载容器
            const container = document.createElement('div')
            document.body.appendChild(container)

            // 2. 销毁函数
            const destroy = () => {
                render(null, container)
                container.remove()
            }

            // 3. 创建虚拟节点
            const vnode = h(ConfirmModal, {
                ...options,
                onConfirm: () => {
                    resolve(true)
                    destroy()
                },
                onCancel: () => {
                    resolve(false)
                    destroy()
                }
            })

            // 4. 渲染到页面
            render(vnode, container)
        })
    }

    return { confirm }
}