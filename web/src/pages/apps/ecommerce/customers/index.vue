<script setup lang="ts">
import { ref, computed } from 'vue'

definePage({
  meta: {
    title: 'Customers',
    description: 'Manage your customers and their orders.',
  },
})
import {
  Users, UserCheck, UserPlus, TrendingDown,
  ArrowUpRight, ArrowDownRight, Search, Filter,
  Download, Plus, MoreHorizontal, ChevronLeft, ChevronRight,
  ChevronUp, ChevronDown, ChevronsUpDown, Mail, Trash2, Eye,
} from 'lucide-vue-next'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

// ── Stats ─────────────────────────────────────────────────────────────────
const stats = [
  { title: 'Total Customers', value: '12,847', change: '+2.5%', trend: 'up' as const, icon: Users, desc: 'from last month' },
  { title: 'Active Customers', value: '9,236', change: '+1.8%', trend: 'up' as const, icon: UserCheck, desc: 'from last month' },
  { title: 'New This Month', value: '1,249', change: '+12.3%', trend: 'up' as const, icon: UserPlus, desc: 'vs last month' },
  { title: 'Churn Rate', value: '2.4%', change: '-0.5%', trend: 'down' as const, icon: TrendingDown, desc: 'vs last month' },
]

// ── Customer data ─────────────────────────────────────────────────────────
interface Customer {
  id: number
  name: string
  email: string
  initials: string
  location: string
  country: string
  orders: number
  spent: string
  status: 'Active' | 'Inactive' | 'Pending'
  joined: string
}

const allCustomers: Customer[] = [
  { id: 1, name: 'Olivia Martin', email: 'olivia.martin@email.com', initials: 'OM', location: 'New York, US', country: '🇺🇸', orders: 24, spent: '$3,240.00', status: 'Active', joined: 'Jan 15, 2023' },
  { id: 2, name: 'Jackson Lee', email: 'jackson.lee@email.com', initials: 'JL', location: 'London, UK', country: '🇬🇧', orders: 12, spent: '$1,890.00', status: 'Active', joined: 'Mar 2, 2023' },
  { id: 3, name: 'Isabella Nguyen', email: 'isabella.nguyen@email.com', initials: 'IN', location: 'Toronto, CA', country: '🇨🇦', orders: 8, spent: '$987.00', status: 'Inactive', joined: 'May 18, 2023' },
  { id: 4, name: 'William Kim', email: 'will@email.com', initials: 'WK', location: 'Seoul, KR', country: '🇰🇷', orders: 36, spent: '$5,640.00', status: 'Active', joined: 'Feb 7, 2023' },
  { id: 5, name: 'Sofia Davis', email: 'sofia.davis@email.com', initials: 'SD', location: 'Sydney, AU', country: '🇦🇺', orders: 4, spent: '$340.00', status: 'Pending', joined: 'Jul 29, 2023' },
  { id: 6, name: 'Ethan Johnson', email: 'ethan.j@email.com', initials: 'EJ', location: 'Berlin, DE', country: '🇩🇪', orders: 19, spent: '$2,780.00', status: 'Active', joined: 'Apr 11, 2023' },
  { id: 7, name: 'Mia Thompson', email: 'mia.t@email.com', initials: 'MT', location: 'Paris, FR', country: '🇫🇷', orders: 7, spent: '$890.00', status: 'Inactive', joined: 'Jun 22, 2023' },
  { id: 8, name: 'Liam Anderson', email: 'liam.a@email.com', initials: 'LA', location: 'Tokyo, JP', country: '🇯🇵', orders: 42, spent: '$7,120.00', status: 'Active', joined: 'Jan 30, 2023' },
  { id: 9, name: 'Charlotte Brown', email: 'charlotte.b@email.com', initials: 'CB', location: 'Chicago, US', country: '🇺🇸', orders: 15, spent: '$1,980.00', status: 'Active', joined: 'Aug 5, 2023' },
  { id: 10, name: 'Noah Wilson', email: 'noah.w@email.com', initials: 'NW', location: 'Madrid, ES', country: '🇪🇸', orders: 3, spent: '$210.00', status: 'Pending', joined: 'Sep 14, 2023' },
]

