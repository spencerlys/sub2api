/**
 * Account health API.
 *
 * Per-account recent health (request/error counts, latency, tokens, cost and the
 * current rate-limit / overload state). AI account owners only receive their own
 * accounts; the backend enforces that scope, so owners never send owner_user_id.
 */

import { apiClient } from '../client'

export type AccountHealthRange = '1h' | '24h' | '7d'

export const ACCOUNT_HEALTH_RANGES: AccountHealthRange[] = ['1h', '24h', '7d']

export type AccountHealthErrorCategory =
  | 'rate_limit'
  | 'overloaded'
  | 'auth'
  | 'upstream_5xx'
  | 'timeout'
  | 'other'

export const ACCOUNT_HEALTH_ERROR_CATEGORIES: AccountHealthErrorCategory[] = [
  'rate_limit',
  'overloaded',
  'auth',
  'upstream_5xx',
  'timeout',
  'other'
]

export type AccountHealthErrorCounts = Record<AccountHealthErrorCategory, number>

export interface AccountHealthItem {
  id: number
  name: string
  platform: string
  type: string
  status: string
  schedulable: boolean
  rate_limit_reset_at: string | null
  overload_until: string | null
  temp_unschedulable_until: string | null
  rate_limited: boolean
  overloaded: boolean
  temp_unschedulable: boolean
  error_message: string
  requests_ok: number
  errors_total: number
  errors: AccountHealthErrorCounts
  /** 0..1 = errors_total / (requests_ok + errors_total); 0 when there is no traffic. */
  error_rate: number
  ttft_p50_ms: number | null
  ttft_p95_ms: number | null
  duration_p50_ms: number | null
  tokens_in: number
  tokens_out: number
  /** Cache creation + cache read tokens. */
  tokens_cache: number
  /** Admins only (absent for owners): USD billed to users, sum of actual_cost. */
  cost?: number
  /** USD at official list price. */
  official_cost: number
  /** USD owner payable for the window (exact settlement ledger). */
  earnings: number
  /** Requests the ledger has not split yet; excluded from earnings. */
  pending_requests: number
  /** Admins only: USD, official price × account multiplier. */
  account_cost?: number
  /** Admins only: the account's owner (null = unowned). */
  owner_user_id?: number | null
  owner_email?: string
  owner_username?: string
  last_success_at: string | null
  last_error_at: string | null
  last_error_category: AccountHealthErrorCategory | ''
  last_error_status: number | null
}

export interface AccountHealthResponse {
  range: AccountHealthRange
  start_time: string
  end_time: string
  items: AccountHealthItem[]
}

/**
 * Get per-account health for the given time range.
 * @param range - 1h / 24h / 7d
 * @param ownerUserId - admins only: restrict to one owner's accounts
 */
export async function getAccountHealth(
  range: AccountHealthRange = '24h',
  ownerUserId?: number
): Promise<AccountHealthResponse> {
  const params: Record<string, string | number> = { range }
  if (ownerUserId !== undefined) params.owner_user_id = ownerUserId
  const { data } = await apiClient.get<AccountHealthResponse>('/admin/accounts/health', { params })
  return data
}

export const accountHealthAPI = {
  getAccountHealth
}

export default accountHealthAPI
