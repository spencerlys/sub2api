<template>
  <BaseDialog :show="show" :title="t('admin.finance.payout.title')" width="narrow" @close="$emit('close')">
    <form v-if="owner" id="owner-payout-form" class="space-y-4" data-testid="payout-form" @submit.prevent="submit">
      <div class="rounded-xl bg-gray-50 p-3 text-sm dark:bg-dark-700" data-testid="payout-dialog">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.finance.payout.owner') }}</p>
        <p class="font-medium text-gray-900 dark:text-white">{{ owner.email }}</p>
      </div>
      <div>
        <label class="input-label" for="owner-payout-amount">{{ t('admin.finance.payout.amount') }}</label>
        <input
          id="owner-payout-amount"
          v-model.number="form.amount"
          type="number"
          step="any"
          min="0"
          class="input"
          data-testid="payout-amount"
        />
      </div>
      <div>
        <label class="input-label" for="owner-payout-paid-at">{{ t('admin.finance.payout.paidAt') }}</label>
        <input
          id="owner-payout-paid-at"
          v-model="form.paidOn"
          type="date"
          class="input"
          :max="today"
          data-testid="payout-paid-at"
        />
      </div>
      <div>
        <label class="input-label" for="owner-payout-notes">{{ t('admin.finance.payout.notes') }}</label>
        <textarea id="owner-payout-notes" v-model="form.notes" rows="2" class="input" data-testid="payout-notes"></textarea>
      </div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="$emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="owner-payout-form" class="btn btn-primary" :disabled="submitting" data-testid="payout-submit">
          {{ submitting ? t('common.saving') : t('common.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { CreateOwnerPayoutRequest } from '@/api/admin/finance'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { toLocalDateString } from './financePeriod'

const props = defineProps<{ show: boolean; owner: { owner_user_id: number; email: string } | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

const { t } = useI18n()
const appStore = useAppStore()

const today = ref(toLocalDateString(new Date()))
const submitting = ref(false)
const form = reactive({ amount: 0, paidOn: today.value, notes: '' })

watch(
  () => props.show,
  (open) => {
    if (!open) return
    today.value = toLocalDateString(new Date())
    form.amount = 0
    form.paidOn = today.value
    form.notes = ''
  },
  { immediate: true }
)

// A payout dated today is recorded at "now" (backend default); an earlier date
// is pinned to local noon so it stays inside that day in the browser timezone.
function paidAtFor(day: string): string | undefined {
  if (!day || day === toLocalDateString(new Date())) return undefined
  const [year, month, date] = day.split('-').map(Number)
  return new Date(year, month - 1, date, 12).toISOString()
}

async function submit() {
  if (!props.owner || submitting.value) return
  if (!form.amount || form.amount <= 0) {
    appStore.showError(t('admin.finance.payout.amountRequired'))
    return
  }
  const body: CreateOwnerPayoutRequest = {
    owner_user_id: props.owner.owner_user_id,
    amount: form.amount,
    notes: form.notes.trim()
  }
  const paidAt = paidAtFor(form.paidOn)
  if (paidAt) body.paid_at = paidAt
  submitting.value = true
  try {
    await adminAPI.finance.createOwnerPayout(body)
    appStore.showSuccess(t('admin.finance.payout.success'))
    emit('saved')
    emit('close')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.finance.payout.failed')))
  } finally {
    submitting.value = false
  }
}
</script>
