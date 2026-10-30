<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center gap-3">
        <FinancePeriodPicker v-model="range" />
        <span v-if="windowText" class="text-xs text-gray-500 dark:text-gray-400">{{ windowText }}</span>
        <div class="flex flex-1 justify-end">
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="anyLoading"
            :title="t('common.refresh')"
            data-testid="finance-refresh"
            @click="loadAll"
          >
            <Icon name="refresh" size="md" :class="anyLoading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <section>
        <h2 class="mb-3 text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.moneyIn.title') }}</h2>
        <FinanceMoneyIn :summary="summary" :loading="summaryLoading" />
      </section>

      <section class="card overflow-hidden">
        <h2 class="border-b border-gray-100 px-4 py-3 text-base font-semibold text-gray-900 dark:border-dark-700 dark:text-white">
          {{ t('admin.finance.settlement.title') }}
        </h2>
        <div class="overflow-x-auto" :class="settlementLoading ? 'opacity-60' : ''">
          <table class="w-full min-w-[960px] text-sm">
            <thead class="bg-gray-50 text-left text-xs font-medium text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-2">{{ t('admin.finance.settlement.owner') }}</th>
                <th v-for="col in moneyColumns" :key="col.key" class="px-4 py-2 text-right">{{ col.label }}</th>
                <th class="px-4 py-2 text-right">{{ t('admin.finance.settlement.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="row in settlementRows" :key="row.owner_user_id" :data-testid="`settlement-row-${row.owner_user_id}`">
                <td class="px-4 py-2">
                  <p class="font-medium text-gray-900 dark:text-white">{{ row.email }}</p>
                  <p v-if="row.username" class="text-xs text-gray-500 dark:text-gray-400">{{ row.username }}</p>
                </td>
                <td
                  v-for="col in moneyColumns"
                  :key="col.key"
                  class="whitespace-nowrap px-4 py-2 text-right text-gray-900 dark:text-gray-100"
                  :class="col.key === 'unpaid' && row.unpaid > 0 ? 'font-semibold text-amber-600 dark:text-amber-400' : ''"
                >
                  {{ col.format(row[col.key]) }}
                </td>
                <td class="px-4 py-2 text-right">
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm whitespace-nowrap"
                    :data-testid="`settlement-payout-${row.owner_user_id}`"
                    @click="openPayout(row)"
                  >
                    {{ t('admin.finance.settlement.recordPayout') }}
                  </button>
                </td>
              </tr>
              <tr v-if="!settlementRows.length">
                <td :colspan="moneyColumns.length + 2" class="px-4 py-6 text-center text-gray-500 dark:text-gray-400">
                  {{ t('admin.finance.settlement.empty') }}
                </td>
              </tr>
            </tbody>
            <tfoot v-if="settlementTotals && settlementRows.length" class="bg-gray-50 font-semibold dark:bg-dark-800">
              <tr data-testid="settlement-totals">
                <td class="px-4 py-2 text-gray-900 dark:text-white">{{ t('admin.finance.settlement.totals') }}</td>
                <td v-for="col in moneyColumns" :key="col.key" class="whitespace-nowrap px-4 py-2 text-right text-gray-900 dark:text-white">
                  {{ col.format(settlementTotals[col.key]) }}
                </td>
                <td></td>
              </tr>
            </tfoot>
          </table>
        </div>
      </section>

      <section class="card overflow-hidden" data-testid="payout-history">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.payout.historyTitle') }}</h2>
          <OwnerSelect v-model="payoutOwnerId" />
        </div>
        <div class="overflow-x-auto" :class="payoutsLoading ? 'opacity-60' : ''">
          <table class="w-full min-w-[720px] text-sm">
            <thead class="bg-gray-50 text-left text-xs font-medium text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-2">{{ t('admin.finance.payout.columns.paidAt') }}</th>
                <th class="px-4 py-2">{{ t('admin.finance.payout.columns.owner') }}</th>
                <th class="px-4 py-2 text-right">{{ t('admin.finance.payout.columns.amount') }}</th>
                <th class="px-4 py-2">{{ t('admin.finance.payout.columns.notes') }}</th>
                <th class="px-4 py-2">{{ t('admin.finance.payout.columns.createdAt') }}</th>
                <th class="px-4 py-2 text-right">{{ t('admin.finance.payout.columns.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="payout in payouts" :key="payout.id" :data-testid="`payout-row-${payout.id}`">
                <td class="whitespace-nowrap px-4 py-2 text-gray-900 dark:text-gray-100">{{ formatDateTime(payout.paid_at) }}</td>
                <td class="px-4 py-2 text-gray-900 dark:text-gray-100">{{ payout.owner_email }}</td>
                <td class="whitespace-nowrap px-4 py-2 text-right font-medium text-gray-900 dark:text-white">{{ formatCurrency(payout.amount) }}</td>
                <td class="max-w-xs truncate px-4 py-2 text-gray-600 dark:text-gray-300" :title="payout.notes">{{ payout.notes || '-' }}</td>
                <td class="whitespace-nowrap px-4 py-2 text-gray-500 dark:text-gray-400">{{ formatDateTime(payout.created_at) }}</td>
                <td class="px-4 py-2 text-right">
                  <button
                    type="button"
                    class="text-sm text-red-600 hover:text-red-700 dark:text-red-400"
                    :data-testid="`payout-delete-${payout.id}`"
                    @click="pendingDelete = payout"
                  >
                    {{ t('admin.finance.payout.delete') }}
                  </button>
                </td>
              </tr>
              <tr v-if="!payouts.length">
                <td colspan="6" class="px-4 py-6 text-center text-gray-500 dark:text-gray-400">{{ t('admin.finance.payout.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="payoutPages > 1" class="flex items-center justify-center gap-2 border-t border-gray-100 py-3 dark:border-dark-700">
          <button class="btn btn-secondary px-3 py-1 text-sm" :disabled="payoutPage <= 1" @click="loadPayouts(payoutPage - 1)">
            {{ t('pagination.previous') }}
          </button>
          <span class="text-sm text-gray-500 dark:text-dark-400">{{ payoutPage }} / {{ payoutPages }}</span>
          <button class="btn btn-secondary px-3 py-1 text-sm" :disabled="payoutPage >= payoutPages" @click="loadPayouts(payoutPage + 1)">
            {{ t('pagination.next') }}
          </button>
        </div>
      </section>
    </div>

    <OwnerPayoutDialog :show="payoutOwner !== null" :owner="payoutOwner" @close="payoutOwner = null" @saved="afterPayoutChange" />
    <ConfirmDialog
      :show="pendingDelete !== null"
      :title="t('admin.finance.payout.deleteTitle')"
      :message="deleteMessage"
      :confirm-text="t('admin.finance.payout.delete')"
      danger
      @confirm="confirmDelete"
      @cancel="pendingDelete = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  FinanceDateRange,
  FinanceSummary,
  OwnerPayout,
  OwnerSettlementRow,
  OwnerSettlementTotals
} from '@/api/admin/finance'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCurrency, formatDateTime, formatNumber } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import FinanceMoneyIn from '@/components/admin/finance/FinanceMoneyIn.vue'
import FinancePeriodPicker from '@/components/admin/finance/FinancePeriodPicker.vue'
import OwnerPayoutDialog from '@/components/admin/finance/OwnerPayoutDialog.vue'
import OwnerSelect from '@/components/admin/finance/OwnerSelect.vue'
import { thisMonthRange } from '@/components/admin/finance/financePeriod'

const { t } = useI18n()
const appStore = useAppStore()

const PAYOUT_PAGE_SIZE = 20

const range = ref<FinanceDateRange>(thisMonthRange())
const summary = ref<FinanceSummary | null>(null)
const summaryLoading = ref(false)
const settlementRows = ref<OwnerSettlementRow[]>([])
const settlementTotals = ref<OwnerSettlementTotals | null>(null)
const settlementLoading = ref(false)
const payouts = ref<OwnerPayout[]>([])
const payoutsLoading = ref(false)
const payoutPage = ref(1)
const payoutPages = ref(1)
const payoutOwnerId = ref<number | null>(null)
const payoutOwner = ref<OwnerSettlementRow | null>(null)
const pendingDelete = ref<OwnerPayout | null>(null)
const windowStart = ref('')
const windowEnd = ref('')

// Each section ignores responses from requests superseded by a newer one.
const seq = { summary: 0, settlement: 0, payouts: 0 }

type MoneyKey = keyof OwnerSettlementTotals

const moneyColumns = computed<{ key: MoneyKey; label: string; format: (v: number) => string }[]>(() => [
  { key: 'requests', label: t('admin.finance.settlement.requests'), format: (v) => formatNumber(v ?? 0) },
  { key: 'official_cost', label: t('admin.finance.settlement.officialCost'), format: (v) => formatCurrency(v ?? 0) },
  { key: 'user_billed', label: t('admin.finance.settlement.userBilled'), format: (v) => formatCurrency(v ?? 0) },
  { key: 'account_cost', label: t('admin.finance.settlement.accountCost'), format: (v) => formatCurrency(v ?? 0) },
  { key: 'earnings', label: t('admin.finance.settlement.earnings'), format: (v) => formatCurrency(v ?? 0) },
  { key: 'earnings_all_time', label: t('admin.finance.settlement.earningsAllTime'), format: (v) => formatCurrency(v ?? 0) },
  { key: 'paid_total', label: t('admin.finance.settlement.paidTotal'), format: (v) => formatCurrency(v ?? 0) },
  { key: 'unpaid', label: t('admin.finance.settlement.unpaid'), format: (v) => formatCurrency(v ?? 0) }
])

const anyLoading = computed(() => summaryLoading.value || settlementLoading.value || payoutsLoading.value)

const windowText = computed(() => {
  if (!windowStart.value || !windowEnd.value) return ''
  return `${formatDateTime(windowStart.value)} - ${formatDateTime(windowEnd.value)}`
})

const deleteMessage = computed(() => {
  const payout = pendingDelete.value
  if (!payout) return ''
  return t('admin.finance.payout.deleteConfirm', { amount: formatCurrency(payout.amount), owner: payout.owner_email })
})

function reportError(err: any) {
  appStore.showError(err?.message || t('admin.finance.loadFailed'))
}

async function loadSummary() {
  const id = ++seq.summary
  summaryLoading.value = true
  try {
    const res = await adminAPI.finance.getFinanceSummary(range.value)
    if (id !== seq.summary) return
    summary.value = res
    windowStart.value = res?.start_time ?? ''
    windowEnd.value = res?.end_time ?? ''
  } catch (err) {
    if (id === seq.summary) reportError(err)
  } finally {
    if (id === seq.summary) summaryLoading.value = false
  }
}

async function loadSettlements() {
  const id = ++seq.settlement
  settlementLoading.value = true
  try {
    const res = await adminAPI.finance.getOwnerSettlements(range.value)
    if (id !== seq.settlement) return
    settlementRows.value = res?.items ?? []
    settlementTotals.value = res?.totals ?? null
  } catch (err) {
    if (id === seq.settlement) reportError(err)
  } finally {
    if (id === seq.settlement) settlementLoading.value = false
  }
}

async function loadPayouts(page = 1) {
  const id = ++seq.payouts
  payoutsLoading.value = true
  try {
    const params = { ...range.value, page, pageSize: PAYOUT_PAGE_SIZE }
    const res = await adminAPI.finance.listOwnerPayouts(
      payoutOwnerId.value === null ? params : { ...params, ownerUserId: payoutOwnerId.value }
    )
    if (id !== seq.payouts) return
    payouts.value = res?.items ?? []
    payoutPage.value = res?.page || page
    payoutPages.value = res?.pages || 1
  } catch (err) {
    if (id === seq.payouts) reportError(err)
  } finally {
    if (id === seq.payouts) payoutsLoading.value = false
  }
}

function loadAll() {
  loadSummary()
  loadSettlements()
  loadPayouts(1)
}

function openPayout(row: OwnerSettlementRow) {
  payoutOwner.value = row
}

// Paid totals and unpaid change with every payout, so refresh both tables.
function afterPayoutChange() {
  loadSettlements()
  loadPayouts(1)
}

async function confirmDelete() {
  const payout = pendingDelete.value
  if (!payout) return
  pendingDelete.value = null
  try {
    await adminAPI.finance.deleteOwnerPayout(payout.id)
    appStore.showSuccess(t('admin.finance.payout.deleted'))
    afterPayoutChange()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.finance.payout.deleteFailed')))
  }
}

watch(range, loadAll)
watch(payoutOwnerId, () => loadPayouts(1))

onMounted(loadAll)
</script>
