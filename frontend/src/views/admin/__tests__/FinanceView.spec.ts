import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'

const api = vi.hoisted(() => ({
  getFinanceSummary: vi.fn(),
  getOwnerSettlements: vi.fn(),
  listOwnerPayouts: vi.fn(),
  createOwnerPayout: vi.fn(),
  deleteOwnerPayout: vi.fn(),
  listUsers: vi.fn()
}))
const toast = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    finance: {
      getFinanceSummary: api.getFinanceSummary,
      getOwnerSettlements: api.getOwnerSettlements,
      listOwnerPayouts: api.listOwnerPayouts,
      createOwnerPayout: api.createOwnerPayout,
      deleteOwnerPayout: api.deleteOwnerPayout
    },
    users: { list: api.listUsers }
  }
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => toast }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key)
    })
  }
})

import FinanceView from '../FinanceView.vue'

enableAutoUnmount(afterEach)

const summary = {
  start_time: '2026-10-01T00:00:00Z',
  end_time: '2026-10-31T00:00:00Z',
  online: { orders: 7, pay_amounts: [{ currency: 'CNY', amount: 700 }, { currency: 'USD', amount: 20 }], credited_usd: 118.5, bonus_usd: 3.25 },
  offline: { count: 2, usd: 50 },
  gift: { total_usd: 31, admin_gift_usd: 10, payment_bonus_usd: 3.25, promo_usd: 5, redeem_code_usd: 7.75, affiliate_usd: 5 },
  corrections: { count: 1, net_usd: -4 },
  deductions: { count: 3, usd: -12 }
}

const settlementRow = {
  owner_user_id: 3,
  email: 'owner-a@example.com',
  username: 'alice',
  requests: 900,
  official_cost: 200,
  user_billed: 150,
  account_cost: 120,
  earnings: 60,
  earnings_all_time: 400,
  paid_total: 300,
  unpaid: 100
}

const settlements = {
  start_time: '',
  end_time: '',
  items: [settlementRow],
  totals: { requests: 900, official_cost: 200, user_billed: 150, account_cost: 120, earnings: 60, earnings_all_time: 400, paid_total: 300, unpaid: 100 }
}

const payouts = {
  items: [
    { id: 21, owner_user_id: 3, owner_email: 'owner-a@example.com', amount: 80, paid_at: '2026-10-05T12:00:00Z', notes: 'bank', created_by: 1, created_at: '2026-10-05T12:00:00Z' }
  ],
  total: 1,
  page: 1,
  page_size: 20,
  pages: 1
}

function mountView() {
  return mount(FinanceView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
        ConfirmDialog: {
          props: ['show', 'message'],
          emits: ['confirm', 'cancel'],
          template: '<div v-if="show" data-testid="confirm-dialog">{{ message }}<button data-testid="confirm-ok" @click="$emit(\'confirm\')" /></div>'
        }
      }
    }
  })
}

