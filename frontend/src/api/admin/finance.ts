/**
 * Finance / owner settlement API.
 *
 * Admins: per-owner settlement (official price vs. what users were billed —
 * split exactly into the paid and gift-funded portions — vs. what the owner is
 * owed and was paid), owner payout records, a money-in summary by source and
 * the settlement ledger status. Owners: only their own settlement via `/me`,
 * which never carries user identities, billed amounts or billing multipliers.
 *
 * Periods are named presets resolved server-side in the site timezone; only a
 * custom range sends dates (YYYY-MM-DD, `end` inclusive).
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type FinanceRangeKind =
  | 'today'
  | 'yesterday'
  | 'this_month'
  | 'last_month'
  | 'last_7d'
  | 'last_30d'
  | 'all'
  | 'custom'

export interface FinanceRange {
  range: FinanceRangeKind
  /** custom only: YYYY-MM-DD. */
  start?: string
  /** custom only: YYYY-MM-DD, inclusive. */
  end?: string
}

/** The window the server resolved for a request (echoed in every response). */
export interface FinanceWindow {
  range: string
  /** Site timezone the day boundaries were computed in, e.g. "Asia/Shanghai". */
  timezone: string
  /** RFC3339; the Unix epoch for `all`. */
  start_time: string
  end_time: string
}

/** Background ledger job that splits each request into paid / gift-funded parts. */
export interface LedgerStatus {
  processed_until: string | null
  last_run_at: string | null
  /** Seconds behind real time; -1 = never ran. */
  lag_seconds: number
  last_error: string
  rebuild_pending: boolean
  last_rebuild_at: string | null
}

export interface OwnerSettlementTotals {
  /** Owner accounts that are active and schedulable right now. */
  active_accounts: number
  requests: number
  /** USD at official list price. */
  official_cost: number
  /** USD receivable from users (actual_cost). */
  user_billed: number
  /** Part of user_billed funded by paid (cash) balance. */
  paid_cost: number
  /** Part of user_billed funded by gift balance. */
  gift_cost: number
  /** USD, official price × account multiplier. */
  account_cost: number
  /** USD owner payable in the period = Σ account_cost × paid_cost / user_billed. */
  earnings: number
  /** Account cost funded by gift balance. Reference only, never payable. */
  gift_account_cost: number
  /** Requests the ledger has not split yet; excluded from earnings. */
  pending_requests: number
  pending_account_cost: number
  earnings_all_time: number
  paid_total: number
  /** earnings_all_time − paid_total. */
  unpaid: number
}

export interface OwnerSettlementRow extends OwnerSettlementTotals {
  owner_user_id: number
  email: string
  username: string
}

export interface OwnerSettlementsResponse extends FinanceWindow {
  ledger: LedgerStatus
  items: OwnerSettlementRow[]
  totals: OwnerSettlementTotals
}

export interface MyOwnerSettlement extends FinanceWindow {
  requests: number
  official_cost: number
  earnings: number
  pending_requests: number
  earnings_all_time: number
  paid_total: number
  unpaid: number
  ledger: LedgerStatus
}

export interface OwnerPayout {
  id: number
  owner_user_id: number
  owner_email: string
  amount: number
  paid_at: string
  notes: string
  created_by: number
  created_at: string
}

export interface CreateOwnerPayoutRequest {
  owner_user_id: number
  amount: number
  /** RFC3339; omitted = now. */
  paid_at?: string
  notes?: string
}

export interface ListOwnerPayoutsParams {
  range: FinanceRange
  ownerUserId?: number
  page?: number
  pageSize?: number
}

export interface FinancePayAmount {
  currency: string
  amount: number
}

/** All usage in the period, split by the exact ledger. */
export interface FinanceConsumption {
  requests: number
  user_billed: number
  paid_cost: number
  gift_cost: number
  pending_requests: number
  pending_user_billed: number
}

export interface FinanceSummary extends FinanceWindow {
  online: {
    orders: number
    pay_amounts: FinancePayAmount[]
    credited_usd: number
    bonus_usd: number
  }
  offline: {
    count: number
    usd: number
  }
  gift: {
    total_usd: number
    admin_gift_usd: number
    payment_bonus_usd: number
    promo_usd: number
    redeem_code_usd: number
    affiliate_usd: number
  }
  corrections: {
    count: number
    net_usd: number
  }
  /** Admin balance deductions that are not corrections; usd is negative. */
  deductions: {
    count: number
    usd: number
  }
  consumption: FinanceConsumption
  ledger: LedgerStatus
}

export interface RebuildLedgerResponse {
  rebuild_pending: boolean
}

function rangeParams(range: FinanceRange): Record<string, string | number> {
  const params: Record<string, string | number> = { range: range.range }
  if (range.range === 'custom') {
    params.start = range.start ?? ''
    params.end = range.end ?? ''
  }
  return params
}

export async function getOwnerSettlements(
  range: FinanceRange,
  ownerUserId?: number
): Promise<OwnerSettlementsResponse> {
  const params = rangeParams(range)
  if (ownerUserId !== undefined) params.owner_user_id = ownerUserId
  const { data } = await apiClient.get<OwnerSettlementsResponse>('/admin/owner-settlements', { params })
  return data
}

export async function getMyOwnerSettlement(range: FinanceRange): Promise<MyOwnerSettlement> {
  const { data } = await apiClient.get<MyOwnerSettlement>('/admin/owner-settlements/me', {
    params: rangeParams(range)
  })
  return data
}

export async function listOwnerPayouts(params: ListOwnerPayoutsParams): Promise<PaginatedResponse<OwnerPayout>> {
  const query = { ...rangeParams(params.range), page: params.page ?? 1, page_size: params.pageSize ?? 20 }
  if (params.ownerUserId !== undefined) Object.assign(query, { owner_user_id: params.ownerUserId })
  const { data } = await apiClient.get<PaginatedResponse<OwnerPayout>>('/admin/owner-payouts', { params: query })
  return data
}

export async function createOwnerPayout(body: CreateOwnerPayoutRequest): Promise<OwnerPayout> {
  const { data } = await apiClient.post<OwnerPayout>('/admin/owner-payouts', body)
  return data
}

export async function deleteOwnerPayout(id: number): Promise<void> {
  await apiClient.delete(`/admin/owner-payouts/${id}`)
}

export async function getFinanceSummary(range: FinanceRange): Promise<FinanceSummary> {
  const { data } = await apiClient.get<FinanceSummary>('/admin/finance/summary', { params: rangeParams(range) })
  return data
}

/** Schedule a full ledger rebuild; the background job picks it up within about a minute. */
export async function rebuildLedger(): Promise<RebuildLedgerResponse> {
  const { data } = await apiClient.post<RebuildLedgerResponse>('/admin/finance/rebuild-ledger')
  return data
}

export const financeAPI = {
  getOwnerSettlements,
  getMyOwnerSettlement,
  listOwnerPayouts,
  createOwnerPayout,
  deleteOwnerPayout,
  getFinanceSummary,
  rebuildLedger
}

export default financeAPI