// ── Filters ───────────────────────────────────────────────────────────────
const search = ref('')
const statusFilter = ref<string>('All')
const sortKey = ref<keyof Customer>('name')
const sortDir = ref<'asc' | 'desc'>('asc')
const page = ref(1)
const pageSize = 7

const statusOptions = ['All', 'Active', 'Inactive', 'Pending']

function setSort(key: keyof Customer) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortDir.value = 'asc'
  }
  page.value = 1
}

const filtered = computed(() => {
  let list = allCustomers.filter(c => {
    const matchSearch = !search.value
      || c.name.toLowerCase().includes(search.value.toLowerCase())
      || c.email.toLowerCase().includes(search.value.toLowerCase())
      || c.location.toLowerCase().includes(search.value.toLowerCase())
    const matchStatus = statusFilter.value === 'All' || c.status === statusFilter.value
    return matchSearch && matchStatus
  })

  list = [...list].sort((a, b) => {
    const av = String(a[sortKey.value])
    const bv = String(b[sortKey.value])
    return sortDir.value === 'asc' ? av.localeCompare(bv) : bv.localeCompare(av)
  })

  return list
})

const totalPages = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize)))
const paged = computed(() => {
  const start = (page.value - 1) * pageSize
  return filtered.value.slice(start, start + pageSize)
})
const rangeStart = computed(() => (page.value - 1) * pageSize + 1)
const rangeEnd = computed(() => Math.min(page.value * pageSize, filtered.value.length))

// ── Selection ─────────────────────────────────────────────────────────────
const selected = ref<Set<number>>(new Set())

const allOnPageSelected = computed(() =>
  paged.value.length > 0 && paged.value.every(c => selected.value.has(c.id))
)

function toggleAll() {
  if (allOnPageSelected.value) {
    paged.value.forEach(c => selected.value.delete(c.id))
  } else {
    paged.value.forEach(c => selected.value.add(c.id))
  }
}

function toggleOne(id: number) {
  if (selected.value.has(id)) selected.value.delete(id)
  else selected.value.add(id)
}

// ── Styles ────────────────────────────────────────────────────────────────
const statusStyle: Record<string, string> = {
  Active: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400',
  Inactive: 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400',
  Pending: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
}

function sortIcon(key: keyof Customer) {
  if (sortKey.value !== key) return ChevronsUpDown
  return sortDir.value === 'asc' ? ChevronUp : ChevronDown
}
</script>

