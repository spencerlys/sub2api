/**
 * Precise USD formatting for settlement / finance figures.
 *
 * Per-request costs are fractions of a cent, so a fixed 2-decimal format hides
 * real differences between official price, billed amount and owner payable.
 * Precision scales with magnitude: 6 decimals below $1, 4 below $100, else 2.
 */
import { getLocale } from '@/i18n'

export function preciseFractionDigits(amount: number): number {
  const abs = Math.abs(amount)
  if (abs === 0) return 2
  if (abs < 1) return 6
  if (abs < 100) return 4
  return 2
}

export function formatPreciseUSD(amount: number | null | undefined, locale?: string): string {
  const value = typeof amount === 'number' && Number.isFinite(amount) ? amount : 0
  const digits = preciseFractionDigits(value)
  return new Intl.NumberFormat(locale ?? getLocale(), {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: digits,
    maximumFractionDigits: digits
  }).format(value)
}
