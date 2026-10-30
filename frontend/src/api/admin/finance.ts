/**
 * Finance / owner settlement API.
 *
 * Admins: per-owner settlement (official price vs. what users were billed vs.
 * what the owner earned and was paid), owner payout records and a money-in
 * summary by source. Owners: only their own settlement via `/me`, which never
 * carries user identities or billing multipliers.
 *
 * Dates are YYYY-MM-DD in the browser timezone; `end` is inclusive.
 */

import { apiClient } from '../client'
import { getBrowserTimeZone } from '@/utils/format'
import type { PaginatedResponse } from '@/types'

export interface FinanceDateRange {
  start: string
  end: string
}

export interface OwnerSettlementTotals {
  requests: number
  /** USD at official list price. */
  official_cost: number
  /** USD actually charged to users. */
  user_billed: number
  /** USD, official price × account multiplier (before the owner's paid share). */
  account_cost: number
  /** USD owner payout earned in the period. */
  earnings: number
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

export interface OwnerSettlementsResponse {
  start_time: string
  end_time: string
  items: OwnerSettlementRow[]
  totals: OwnerSettlementTotals
}

export interface MyOwnerSettlement {
  start_time: string
  end_time: string
  requests: number
  official_cost: number
  earnings: number
  earnings_all_time: number
  paid_total: number
  unpaid: number
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

export interface ListOwnerPayoutsParams extends FinanceDateRange {
  ownerUserId?: number
  page?: number
  pageSize?: number
}

export interface FinancePayAmount {
  currency: string
  amount: number
}

export interface FinanceSummary {
  start_time: string
  end_time: string
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
}

function rangeParams(range: FinanceDateRange): Record<string, string | number> {
  return { start: range.start, end: range.end, timezone: getBrowserTimeZone() }
}

export async function getOwnerSettlements(
  range: FinanceDateRange,
  ownerUserId?: number
): Promise<OwnerSettlementsResponse> {
  const params = rangeParams(range)
  if (ownerUserId !== undefined) params.owner_user_id = ownerUserId
  const { data } = await apiClient.get<OwnerSettlementsResponse>('/admin/owner-settlements', { params })
  return data
}

export async function getMyOwnerSettlement(range: FinanceDateRange): Promise<MyOwnerSettlement> {
  const { data } = await apiClient.get<MyOwnerSettlement>('/admin/owner-settlements/me', {
    params: rangeParams(range)
  })
  return data
}

export async function listOwnerPayouts(params: ListOwnerPayoutsParams): Promise<PaginatedResponse<OwnerPayout>> {
  const query = { ...rangeParams(params), page: params.page ?? 1, page_size: params.pageSize ?? 20 }
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

export async function getFinanceSummary(range: FinanceDateRange): Promise<FinanceSummary> {
  const { data } = await apiClient.get<FinanceSummary>('/admin/finance/summary', { params: rangeParams(range) })
  return data
}

export const financeAPI = {
  getOwnerSettlements,
  getMyOwnerSettlement,
  listOwnerPayouts,
  createOwnerPayout,
  deleteOwnerPayout,
  getFinanceSummary
}

export default financeAPI
