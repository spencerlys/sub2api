<template>
  <div class="flex flex-wrap items-center gap-2">
    <div class="tabs inline-flex" role="tablist" :aria-label="t('admin.finance.period.label')">
      <button
        v-for="option in FINANCE_PERIOD_PRESETS"
        :key="option"
        type="button"
        role="tab"
        class="tab"
        :class="preset === option ? 'tab-active' : ''"
        :aria-selected="preset === option"
        :data-testid="`finance-period-${option}`"
        @click="selectPreset(option)"
      >
        {{ t(`admin.finance.period.${option}`) }}
      </button>
    </div>

    <template v-if="preset === 'custom'">
      <input
        v-model="customStart"
        type="date"
        class="input w-40"
        :aria-label="t('admin.finance.period.start')"
        data-testid="finance-period-start"
      />
      <span class="text-gray-400">-</span>
      <input
        v-model="customEnd"
        type="date"
        class="input w-40"
        :aria-label="t('admin.finance.period.end')"
        data-testid="finance-period-end"
      />
      <button
        type="button"
        class="btn btn-secondary"
        :disabled="!customValid"
        data-testid="finance-period-apply"
        @click="applyCustom"
      >
        {{ t('admin.finance.period.apply') }}
      </button>
      <span v-if="!customValid" class="text-xs text-red-600 dark:text-red-400">
        {{ t('admin.finance.period.invalidRange') }}
      </span>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FinanceDateRange } from '@/api/admin/finance'
import {
  FINANCE_PERIOD_PRESETS,
  isValidFinanceRange,
  presetRange,
  type FinancePeriodPreset
} from './financePeriod'

const props = defineProps<{ modelValue: FinanceDateRange }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: FinanceDateRange): void }>()

const { t } = useI18n()

function sameRange(a: FinanceDateRange, b: FinanceDateRange): boolean {
  return a.start === b.start && a.end === b.end
}

function initialPreset(): FinancePeriodPreset {
  if (sameRange(props.modelValue, presetRange('thisMonth'))) return 'thisMonth'
  if (sameRange(props.modelValue, presetRange('lastMonth'))) return 'lastMonth'
  return 'custom'
}

const preset = ref<FinancePeriodPreset>(initialPreset())
const customStart = ref(props.modelValue.start)
const customEnd = ref(props.modelValue.end)

const customValid = computed(() => isValidFinanceRange({ start: customStart.value, end: customEnd.value }))

function selectPreset(next: FinancePeriodPreset) {
  preset.value = next
  if (next === 'custom') return
  const range = presetRange(next)
  customStart.value = range.start
  customEnd.value = range.end
  emit('update:modelValue', range)
}

function applyCustom() {
  if (!customValid.value) return
  emit('update:modelValue', { start: customStart.value, end: customEnd.value })
}
</script>
