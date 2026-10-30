<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center gap-3">
        <FinancePeriodPicker v-model="range" />
        <FinanceWindowText :window="summary" />
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

      <section>
        <h2 class="mb-3 text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.consumption.title') }}</h2>
        <FinanceConsumption :consumption="summary?.consumption" :loading="summaryLoading" />
      </section>

      <FinanceLedgerStatus :ledger="summary?.ledger" :rebuilding="rebuilding" @rebuild="rebuildConfirmOpen = true" />

      <section class="card overflow-hidden">
        <h2 class="border-b border-gray-100 px-4 py-3 text-base font-semibold text-gray-900 dark:border-dark-700 dark:text-white">
          {{ t('admin.finance.settlement.title') }}
        </h2>
        <LedgerStatusNote class="px-4 pt-2" :ledger="summary?.ledger" />
        <OwnerSettlementTable
          :items="settlementRows"
          :totals="settlementTotals"
          :loading="settlementLoading"
          payout-action
          @record-payout="openPayout"
        />
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
    <ConfirmDialog
      :show="rebuildConfirmOpen"
      :title="t('admin.finance.ledger.rebuildTitle')"
      :message="t('admin.finance.ledger.rebuildConfirm')"
      :confirm-text="t('admin.finance.ledger.rebuild')"
      @confirm="confirmRebuild"
      @cancel="rebuildConfirmOpen = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  FinanceRange,
  FinanceSummary,
  LedgerStatus,
  OwnerPayout,
  OwnerSettlementRow,
  OwnerSettlementTotals
} from '@/api/admin/finance'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCurrency, formatDateTime } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import FinanceConsumption from '@/components/admin/finance/FinanceConsumption.vue'
import FinanceLedgerStatus from '@/components/admin/finance/FinanceLedgerStatus.vue'
import FinanceMoneyIn from '@/components/admin/finance/FinanceMoneyIn.vue'
import FinancePeriodPicker from '@/components/admin/finance/FinancePeriodPicker.vue'
import FinanceWindowText from '@/components/admin/finance/FinanceWindowText.vue'
import LedgerStatusNote from '@/components/admin/finance/LedgerStatusNote.vue'
import OwnerPayoutDialog from '@/components/admin/finance/OwnerPayoutDialog.vue'
import OwnerSelect from '@/components/admin/finance/OwnerSelect.vue'
import OwnerSettlementTable from '@/components/admin/finance/OwnerSettlementTable.vue'
import { defaultFinanceRange } from '@/components/admin/finance/financePeriod'

const { t } = useI18n()
const appStore = useAppStore()

const PAYOUT_PAGE_SIZE = 20
// While a ledger rebuild is pending, re-check its status (the job runs within ~1 min).
const LEDGER_POLL_MS = 15000
const LEDGER_POLL_MAX = 20

const range = ref<FinanceRange>(defaultFinanceRange())
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
const rebuildConfirmOpen = ref(false)
const rebuilding = ref(false)
let ledgerPollTimer: ReturnType<typeof setTimeout> | null = null
let ledgerPollsLeft = 0
let ledgerPolling = false

// Each section ignores responses from requests superseded by a newer one.
const seq = { summary: 0, settlement: 0, payouts: 0 }

const anyLoading = computed(() => summaryLoading.value || settlementLoading.value || payoutsLoading.value)

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
    syncLedgerPoll(res?.ledger)
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
    const params = { range: range.value, page, pageSize: PAYOUT_PAGE_SIZE }
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

function stopLedgerPoll() {
  if (ledgerPollTimer !== null) clearTimeout(ledgerPollTimer)
  ledgerPollTimer = null
}

// Poll the summary while a rebuild is pending; once it clears, the exact
// paid / gift split changed, so the settlement table is refreshed as well.
function syncLedgerPoll(ledger: LedgerStatus | null | undefined) {
  if (ledger?.rebuild_pending) {
    if (!ledgerPolling) {
      ledgerPolling = true
      ledgerPollsLeft = LEDGER_POLL_MAX
    }
    if (ledgerPollTimer === null && ledgerPollsLeft > 0) {
      ledgerPollsLeft--
      ledgerPollTimer = setTimeout(() => {
        ledgerPollTimer = null
        loadSummary()
      }, LEDGER_POLL_MS)
    }
    return
  }
  if (ledgerPolling) {
    ledgerPolling = false
    stopLedgerPoll()
    loadSettlements()
  }
}

async function confirmRebuild() {
  rebuildConfirmOpen.value = false
  if (rebuilding.value) return
  rebuilding.value = true
  try {
    const res = await adminAPI.finance.rebuildLedger()
    appStore.showSuccess(t('admin.finance.ledger.rebuildStarted'))
    const current = summary.value
    if (current?.ledger) {
      summary.value = { ...current, ledger: { ...current.ledger, rebuild_pending: res?.rebuild_pending ?? true } }
      syncLedgerPoll(summary.value.ledger)
    }
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.finance.ledger.rebuildFailed')))
  } finally {
    rebuilding.value = false
  }
}

watch(range, loadAll)
watch(payoutOwnerId, () => loadPayouts(1))

onMounted(loadAll)
onBeforeUnmount(stopLedgerPoll)
</script>
