import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn()
}))
vi.mock('@/api/client', () => ({ apiClient: { get, post, put, delete: del } }))
vi.mock('@/utils/format', () => ({ getBrowserTimeZone: () => 'Asia/Shanghai' }))

import {
  createOwnerPayout,
  deleteOwnerPayout,
  getFinanceSummary,
  getMyOwnerSettlement,
  getOwnerSettlements,
  listOwnerPayouts
} from '@/api/admin/finance'
import { updateBalance, updateBalanceRecordSource } from '@/api/admin/users'

const range = { start: '2026-10-01', end: '2026-10-30' }

describe('admin finance API', () => {
  beforeEach(() => {
    for (const fn of [get, post, put, del]) {
      fn.mockReset()
      fn.mockResolvedValue({ data: { ok: true } })
    }
  })

  it('requests owner settlements with the range, timezone and optional owner filter', async () => {
    await expect(getOwnerSettlements(range)).resolves.toEqual({ ok: true })
    expect(get).toHaveBeenLastCalledWith('/admin/owner-settlements', {
      params: { start: '2026-10-01', end: '2026-10-30', timezone: 'Asia/Shanghai' }
    })

    await getOwnerSettlements(range, 7)
    expect(get).toHaveBeenLastCalledWith('/admin/owner-settlements', {
      params: { start: '2026-10-01', end: '2026-10-30', timezone: 'Asia/Shanghai', owner_user_id: 7 }
    })
  })

  it('requests the caller-owned settlement', async () => {
    await getMyOwnerSettlement(range)
    expect(get).toHaveBeenLastCalledWith('/admin/owner-settlements/me', {
      params: { start: '2026-10-01', end: '2026-10-30', timezone: 'Asia/Shanghai' }
    })
  })

  it('lists owner payouts with pagination and an optional owner', async () => {
    await listOwnerPayouts({ ...range, page: 2, pageSize: 10 })
    expect(get).toHaveBeenLastCalledWith('/admin/owner-payouts', {
      params: { start: '2026-10-01', end: '2026-10-30', timezone: 'Asia/Shanghai', page: 2, page_size: 10 }
    })

    await listOwnerPayouts({ ...range, ownerUserId: 3 })
    expect(get).toHaveBeenLastCalledWith('/admin/owner-payouts', {
      params: { start: '2026-10-01', end: '2026-10-30', timezone: 'Asia/Shanghai', page: 1, page_size: 20, owner_user_id: 3 }
    })
  })

  it('creates and deletes owner payouts', async () => {
    const body = { owner_user_id: 3, amount: 12.5, paid_at: '2026-10-29T00:00:00.000Z', notes: 'bank' }
    await createOwnerPayout(body)
    expect(post).toHaveBeenLastCalledWith('/admin/owner-payouts', body)

    await deleteOwnerPayout(9)
    expect(del).toHaveBeenLastCalledWith('/admin/owner-payouts/9')
  })

  it('requests the finance summary', async () => {
    await getFinanceSummary(range)
    expect(get).toHaveBeenLastCalledWith('/admin/finance/summary', {
      params: { start: '2026-10-01', end: '2026-10-30', timezone: 'Asia/Shanghai' }
    })
  })
})

describe('admin users balance source API', () => {
  beforeEach(() => {
    post.mockReset()
    put.mockReset()
    post.mockResolvedValue({ data: { id: 1 } })
    put.mockResolvedValue({ data: { id: 5, balance_source: 'offline', notes: 'n' } })
  })

  it('omits source for legacy callers and sends it when given', async () => {
    await updateBalance(1, 10, 'add', 'hi')
    expect(post).toHaveBeenLastCalledWith('/admin/users/1/balance', { balance: 10, operation: 'add', notes: 'hi' })

    await updateBalance(1, 10, 'add', 'hi', 'offline')
    expect(post).toHaveBeenLastCalledWith('/admin/users/1/balance', {
      balance: 10,
      operation: 'add',
      notes: 'hi',
      source: 'offline'
    })
  })

  it('updates the source of one balance record', async () => {
    await expect(updateBalanceRecordSource(1, 5, 'offline')).resolves.toEqual({
      id: 5,
      balance_source: 'offline',
      notes: 'n'
    })
    expect(put).toHaveBeenLastCalledWith('/admin/users/1/balance-history/5/source', { source: 'offline' })
  })
})
