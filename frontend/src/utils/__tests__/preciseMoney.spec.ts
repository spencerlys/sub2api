import { describe, expect, it } from 'vitest'

import { formatPreciseUSD, preciseFractionDigits } from '../preciseMoney'

describe('preciseMoney', () => {
  it('uses more decimals for smaller amounts', () => {
    expect(preciseFractionDigits(0)).toBe(2)
    expect(preciseFractionDigits(0.000123)).toBe(6)
    expect(preciseFractionDigits(0.99)).toBe(6)
    expect(preciseFractionDigits(-0.5)).toBe(6)
    expect(preciseFractionDigits(1)).toBe(4)
    expect(preciseFractionDigits(99.99)).toBe(4)
    expect(preciseFractionDigits(-42)).toBe(4)
    expect(preciseFractionDigits(100)).toBe(2)
    expect(preciseFractionDigits(12345.678)).toBe(2)
  })

  it('formats USD with the precision for its size', () => {
    expect(formatPreciseUSD(0.000123, 'en-US')).toBe('$0.000123')
    expect(formatPreciseUSD(0.5, 'en-US')).toBe('$0.500000')
    expect(formatPreciseUSD(12.34567, 'en-US')).toBe('$12.3457')
    expect(formatPreciseUSD(1234.5, 'en-US')).toBe('$1,234.50')
    expect(formatPreciseUSD(-4, 'en-US')).toBe('-$4.0000')
    expect(formatPreciseUSD(0, 'en-US')).toBe('$0.00')
  })

  it('treats missing or non-finite values as zero', () => {
    expect(formatPreciseUSD(null, 'en-US')).toBe('$0.00')
    expect(formatPreciseUSD(undefined, 'en-US')).toBe('$0.00')
    expect(formatPreciseUSD(Number.NaN, 'en-US')).toBe('$0.00')
  })
})
