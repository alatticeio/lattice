import { defineStore } from 'pinia'
import { ref, reactive, computed } from 'vue'
import { listPolicy, createPolicy, deletePolicy, updatePolicy } from '@/api/policy'
import { useTable } from "@/composables/useApi"
import { useConfirm } from '@/composables/useConfirm'

export const usePolicyPageStore = defineStore('policyPage', () => {
    const { confirm } = useConfirm()

    // --- 1. State ---
    const { rows, total, loading, params, refresh } = useTable(listPolicy, {
        successMsg: '刷新列表成功',
        errorMsg: '刷新列表失败',
        // 显式声明初始参数，防止 0, 0 出现
        initialParams: {
            page: 1,
            pageSize: 4,
        }
    })

    const ui = reactive({
        isDrawerOpen: false,
        drawerType: 'view' as 'view' | 'edit' | 'create',
    })

    const getEmptyPolicy = () => ({
        name: '',
        action: 'Allow',
        description: '',
        _targetLabel: 'app=web',
        peerSelector: { matchLabels: {} },
        policyTypes: ['Ingress'] as string[],
        ingress: [] as any[],
        egress: [] as any[]
    })

    const form = ref(getEmptyPolicy())
    const activePolicy = ref(getEmptyPolicy())

    // --- 2. Helpers ---
    const parseLabel = (str: string) => {
        if (!str || !str.includes('=')) return { key: 'app', value: str || '""' };
        const [k, v] = str.split('=');
        return { key: k.trim(), value: v.trim() };
    }

    const validateLabel = (str: string) => {
        return /^[a-z0-9A-Z/._-]+=[a-z0-9A-Z/._-]+$/.test(str);
    }

    // --- 3. Getters (YAML Preview) ---
    const yamlPreview = computed(() => {
        const target = parseLabel(form.value._targetLabel);
        let yaml = `apiVersion: wireflowcontroller.wireflow.run/v1alpha1\nkind: WireflowPolicy\nmetadata:\n  name: ${form.value.name || 'new-policy'}\nspec:\n  peerSelector:\n    matchLabels:\n      ${target.key}: ${target.value}`

        const renderRules = (rules: any[], direction: 'ingress' | 'egress') => {
            if (!form.value.policyTypes.includes(direction === 'ingress' ? 'Ingress' : 'Egress')) return ''
            let res = `\n  ${direction}:`
            rules.forEach(r => {
                const p = parseLabel(r._rawLabel);
                const peerKey = direction === 'ingress' ? 'from' : 'to'
                res += `\n    - ${peerKey}:\n        - peerSelector:\n            matchLabels:\n              ${p.key}: ${p.value}`
                if (r.ports?.[0]?.port) {
                    res += `\n      ports:\n        - protocol: ${r.ports[0].protocol || 'TCP'}\n          port: ${r.ports[0].port}`
                }
            })
            return res
        }

        yaml += renderRules(form.value.ingress, 'ingress')
        yaml += renderRules(form.value.egress, 'egress')
        return yaml
    })

    // --- 4. Actions ---
    const actions = {
        refresh,

        openDrawer(type: 'view' | 'edit' | 'create', policy?: any) {
            ui.drawerType = type
            if (type === 'create') {
                form.value = getEmptyPolicy()
                activePolicy.value = getEmptyPolicy()
            } else {
                const data = JSON.parse(JSON.stringify(policy))
                // 还原辅助字段 _targetLabel
                const labels = data.peerSelector?.matchLabels || {}
                const firstKey = Object.keys(labels)[0]
                data._targetLabel = firstKey ? `${firstKey}=${labels[firstKey]}` : 'app=web'

                // 还原规则中的 _rawLabel
                const restoreRaw = (rules: any[], dir: string) => {
                    return (rules || []).map(r => {
                        const peerKey = dir === 'ingress' ? 'from' : 'to'
                        const mLabels = r[peerKey]?.[0]?.peerSelector?.matchLabels || {}
                        const k = Object.keys(mLabels)[0]
                        return { ...r, _rawLabel: k ? `${k}=${mLabels[k]}` : 'app=web' }
                    })
                }
                data.ingress = restoreRaw(data.ingress, 'ingress')
                data.egress = restoreRaw(data.egress, 'egress')

                form.value = data
                activePolicy.value = data
            }
            ui.isDrawerOpen = true
        },

        addRule(direction: 'ingress' | 'egress') {
            const newRule = {
                _rawLabel: 'app=web',
                [direction === 'ingress' ? 'from' : 'to']: [{ peerSelector: { matchLabels: {} } }],
                ports: [{ protocol: 'TCP', port: '' }]
            }
            form.value[direction].push(newRule)
        },

        applyTemplate(key: string) {
            const base = getEmptyPolicy()
            const templates: any = {
                isolate: { name: 'deny-all', policyTypes: ['Ingress', 'Egress'], ingress: [], egress: [] },
                db: {
                    name: 'db-protection', _targetLabel: 'app=postgres', policyTypes: ['Ingress'],
                    ingress: [{ _rawLabel: 'app=backend', ports: [{ protocol: 'TCP', port: '5432' }] }]
                },
                internet: { name: 'allow-egress', policyTypes: ['Egress'], egress: [{ _rawLabel: 'role=any', ports: [{ protocol: 'TCP', port: '443' }] }] }
            }
            form.value = { ...base, ...templates[key] }
        },

        async handleCreateOrUpdate(toast:any) {
            // 1. 校验逻辑
            if (!validateLabel(form.value._targetLabel)) return toast("主标签格式错误", "error")

            loading.value = true
            try {
                const payload = JSON.parse(JSON.stringify(form.value))
                const target = parseLabel(payload._targetLabel)
                payload.peerSelector.matchLabels = { [target.key]: target.value }

                const process = (rules: any[], dir: string) => {
                    rules.forEach(r => {
                        const p = parseLabel(r._rawLabel)
                        const peerKey = dir === 'ingress' ? 'from' : 'to'
                        r[peerKey] = [{ peerSelector: { matchLabels: { [p.key]: p.value } } }]
                        delete r._rawLabel
                        if (r.ports?.[0]) r.ports[0].port = parseInt(r.ports[0].port, 10)
                    })
                }
                process(payload.ingress, 'ingress')
                process(payload.egress, 'egress')
                delete payload._targetLabel

                if (ui.drawerType === 'create') await createPolicy(payload)
                else await updatePolicy(payload)

                toast("操作成功")
                ui.isDrawerOpen = false
                refresh()
            } catch (e) {
                toast("请求失败", "error")
            } finally {
                loading.value = false
            }
        },

        async handleDelete(policy: any, toast:any) {
            const ok = await confirm({ title: '确认删除？', message: `策略: ${policy.name}`, type: 'danger' })
            if (ok) {
                loading.value = true
                try {
                    await deletePolicy(policy.name)
                    toast("删除成功")
                    refresh()
                } finally {
                    loading.value = false
                }
            }
        }
    }

    return { rows, total, loading, params, ui, form, activePolicy, yamlPreview, actions }
})