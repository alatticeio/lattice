<script setup lang="ts">
import { ref, reactive } from "vue"
import { toast } from "vue-sonner"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogClose,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { add } from "@/api/workspace"

const open = defineModel<boolean>("open", { default: false })

const emit = defineEmits<{
  created: []
}>()

const form = reactive({
  slug: "",
  namespace: "",
  maxNodes: 10,
})

const errors = reactive({
  slug: "",
  namespace: "",
  maxNodes: "",
})

const loading = ref(false)

function validate() {
  errors.slug = ""
  errors.namespace = ""
  errors.maxNodes = ""

  let valid = true

  if (!form.slug.trim()) {
    errors.slug = "Slug 不能为空"
    valid = false
  } else if (!/^[a-z0-9-]+$/.test(form.slug.trim())) {
    errors.slug = "Slug 只能包含小写字母、数字和连字符"
    valid = false
  }

  if (!form.namespace.trim()) {
    errors.namespace = "Namespace 不能为空"
    valid = false
  } else if (!/^[a-z0-9-]+$/.test(form.namespace.trim())) {
    errors.namespace = "Namespace 只能包含小写字母、数字和连字符"
    valid = false
  }

  if (!form.maxNodes || form.maxNodes < 1) {
    errors.maxNodes = "节点数至少为 1"
    valid = false
  }

  return valid
}

async function handleSubmit() {
  if (!validate()) return

  loading.value = true
  try {
    await add({
      slug: form.slug.trim(),
      namespace: form.namespace.trim(),
      maxNodes: form.maxNodes,
    })
    toast.success("Workspace 创建成功")
    open.value = false
    emit("created")
    resetForm()
  } catch (err: any) {
    toast.error(err?.response?.data?.message || "创建失败，请重试")
  } finally {
    loading.value = false
  }
}

function resetForm() {
  form.slug = ""
  form.namespace = ""
  form.maxNodes = 10
  errors.slug = ""
  errors.namespace = ""
  errors.maxNodes = ""
}

function handleOpenChange(val: boolean) {
  if (!val) resetForm()
  open.value = val
}
</script>

<template>
  <Dialog :open="open" @update:open="handleOpenChange">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Add Workspace</DialogTitle>
        <DialogDescription>
          创建一个新的工作空间，需要指定 Slug、Namespace 及资源配额。
        </DialogDescription>
      </DialogHeader>

      <form class="grid gap-4 py-2" @submit.prevent="handleSubmit">
        <!-- Slug -->
        <div class="grid gap-1.5">
          <Label for="ws-slug">Slug</Label>
          <Input
            id="ws-slug"
            v-model="form.slug"
            placeholder="my-workspace"
            :disabled="loading"
          />
          <p v-if="errors.slug" class="text-destructive text-xs">{{ errors.slug }}</p>
        </div>

        <!-- Namespace -->
        <div class="grid gap-1.5">
          <Label for="ws-namespace">Namespace</Label>
          <Input
            id="ws-namespace"
            v-model="form.namespace"
            placeholder="default"
            :disabled="loading"
          />
          <p v-if="errors.namespace" class="text-destructive text-xs">{{ errors.namespace }}</p>
        </div>

        <!-- Max Nodes -->
        <div class="grid gap-1.5">
          <Label for="ws-max-nodes">节点数配额</Label>
          <Input
            id="ws-max-nodes"
            v-model.number="form.maxNodes"
            type="number"
            min="1"
            placeholder="10"
            :disabled="loading"
          />
          <p v-if="errors.maxNodes" class="text-destructive text-xs">{{ errors.maxNodes }}</p>
        </div>
      </form>

      <DialogFooter>
        <DialogClose as-child>
          <Button variant="outline" :disabled="loading">取消</Button>
        </DialogClose>
        <Button :disabled="loading" @click="handleSubmit">
          {{ loading ? "创建中..." : "创建" }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
