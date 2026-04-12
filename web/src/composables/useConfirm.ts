import { reactive } from 'vue'

interface ConfirmOptions {
  title?: string
  message: string
  type?: 'danger' | 'warning' | 'info'
  confirmText?: string
  cancelText?: string
}

// Global singleton state — consumed by <ConfirmDialog /> mounted in the layout
export const confirmState = reactive({
  open: false,
  title: '',
  message: '',
  type: 'danger' as 'danger' | 'warning' | 'info',
  confirmText: '确认',
  cancelText: '取消',
  resolve: null as ((v: boolean) => void) | null,
})

export function useConfirm() {
  function confirm(options: ConfirmOptions): Promise<boolean> {
    confirmState.open = true
    confirmState.title = options.title ?? '确认操作'
    confirmState.message = options.message
    confirmState.type = options.type ?? 'danger'
    confirmState.confirmText = options.confirmText ?? (options.type === 'danger' ? '删除' : '确认')
    confirmState.cancelText = options.cancelText ?? '取消'

    return new Promise<boolean>(resolve => {
      confirmState.resolve = resolve
    })
  }

  return { confirm }
}
