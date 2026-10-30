<template>
  <div class="flex flex-wrap items-center gap-2">
    <div class="tabs inline-flex flex-wrap" role="tablist" :aria-label="t('admin.finance.period.label')">
      <button
        v-for="option in FINANCE_RANGE_PRESETS"
        :key="option"
        type="button"
        role="tab"
        class="tab"
        :class="preset === option ? 'tab-active' : ''"
        :aria-selected="preset === option"
        :data-testid="`finance-period-${option}`"
        @click="selectPreset(option)"
      >
        {{ t(`admin.finance.period.presets.${option}`) }}
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
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FinanceRange, FinanceRangeKind } from '@/api/admin/finance'
import { FINANCE_RANGE_PRESETS, isValidCustomRange, toLocalDateString } from './financePeriod'

const props = defineProps<{ modelValue: FinanceRange }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: FinanceRange): void }>()

const { t } = useI18n()

// Custom dates only seed the inputs; presets are resolved by the server.
function seedDates(value: FinanceRange): { start: string; end: string } {
  if (value.range === 'custom' && value.start && value.end) return { start: value.start, end: value.end }
  const now = new Date()
  return { start: toLocalDateString(new Date(now.getFullYear(), now.getMonth(), 1)), end: toLocalDateString(now) }
}

const preset = ref<FinanceRangeKind>(props.modelValue.range)
const seed = seedDates(props.modelValue)
const customStart = ref(seed.start)
const customEnd = ref(seed.end)

const customValid = computed(() => isValidCustomRange(customStart.value, customEnd.value))

watch(
  () => props.modelValue,
  (value) => {
    preset.value = value.range
    if (value.range === 'custom' && value.start && value.end) {
      customStart.value = value.start
      customEnd.value = value.end
    }
  }
)

function selectPreset(next: FinanceRangeKind) {
  preset.value = next
  if (next === 'custom') return
  emit('update:modelValue', { range: next })
}

function applyCustom() {
  if (!customValid.value) return
  emit('update:modelValue', { range: 'custom', start: customStart.value, end: customEnd.value })
}
</script>
