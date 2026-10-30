<template>
  <div class="space-y-3" data-testid="balance-recharge-rate-settings">
    <!-- 自动汇率开关 -->
    <div class="flex items-start justify-between gap-4">
      <div class="min-w-0">
        <label class="input-label mb-0">{{ t('admin.settings.payment.rechargeRate.autoLabel') }}</label>
        <p class="mt-0.5 text-xs text-gray-400">{{ t('admin.settings.payment.rechargeRate.autoHint') }}</p>
      </div>
      <Toggle
        :model-value="autoEnabled"
        data-testid="recharge-rate-auto-toggle"
        @update:model-value="emit('update:autoEnabled', $event)"
      />
    </div>

    <!-- 加价方式 + 加价值 -->
    <div v-if="autoEnabled" class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <div>
        <label class="input-label">{{ t('admin.settings.payment.rechargeRate.markupMode') }}</label>
        <select
          class="input"
          :value="markupMode"
          data-testid="recharge-rate-markup-mode"
          @change="onModeChange(($event.target as HTMLSelectElement).value)"
        >
          <option value="amount">{{ t('admin.settings.payment.rechargeRate.markupModeAmount') }}</option>
          <option value="percent">{{ t('admin.settings.payment.rechargeRate.markupModePercent') }}</option>
        </select>
      </div>
      <div>
        <label class="input-label">{{ t('admin.settings.payment.rechargeRate.markupValue') }}</label>
        <div class="relative">
          <input
            :value="markupValue ?? ''"
            type="number"
            step="0.01"
            min="0"
            :max="markupMax"
            class="input pr-10"
            data-testid="recharge-rate-markup-value"
            @input="onMarkupInput(($event.target as HTMLInputElement).value)"
          />
          <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-sm text-gray-400">
            {{ markupMode === 'amount' ? '¥' : '%' }}
          </span>
        </div>
        <p class="mt-0.5 text-xs text-gray-400">
          {{ t('admin.settings.payment.rechargeRate.markupMaxHint', { max: markupMax, unit: markupMode === 'amount' ? '¥' : '%' }) }}
        </p>
      </div>
    </div>

    <!-- 手动倍率：自动开启时禁用并显示计算值 -->
    <div>
      <label class="input-label">{{ t('admin.settings.payment.balanceRechargeMultiplier') }}</label>
      <input
        :value="displayedMultiplier || ''"
        type="number"
        step="0.000001"
        min="0.000001"
        class="input"
        :disabled="autoEnabled"
        data-testid="recharge-rate-multiplier-input"
        @input="onMultiplierInput(($event.target as HTMLInputElement).value)"
      />
      <p class="mt-0.5 text-xs text-gray-400">
        {{ autoEnabled
          ? t('admin.settings.payment.rechargeRate.multiplierManagedHint')
          : t('admin.settings.payment.balanceRechargeMultiplierHint') }}
      </p>
      <p class="mt-1 text-xs font-medium text-primary-600 dark:text-primary-400" data-testid="recharge-rate-preview">
        {{ t('admin.settings.payment.rechargeRate.perCny', { usd: formatRechargeMultiplier(displayedMultiplier || 0) }) }}
        <template v-if="displayedMultiplier > 0">
          · {{ t('admin.settings.payment.rechargeRate.perUsd', { cny: formatCnyPerUsd(cnyPerUsd) }) }}
        </template>
      </p>
    </div>

    <!-- 市场汇率只读信息 + 立即更新 -->
    <div
      class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300"
      data-testid="recharge-rate-market-info"
    >
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="space-y-0.5">
          <p v-if="market > 0" data-testid="recharge-rate-market">
            {{ t('admin.settings.payment.rechargeRate.marketRate', { cny: market.toFixed(4) }) }}
            <span v-if="source" class="text-gray-400">({{ source }})</span>
          </p>
          <p v-else data-testid="recharge-rate-market">{{ t('admin.settings.payment.rechargeRate.noMarketRate') }}</p>
          <p v-if="fetchedAt" class="text-gray-400" data-testid="recharge-rate-fetched-at">
            {{ t('admin.settings.payment.rechargeRate.fetchedAt', { time: formatDateTime(fetchedAt) }) }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="refreshing"
          data-testid="recharge-rate-refresh"
          @click="refresh"
        >
          {{ refreshing ? t('admin.settings.payment.rechargeRate.refreshing') : t('admin.settings.payment.rechargeRate.refresh') }}
        </button>
      </div>
      <p v-if="autoEnabled && !(market > 0)" class="mt-1 text-amber-600 dark:text-amber-400">
        {{ t('admin.settings.payment.rechargeRate.waitingForRate') }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import { adminPaymentAPI, type RechargeRateState } from '@/api/admin/payment'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import {
  computeRechargeRate,
  formatCnyPerUsd,
  formatRechargeMultiplier,
  maxRechargeRateMarkup,
  normalizeRechargeRateMarkupMode,
  type RechargeRateMarkupMode,
} from '@/utils/rechargeRate'

const props = withDefaults(defineProps<{
  multiplier: number
  autoEnabled?: boolean
  markupMode?: RechargeRateMarkupMode
  markupValue?: number | null
  /** CNY per 1 USD (read-only, written by the backend job) */
  market?: number
  fetchedAt?: string
  source?: string
}>(), {
  autoEnabled: false,
  markupMode: 'percent',
  markupValue: 0,
  market: 0,
  fetchedAt: '',
  source: '',
})

const emit = defineEmits<{
  'update:multiplier': [value: number]
  'update:autoEnabled': [value: boolean]
  'update:markupMode': [value: RechargeRateMarkupMode]
  'update:markupValue': [value: number]
  refreshed: [state: RechargeRateState]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const refreshing = ref(false)

const markupMax = computed(() => maxRechargeRateMarkup(props.markupMode))

// 自动模式下按当前（可能未保存的）加价实时预览；保存后由后端写入同一值。
const preview = computed(() => computeRechargeRate(props.market, props.markupMode, props.markupValue))

const displayedMultiplier = computed(() => {
  if (props.autoEnabled && preview.value) return preview.value.multiplier
  const m = Number(props.multiplier)
  return Number.isFinite(m) && m > 0 ? m : 0
})

const cnyPerUsd = computed(() => {
  if (props.autoEnabled && preview.value) return preview.value.effective
  return displayedMultiplier.value > 0 ? 1 / displayedMultiplier.value : 0
})

function onModeChange(raw: string) {
  emit('update:markupMode', normalizeRechargeRateMarkupMode(raw))
}

function onMarkupInput(raw: string) {
  const v = parseFloat(raw)
  emit('update:markupValue', Number.isFinite(v) && v >= 0 ? v : 0)
}

function onMultiplierInput(raw: string) {
  const v = parseFloat(raw)
  emit('update:multiplier', Number.isFinite(v) && v > 0 ? v : 1)
}

async function refresh() {
  refreshing.value = true
  try {
    const { data } = await adminPaymentAPI.refreshRechargeRate()
    emit('refreshed', data)
    appStore.showSuccess(t('admin.settings.payment.rechargeRate.refreshSuccess'))
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.settings.payment.rechargeRate.refreshFailed')))
  } finally {
    refreshing.value = false
  }
}
</script>
