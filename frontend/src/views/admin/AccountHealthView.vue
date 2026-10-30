<template>
  <AppLayout>
    <TablePageLayout>
      <template #actions>
        <OwnerEarningsCard v-if="isOwner" />
        <AccountHealthTotals v-else :items="items" />
      </template>

      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="tabs inline-flex" role="tablist" :aria-label="t('admin.accountHealth.rangeLabel')">
            <button
              v-for="option in ACCOUNT_HEALTH_RANGES"
              :key="option"
              type="button"
              role="tab"
              class="tab"
              :class="range === option ? 'tab-active' : ''"
              :aria-selected="range === option"
              :data-testid="`account-health-range-${option}`"
              @click="setRange(option)"
            >
              {{ t(`admin.ops.timeRange.${option}`) }}
            </button>
          </div>

          <OwnerSelect v-if="!isOwner" v-model="ownerUserId" />

          <span v-if="windowText" class="text-xs text-gray-500 dark:text-gray-400">{{ windowText }}</span>

          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="loading"
              :title="t('common.refresh')"
              data-testid="account-health-refresh"
              @click="load"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading" row-key="id">
          <template #cell-name="{ row }">
            <div class="flex min-w-0 flex-col gap-1">
              <router-link
                to="/admin/accounts"
                class="truncate font-medium text-gray-900 hover:text-primary-600 dark:text-white dark:hover:text-primary-400"
                :title="row.name"
              >
                {{ row.name }}
              </router-link>
              <PlatformTypeBadge :platform="row.platform" :type="row.type" />
            </div>
          </template>

          <template #cell-health="{ row }">
            <span
              class="badge"
              :class="ACCOUNT_HEALTH_BADGE_CLASS[healthOf(row)]"
              :title="healthTitle(row)"
              :data-testid="`account-health-badge-${row.id}`"
              :data-health="healthOf(row)"
            >
              {{ t(`admin.accountHealth.status.${healthOf(row)}`) }}
            </span>
          </template>

          <template #cell-requests_ok="{ row }">
            <span class="text-sm text-gray-900 dark:text-gray-100">{{ formatNumber(row.requests_ok) }}</span>
          </template>

          <template #cell-error_rate="{ row }">
            <span
              class="text-sm"
              :class="errorRateClass(row)"
              :title="errorBreakdownTitle(row)"
              :data-testid="`account-health-error-rate-${row.id}`"
            >
              {{ formatPercent(row.error_rate) }}
              <span class="text-xs text-gray-400 dark:text-gray-500">({{ formatNumber(row.errors_total) }})</span>
            </span>
          </template>

          <template #cell-ttft_p50_ms="{ row }">
            <span class="whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">
              {{ formatMs(row.ttft_p50_ms) }}
              <span class="text-gray-400 dark:text-gray-500">/ {{ formatMs(row.ttft_p95_ms) }}</span>
            </span>
          </template>

          <template #cell-duration_p50_ms="{ row }">
            <span class="whitespace-nowrap text-sm text-gray-900 dark:text-gray-100">{{ formatMs(row.duration_p50_ms) }}</span>
          </template>

          <template #cell-tokens_in="{ row }">
            <span
              class="whitespace-nowrap text-sm text-gray-900 dark:text-gray-100"
              :title="t('admin.accountHealth.tokensTitle', {
                input: formatNumberLocaleString(row.tokens_in),
                output: formatNumberLocaleString(row.tokens_out),
                cache: formatNumberLocaleString(row.tokens_cache)
              })"
            >
              {{ formatCompactNumber(row.tokens_in) }}
              <span class="text-gray-400 dark:text-gray-500">/ {{ formatCompactNumber(row.tokens_out) }}</span>
            </span>
          </template>

          <template #cell-cost="{ row }">
            <span class="text-sm text-gray-900 dark:text-gray-100">{{ formatCurrency(row.cost) }}</span>
          </template>

          <template #cell-last_success_at="{ row }">
            <span class="whitespace-nowrap text-sm text-gray-600 dark:text-gray-300" :title="formatDateTime(row.last_success_at)">
              {{ row.last_success_at ? formatRelativeTime(row.last_success_at) : '-' }}
            </span>
          </template>

          <template #cell-last_error_at="{ row }">
            <div v-if="row.last_error_at" class="flex flex-col text-sm" :title="formatDateTime(row.last_error_at)">
              <span class="whitespace-nowrap text-gray-600 dark:text-gray-300">{{ formatRelativeTime(row.last_error_at) }}</span>
              <span v-if="row.last_error_category" class="whitespace-nowrap text-xs text-gray-400 dark:text-gray-500">
                {{ t(`admin.accountHealth.errorCategory.${row.last_error_category}`) }}
                <template v-if="row.last_error_status">· {{ row.last_error_status }}</template>
              </span>
            </div>
            <span v-else class="text-sm text-gray-400 dark:text-gray-500">-</span>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.accountHealth.emptyTitle')"
              :description="t('admin.accountHealth.emptyDescription')"
            />
          </template>
        </DataTable>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { adminAPI } from '@/api/admin'
