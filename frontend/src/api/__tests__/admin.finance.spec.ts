import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn()
}))
vi.mock('@/api/client', () => ({ apiClient: { get, post, put, delete: del } }))

import {
  createOwnerPayout,
  deleteOwnerPayout,
  getFinanceSummary,
  getMyOwnerSettlement,
  getOwnerSettlements,
  listOwnerPayouts,
  rebuildLedger
} from '@/api/admin/finance'
import { updateBalance, updateBalanceRecordSource } from '@/api/admin/users'

const custom = { range: 'custom' as const, start: '2026-10-01', end: '2026-10-30' }

describe('admin finance API', () => {
  beforeEach(() => {
    for (const fn of [get, post, put, del]) {
      fn.mockReset()
      fn.mockResolvedValue({ data: { ok: true } })
    }
  })

  it('sends only the preset name for preset ranges (server computes the days, no timezone)', async () => {
    for (const range of ['today', 'yesterday', 'this_month', 'last_month', 'last_7d', 'last_30d', 'all'] as const) {
      await getOwnerSettlements({ range })
      expect(get).toHaveBeenLastCalledWith('/admin/owner-settlements', { params: { range } })
    }
    // Stray dates on a preset are not sent.
    await getOwnerSettlements({ range: 'today', start: '2026-01-01', end: '2026-01-02' })
    expect(get).toHaveBeenLastCalledWith('/admin/owner-settlements', { params: { range: 'today' } })
  })

  it('requests owner settlements with a custom range and an optional owner filter', async () => {
    await expect(getOwnerSettlements(custom)).resolves.toEqual({ ok: true })
    expect(get).toHaveBeenLastCalledWith('/admin/owner-settlements', {
      params: { range: 'custom', start: '2026-10-01', end: '2026-10-30' }
    })

    await getOwnerSettlements({ range: 'all' }, 7)
    expect(get).toHaveBeenLastCalledWith('/admin/owner-settlements', {
      params: { range: 'all', owner_user_id: 7 }
    })
  })

  it('requests the caller-owned settlement', async () => {
    await getMyOwnerSettlement({ range: 'today' })
    expect(get).toHaveBeenLastCalledWith('/admin/owner-settlements/me', { params: { range: 'today' } })

    await getMyOwnerSettlement(custom)
    expect(get).toHaveBeenLastCalledWith('/admin/owner-settlements/me', {
      params: { range: 'custom', start: '2026-10-01', end: '2026-10-30' }
    })
  })

  it('lists owner payouts with the range, pagination and an optional owner', async () => {
    await listOwnerPayouts({ range: custom, page: 2, pageSize: 10 })
    expect(get).toHaveBeenLastCalledWith('/admin/owner-payouts', {
      params: { range: 'custom', start: '2026-10-01', end: '2026-10-30', page: 2, page_size: 10 }
    })

    await listOwnerPayouts({ range: { range: 'last_month' }, ownerUserId: 3 })
    expect(get).toHaveBeenLastCalledWith('/admin/owner-payouts', {
      params: { range: 'last_month', page: 1, page_size: 20, owner_user_id: 3 }
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
    await getFinanceSummary({ range: 'last_7d' })
    expect(get).toHaveBeenLastCalledWith('/admin/finance/summary', { params: { range: 'last_7d' } })
  })

  it('asks the backend to rebuild the settlement ledger', async () => {
    post.mockResolvedValue({ data: { rebuild_pending: true } })
    await expect(rebuildLedger()).resolves.toEqual({ rebuild_pending: true })
    expect(post).toHaveBeenLastCalledWith('/admin/finance/rebuild-ledger')
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
