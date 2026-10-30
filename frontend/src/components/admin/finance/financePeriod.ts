/**
 * Period helpers for finance / settlement pages.
 *
 * Ranges are YYYY-MM-DD in the browser's local timezone with an inclusive end,
 * matching the backend contract (which also caps a range at 366 days).
 */
import type { FinanceDateRange } from '@/api/admin/finance'

export type FinancePeriodPreset = 'thisMonth' | 'lastMonth' | 'custom'

export const FINANCE_PERIOD_PRESETS: FinancePeriodPreset[] = ['thisMonth', 'lastMonth', 'custom']

export const FINANCE_MAX_RANGE_DAYS = 366

const DATE_RE = /^(\d{4})-(\d{2})-(\d{2})$/
const DAY_MS = 24 * 60 * 60 * 1000

export function toLocalDateString(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function thisMonthRange(now: Date = new Date()): FinanceDateRange {
  return {
    start: toLocalDateString(new Date(now.getFullYear(), now.getMonth(), 1)),
    end: toLocalDateString(now)
  }
}

export function lastMonthRange(now: Date = new Date()): FinanceDateRange {
  return {
    start: toLocalDateString(new Date(now.getFullYear(), now.getMonth() - 1, 1)),
    end: toLocalDateString(new Date(now.getFullYear(), now.getMonth(), 0))
  }
}

/** Range for a fixed preset; 'custom' falls back to this month. */
export function presetRange(preset: FinancePeriodPreset, now: Date = new Date()): FinanceDateRange {
  return preset === 'lastMonth' ? lastMonthRange(now) : thisMonthRange(now)
}

function parseDay(value: string): number | null {
  const match = DATE_RE.exec(value || '')
  if (!match) return null
  const [year, month, day] = [Number(match[1]), Number(match[2]), Number(match[3])]
  const ts = Date.UTC(year, month - 1, day)
  const check = new Date(ts)
  if (check.getUTCFullYear() !== year || check.getUTCMonth() !== month - 1 || check.getUTCDate() !== day) return null
  return ts
}

export function isValidFinanceRange(range: FinanceDateRange): boolean {
  const start = parseDay(range.start)
  const end = parseDay(range.end)
  if (start === null || end === null || end < start) return false
  return Math.round((end - start) / DAY_MS) + 1 <= FINANCE_MAX_RANGE_DAYS
}
