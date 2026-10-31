<template>
  <!-- Phones: one card per owner. The 12-column table would be a 1280px-wide
       horizontal scroller that takes over the page. -->
  <div class="divide-y divide-gray-100 md:hidden dark:divide-dark-700" :class="loading ? 'opacity-60' : ''" data-testid="settlement-cards">
    <div v-for="row in items" :key="row.owner_user_id" class="space-y-3 px-4 py-3" :data-testid="`settlement-card-${row.owner_user_id}`">
      <div class="min-w-0">
        <p class="break-all font-medium text-gray-900 dark:text-white">{{ row.email }}</p>
        <p v-if="row.username" class="text-xs text-gray-500 dark:text-gray-400">{{ row.username }}</p>
      </div>
      <dl class="grid grid-cols-2 gap-3">
        <div v-for="col in keyColumns" :key="col.key">
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ col.label }}</dt>
          <dd class="text-base" :class="cellClass(col, row)">{{ col.format(row[col.key]) }}</dd>
        </div>
      </dl>
      <details class="text-sm">
        <summary class="cursor-pointer text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.settlement.details') }}</summary>
        <dl class="mt-2 space-y-1">
          <div v-for="col in columns" :key="col.key" class="flex justify-between gap-3">
            <dt class="text-gray-500 dark:text-gray-400">{{ col.label }}</dt>
            <dd class="whitespace-nowrap" :class="cellClass(col, row)">{{ col.format(row[col.key]) }}</dd>
          </div>
        </dl>
      </details>
      <button
        v-if="payoutAction"
        type="button"
        class="btn btn-secondary btn-sm w-full"
        :data-testid="`settlement-card-payout-${row.owner_user_id}`"
        @click="emit('record-payout', row)"
      >
        {{ t('admin.finance.settlement.recordPayout') }}
      </button>
    </div>
    <p v-if="!items.length" class="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.finance.settlement.empty') }}
    </p>
    <div v-if="totals && items.length" class="bg-gray-50 px-4 py-3 dark:bg-dark-800" data-testid="settlement-card-totals">
      <p class="mb-2 font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.settlement.totals') }}</p>
      <dl class="grid grid-cols-2 gap-3">
        <div v-for="col in keyColumns" :key="col.key">
          <dt class="text-xs text-gray-500 dark:text-gray-400">{{ col.label }}</dt>
          <dd class="font-semibold text-gray-900 dark:text-white">{{ col.format(totals[col.key]) }}</dd>
        </div>
      </dl>
    </div>
  </div>

  <div class="hidden overflow-x-auto md:block" :class="loading ? 'opacity-60' : ''">
    <table class="w-full min-w-[1280px] text-sm">
      <thead class="bg-gray-50 text-left text-xs font-medium text-gray-500 dark:bg-dark-800 dark:text-gray-400">
        <tr>
          <th class="px-3 py-2">{{ t('admin.finance.settlement.owner') }}</th>
          <th
            v-for="col in columns"
            :key="col.key"
            class="px-3 py-2 text-right align-bottom"
            :title="col.hint"
            :data-testid="`settlement-col-${col.key}`"
          >
            {{ col.label }}
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
            {{ col.format(row[col.key]) }}
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
            class="whitespace-nowrap px-3 py-2 text-right text-gray-900 dark:text-white"
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
// account cost of paid use × the group's owner payout rate at request time.
// `gift_account_cost` (internal-use cost, same payout basis) is never added to
// anything payable.
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

type ColumnKey = Exclude<keyof OwnerSettlementTotals, 'pending_requests' | 'pending_account_cost' | 'account_cost'>

interface Column {
  key: ColumnKey
  label: string
  hint?: string
  format: (v: number) => string
}

const count = (v: number) => formatNumber(v ?? 0)
const money = (v: number) => formatPreciseUSD(v)

// Reads left to right: who → usage → billed to users (paid + internal) →
// payout basis − internal-use cost = payable this period → all-time − paid out = unpaid.
// account_cost stays in the API but is not shown: with a group payout rate ≠ 1 it no longer
// adds up with internal-use cost and payable.
const columns = computed<Column[]>(() => [
  { key: 'active_accounts', label: t('admin.finance.settlement.activeAccounts'), hint: t('admin.finance.settlement.hints.activeAccounts'), format: count },
  { key: 'requests', label: t('admin.finance.settlement.requests'), format: count },
  { key: 'official_cost', label: t('admin.finance.settlement.officialCost'), hint: t('admin.finance.settlement.hints.officialCost'), format: money },
  { key: 'user_billed', label: t('admin.finance.settlement.userBilled'), hint: t('admin.finance.settlement.hints.userBilled'), format: money },
  { key: 'paid_cost', label: t('admin.finance.settlement.paidCost'), hint: t('admin.finance.settlement.hints.paidCost'), format: money },
  { key: 'gift_cost', label: t('admin.finance.settlement.giftCost'), hint: t('admin.finance.settlement.hints.giftCost'), format: money },
  { key: 'payout_base', label: t('admin.finance.settlement.payoutBase'), hint: t('admin.finance.settlement.hints.payoutBase'), format: money },
  {
    key: 'gift_account_cost',
    label: t('admin.finance.settlement.giftAccountCost'),
    hint: t('admin.finance.settlement.hints.giftAccountCost'),
    format: money
  },
  { key: 'earnings', label: t('admin.finance.settlement.earnings'), hint: t('admin.finance.settlement.hints.earnings'), format: money },
  { key: 'earnings_all_time', label: t('admin.finance.settlement.earningsAllTime'), format: money },
  { key: 'paid_total', label: t('admin.finance.settlement.paidTotal'), format: money },
  { key: 'unpaid', label: t('admin.finance.settlement.unpaid'), hint: t('admin.finance.settlement.hints.unpaid'), format: money }
])

// What a phone card shows without expanding: owed this period and still unpaid.
const keyColumns = computed(() => columns.value.filter((col) => col.key === 'earnings' || col.key === 'unpaid'))

function cellClass(col: Column, row: OwnerSettlementRow): string {
  if (col.key === 'unpaid' && row.unpaid > 0) return 'font-semibold text-amber-600 dark:text-amber-400'
  if (col.key === 'earnings') return 'font-medium text-gray-900 dark:text-white'
  return 'text-gray-900 dark:text-gray-100'
}
</script>