describe('admin FinanceView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 9, 30, 15))
    api.getFinanceSummary.mockResolvedValue(summary)
    api.getOwnerSettlements.mockResolvedValue(settlements)
    api.listOwnerPayouts.mockResolvedValue(payouts)
    api.listUsers.mockResolvedValue({ items: [{ id: 3, email: 'owner-a@example.com', username: 'alice' }], total: 1 })
  })
  afterEach(() => vi.useRealTimers())

  it('loads this month and renders money-in, settlements and payouts', async () => {
    const wrapper = mountView()
    await flushPromises()

    const range = { start: '2026-10-01', end: '2026-10-30' }
    expect(api.getFinanceSummary).toHaveBeenCalledWith(range)
    expect(api.getOwnerSettlements).toHaveBeenCalledWith(range)
    expect(api.listOwnerPayouts).toHaveBeenCalledWith({ ...range, page: 1, pageSize: 20 })

    expect(wrapper.get('[data-testid="finance-online"]').text()).toContain('118.50')
    expect(wrapper.get('[data-testid="finance-online"]').text()).toContain('{"count":7}')
    expect(wrapper.get('[data-testid="finance-pay-amount-CNY"]').text()).toContain('700.00')
    expect(wrapper.get('[data-testid="finance-pay-amount-USD"]').text()).toContain('20.00')
    expect(wrapper.get('[data-testid="finance-offline"]').text()).toContain('50.00')
    expect(wrapper.get('[data-testid="finance-gift"]').text()).toContain('31.00')
    expect(wrapper.get('[data-testid="finance-gift-redeem_code_usd"]').text()).toContain('7.75')
    expect(wrapper.get('[data-testid="finance-corrections"]').text()).toContain('-$4.00')
    expect(wrapper.get('[data-testid="finance-deductions"]').text()).toContain('-$12.00')

    const row = wrapper.get('[data-testid="settlement-row-3"]').text()
    expect(row).toContain('owner-a@example.com')
    for (const value of ['200.00', '150.00', '120.00', '60.00', '400.00', '300.00', '100.00']) expect(row).toContain(value)
    expect(wrapper.get('[data-testid="settlement-totals"]').text()).toContain('admin.finance.settlement.totals')

    expect(wrapper.get('[data-testid="payout-row-21"]').text()).toContain('80.00')
    expect(wrapper.get('[data-testid="payout-row-21"]').text()).toContain('bank')
  })

  it('refetches everything when the period changes', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="finance-period-lastMonth"]').trigger('click')
    await flushPromises()

    const range = { start: '2026-09-01', end: '2026-09-30' }
    expect(api.getFinanceSummary).toHaveBeenLastCalledWith(range)
    expect(api.getOwnerSettlements).toHaveBeenLastCalledWith(range)
    expect(api.listOwnerPayouts).toHaveBeenLastCalledWith({ ...range, page: 1, pageSize: 20 })
  })

  it('filters payout history by owner', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="payout-history"] [data-testid="owner-select"]').setValue('3')
    await flushPromises()
    expect(api.listOwnerPayouts).toHaveBeenLastCalledWith({ start: '2026-10-01', end: '2026-10-30', ownerUserId: 3, page: 1, pageSize: 20 })
  })

  it('records a payout for an owner and refreshes', async () => {
    api.createOwnerPayout.mockResolvedValue({ id: 22 })
    const wrapper = mountView()
    await flushPromises()
    const calls = api.getOwnerSettlements.mock.calls.length

    await wrapper.get('[data-testid="settlement-payout-3"]').trigger('click')
    expect(wrapper.get('[data-testid="payout-dialog"]').text()).toContain('owner-a@example.com')
    await wrapper.get('[data-testid="payout-amount"]').setValue('100')
    await wrapper.get('[data-testid="payout-paid-at"]').setValue('2026-10-15')
    await wrapper.get('[data-testid="payout-notes"]').setValue('October')
    await wrapper.get('[data-testid="payout-form"]').trigger('submit')
    await flushPromises()

    expect(api.createOwnerPayout).toHaveBeenCalledWith({
      owner_user_id: 3,
      amount: 100,
      paid_at: new Date(2026, 9, 15, 12).toISOString(),
      notes: 'October'
    })
    expect(toast.showSuccess).toHaveBeenCalledWith('admin.finance.payout.success')
    expect(api.getOwnerSettlements.mock.calls.length).toBe(calls + 1)
    expect(wrapper.find('[data-testid="payout-dialog"]').exists()).toBe(false)
  })

  it('omits paid_at (= now) when the payout is dated today and rejects a zero amount', async () => {
    api.createOwnerPayout.mockResolvedValue({ id: 23 })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="settlement-payout-3"]').trigger('click')

    await wrapper.get('[data-testid="payout-form"]').trigger('submit')
    await flushPromises()
    expect(api.createOwnerPayout).not.toHaveBeenCalled()
    expect(toast.showError).toHaveBeenCalledWith('admin.finance.payout.amountRequired')

    await wrapper.get('[data-testid="payout-amount"]').setValue('25.5')
    await wrapper.get('[data-testid="payout-form"]').trigger('submit')
    await flushPromises()
    expect(api.createOwnerPayout).toHaveBeenCalledWith({ owner_user_id: 3, amount: 25.5, notes: '' })
  })

  it('deletes a payout after confirmation', async () => {
    api.deleteOwnerPayout.mockResolvedValue(undefined)
    const wrapper = mountView()
    await flushPromises()
    const calls = api.listOwnerPayouts.mock.calls.length

    await wrapper.get('[data-testid="payout-delete-21"]').trigger('click')
    expect(api.deleteOwnerPayout).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="confirm-dialog"]').text()).toContain('owner-a@example.com')
    await wrapper.get('[data-testid="confirm-ok"]').trigger('click')
    await flushPromises()

    expect(api.deleteOwnerPayout).toHaveBeenCalledWith(21)
    expect(toast.showSuccess).toHaveBeenCalledWith('admin.finance.payout.deleted')
    expect(api.listOwnerPayouts.mock.calls.length).toBeGreaterThan(calls)
  })

  it('reports load failures through the app toast', async () => {
    api.getFinanceSummary.mockRejectedValue(new Error('summary down'))
    mountView()
    await flushPromises()
    expect(toast.showError).toHaveBeenCalledWith('summary down')
  })
})
