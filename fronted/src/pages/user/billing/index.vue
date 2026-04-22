<script setup lang="ts">
import { ref } from 'vue'
import { CreditCard, Download, Zap, Check, HardDrive, FolderOpen } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import UserSettingsNav from '@/components/UserSettingsNav.vue'

definePage({
  meta: { title: 'Billing', description: 'Manage your subscription and payment methods.' },
})

const currentPlan = ref<'free' | 'pro' | 'enterprise'>('pro')

const plans = [
  {
    id: 'free',
    name: 'Free',
    price: '$0',
    period: '/mo',
    description: 'For individuals getting started.',
    features: ['Up to 5 projects', '1 GB storage', 'Basic analytics', 'Email support'],
  },
  {
    id: 'pro',
    name: 'Pro',
    price: '$12',
    period: '/mo',
    description: 'For professionals and small teams.',
    features: ['Unlimited projects', '50 GB storage', 'Advanced analytics', 'Priority support', 'API access', 'Custom domains'],
  },
  {
    id: 'enterprise',
    name: 'Enterprise',
    price: '$49',
    period: '/mo',
    description: 'For large teams and organizations.',
    features: ['Everything in Pro', '500 GB storage', 'Dedicated support', 'SSO / SAML', 'Audit logs', 'SLA guarantee'],
  },
]

const invoices = [
  { id: 'INV-2024-012', date: 'Dec 1, 2024', amount: '$12.00', status: 'Paid' },
  { id: 'INV-2024-011', date: 'Nov 1, 2024', amount: '$12.00', status: 'Paid' },
  { id: 'INV-2024-010', date: 'Oct 1, 2024', amount: '$12.00', status: 'Paid' },
  { id: 'INV-2024-009', date: 'Sep 1, 2024', amount: '$12.00', status: 'Paid' },
  { id: 'INV-2024-008', date: 'Aug 1, 2024', amount: '$12.00', status: 'Paid' },
]
</script>

