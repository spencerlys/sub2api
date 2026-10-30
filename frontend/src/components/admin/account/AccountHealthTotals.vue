<template>
  <div class="card p-4" data-testid="account-health-totals">
    <p class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.accountHealth.totals.title') }}</p>
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-8">
      <div v-for="stat in stats" :key="stat.key" :data-testid="`account-health-total-${stat.key}`" :title="stat.title">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ stat.label }}</p>
        <p class="text-base font-semibold text-gray-900 dark:text-white">{{ stat.value }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountHealthItem } from '@/api/admin/accountHealth'
import { formatCompactNumber, formatNumber, formatNumberLocaleString } from '@/utils/format'
import { formatPreciseUSD } from '@/utils/preciseMoney'

// Admin-only totals over the visible rows. `cost` is what users were billed
// (actual_cost); `earnings` is the owner payable from the exact ledger.
const props = defineProps<{ items: AccountHealthItem[] }>()
const { t } = useI18n()

const totals = computed(() =>
  props.items.reduce(
    (acc, row) => {
      acc.requests_ok += row.requests_ok || 0
      acc.errors_total += row.errors_total || 0
      acc.tokens_in += row.tokens_in || 0
      acc.tokens_out += row.tokens_out || 0
      acc.tokens_cache += row.tokens_cache || 0
      acc.cost += row.cost || 0
      acc.official_cost += row.official_cost || 0
      acc.earnings += row.earnings || 0
      return acc
    },
    { requests_ok: 0, errors_total: 0, tokens_in: 0, tokens_out: 0, tokens_cache: 0, cost: 0, official_cost: 0, earnings: 0 }
  )
)

const stats = computed(() => {
  const sum = totals.value
  const tokens = (key: 'tokens_in' | 'tokens_out' | 'tokens_cache', label: string) => ({
    key,
    label,
    value: formatCompactNumber(sum[key]),
    title: formatNumberLocaleString(sum[key])
  })
  return [
    { key: 'requests_ok', label: t('admin.accountHealth.totals.requests'), value: formatNumber(sum.requests_ok), title: '' },
    { key: 'errors_total', label: t('admin.accountHealth.totals.errors'), value: formatNumber(sum.errors_total), title: '' },
    tokens('tokens_in', t('admin.accountHealth.totals.tokensIn')),
    tokens('tokens_out', t('admin.accountHealth.totals.tokensOut')),
    tokens('tokens_cache', t('admin.accountHealth.totals.tokensCache')),
    { key: 'cost', label: t('admin.accountHealth.totals.billed'), value: formatPreciseUSD(sum.cost), title: '' },
    { key: 'official_cost', label: t('admin.accountHealth.totals.officialCost'), value: formatPreciseUSD(sum.official_cost), title: '' },
    { key: 'earnings', label: t('admin.accountHealth.totals.earnings'), value: formatPreciseUSD(sum.earnings), title: '' }
  ]
})
</script>
