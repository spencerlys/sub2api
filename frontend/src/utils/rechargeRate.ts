// 余额充值自动汇率：市场 USD/CNY + 加价 → 充值倍率（¥1 可得多少 USD 余额）。
// 与后端 ComputeRechargeRate 镜像，仅用于后台预览；实际写入以后端计算为准。

export type RechargeRateMarkupMode = 'amount' | 'percent'

export const RECHARGE_RATE_MARKUP_MODES: RechargeRateMarkupMode[] = ['amount', 'percent']
export const MAX_RECHARGE_RATE_MARKUP_AMOUNT = 5
export const MAX_RECHARGE_RATE_MARKUP_PERCENT = 100
export const MIN_RECHARGE_MARKET_RATE = 5
export const MAX_RECHARGE_MARKET_RATE = 10

export function normalizeRechargeRateMarkupMode(raw: unknown): RechargeRateMarkupMode {
  return raw === 'amount' ? 'amount' : 'percent'
}

export function maxRechargeRateMarkup(mode: RechargeRateMarkupMode): number {
  return mode === 'amount' ? MAX_RECHARGE_RATE_MARKUP_AMOUNT : MAX_RECHARGE_RATE_MARKUP_PERCENT
}

export interface RechargeRatePreview {
  /** 含加价后的 CNY / 1 USD */
  effective: number
  /** USD / ¥1，6 位小数 */
  multiplier: number
}

/** 市场汇率无效（未获取或越界）或加价越界时返回 null。 */
export function computeRechargeRate(
  market: number | null | undefined,
  mode: RechargeRateMarkupMode,
  markup: number | null | undefined,
): RechargeRatePreview | null {
  const m = Number(market)
  const v = Number(markup ?? 0)
  if (!Number.isFinite(m) || m < MIN_RECHARGE_MARKET_RATE || m > MAX_RECHARGE_MARKET_RATE) return null
  if (!Number.isFinite(v) || v < 0 || v > maxRechargeRateMarkup(mode)) return null
  const effective = mode === 'amount' ? m + v : m * (1 + v / 100)
  if (!(effective > 0)) return null
  return {
    effective: Math.round(effective * 1e6) / 1e6,
    multiplier: Math.round((1 / effective) * 1e6) / 1e6,
  }
}

/** "0.133333" → "0.1333"；保留 4 位小数用于展示。 */
export function formatRechargeMultiplier(multiplier: number): string {
  return Number.isFinite(multiplier) ? multiplier.toFixed(4) : '-'
}

/** CNY per USD，2 位小数（$1 = ¥7.50）。 */
export function formatCnyPerUsd(cnyPerUsd: number): string {
  return Number.isFinite(cnyPerUsd) ? cnyPerUsd.toFixed(2) : '-'
}
