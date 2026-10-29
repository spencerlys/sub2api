import type { AccountHealthItem } from '@/api/admin/accountHealth'

/**
 * Account health thresholds — the single source of truth for the Account Health page.
 *
 * Evaluated in this order (first match wins):
 *   1. failing       account status is "error" (upstream marked it broken, e.g. revoked token)
 *   2. disabled      status is not "active", or the account is not schedulable (paused by hand)
 *   3. rate_limited  currently rate-limited, overloaded or temporarily unschedulable
 *                    (a live cooldown is more actionable than the historical error rate)
 *   4. failing       error rate >= FAILING_ERROR_RATE with at least MIN_SAMPLE requests
 *   5. degraded      error rate >= DEGRADED_ERROR_RATE with at least MIN_SAMPLE requests
 *   6. idle          no traffic at all in the selected range
 *   7. healthy       otherwise
 *
 * MIN_SAMPLE keeps one stray error on a barely used account from flagging it.
 */
export const ACCOUNT_HEALTH_THRESHOLDS = {
  /** Minimum requests (ok + errors) before the error rate is trusted. */
  MIN_SAMPLE: 5,
  /** Error rate (0..1) at or above which the account is failing. */
  FAILING_ERROR_RATE: 0.2,
  /** Error rate (0..1) at or above which the account is degraded. */
  DEGRADED_ERROR_RATE: 0.05
} as const

export type AccountHealthStatus = 'healthy' | 'degraded' | 'failing' | 'rate_limited' | 'disabled' | 'idle'

type HealthInput = Pick<
  AccountHealthItem,
  | 'status'
  | 'schedulable'
  | 'rate_limited'
  | 'overloaded'
  | 'temp_unschedulable'
  | 'requests_ok'
  | 'errors_total'
  | 'error_rate'
>

export function deriveAccountHealth(item: HealthInput): AccountHealthStatus {
  const { MIN_SAMPLE, FAILING_ERROR_RATE, DEGRADED_ERROR_RATE } = ACCOUNT_HEALTH_THRESHOLDS

  if (item.status === 'error') return 'failing'
  if (item.status !== 'active' || !item.schedulable) return 'disabled'
  if (item.rate_limited || item.overloaded || item.temp_unschedulable) return 'rate_limited'

  const sample = (item.requests_ok || 0) + (item.errors_total || 0)
  const errorRate = item.error_rate || 0
  if (sample >= MIN_SAMPLE) {
    if (errorRate >= FAILING_ERROR_RATE) return 'failing'
    if (errorRate >= DEGRADED_ERROR_RATE) return 'degraded'
  }
  if (sample === 0) return 'idle'
  return 'healthy'
}

export const ACCOUNT_HEALTH_BADGE_CLASS: Record<AccountHealthStatus, string> = {
  healthy: 'badge-success',
  degraded: 'badge-warning',
  failing: 'badge-danger',
  rate_limited: 'badge-warning',
  disabled: 'badge-gray',
  idle: 'badge-gray'
}
