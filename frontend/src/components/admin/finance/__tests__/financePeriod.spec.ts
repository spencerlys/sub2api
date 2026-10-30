import { describe, expect, it } from 'vitest'

import {
  FINANCE_RANGE_PRESETS,
  LEDGER_LAG_WARN_SECONDS,
  defaultFinanceRange,
  isValidCustomRange,
  ledgerLag,
  ledgerIsLagging,
  ledgerWarning,
  toLocalDateString
} from '../financePeriod'

const ledger = (lag: number) => ({
  processed_until: null,
  last_run_at: null,
  lag_seconds: lag,
  last_error: '',
  rebuild_pending: false,
  last_rebuild_at: null
})

describe('financePeriod', () => {
  it('formats local dates as YYYY-MM-DD', () => {
    expect(toLocalDateString(new Date(2026, 0, 5, 23, 59))).toBe('2026-01-05')
  })

  it('offers every server preset plus custom, defaulting to this month', () => {
    expect(FINANCE_RANGE_PRESETS).toEqual([
      'today',
      'yesterday',
      'this_month',
      'last_month',
      'last_7d',
      'last_30d',
      'all',
      'custom'
    ])
    expect(defaultFinanceRange()).toEqual({ range: 'this_month' })
  })

  it('validates custom ranges (ordered and well-formed, no length cap)', () => {
    expect(isValidCustomRange('2026-10-01', '2026-10-01')).toBe(true)
    expect(isValidCustomRange('2020-01-01', '2026-12-31')).toBe(true)
    expect(isValidCustomRange('2026-10-02', '2026-10-01')).toBe(false)
    expect(isValidCustomRange('', '2026-10-01')).toBe(false)
    expect(isValidCustomRange('2026-13-01', '2026-10-01')).toBe(false)
    expect(isValidCustomRange('2026-02-30', '2026-03-01')).toBe(false)
  })

  it('flags a lagging or never-run ledger', () => {
    expect(LEDGER_LAG_WARN_SECONDS).toBe(600)
    expect(ledgerIsLagging(null)).toBe(false)
    expect(ledgerIsLagging(ledger(30))).toBe(false)
    expect(ledgerIsLagging(ledger(600))).toBe(false)
    expect(ledgerIsLagging(ledger(601))).toBe(true)
    expect(ledgerIsLagging(ledger(-1))).toBe(true)
  })

  it('warns only when the ledger is abnormal, never for routine lag', () => {
    expect(ledgerWarning(null)).toBeNull()
    expect(ledgerWarning(ledger(-1))).toBeNull()
    expect(ledgerWarning(ledger(180))).toBeNull()
    expect(ledgerWarning(ledger(600))).toBeNull()
    expect(ledgerWarning(ledger(601))).toEqual({ kind: 'lag', minutes: 10 })
    expect(ledgerWarning(ledger(1500))).toEqual({ kind: 'lag', minutes: 25 })
    expect(ledgerWarning({ ...ledger(30), rebuild_pending: true })).toEqual({ kind: 'rebuild' })
    expect(ledgerWarning({ ...ledger(30), last_error: 'boom' })).toEqual({ kind: 'error' })
    // An error wins over a rebuild, which wins over lag.
    expect(ledgerWarning({ ...ledger(9000), rebuild_pending: true, last_error: 'boom' })).toEqual({ kind: 'error' })
    expect(ledgerWarning({ ...ledger(9000), rebuild_pending: true })).toEqual({ kind: 'rebuild' })
  })

  it('expresses lag in the largest whole unit', () => {
    expect(ledgerLag(45)).toEqual({ unit: 'seconds', n: 45 })
    expect(ledgerLag(600)).toEqual({ unit: 'minutes', n: 10 })
    expect(ledgerLag(7300)).toEqual({ unit: 'hours', n: 2 })
    expect(ledgerLag(3 * 86400 + 5)).toEqual({ unit: 'days', n: 3 })
  })
})