<template>
  <div class="flex flex-col gap-5 p-6">

    <!-- ── Stats Cards ────────────────────────────────────────────── -->
    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <div
        v-for="stat in stats"
        :key="stat.title"
        class="border-border bg-card rounded-xl border p-5"
      >
        <div class="flex items-center justify-between">
          <span class="text-muted-foreground text-sm font-medium">{{ stat.title }}</span>
          <div class="bg-muted rounded-lg p-1.5">
            <component :is="stat.icon" class="text-muted-foreground size-4" />
          </div>
        </div>
        <p class="mt-2 text-2xl font-bold">{{ stat.value }}</p>
        <div class="mt-1 flex items-center gap-1 text-xs">
          <component
            :is="stat.trend === 'up' ? ArrowUpRight : ArrowDownRight"
            :class="stat.trend === 'up' ? 'text-emerald-600' : 'text-red-500'"
            class="size-3.5"
          />
          <span :class="stat.trend === 'up' ? 'text-emerald-600' : 'text-red-500'" class="font-semibold">
            {{ stat.change }}
          </span>
          <span class="text-muted-foreground">{{ stat.desc }}</span>
        </div>
      </div>
    </div>

    <!-- ── Table Card ─────────────────────────────────────────────── -->
    <div class="border-border bg-card rounded-xl border">

      <!-- Filter Bar -->
      <div class="flex flex-wrap items-center gap-3 border-b border-border p-4">
        <div class="relative flex-1 min-w-48">
          <Search class="text-muted-foreground absolute left-2.5 top-1/2 size-4 -translate-y-1/2" />
          <Input
            v-model="search"
            type="search"
            placeholder="Search by name, email or location..."
            class="pl-8 h-9"
            @input="page = 1"
          />
        </div>

        <!-- Status filter -->
        <div class="flex items-center gap-1 rounded-lg border border-border p-0.5">
          <button
            v-for="opt in statusOptions"
            :key="opt"
            @click="statusFilter = opt; page = 1"
            class="rounded-md px-3 py-1 text-sm font-medium transition-colors"
            :class="statusFilter === opt
              ? 'bg-background text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground'"
          >
            {{ opt }}
          </button>
        </div>

        <div class="ml-auto flex items-center gap-2">
          <!-- Bulk actions (shown when selection active) -->
          <template v-if="selected.size > 0">
            <span class="text-muted-foreground text-sm">{{ selected.size }} selected</span>
            <Button variant="outline" size="sm" class="gap-1.5 h-9">
              <Mail class="size-3.5" />
              Email
            </Button>
            <Button variant="outline" size="sm" class="gap-1.5 h-9 text-destructive hover:text-destructive">
              <Trash2 class="size-3.5" />
              Delete
            </Button>
          </template>

          <Button variant="outline" size="sm" class="gap-1.5 h-9">
            <Filter class="size-3.5" />
            Filter
          </Button>
          <Button variant="outline" size="sm" class="gap-1.5 h-9">
            <Download class="size-3.5" />
            Export
          </Button>
          <Button size="sm" class="gap-1.5 h-9">
            <Plus class="size-3.5" />
            Add Customer
          </Button>
        </div>
      </div>

      <!-- Table -->
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-border border-b">
              <!-- Checkbox -->
              <th class="w-12 px-4 py-3">
                <input
                  type="checkbox"
                  :checked="allOnPageSelected"
                  @change="toggleAll"
                  class="accent-primary size-4 cursor-pointer rounded"
                />
              </th>
              <!-- Sortable columns -->
              <th class="px-4 py-3 text-left">
                <button @click="setSort('name')" class="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs font-medium uppercase tracking-wider">
                  Customer
                  <component :is="sortIcon('name')" class="size-3.5" />
                </button>
              </th>
              <th class="hidden px-4 py-3 text-left md:table-cell">
                <button @click="setSort('location')" class="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs font-medium uppercase tracking-wider">
                  Location
                  <component :is="sortIcon('location')" class="size-3.5" />
                </button>
              </th>
              <th class="hidden px-4 py-3 text-left lg:table-cell">
                <button @click="setSort('orders')" class="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs font-medium uppercase tracking-wider">
                  Orders
                  <component :is="sortIcon('orders')" class="size-3.5" />
                </button>
              </th>
              <th class="hidden px-4 py-3 text-left lg:table-cell">
                <button @click="setSort('spent')" class="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs font-medium uppercase tracking-wider">
                  Total Spent
                  <component :is="sortIcon('spent')" class="size-3.5" />
                </button>
              </th>
              <th class="px-4 py-3 text-left">
                <button @click="setSort('status')" class="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs font-medium uppercase tracking-wider">
                  Status
                  <component :is="sortIcon('status')" class="size-3.5" />
                </button>
              </th>
              <th class="hidden px-4 py-3 text-left xl:table-cell">
                <button @click="setSort('joined')" class="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs font-medium uppercase tracking-wider">
                  Joined
                  <component :is="sortIcon('joined')" class="size-3.5" />
                </button>
              </th>
              <th class="w-12 px-4 py-3"></th>
            </tr>
          </thead>

          <tbody>
            <tr
              v-for="customer in paged"
              :key="customer.id"
              class="border-border hover:bg-muted/30 border-b transition-colors last:border-0"
              :class="{ 'bg-muted/20': selected.has(customer.id) }"
            >
              <!-- Checkbox -->
              <td class="px-4 py-3">
                <input
                  type="checkbox"
                  :checked="selected.has(customer.id)"
                  @change="toggleOne(customer.id)"
                  class="accent-primary size-4 cursor-pointer rounded"
                />
              </td>

              <!-- Customer -->
              <td class="px-4 py-3">
                <div class="flex items-center gap-3">
                  <Avatar class="size-8 shrink-0">
                    <AvatarFallback class="text-xs font-semibold">{{ customer.initials }}</AvatarFallback>
                  </Avatar>
                  <div class="min-w-0">
                    <p class="truncate font-medium">{{ customer.name }}</p>
                    <p class="text-muted-foreground truncate text-xs">{{ customer.email }}</p>
                  </div>
                </div>
              </td>

              <!-- Location -->
              <td class="hidden px-4 py-3 md:table-cell">
                <span>{{ customer.country }}</span>
                <span class="text-muted-foreground ml-1 text-xs">{{ customer.location }}</span>
              </td>

              <!-- Orders -->
              <td class="hidden px-4 py-3 lg:table-cell">
                <span class="font-medium">{{ customer.orders }}</span>
              </td>

              <!-- Total Spent -->
              <td class="hidden px-4 py-3 lg:table-cell font-medium">
                {{ customer.spent }}
              </td>

              <!-- Status -->
              <td class="px-4 py-3">
                <span :class="statusStyle[customer.status]" class="rounded-full px-2.5 py-0.5 text-xs font-medium">
                  {{ customer.status }}
                </span>
              </td>

              <!-- Joined -->
              <td class="hidden px-4 py-3 text-muted-foreground text-xs xl:table-cell">
                {{ customer.joined }}
              </td>

              <!-- Actions -->
              <td class="px-4 py-3">
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <button class="text-muted-foreground hover:text-foreground hover:bg-muted rounded-md p-1 transition-colors">
                      <MoreHorizontal class="size-4" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" class="w-40">
                    <DropdownMenuItem class="gap-2">
                      <Eye class="size-3.5" />
                      View Profile
                    </DropdownMenuItem>
                    <DropdownMenuItem class="gap-2">
                      <Mail class="size-3.5" />
                      Send Email
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem class="gap-2 text-destructive focus:text-destructive">
                      <Trash2 class="size-3.5" />
                      Delete
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </td>
            </tr>

            <!-- Empty state -->
            <tr v-if="paged.length === 0">
              <td colspan="8" class="px-4 py-12 text-center">
                <Users class="text-muted-foreground mx-auto mb-3 size-10" />
                <p class="text-muted-foreground text-sm">No customers found.</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- ── Pagination ──────────────────────────────────────────── -->
      <div class="border-border flex items-center justify-between border-t px-4 py-3">
        <p class="text-muted-foreground text-sm">
          Showing <span class="text-foreground font-medium">{{ rangeStart }}–{{ rangeEnd }}</span>
          of <span class="text-foreground font-medium">{{ filtered.length }}</span> customers
        </p>
        <div class="flex items-center gap-1">
          <Button
            variant="outline"
            size="sm"
            class="h-8 w-8 p-0"
            :disabled="page === 1"
            @click="page--"
          >
            <ChevronLeft class="size-4" />
          </Button>

          <button
            v-for="p in totalPages"
            :key="p"
            @click="page = p"
            class="flex size-8 items-center justify-center rounded-md text-sm font-medium transition-colors"
            :class="p === page
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:bg-muted hover:text-foreground'"
          >
            {{ p }}
          </button>

          <Button
            variant="outline"
            size="sm"
            class="h-8 w-8 p-0"
            :disabled="page === totalPages"
            @click="page++"
          >
            <ChevronRight class="size-4" />
          </Button>
        </div>
      </div>
    </div>

  </div>
</template>
