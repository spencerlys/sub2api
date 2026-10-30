<template>
  <div class="card p-4" data-testid="finance-ledger">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.ledger.title') }}</h3>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.ledger.description') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <span
          v-if="ledger?.rebuild_pending"
          class="badge badge-warning"
          data-testid="finance-ledger-rebuild-pending"
        >
          {{ t('admin.finance.ledger.rebuildPending') }}
        </span>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="rebuilding || !!ledger?.rebuild_pending"
          data-testid="finance-rebuild-ledger"
          @click="emit('rebuild')"
        >
          {{ t('admin.finance.ledger.rebuild') }}
        </button>
      </div>
    </div>
    <dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2 lg:grid-cols-4">
      <div>
        <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.ledger.processedUntil') }}</dt>
        <dd class="text-gray-900 dark:text-gray-100" data-testid="finance-ledger-processed-until">{{ dateOrDash(ledger?.processed_until) }}</dd>
      </div>
      <div>
        <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.ledger.lag') }}</dt>
        <dd :class="lagging ? 'font-medium text-amber-600 dark:text-amber-400' : 'text-gray-900 dark:text-gray-100'" data-testid="finance-ledger-lag">
          {{ lagText }}
        </dd>
      </div>
      <div>
        <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.ledger.lastRebuild') }}</dt>
        <dd class="text-gray-900 dark:text-gray-100">{{ dateOrDash(ledger?.last_rebuild_at) }}</dd>
      </div>
      <div>
        <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.ledger.lastError') }}</dt>
        <dd
          class="break-words"
          :class="ledger?.last_error ? 'text-red-600 dark:text-red-400' : 'text-gray-500 dark:text-gray-400'"
          data-testid="finance-ledger-last-error"
        >
          {{ ledger?.last_error || t('admin.finance.ledger.noError') }}
        </dd>
      </div>
    </dl>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { LedgerStatus } from '@/api/admin/finance'
import { formatDateTime } from '@/utils/format'
import { ledgerIsLagging, ledgerLagText } from './financePeriod'

// Admin-only view of the background ledger that splits each request into its
// paid / gift-funded parts, with a full-rebuild trigger.
const props = defineProps<{ ledger: LedgerStatus | null | undefined; rebuilding?: boolean }>()
const emit = defineEmits<{ (e: 'rebuild'): void }>()
const { t } = useI18n()

const lagging = computed(() => ledgerIsLagging(props.ledger))

const lagText = computed(() => {
  const ledger = props.ledger
  if (!ledger) return '-'
  if (ledger.lag_seconds < 0) return t('admin.finance.ledger.neverRanShort')
  return ledgerLagText(t, ledger.lag_seconds)
})

function dateOrDash(value: string | null | undefined): string {
  return value ? formatDateTime(value) : '-'
}
</script>
