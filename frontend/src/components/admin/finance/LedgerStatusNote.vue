<template>
  <div v-if="notes.length" class="space-y-0.5 text-xs text-amber-700 dark:text-amber-400" data-testid="ledger-status-note">
    <p v-for="note in notes" :key="note.key" :data-testid="`ledger-${note.key}-note`">{{ note.text }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { LedgerStatus } from '@/api/admin/finance'
import { formatNumber } from '@/utils/format'
import { ledgerIsLagging, ledgerLagText } from './financePeriod'

// Short caveats under settlement figures: requests the ledger has not split
// yet are excluded from earnings, and a lagging ledger means recent usage is
// missing. Safe for owners (no amounts, no errors).
const props = defineProps<{ ledger: LedgerStatus | null | undefined; pendingRequests?: number }>()
const { t } = useI18n()

const notes = computed(() => {
  const out: { key: string; text: string }[] = []
  const pending = props.pendingRequests ?? 0
  if (pending > 0) out.push({ key: 'pending', text: t('admin.finance.ledger.pendingNote', { count: formatNumber(pending) }) })
  const ledger = props.ledger
  if (ledger && ledgerIsLagging(ledger)) {
    out.push({
      key: 'lag',
      text: ledger.lag_seconds < 0 ? t('admin.finance.ledger.neverRan') : t('admin.finance.ledger.lagNote', { lag: ledgerLagText(t, ledger.lag_seconds) })
    })
  }
  if (ledger?.rebuild_pending) out.push({ key: 'rebuild', text: t('admin.finance.ledger.rebuildPending') })
  return out
})
</script>
