<template>
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4" :class="loading ? 'opacity-60' : ''" data-testid="finance-consumption">
    <div v-for="stat in stats" :key="stat.key" class="card p-4" :data-testid="`finance-consumption-${stat.key}`" :title="stat.hint">
      <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ stat.label }}</p>
      <p class="mt-1 text-xl font-semibold" :class="stat.className">{{ stat.value }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FinanceConsumption } from '@/api/admin/finance'
import { formatNumber } from '@/utils/format'
import { formatPreciseUSD } from '@/utils/preciseMoney'

// Usage consumed in the period: receivable from users split exactly into paid
// use (funded by paid balance) and internal use (funded by gift balance).
const props = defineProps<{ consumption: FinanceConsumption | null | undefined; loading?: boolean }>()
const { t } = useI18n()

const stats = computed(() => {
  const c = props.consumption
  const base = 'text-gray-900 dark:text-white'
  return [
    { key: 'requests', label: t('admin.finance.consumption.requests'), value: formatNumber(c?.requests ?? 0), className: base, hint: '' },
    {
      key: 'user_billed',
      label: t('admin.finance.consumption.userBilled'),
      value: formatPreciseUSD(c?.user_billed),
      className: base,
      hint: t('admin.finance.consumption.userBilledHint')
    },
    {
      key: 'paid_cost',
      label: t('admin.finance.consumption.paidCost'),
      value: formatPreciseUSD(c?.paid_cost),
      className: base,
      hint: t('admin.finance.consumption.paidCostHint')
    },
    {
      key: 'gift_cost',
      label: t('admin.finance.consumption.giftCost'),
      value: formatPreciseUSD(c?.gift_cost),
      className: base,
      hint: t('admin.finance.consumption.giftCostHint')
    }
  ]
})
</script>
