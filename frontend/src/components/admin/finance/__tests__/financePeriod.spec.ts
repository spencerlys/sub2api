import { describe, expect, it } from 'vitest'

import {
  FINANCE_MAX_RANGE_DAYS,
  isValidFinanceRange,
  lastMonthRange,
  presetRange,
  thisMonthRange,
  toLocalDateString
} from '../financePeriod'

describe('financePeriod', () => {
  it('formats local dates as YYYY-MM-DD', () => {
    expect(toLocalDateString(new Date(2026, 0, 5, 23, 59))).toBe('2026-01-05')
  })

  it('this month runs from the 1st to today (inclusive)', () => {
    expect(thisMonthRange(new Date(2026, 9, 30, 10))).toEqual({ start: '2026-10-01', end: '2026-10-30' })
    expect(thisMonthRange(new Date(2026, 9, 1, 0, 1))).toEqual({ start: '2026-10-01', end: '2026-10-01' })
  })

  it('last month covers the whole previous calendar month', () => {
    expect(lastMonthRange(new Date(2026, 9, 30))).toEqual({ start: '2026-09-01', end: '2026-09-30' })
    expect(lastMonthRange(new Date(2026, 2, 31))).toEqual({ start: '2026-02-01', end: '2026-02-28' })
    expect(lastMonthRange(new Date(2027, 0, 15))).toEqual({ start: '2026-12-01', end: '2026-12-31' })
  })

  it('resolves presets', () => {
    const now = new Date(2026, 9, 30)
    expect(presetRange('thisMonth', now)).toEqual(thisMonthRange(now))
    expect(presetRange('lastMonth', now)).toEqual(lastMonthRange(now))
  })

  it('validates custom ranges (ordered, well-formed, at most the backend maximum)', () => {
    expect(FINANCE_MAX_RANGE_DAYS).toBe(366)
    expect(isValidFinanceRange({ start: '2026-10-01', end: '2026-10-01' })).toBe(true)
    expect(isValidFinanceRange({ start: '2026-01-01', end: '2026-12-31' })).toBe(true)
    expect(isValidFinanceRange({ start: '2025-01-01', end: '2026-01-01' })).toBe(true)
    expect(isValidFinanceRange({ start: '2025-01-01', end: '2026-01-02' })).toBe(false)
    expect(isValidFinanceRange({ start: '2026-10-02', end: '2026-10-01' })).toBe(false)
    expect(isValidFinanceRange({ start: '', end: '2026-10-01' })).toBe(false)
    expect(isValidFinanceRange({ start: '2026-13-01', end: '2026-10-01' })).toBe(false)
  })
})
