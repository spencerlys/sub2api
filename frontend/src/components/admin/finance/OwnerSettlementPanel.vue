<template>
  <section class="card overflow-hidden" data-testid="owner-settlement-panel">
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-4 py-3 dark:border-dark-700">
      <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.settlement.title') }}</h2>
        <FinanceWindowText :window="data" />
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <FinancePeriodPicker v-model="range" />
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="loading"
          :title="t('common.refresh')"
          data-testid="owner-settlement-refresh"
          @click="load"
        >
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>
    </div>
    <LedgerStatusNote class="px-4 pt-2" :ledger="data?.ledger" :pending-requests="data?.totals?.pending_requests ?? 0" />
    <OwnerSettlementTable :items="data?.items ?? []" :totals="data?.totals ?? null" :loading="loading" />
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { FinanceRange, OwnerSettlementsResponse } from '@/api/admin/finance'
import { useAppStore } from '@/stores/app'
import Icon from '@/components/icons/Icon.vue'
import FinancePeriodPicker from './FinancePeriodPicker.vue'
import FinanceWindowText from './FinanceWindowText.vue'
import LedgerStatusNote from './LedgerStatusNote.vue'
import OwnerSettlementTable from './OwnerSettlementTable.vue'
import { defaultFinanceRange } from './financePeriod'

// Admin-only settlement panel with its own period; the owner filter comes from
// the hosting page so both views stay on the same owner.
const props = defineProps<{ ownerUserId: number | null }>()

const { t } = useI18n()
const appStore = useAppStore()

const range = ref<FinanceRange>(defaultFinanceRange())
const data = ref<OwnerSettlementsResponse | null>(null)
const loading = ref(false)
let requestSeq = 0

async function load() {
  const seq = ++requestSeq
  loading.value = true
  try {
    const res =
      props.ownerUserId === null
        ? await adminAPI.finance.getOwnerSettlements(range.value)
        : await adminAPI.finance.getOwnerSettlements(range.value, props.ownerUserId)
    if (seq !== requestSeq) return
    data.value = res
  } catch (err: any) {
    if (seq !== requestSeq) return
    appStore.showError(err?.message || t('admin.finance.loadFailed'))
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

watch([range, () => props.ownerUserId], load)
onMounted(load)
</script>
