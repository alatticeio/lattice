<template>
  <Teleport to="body">
    <div class="modal modal-open modal-bottom sm:modal-middle backdrop-blur-sm transition-all">
      <div class="modal-box border border-base-300 shadow-2xl p-8 text-center ">
        <div :class="['w-20 h-20 rounded-full flex items-center justify-center mx-auto mb-6',
                  type === 'danger' ? 'bg-error/10 text-error' : 'bg-primary/10 text-primary']">
          <svg v-if="type === 'danger'" class="w-10 h-10" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
          </svg>
          <svg v-else class="w-10 h-10" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/>
          </svg>
        </div>

        <h3 class="text-xl font-black mb-2">{{ title }}</h3>
        <p class="text-sm opacity-60 mb-8 leading-relaxed" v-html="message"></p>

        <div class="flex gap-3">
          <button :class="['btn flex-1 rounded-xl border-none text-white',
                        type === 'danger' ? 'bg-error hover:bg-error/80' : 'bg-primary hover:bg-primary/80']"
                  @click="$emit('confirm')">
            {{ confirmText }}
          </button>
          <button class="btn btn-ghost flex-1 rounded-xl bg-base-200" @click="$emit('cancel')">
            {{ cancelText }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
defineProps({
  title: {type: String, default: '确认操作'},
  message: {type: String, default: '您确定要执行此操作吗？'},
  confirmText: {type: String, default: '确定'},
  cancelText: {type: String, default: '取消'},
  type: {type: String, default: 'danger'} // danger 或 primary
})
defineEmits(['confirm', 'cancel'])
</script>