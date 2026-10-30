<template>
  <div class="card p-4" data-testid="owner-earnings-card">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
      <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.ownerEarnings.title') }}</h3>
        <FinanceWindowText :window="data" />
      </div>
      <FinancePeriodPicker v-model="range" />
    </div>
    <div class="grid grid-cols-2 gap-3 lg:grid-cols-5" :class="loading ? 'opacity-60' : ''">
      <div
        v-for="stat in stats"
        :key="stat.key"
        class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700"
        :data-testid="`owner-earnings-${stat.key}`"
      >
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ stat.label }}</p>
        <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ stat.value }}</p>
      </div>
    </div>
    <LedgerStatusNote class="mt-2" :ledger="data?.ledger" :pending-requests="data?.pending_requests ?? 0" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { FinanceRange, MyOwnerSettlement } from '@/api/admin/finance'
import { useAppStore } from '@/stores/app'
import { formatNumber } from '@/utils/format'
import { formatPreciseUSD } from '@/utils/preciseMoney'
import FinancePeriodPicker from './FinancePeriodPicker.vue'
import FinanceWindowText from './FinanceWindowText.vue'
import LedgerStatusNote from './LedgerStatusNote.vue'
import { defaultFinanceRange } from './financePeriod'

// Owner-only summary. Shows official-price cost and the owner's own earnings /
// payouts; never user identities, billed amounts or billing multipliers.
const { t } = useI18n()
const appStore = useAppStore()

const range = ref<FinanceRange>(defaultFinanceRange())
const data = ref<MyOwnerSettlement | null>(null)
const loading = ref(false)
let requestSeq = 0

const stats = computed(() => {
  const d = data.value
  return [
    { key: 'requests', label: t('admin.finance.ownerEarnings.requests'), value: formatNumber(d?.requests ?? 0) },
    { key: 'official_cost', label: t('admin.finance.ownerEarnings.officialCost'), value: formatPreciseUSD(d?.official_cost) },
    { key: 'earnings', label: t('admin.finance.ownerEarnings.earnings'), value: formatPreciseUSD(d?.earnings) },
    { key: 'paid_total', label: t('admin.finance.ownerEarnings.paidTotal'), value: formatPreciseUSD(d?.paid_total) },
    { key: 'unpaid', label: t('admin.finance.ownerEarnings.unpaid'), value: formatPreciseUSD(d?.unpaid) }
  ]
})

async function load() {
  const seq = ++requestSeq
  loading.value = true
  try {
    const res = await adminAPI.finance.getMyOwnerSettlement(range.value)
    if (seq !== requestSeq) return
    data.value = res
  } catch (err: any) {
    if (seq !== requestSeq) return
    appStore.showError(err?.message || t('admin.finance.ownerEarnings.loadFailed'))
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

watch(range, load)
onMounted(load)
</script>
