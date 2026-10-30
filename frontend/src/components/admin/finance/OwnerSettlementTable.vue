<template>
  <div class="overflow-x-auto" :class="loading ? 'opacity-60' : ''">
    <table class="w-full min-w-[1280px] text-sm">
      <thead class="bg-gray-50 text-left text-xs font-medium text-gray-500 dark:bg-dark-800 dark:text-gray-400">
        <tr>
          <th class="px-3 py-2">{{ t('admin.finance.settlement.owner') }}</th>
          <th
            v-for="col in columns"
            :key="col.key"
            class="px-3 py-2 text-right align-bottom"
            :class="col.reference ? 'font-normal text-gray-400 dark:text-gray-500' : ''"
            :title="col.hint"
            :data-testid="`settlement-col-${col.key}`"
            :data-reference="col.reference ? 'true' : undefined"
          >
            <span class="block">{{ col.label }}</span>
            <span v-if="col.reference" class="block text-[11px] italic">{{ t('admin.finance.settlement.referenceOnly') }}</span>
          </th>
          <th v-if="payoutAction" class="px-3 py-2 text-right">{{ t('admin.finance.settlement.actions') }}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
        <tr v-for="row in items" :key="row.owner_user_id" :data-testid="`settlement-row-${row.owner_user_id}`">
          <td class="px-3 py-2">
            <p class="font-medium text-gray-900 dark:text-white">{{ row.email }}</p>
            <p v-if="row.username" class="text-xs text-gray-500 dark:text-gray-400">{{ row.username }}</p>
          </td>
          <td
            v-for="col in columns"
            :key="col.key"
            class="whitespace-nowrap px-3 py-2 text-right"
            :class="cellClass(col, row)"
            :data-testid="`settlement-cell-${col.key}`"
          >
            <span>{{ col.format(row[col.key]) }}</span>
            <span
              v-if="col.key === 'requests' && row.pending_requests > 0"
              class="block text-xs font-normal not-italic text-amber-600 dark:text-amber-400"
              data-testid="settlement-pending"
            >
              {{ t('admin.finance.settlement.pendingRequests', { count: formatNumber(row.pending_requests) }) }}
            </span>
          </td>
          <td v-if="payoutAction" class="px-3 py-2 text-right">
            <button
              type="button"
              class="btn btn-secondary btn-sm whitespace-nowrap"
              :data-testid="`settlement-payout-${row.owner_user_id}`"
              @click="emit('record-payout', row)"
            >
              {{ t('admin.finance.settlement.recordPayout') }}
            </button>
          </td>
        </tr>
        <tr v-if="!items.length">
          <td :colspan="columns.length + (payoutAction ? 2 : 1)" class="px-3 py-6 text-center text-gray-500 dark:text-gray-400">
            {{ t('admin.finance.settlement.empty') }}
          </td>
        </tr>
      </tbody>
      <tfoot v-if="totals && items.length" class="bg-gray-50 font-semibold dark:bg-dark-800">
        <tr data-testid="settlement-totals">
          <td class="px-3 py-2 text-gray-900 dark:text-white">{{ t('admin.finance.settlement.totals') }}</td>
          <td
            v-for="col in columns"
            :key="col.key"
            class="whitespace-nowrap px-3 py-2 text-right"
            :class="col.reference ? 'font-normal italic text-gray-400 dark:text-gray-500' : 'text-gray-900 dark:text-white'"
            :data-testid="`settlement-cell-${col.key}`"
          >
            {{ col.format(totals[col.key]) }}
          </td>
          <td v-if="payoutAction"></td>
        </tr>
      </tfoot>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OwnerSettlementRow, OwnerSettlementTotals } from '@/api/admin/finance'
import { formatNumber } from '@/utils/format'
import { formatPreciseUSD } from '@/utils/preciseMoney'

// Per-owner settlement. `earnings` (payable) is what the owner is owed: the
// account cost of the paid-funded part only. `gift_account_cost` is shown for
// reference and is never added to anything payable.
withDefaults(
  defineProps<{
    items: OwnerSettlementRow[]
    totals: OwnerSettlementTotals | null
    loading?: boolean
    payoutAction?: boolean
  }>(),
  { loading: false, payoutAction: false }
)
const emit = defineEmits<{ (e: 'record-payout', row: OwnerSettlementRow): void }>()

const { t } = useI18n()

type ColumnKey = Exclude<keyof OwnerSettlementTotals, 'pending_requests' | 'pending_account_cost'>

interface Column {
  key: ColumnKey
  label: string
  hint?: string
  reference?: boolean
  format: (v: number) => string
}

const count = (v: number) => formatNumber(v ?? 0)
const money = (v: number) => formatPreciseUSD(v)

const columns = computed<Column[]>(() => [
  { key: 'active_accounts', label: t('admin.finance.settlement.activeAccounts'), hint: t('admin.finance.settlement.hints.activeAccounts'), format: count },
  { key: 'requests', label: t('admin.finance.settlement.requests'), format: count },
  { key: 'official_cost', label: t('admin.finance.settlement.officialCost'), hint: t('admin.finance.settlement.hints.officialCost'), format: money },
  { key: 'user_billed', label: t('admin.finance.settlement.userBilled'), hint: t('admin.finance.settlement.hints.userBilled'), format: money },
  { key: 'paid_cost', label: t('admin.finance.settlement.paidCost'), hint: t('admin.finance.settlement.hints.paidCost'), format: money },
  { key: 'gift_cost', label: t('admin.finance.settlement.giftCost'), hint: t('admin.finance.settlement.hints.giftCost'), format: money },
  { key: 'account_cost', label: t('admin.finance.settlement.accountCost'), hint: t('admin.finance.settlement.hints.accountCost'), format: money },
  { key: 'earnings', label: t('admin.finance.settlement.earnings'), hint: t('admin.finance.settlement.hints.earnings'), format: money },
  {
    key: 'gift_account_cost',
    label: t('admin.finance.settlement.giftAccountCost'),
    hint: t('admin.finance.settlement.hints.giftAccountCost'),
    reference: true,
    format: money
  },
  { key: 'earnings_all_time', label: t('admin.finance.settlement.earningsAllTime'), format: money },
  { key: 'paid_total', label: t('admin.finance.settlement.paidTotal'), format: money },
  { key: 'unpaid', label: t('admin.finance.settlement.unpaid'), hint: t('admin.finance.settlement.hints.unpaid'), format: money }
])

function cellClass(col: Column, row: OwnerSettlementRow): string {
  if (col.reference) return 'italic text-gray-400 dark:text-gray-500'
  if (col.key === 'unpaid' && row.unpaid > 0) return 'font-semibold text-amber-600 dark:text-amber-400'
  if (col.key === 'earnings') return 'font-medium text-gray-900 dark:text-white'
  return 'text-gray-900 dark:text-gray-100'
}
</script>
