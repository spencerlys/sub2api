<template>
  <div class="grid grid-cols-2 gap-4" :class="variant === 'user' ? 'lg:grid-cols-4' : 'md:grid-cols-3 xl:grid-cols-5'">
    <div class="card p-4 flex items-center gap-3">
      <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30 text-blue-600">
        <Icon name="document" size="md" />
      </div>
      <div>
        <p class="text-xs font-medium text-gray-500">{{ t('usage.totalRequests') }}</p>
        <p class="text-xl font-bold">{{ stats?.total_requests?.toLocaleString() || '0' }}</p>
        <p class="text-xs text-gray-400">{{ t('usage.inSelectedRange') }}</p>
        <p v-if="variant !== 'user' && pendingRequests > 0" data-testid="stats-pending" class="text-xs text-amber-600 dark:text-amber-400">{{ t('admin.usage.pendingRequests', { count: pendingRequests.toLocaleString() }) }}</p>
      </div>
    </div>
    <div class="card p-4 flex items-center gap-3">
      <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30 text-amber-600"><svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" /></svg></div>
      <div>
        <p class="text-xs font-medium text-gray-500">{{ t('usage.totalTokens') }}</p>
        <p class="text-xl font-bold">{{ formatTokens(stats?.total_tokens || 0) }}</p>
        <p class="flex flex-wrap items-center gap-x-1 text-xs text-gray-500">
          <span>{{ t('usage.in') }}: {{ formatTokens(stats?.total_input_tokens || 0) }}</span>
          <span>/</span>
          <span>{{ t('usage.out') }}: {{ formatTokens(stats?.total_output_tokens || 0) }}</span>
          <span>/</span>
          <span class="group relative inline-flex cursor-help items-center gap-0.5" tabindex="0">
            <span>{{ cacheLabel() }}: {{ formatTokens(stats?.total_cache_tokens || 0) }}</span>
            <svg
              class="h-3.5 w-3.5 text-gray-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span
              class="pointer-events-none absolute left-1/2 top-full z-30 mt-2 hidden w-56 -translate-x-1/2 rounded-lg border border-gray-200 bg-white p-3 text-left text-xs text-gray-700 shadow-lg group-hover:block group-focus:block dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200"
            >
              <span class="mb-2 block font-medium text-gray-900 dark:text-white">
                {{ cacheDetailLabel() }}
              </span>
              <span class="flex items-center justify-between gap-3">
                <span>{{ t('usage.cacheCreationTokensLabel') }}</span>
                <span class="tabular-nums">
                  {{ formatTokens(stats?.total_cache_creation_tokens || 0) }}
                </span>
              </span>
              <span class="mt-1 flex items-center justify-between gap-3">
                <span>{{ t('usage.cacheReadTokensLabel') }}</span>
                <span class="tabular-nums">
                  {{ formatTokens(stats?.total_cache_read_tokens || 0) }}
                </span>
              </span>
            </span>
          </span>
        </p>
      </div>
    </div>
    <!-- owner:只看官方标价消耗与自己的收益(不含赠送),绝不展示用户实扣/倍率/账号成本 -->
    <template v-if="variant === 'owner'">
      <div class="card p-4 flex items-center gap-3">
        <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30 text-green-600">
          <Icon name="dollar" size="md" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-xs font-medium text-gray-500">{{ t('admin.usage.officialCostTotal') }}</p>
          <p class="text-xl font-bold text-green-600" data-testid="stats-official-cost">${{ (stats?.total_cost || 0).toFixed(4) }}</p>
          <p class="text-xs text-gray-400">{{ t('usage.inSelectedRange') }}</p>
        </div>
      </div>
      <div class="card p-4 flex items-center gap-3">
        <div class="rounded-lg bg-emerald-100 p-2 dark:bg-emerald-900/30 text-emerald-600">
          <Icon name="dollar" size="md" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-xs font-medium text-gray-500">{{ t('admin.usage.totalEarnings') }}</p>
          <p class="text-xl font-bold text-emerald-600" data-testid="stats-earnings">${{ (adminStats?.total_earnings || 0).toFixed(4) }}</p>
          <p class="text-xs text-gray-400">{{ t('admin.usage.earningsGiftExcluded') }}</p>
        </div>
      </div>
    </template>
    <template v-else>
      <div class="card p-4 flex items-center gap-3">
        <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30 text-green-600">
          <Icon name="dollar" size="md" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-xs font-medium text-gray-500">{{ variant === 'admin' ? t('admin.usage.receivable') : t('usage.totalCost') }}</p>
          <p class="text-xl font-bold text-green-600" :data-testid="variant === 'admin' ? 'stats-receivable' : 'stats-charged'">
            ${{ (stats?.total_actual_cost || 0).toFixed(4) }}
          </p>
          <p class="flex flex-wrap gap-x-1 text-xs text-gray-400">
            <span>
              {{ t('usage.officialCost') }}
              <span data-testid="stats-official-cost" :class="{ 'line-through': variant === 'user' && discountRatio != null && discountRatio < 1 }">${{ (stats?.total_cost || 0).toFixed(4) }}</span>
            </span>
            <span>·</span>
            <span>{{ t('usage.discountRate') }} <span data-testid="stats-discount">{{ discountRatio != null ? `${formatMultiplier(discountRatio)}x` : '-' }}</span></span>
          </p>
          <p v-if="variant === 'admin' && (adminStats?.total_paid_cost != null || adminStats?.total_gift_cost != null)" class="flex flex-wrap gap-x-1 text-xs text-gray-400">
            <span>{{ t('admin.usage.paidPortion') }} <span data-testid="stats-paid">${{ (adminStats?.total_paid_cost || 0).toFixed(4) }}</span></span>
            <span>·</span>
            <span>{{ t('admin.usage.giftPortion') }} <span data-testid="stats-gift">${{ (adminStats?.total_gift_cost || 0).toFixed(4) }}</span></span>
          </p>
        </div>
      </div>
      <div v-if="variant === 'admin'" class="card p-4 flex items-center gap-3">
        <div class="rounded-lg bg-orange-100 p-2 dark:bg-orange-900/30 text-orange-600">
          <Icon name="dollar" size="md" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-xs font-medium text-gray-500">{{ t('admin.usage.accountCostTotal') }}</p>
          <p class="text-xl font-bold text-orange-500" data-testid="stats-account-cost">${{ (adminStats?.total_account_cost || 0).toFixed(4) }}</p>
          <p class="text-xs text-gray-400">
            {{ t('admin.usage.ownerPayable') }}
            <span class="text-emerald-600 dark:text-emerald-400" data-testid="stats-owner-payable">${{ (adminStats?.total_earnings || 0).toFixed(4) }}</span>
          </p>
          <p v-if="adminStats?.total_gift_account_cost != null" class="text-xs text-gray-400">
            {{ t('admin.usage.giftAccountCostRef') }}
            <span data-testid="stats-gift-account-cost">${{ adminStats.total_gift_account_cost.toFixed(4) }}</span>
          </p>
        </div>
      </div>
    </template>
    <div class="card p-4 flex items-center gap-3">
      <div class="rounded-lg bg-purple-100 p-2 dark:bg-purple-900/30 text-purple-600">
        <Icon name="clock" size="md" />
      </div>
      <div><p class="text-xs font-medium text-gray-500">{{ t('usage.avgDuration') }}</p><p class="text-xl font-bold">{{ formatDuration(stats?.average_duration_ms || 0) }}</p></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'
