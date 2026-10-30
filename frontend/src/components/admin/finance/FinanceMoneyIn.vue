<template>
  <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-5" :class="loading ? 'opacity-60' : ''">
    <div class="card p-4" data-testid="finance-online">
      <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.finance.moneyIn.online') }}</p>
      <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ usd(summary?.online.credited_usd) }}</p>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.finance.moneyIn.onlineOrders', { count: summary?.online.orders ?? 0 }) }}
        · {{ t('admin.finance.moneyIn.bonus') }} {{ usd(summary?.online.bonus_usd) }}
      </p>
      <ul class="mt-2 space-y-0.5 text-sm text-gray-700 dark:text-gray-300">
        <li
          v-for="pay in summary?.online.pay_amounts ?? []"
          :key="pay.currency"
          class="flex justify-between"
          :data-testid="`finance-pay-amount-${pay.currency}`"
        >
          <span class="text-gray-500 dark:text-gray-400">{{ pay.currency }}</span>
          <span>{{ money(pay.amount, pay.currency) }}</span>
        </li>
        <li v-if="!summary?.online.pay_amounts?.length" class="text-xs text-gray-400 dark:text-gray-500">
          {{ t('admin.finance.moneyIn.noPayments') }}
        </li>
      </ul>
    </div>

    <div class="card p-4" data-testid="finance-offline">
      <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.finance.moneyIn.offline') }}</p>
      <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ usd(summary?.offline.usd) }}</p>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.finance.moneyIn.offlineCount', { count: summary?.offline.count ?? 0 }) }}
      </p>
    </div>

    <div class="card p-4" data-testid="finance-gift">
      <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.finance.moneyIn.gift') }}</p>
      <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ usd(summary?.gift.total_usd) }}</p>
      <ul class="mt-2 space-y-0.5 text-sm text-gray-700 dark:text-gray-300">
        <li v-for="part in giftParts" :key="part.key" class="flex justify-between" :data-testid="`finance-gift-${part.key}`">
          <span class="text-gray-500 dark:text-gray-400">{{ part.label }}</span>
          <span>{{ usd(part.value) }}</span>
        </li>
      </ul>
    </div>

    <div class="card p-4" data-testid="finance-corrections">
      <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.finance.moneyIn.corrections') }}</p>
      <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ usd(summary?.corrections.net_usd) }}</p>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.finance.moneyIn.correctionsCount', { count: summary?.corrections.count ?? 0 }) }}
      </p>
    </div>

    <div class="card p-4" data-testid="finance-deductions">
      <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.finance.moneyIn.deductions') }}</p>
      <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ usd(summary?.deductions?.usd) }}</p>
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.finance.moneyIn.deductionsCount', { count: summary?.deductions?.count ?? 0 }) }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FinanceSummary } from '@/api/admin/finance'
import { formatCurrency } from '@/utils/format'

const props = defineProps<{ summary: FinanceSummary | null; loading?: boolean }>()
const { t } = useI18n()

function usd(value: number | null | undefined): string {
  return formatCurrency(value ?? 0)
}

// Payment currencies come from provider data; fall back to plain text for codes Intl does not know.
function money(amount: number, currency: string): string {
  try {
    return formatCurrency(amount, currency)
  } catch {
    return `${(amount ?? 0).toFixed(2)} ${currency}`
  }
}

const giftParts = computed(() => {
  const gift = props.summary?.gift
  return [
    { key: 'admin_gift_usd', label: t('admin.finance.moneyIn.giftAdmin'), value: gift?.admin_gift_usd },
    { key: 'payment_bonus_usd', label: t('admin.finance.moneyIn.giftPaymentBonus'), value: gift?.payment_bonus_usd },
    { key: 'promo_usd', label: t('admin.finance.moneyIn.giftPromo'), value: gift?.promo_usd },
    { key: 'redeem_code_usd', label: t('admin.finance.moneyIn.giftRedeem'), value: gift?.redeem_code_usd },
    { key: 'affiliate_usd', label: t('admin.finance.moneyIn.giftAffiliate'), value: gift?.affiliate_usd }
  ]
})
</script>
