<template>
  <span v-if="text" class="text-xs text-gray-500 dark:text-gray-400" data-testid="finance-window">{{ text }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { FinanceWindow } from '@/api/admin/finance'

// The half-open window [start, end) the server actually used, shown in the site
// timezone it was computed in (not the browser's). Whole-day windows read as
// inclusive dates; otherwise minutes are shown.
const props = defineProps<{ window: Partial<FinanceWindow> | null | undefined }>()
const { t } = useI18n()

interface ZonedParts {
  date: string
  time: string
}

function zoned(value: string | Date, timeZone: string): ZonedParts | null {
  const d = value instanceof Date ? value : new Date(value)
  if (isNaN(d.getTime())) return null
  const options: Intl.DateTimeFormatOptions = {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23'
  }
  let parts: Intl.DateTimeFormatPart[]
  try {
    parts = new Intl.DateTimeFormat('en-CA', timeZone ? { ...options, timeZone } : options).formatToParts(d)
  } catch {
    parts = new Intl.DateTimeFormat('en-CA', options).formatToParts(d)
  }
  const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? ''
  return { date: `${get('year')}-${get('month')}-${get('day')}`, time: `${get('hour')}:${get('minute')}` }
}

const text = computed(() => {
  const w = props.window
  if (!w) return ''
  const timezone = w.timezone ?? ''
  const suffix = timezone ? ` (${timezone})` : ''
  if (w.range === 'all') return `${t('admin.finance.period.allTime')}${suffix}`
  if (!w.start_time || !w.end_time) return ''
  const start = zoned(w.start_time, timezone)
  const end = zoned(w.end_time, timezone)
  if (!start || !end) return ''
  if (start.time === '00:00' && end.time === '00:00') {
    const lastDay = zoned(new Date(new Date(w.end_time).getTime() - 1), timezone)
    return `${start.date} - ${lastDay?.date ?? end.date}${suffix}`
  }
  return `${start.date} ${start.time} - ${end.date} ${end.time}${suffix}`
})
</script>
