<template>
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-5" :class="loading ? 'opacity-60' : ''" data-testid="finance-consumption">
    <div v-for="stat in stats" :key="stat.key" class="card p-4" :data-testid="`finance-consumption-${stat.key}`" :title="stat.hint">
      <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ stat.label }}</p>
      <p class="mt-1 text-xl font-semibold" :class="stat.className">{{ stat.value }}</p>
      <p v-if="stat.sub" class="text-xs text-gray-500 dark:text-gray-400">{{ stat.sub }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FinanceConsumption } from '@/api/admin/finance'
import { formatNumber } from '@/utils/format'
import { formatPreciseUSD } from '@/utils/preciseMoney'

// Usage consumed in the period: receivable from users split exactly into the
// part funded by paid balance and the part funded by gift balance. Requests the
// ledger has not split yet are listed separately.
const props = defineProps<{ consumption: FinanceConsumption | null | undefined; loading?: boolean }>()
const { t } = useI18n()

const stats = computed(() => {
  const c = props.consumption
  const pending = c?.pending_requests ?? 0
  const base = 'text-gray-900 dark:text-white'
  return [
    { key: 'requests', label: t('admin.finance.consumption.requests'), value: formatNumber(c?.requests ?? 0), className: base, sub: '', hint: '' },
    {
      key: 'user_billed',
      label: t('admin.finance.consumption.userBilled'),
      value: formatPreciseUSD(c?.user_billed),
      className: base,
      sub: '',
      hint: t('admin.finance.consumption.userBilledHint')
    },
    {
      key: 'paid_cost',
      label: t('admin.finance.consumption.paidCost'),
      value: formatPreciseUSD(c?.paid_cost),
      className: base,
      sub: '',
      hint: t('admin.finance.consumption.paidCostHint')
    },
    {
      key: 'gift_cost',
      label: t('admin.finance.consumption.giftCost'),
      value: formatPreciseUSD(c?.gift_cost),
      className: base,
      sub: '',
      hint: t('admin.finance.consumption.giftCostHint')
    },
    {
      key: 'pending',
      label: t('admin.finance.consumption.pending'),
      value: formatPreciseUSD(c?.pending_user_billed),
      className: pending > 0 ? 'text-amber-600 dark:text-amber-400' : base,
      sub: t('admin.finance.consumption.pendingRequests', { count: formatNumber(pending) }),
      hint: t('admin.finance.consumption.pendingHint')
    }
  ]
})
</script>
