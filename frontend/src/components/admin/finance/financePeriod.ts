/**
 * Period helpers for finance / settlement pages.
 *
 * Presets are resolved by the backend in the site timezone, so the browser
 * never computes preset dates. Only a custom range carries YYYY-MM-DD dates
 * (inclusive end, no length cap).
 */
import type { FinanceRange, FinanceRangeKind, LedgerStatus } from '@/api/admin/finance'

export const FINANCE_RANGE_PRESETS: FinanceRangeKind[] = [
  'today',
  'yesterday',
  'this_month',
  'last_month',
  'last_7d',
  'last_30d',
  'all',
  'custom'
]

/** Ledger lag above this is surfaced as "not settled yet". */
export const LEDGER_LAG_WARN_SECONDS = 300

const DATE_RE = /^(\d{4})-(\d{2})-(\d{2})$/

export function defaultFinanceRange(): FinanceRange {
  return { range: 'this_month' }
}

export function toLocalDateString(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
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

export function isValidCustomRange(start: string, end: string): boolean {
  const from = parseDay(start)
  const to = parseDay(end)
  return from !== null && to !== null && to >= from
}

export function ledgerIsLagging(ledger: LedgerStatus | null | undefined): boolean {
  if (!ledger) return false
  return ledger.lag_seconds < 0 || ledger.lag_seconds > LEDGER_LAG_WARN_SECONDS
}

export type LedgerLagUnit = 'seconds' | 'minutes' | 'hours' | 'days'

export function ledgerLag(seconds: number): { unit: LedgerLagUnit; n: number } {
  const s = Math.max(0, Math.floor(seconds))
  if (s < 60) return { unit: 'seconds', n: s }
  if (s < 3600) return { unit: 'minutes', n: Math.floor(s / 60) }
  if (s < 86400) return { unit: 'hours', n: Math.floor(s / 3600) }
  return { unit: 'days', n: Math.floor(s / 86400) }
}

/** Localised lag, e.g. "12 min". */
export function ledgerLagText(t: (key: string, params?: Record<string, unknown>) => string, seconds: number): string {
  const { unit, n } = ledgerLag(seconds)
  return t(`admin.finance.ledger.lagUnits.${unit}`, { n })
}
