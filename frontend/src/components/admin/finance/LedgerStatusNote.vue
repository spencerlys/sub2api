<template>
  <p v-if="warning" class="text-xs text-amber-700 dark:text-amber-400" data-testid="ledger-status-note" :data-kind="warning.kind">{{ text }}</p>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { LedgerStatus } from '@/api/admin/finance'
import { formatNumber } from '@/utils/format'
import { ledgerWarning } from './financePeriod'

// One caveat under settlement figures, shown only when the ledger is abnormal
// (error, rebuild in progress, or more than 10 min behind). Routine pending
// rows are never surfaced. Safe for owners (no amounts, no error text).
const props = defineProps<{ ledger: LedgerStatus | null | undefined }>()
const { t } = useI18n()

const warning = computed(() => ledgerWarning(props.ledger))

const text = computed(() => {
  const w = warning.value
  if (!w) return ''
  if (w.kind === 'lag') return t('admin.finance.ledger.warning.lag', { minutes: formatNumber(w.minutes) })
  if (w.kind === 'rebuild') return t('admin.finance.ledger.warning.rebuild')
  return t('admin.finance.ledger.warning.error')
})
</script>
