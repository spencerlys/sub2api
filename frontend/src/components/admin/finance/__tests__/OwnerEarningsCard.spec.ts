import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'

const { getMyOwnerSettlement, showError } = vi.hoisted(() => ({
  getMyOwnerSettlement: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({ adminAPI: { finance: { getMyOwnerSettlement } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/utils/format', () => ({ formatCurrency: (v: number) => `$${v.toFixed(2)}`, formatNumber: (v: number) => String(v) }))

import OwnerEarningsCard from '../OwnerEarningsCard.vue'

enableAutoUnmount(afterEach)

function settlement(overrides: Record<string, number> = {}) {
  return {
    start_time: '2026-10-01T00:00:00Z',
    end_time: '2026-10-31T00:00:00Z',
    requests: 42,
    official_cost: 120.5,
    earnings: 60.25,
    earnings_all_time: 300,
    paid_total: 250,
    unpaid: 50,
    ...overrides
  }
}

describe('OwnerEarningsCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 9, 30, 12))
  })
  afterEach(() => vi.useRealTimers())

  it('loads this month and shows official cost, earnings, paid out and unpaid', async () => {
    getMyOwnerSettlement.mockResolvedValue(settlement())
    const wrapper = mount(OwnerEarningsCard)
    await flushPromises()

    expect(getMyOwnerSettlement).toHaveBeenCalledWith({ start: '2026-10-01', end: '2026-10-30' })
    expect(wrapper.get('[data-testid="owner-earnings-official_cost"]').text()).toContain('$120.50')
    expect(wrapper.get('[data-testid="owner-earnings-earnings"]').text()).toContain('$60.25')
    expect(wrapper.get('[data-testid="owner-earnings-paid_total"]').text()).toContain('$250.00')
    expect(wrapper.get('[data-testid="owner-earnings-unpaid"]').text()).toContain('$50.00')
    expect(wrapper.text()).not.toMatch(/multiplier|user_billed|account_cost/i)
  })

  it('refetches when the period changes', async () => {
    getMyOwnerSettlement.mockResolvedValue(settlement())
    const wrapper = mount(OwnerEarningsCard)
    await flushPromises()

    getMyOwnerSettlement.mockResolvedValue(settlement({ earnings: 10 }))
    await wrapper.get('[data-testid="finance-period-lastMonth"]').trigger('click')
    await flushPromises()

    expect(getMyOwnerSettlement).toHaveBeenLastCalledWith({ start: '2026-09-01', end: '2026-09-30' })
    expect(wrapper.get('[data-testid="owner-earnings-earnings"]').text()).toContain('$10.00')
  })

  it('reports load failures through the app toast', async () => {
    getMyOwnerSettlement.mockRejectedValue(new Error('nope'))
    mount(OwnerEarningsCard)
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('nope')
  })
})