<template>
  <div class="flex flex-col">
    <UserSettingsNav />

    <div class="mx-auto w-full max-w-3xl space-y-5 p-6">

      <!-- Current plan -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 border-b border-border flex items-center justify-between">
          <div class="flex items-center gap-2.5">
            <div class="size-8 rounded-lg bg-primary/10 flex items-center justify-center">
              <Zap class="size-4 text-primary" />
            </div>
            <div>
              <h2 class="text-sm font-semibold">Current Plan</h2>
              <p class="text-xs text-muted-foreground">
                Next billing: <span class="text-foreground font-medium">Jan 1, 2025</span>
              </p>
            </div>
          </div>
          <span class="text-xs font-bold px-3 py-1 rounded-full bg-primary/10 text-primary ring-1 ring-primary/20">Pro</span>
        </div>
        <div class="p-6 space-y-3">
          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs">
              <span class="flex items-center gap-1.5 text-muted-foreground"><HardDrive class="size-3" /> Storage</span>
              <span class="font-medium">18.4 GB <span class="text-muted-foreground font-normal">/ 50 GB</span></span>
            </div>
            <div class="h-1.5 bg-muted rounded-full overflow-hidden">
              <div class="h-full bg-primary rounded-full" style="width:37%" />
            </div>
          </div>
          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs">
              <span class="flex items-center gap-1.5 text-muted-foreground"><FolderOpen class="size-3" /> Projects</span>
              <span class="font-medium">12 <span class="text-muted-foreground font-normal">/ Unlimited</span></span>
            </div>
            <div class="h-1.5 bg-muted rounded-full overflow-hidden">
              <div class="h-full bg-primary/40 rounded-full" style="width:8%" />
            </div>
          </div>
        </div>
      </div>

      <!-- Plans -->
      <div>
        <h3 class="text-sm font-semibold mb-3 px-0.5">Available Plans</h3>
        <div class="grid gap-3 sm:grid-cols-3">
          <div
            v-for="plan in plans" :key="plan.id"
            class="relative flex flex-col bg-card border rounded-xl p-5 transition-all"
            :class="plan.id === currentPlan
              ? 'border-primary ring-1 ring-primary/20 shadow-sm shadow-primary/10'
              : 'border-border hover:border-muted-foreground/30'"
          >
            <div v-if="plan.id === currentPlan"
              class="absolute -top-2.5 left-1/2 -translate-x-1/2 bg-primary text-primary-foreground text-[10px] font-bold px-2.5 py-0.5 rounded-full whitespace-nowrap">
              Current plan
            </div>
            <div class="mb-4">
              <p class="text-sm font-bold">{{ plan.name }}</p>
              <div class="flex items-baseline gap-1 mt-1">
                <span class="text-2xl font-black tracking-tight">{{ plan.price }}</span>
                <span class="text-xs text-muted-foreground">{{ plan.period }}</span>
              </div>
              <p class="text-xs text-muted-foreground mt-1">{{ plan.description }}</p>
            </div>
            <ul class="flex-1 space-y-2 mb-5">
              <li v-for="f in plan.features" :key="f" class="flex items-start gap-2 text-xs">
                <Check class="size-3.5 text-primary shrink-0 mt-0.5" />
                <span class="text-muted-foreground">{{ f }}</span>
              </li>
            </ul>
            <Button :variant="plan.id === currentPlan ? 'outline' : 'default'" size="sm" :disabled="plan.id === currentPlan" class="w-full">
              {{ plan.id === currentPlan ? 'Current plan' : plan.id === 'free' ? 'Downgrade' : 'Upgrade' }}
            </Button>
          </div>
        </div>
      </div>

      <!-- Payment method -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 border-b border-border flex items-center justify-between">
          <div>
            <h2 class="text-sm font-semibold">Payment Method</h2>
            <p class="text-xs text-muted-foreground mt-0.5">Your default billing card.</p>
          </div>
          <Button variant="outline" size="sm">Update card</Button>
        </div>
        <div class="p-6">
          <div class="flex items-center gap-4 p-4 rounded-xl bg-muted/30 border border-border">
            <div class="size-11 bg-background border border-border rounded-lg flex items-center justify-center shrink-0">
              <CreditCard class="size-5 text-muted-foreground" />
            </div>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium">Visa ending in 4242</p>
              <p class="text-xs text-muted-foreground">Expires 08/2026 · admin@example.com</p>
            </div>
            <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20 shrink-0">
              Default
            </span>
          </div>
        </div>
      </div>

      <!-- Invoice history -->
      <div class="bg-card border border-border rounded-xl overflow-hidden">
        <div class="px-6 py-4 border-b border-border flex items-center justify-between">
          <div>
            <h2 class="text-sm font-semibold">Billing History</h2>
            <p class="text-xs text-muted-foreground mt-0.5">View and download past invoices.</p>
          </div>
          <Button variant="ghost" size="sm" class="gap-1.5 text-xs text-muted-foreground">
            <Download class="size-3.5" /> Export all
          </Button>
        </div>
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-border bg-muted/20">
              <th class="px-6 py-2.5 text-left text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60">Invoice</th>
              <th class="px-4 py-2.5 text-left text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60">Date</th>
              <th class="px-4 py-2.5 text-left text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60">Amount</th>
              <th class="px-4 py-2.5 text-left text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60">Status</th>
              <th class="px-6 py-2.5 w-14" />
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-for="inv in invoices" :key="inv.id" class="hover:bg-muted/20 transition-colors">
              <td class="px-6 py-3.5 font-mono text-xs font-medium">{{ inv.id }}</td>
              <td class="px-4 py-3.5 text-xs text-muted-foreground">{{ inv.date }}</td>
              <td class="px-4 py-3.5 text-sm font-semibold">{{ inv.amount }}</td>
              <td class="px-4 py-3.5">
                <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20">
                  {{ inv.status }}
                </span>
              </td>
              <td class="px-6 py-3.5">
                <button class="ml-auto flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors">
                  <Download class="size-3" /> PDF
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <p class="text-center">
        <button class="text-xs text-muted-foreground/50 hover:text-destructive transition-colors hover:underline underline-offset-4">
          Cancel subscription
        </button>
      </p>

    </div>
  </div>
</template>