import {
  ACCOUNT_HEALTH_ERROR_CATEGORIES,
  ACCOUNT_HEALTH_RANGES,
  type AccountHealthItem,
  type AccountHealthRange
} from '@/api/admin/accountHealth'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import AccountHealthTotals from '@/components/admin/account/AccountHealthTotals.vue'
import OwnerEarningsCard from '@/components/admin/finance/OwnerEarningsCard.vue'
import OwnerSelect from '@/components/admin/finance/OwnerSelect.vue'
import {
  formatCompactNumber,
  formatCurrency,
  formatDateTime,
  formatNumber,
  formatNumberLocaleString,
  formatRelativeTime
} from '@/utils/format'
import {
  ACCOUNT_HEALTH_BADGE_CLASS,
  ACCOUNT_HEALTH_THRESHOLDS,
  deriveAccountHealth,
  type AccountHealthStatus
} from './accountHealthStatus'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
// Owners get their own earnings card; only admins can filter by owner.
const isOwner = computed(() => authStore.isOwner)

const range = ref<AccountHealthRange>('24h')
const ownerUserId = ref<number | null>(null)
const items = ref<AccountHealthItem[]>([])
const loading = ref(false)
const windowStart = ref('')
const windowEnd = ref('')
// Ignore responses from superseded requests when the range is switched quickly.
let requestSeq = 0

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.accountHealth.columns.account'), sortable: true },
  { key: 'health', label: t('admin.accountHealth.columns.health') },
  { key: 'requests_ok', label: t('admin.accountHealth.columns.requests'), sortable: true },
  { key: 'error_rate', label: t('admin.accountHealth.columns.errorRate'), sortable: true },
  { key: 'ttft_p50_ms', label: t('admin.accountHealth.columns.ttft'), sortable: true },
  { key: 'duration_p50_ms', label: t('admin.accountHealth.columns.duration'), sortable: true },
  { key: 'tokens_in', label: t('admin.accountHealth.columns.tokens'), sortable: true },
  { key: 'cost', label: t('admin.accountHealth.columns.cost'), sortable: true },
  { key: 'last_success_at', label: t('admin.accountHealth.columns.lastSuccess'), sortable: true },
  { key: 'last_error_at', label: t('admin.accountHealth.columns.lastError'), sortable: true }
])

const windowText = computed(() => {
  if (!windowStart.value || !windowEnd.value) return ''
  return `${formatDateTime(windowStart.value)} - ${formatDateTime(windowEnd.value)}`
})

async function load() {
  const seq = ++requestSeq
  loading.value = true
  try {
    const res =
      ownerUserId.value === null
        ? await adminAPI.accountHealth.getAccountHealth(range.value)
        : await adminAPI.accountHealth.getAccountHealth(range.value, ownerUserId.value)
    if (seq !== requestSeq) return
    items.value = res?.items ?? []
    windowStart.value = res?.start_time ?? ''
    windowEnd.value = res?.end_time ?? ''
  } catch (err: any) {
    if (seq !== requestSeq) return
    appStore.showError(err?.message || t('admin.accountHealth.loadFailed'))
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

function setRange(next: AccountHealthRange) {
  if (range.value === next) return
  range.value = next
  load()
}

function healthOf(row: AccountHealthItem): AccountHealthStatus {
  return deriveAccountHealth(row)
}

function healthTitle(row: AccountHealthItem): string {
  const lines = [t(`admin.accountHealth.statusHint.${healthOf(row)}`)]
  if (row.rate_limited && row.rate_limit_reset_at) {
    lines.push(t('admin.accountHealth.rateLimitedUntil', { time: formatDateTime(row.rate_limit_reset_at) }))
  }
  if (row.overloaded && row.overload_until) {
    lines.push(t('admin.accountHealth.overloadedUntil', { time: formatDateTime(row.overload_until) }))
  }
  if (row.temp_unschedulable && row.temp_unschedulable_until) {
    lines.push(t('admin.accountHealth.tempUnschedulableUntil', { time: formatDateTime(row.temp_unschedulable_until) }))
  }
  if (row.error_message) {
    lines.push(`${t('admin.accountHealth.accountError')}: ${row.error_message}`)
  }
  return lines.join('\n')
}

function errorBreakdownTitle(row: AccountHealthItem): string {
  const lines = [
    t('admin.accountHealth.errorBreakdown', {
      errors: formatNumberLocaleString(row.errors_total),
      total: formatNumberLocaleString(row.requests_ok + row.errors_total)
    })
  ]
  for (const category of ACCOUNT_HEALTH_ERROR_CATEGORIES) {
    const count = row.errors?.[category] ?? 0
    if (count > 0) lines.push(`${t(`admin.accountHealth.errorCategory.${category}`)}: ${formatNumberLocaleString(count)}`)
  }
  return lines.join('\n')
}

function errorRateClass(row: AccountHealthItem): string {
  const sample = row.requests_ok + row.errors_total
  if (sample < ACCOUNT_HEALTH_THRESHOLDS.MIN_SAMPLE) return 'text-gray-900 dark:text-gray-100'
  if (row.error_rate >= ACCOUNT_HEALTH_THRESHOLDS.FAILING_ERROR_RATE) return 'font-medium text-red-600 dark:text-red-400'
  if (row.error_rate >= ACCOUNT_HEALTH_THRESHOLDS.DEGRADED_ERROR_RATE) return 'font-medium text-amber-600 dark:text-amber-400'
  return 'text-gray-900 dark:text-gray-100'
}

function formatPercent(rate: number): string {
  const pct = (rate || 0) * 100
  return `${pct >= 10 || pct === 0 ? pct.toFixed(0) : pct.toFixed(1)}%`
}

function formatMs(ms: number | null): string {
  if (ms === null || ms === undefined) return '-'
  if (ms >= 1000) return `${(ms / 1000).toFixed(ms >= 10000 ? 0 : 1)}s`
  return `${Math.round(ms)}ms`
}

watch(ownerUserId, load)

onMounted(load)
</script>
