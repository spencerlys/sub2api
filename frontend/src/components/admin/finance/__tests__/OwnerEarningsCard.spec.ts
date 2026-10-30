import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'

const { getMyOwnerSettlement, showError } = vi.hoisted(() => ({
  getMyOwnerSettlement: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({ adminAPI: { finance: { getMyOwnerSettlement } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key)
    })
  }
})

import OwnerEarningsCard from '../OwnerEarningsCard.vue'

enableAutoUnmount(afterEach)

const ledger = {
  processed_until: '2026-10-30T03:59:00Z',
  last_run_at: '2026-10-30T04:00:00Z',
  lag_seconds: 60,
  last_error: 'secret db error',
  rebuild_pending: false,
  last_rebuild_at: null
}

function settlement(overrides: Record<string, unknown> = {}) {
  return {
    range: 'this_month',
    timezone: 'Asia/Shanghai',
    start_time: '2026-09-30T16:00:00Z',
    end_time: '2026-10-31T16:00:00Z',
    requests: 42,
    official_cost: 120.5,
    earnings: 60.25,
    pending_requests: 0,
    earnings_all_time: 300,
    paid_total: 250,
    unpaid: 50,
    ledger,
    ...overrides
  }
}

describe('OwnerEarningsCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads this month and shows requests, official cost, earnings, paid out and unpaid', async () => {
    getMyOwnerSettlement.mockResolvedValue(settlement())
    const wrapper = mount(OwnerEarningsCard)
    await flushPromises()

    expect(getMyOwnerSettlement).toHaveBeenCalledWith({ range: 'this_month' })
    expect(wrapper.get('[data-testid="owner-earnings-requests"]').text()).toContain('42')
    expect(wrapper.get('[data-testid="owner-earnings-official_cost"]').text()).toContain('$120.50')
    expect(wrapper.get('[data-testid="owner-earnings-earnings"]').text()).toContain('$60.2500')
    expect(wrapper.get('[data-testid="owner-earnings-paid_total"]').text()).toContain('$250.00')
    expect(wrapper.get('[data-testid="owner-earnings-unpaid"]').text()).toContain('$50.0000')
    expect(wrapper.get('[data-testid="finance-window"]').text()).toContain('Asia/Shanghai')
    // No pending note, no lag note, and never the ledger error or billed amounts.
    expect(wrapper.find('[data-testid="ledger-status-note"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('secret db error')
    expect(wrapper.text()).not.toMatch(/multiplier|user_billed|userBilled|account_cost|accountCost|paid_cost|gift_cost/i)
  })

  it('offers today and all time and refetches for them', async () => {
    getMyOwnerSettlement.mockResolvedValue(settlement())
    const wrapper = mount(OwnerEarningsCard)
    await flushPromises()

    getMyOwnerSettlement.mockResolvedValue(settlement({ range: 'today', earnings: 0.001234 }))
    await wrapper.get('[data-testid="finance-period-today"]').trigger('click')
    await flushPromises()
    expect(getMyOwnerSettlement).toHaveBeenLastCalledWith({ range: 'today' })
    expect(wrapper.get('[data-testid="owner-earnings-earnings"]').text()).toContain('$0.001234')

    getMyOwnerSettlement.mockResolvedValue(settlement({ range: 'all', start_time: '1970-01-01T00:00:00Z' }))
    await wrapper.get('[data-testid="finance-period-all"]').trigger('click')
    await flushPromises()
    expect(getMyOwnerSettlement).toHaveBeenLastCalledWith({ range: 'all' })
    expect(wrapper.get('[data-testid="finance-window"]').text()).toContain('admin.finance.period.allTime')
  })

  it('notes requests still pending settlement and a lagging ledger', async () => {
    getMyOwnerSettlement.mockResolvedValue(settlement({ pending_requests: 17, ledger: { ...ledger, lag_seconds: 900 } }))
    const wrapper = mount(OwnerEarningsCard)
    await flushPromises()

    expect(wrapper.get('[data-testid="ledger-pending-note"]').text()).toContain('admin.finance.ledger.pendingNote')
    expect(wrapper.get('[data-testid="ledger-pending-note"]').text()).toContain('"count":"17"')
    expect(wrapper.get('[data-testid="ledger-lag-note"]').text()).toContain('admin.finance.ledger.lagNote')
    expect(wrapper.text()).not.toContain('secret db error')
  })

  it('reports load failures through the app toast', async () => {
    getMyOwnerSettlement.mockRejectedValue(new Error('nope'))
    mount(OwnerEarningsCard)
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('nope')
  })
})
