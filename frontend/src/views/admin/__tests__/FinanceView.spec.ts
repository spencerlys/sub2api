import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'

const api = vi.hoisted(() => ({
  getFinanceSummary: vi.fn(),
  getOwnerSettlements: vi.fn(),
  listOwnerPayouts: vi.fn(),
  createOwnerPayout: vi.fn(),
  deleteOwnerPayout: vi.fn(),
  rebuildLedger: vi.fn(),
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
      deleteOwnerPayout: api.deleteOwnerPayout,
      rebuildLedger: api.rebuildLedger
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

const ledger = {
  processed_until: '2026-10-30T06:55:00Z',
  last_run_at: '2026-10-30T06:56:00Z',
  lag_seconds: 1200,
  last_error: 'split failed: deadlock detected',
  rebuild_pending: false,
  last_rebuild_at: null
}

const summary = {
  range: 'this_month',
  timezone: 'Asia/Shanghai',
  start_time: '2026-09-30T16:00:00Z',
  end_time: '2026-10-31T16:00:00Z',
  ledger,
  consumption: { requests: 950, user_billed: 150, paid_cost: 100, gift_cost: 50, pending_requests: 12, pending_user_billed: 0.0425 },
  online: { orders: 7, pay_amounts: [{ currency: 'CNY', amount: 700 }, { currency: 'USD', amount: 20 }], credited_usd: 118.5, bonus_usd: 3.25 },
  offline: { count: 2, usd: 50 },
  gift: { total_usd: 31, admin_gift_usd: 10, payment_bonus_usd: 3.25, promo_usd: 5, redeem_code_usd: 7.75, affiliate_usd: 5 },
  corrections: { count: 1, net_usd: -4 },
  deductions: { count: 3, usd: -12 }
}

const settlementTotals = {
  active_accounts: 2,
  requests: 900,
  official_cost: 200,
  user_billed: 150,
  paid_cost: 100,
  gift_cost: 50,
  account_cost: 120,
  earnings: 60,
  gift_account_cost: 40,
  pending_requests: 0,
  pending_account_cost: 0,
  earnings_all_time: 400,
  paid_total: 300,
  unpaid: 100
}

const settlementRow = { ...settlementTotals, owner_user_id: 3, email: 'owner-a@example.com', username: 'alice' }

const settlements = {
  range: 'this_month',
  timezone: 'Asia/Shanghai',
  start_time: '',
  end_time: '',
  ledger,
  items: [settlementRow],
  totals: settlementTotals
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

    const range = { range: 'this_month' }
    expect(api.getFinanceSummary).toHaveBeenCalledWith(range)
    expect(api.getOwnerSettlements).toHaveBeenCalledWith(range)
    expect(api.listOwnerPayouts).toHaveBeenCalledWith({ range, page: 1, pageSize: 20 })
    expect(wrapper.get('[data-testid="finance-window"]').text()).toContain('Asia/Shanghai')

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
    expect(wrapper.find('[data-testid="settlement-col-gift_account_cost"]').attributes('data-reference')).toBe('true')
    expect(wrapper.get('[data-testid="settlement-totals"]').text()).toContain('admin.finance.settlement.totals')

    expect(wrapper.get('[data-testid="payout-row-21"]').text()).toContain('80.00')
    expect(wrapper.get('[data-testid="payout-row-21"]').text()).toContain('bank')
  })

  it('refetches everything when the period changes', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="finance-period-last_month"]').trigger('click')
    await flushPromises()

    const range = { range: 'last_month' }
    expect(api.getFinanceSummary).toHaveBeenLastCalledWith(range)
    expect(api.getOwnerSettlements).toHaveBeenLastCalledWith(range)
    expect(api.listOwnerPayouts).toHaveBeenLastCalledWith({ range, page: 1, pageSize: 20 })

    await wrapper.get('[data-testid="finance-period-all"]').trigger('click')
    await flushPromises()
    expect(api.getFinanceSummary).toHaveBeenLastCalledWith({ range: 'all' })
    expect(api.getOwnerSettlements).toHaveBeenLastCalledWith({ range: 'all' })
  })

  it('filters payout history by owner', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="payout-history"] [data-testid="owner-select"]').setValue('3')
    await flushPromises()
    expect(api.listOwnerPayouts).toHaveBeenLastCalledWith({ range: { range: 'this_month' }, ownerUserId: 3, page: 1, pageSize: 20 })
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
  it('shows usage consumption split into paid and gift portions, with pending', async () => {
    const wrapper = mountView()
    await flushPromises()

    const block = wrapper.get('[data-testid="finance-consumption"]')
    expect(block.get('[data-testid="finance-consumption-requests"]').text()).toContain('950')
    expect(block.get('[data-testid="finance-consumption-user_billed"]').text()).toContain('$150.00')
    expect(block.get('[data-testid="finance-consumption-paid_cost"]').text()).toContain('$100.00')
    expect(block.get('[data-testid="finance-consumption-gift_cost"]').text()).toContain('$50.0000')
    const pending = block.get('[data-testid="finance-consumption-pending"]').text()
    expect(pending).toContain('12')
    expect(pending).toContain('$0.042500')
  })

  it('shows the ledger status (processed until, lag, last error)', async () => {
    const wrapper = mountView()
    await flushPromises()

    const status = wrapper.get('[data-testid="finance-ledger"]')
    expect(status.get('[data-testid="finance-ledger-processed-until"]').text()).not.toBe('')
    expect(status.get('[data-testid="finance-ledger-processed-until"]').text()).not.toContain('-')
    expect(status.get('[data-testid="finance-ledger-lag"]').text()).toContain('admin.finance.ledger.lagUnits.minutes {"n":20}')
    expect(status.get('[data-testid="finance-ledger-last-error"]').text()).toContain('split failed: deadlock detected')
    expect(status.find('[data-testid="finance-ledger-rebuild-pending"]').exists()).toBe(false)
  })

  it('rebuilds the ledger after confirmation and shows the rebuild as pending', async () => {
    api.rebuildLedger.mockResolvedValue({ rebuild_pending: true })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="finance-rebuild-ledger"]').trigger('click')
    expect(api.rebuildLedger).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="confirm-dialog"]').text()).toContain('admin.finance.ledger.rebuildConfirm')
    await wrapper.get('[data-testid="confirm-ok"]').trigger('click')
    await flushPromises()

    expect(api.rebuildLedger).toHaveBeenCalledTimes(1)
    expect(toast.showSuccess).toHaveBeenCalledWith('admin.finance.ledger.rebuildStarted')
    expect(wrapper.get('[data-testid="finance-ledger-rebuild-pending"]').text()).toContain('admin.finance.ledger.rebuildPending')
    expect(wrapper.get('[data-testid="finance-rebuild-ledger"]').attributes('disabled')).toBeDefined()
  })

  it('reports a failed ledger rebuild', async () => {
    api.rebuildLedger.mockRejectedValue(new Error('rebuild refused'))
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="finance-rebuild-ledger"]').trigger('click')
    await wrapper.get('[data-testid="confirm-ok"]').trigger('click')
    await flushPromises()
    expect(toast.showError).toHaveBeenCalledWith('rebuild refused')
    expect(wrapper.find('[data-testid="finance-ledger-rebuild-pending"]').exists()).toBe(false)
  })
})
