import { describe, expect, it } from 'vitest'
import {
  computeRechargeRate,
  formatCnyPerUsd,
  formatRechargeMultiplier,
  maxRechargeRateMarkup,
  normalizeRechargeRateMarkupMode,
} from '../rechargeRate'

describe('rechargeRate', () => {
  it('derives the multiplier from market rate plus a percentage markup (mirrors backend)', () => {
    // 7.2 * 1.05 = 7.56 CNY per USD → 1 / 7.56 = 0.132275
    expect(computeRechargeRate(7.2, 'percent', 5)).toEqual({ effective: 7.56, multiplier: 0.132275 })
  })

  it('derives the multiplier from market rate plus a fixed CNY markup', () => {
    expect(computeRechargeRate(7.2, 'amount', 0.3)).toEqual({ effective: 7.5, multiplier: 0.133333 })
  })

  it('returns null without a valid market rate or with an out-of-range markup', () => {
    expect(computeRechargeRate(0, 'percent', 5)).toBeNull()
    expect(computeRechargeRate(undefined, 'percent', 5)).toBeNull()
    expect(computeRechargeRate(4.99, 'percent', 0)).toBeNull()
    expect(computeRechargeRate(10.01, 'percent', 0)).toBeNull()
    expect(computeRechargeRate(7.2, 'percent', -1)).toBeNull()
    expect(computeRechargeRate(7.2, 'percent', 101)).toBeNull()
    expect(computeRechargeRate(7.2, 'amount', 5.01)).toBeNull()
  })

  it('normalizes the markup mode and exposes per-mode caps', () => {
    expect(normalizeRechargeRateMarkupMode('amount')).toBe('amount')
    expect(normalizeRechargeRateMarkupMode('percent')).toBe('percent')
    expect(normalizeRechargeRateMarkupMode('bogus')).toBe('percent')
    expect(maxRechargeRateMarkup('amount')).toBe(5)
    expect(maxRechargeRateMarkup('percent')).toBe(100)
  })

  it('formats multiplier and CNY-per-USD for display', () => {
    expect(formatRechargeMultiplier(0.133333)).toBe('0.1333')
    expect(formatRechargeMultiplier(Number.NaN)).toBe('-')
    expect(formatCnyPerUsd(7.5)).toBe('7.50')
  })
})