import type { UsageStatsResponse } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import { formatMultiplier } from '@/utils/formatters'

/**
 * admin: 官方价/折扣/应收(现金+赠送)/账号成本/号主应付
 * owner: 仅官方价与收益(不含赠送),不渲染用户实扣、倍率与账号成本
 * user:  官方价/折扣/实扣
 */
type UsageStatsVariant = 'admin' | 'owner' | 'user'

const props = withDefaults(defineProps<{
  stats: (AdminUsageStatsResponse | UsageStatsResponse) | null
  variant?: UsageStatsVariant
}>(), {
  variant: 'admin',
})

const { t } = useI18n()

const adminStats = computed(() => props.stats as AdminUsageStatsResponse | null)
const pendingRequests = computed(() => adminStats.value?.pending_requests ?? 0)
// 汇总折扣 = 应收 / 官方价
const discountRatio = computed(() => {
  const totalCost = props.stats?.total_cost ?? 0
  const actual = props.stats?.total_actual_cost
  if (!(totalCost > 0) || actual == null) return null
  return actual / totalCost
})

const formatDuration = (ms: number) =>
  ms < 1000 ? `${ms.toFixed(0)}ms` : `${(ms / 1000).toFixed(2)}s`

const formatTokens = (value: number) => {
  if (value >= 1e9) return (value / 1e9).toFixed(2) + 'B'
  if (value >= 1e6) return (value / 1e6).toFixed(2) + 'M'
  if (value >= 1e3) return (value / 1e3).toFixed(2) + 'K'
  return value.toLocaleString()
}

const cacheLabel = () => t('usage.cacheTotal')
const cacheDetailLabel = () => t('usage.cacheBreakdown')
</script>
