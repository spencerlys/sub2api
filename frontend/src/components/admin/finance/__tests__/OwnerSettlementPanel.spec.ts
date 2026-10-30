import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'

const { getOwnerSettlements, showError } = vi.hoisted(() => ({ getOwnerSettlements: vi.fn(), showError: vi.fn() }))

vi.mock('@/api/admin', () => ({ adminAPI: { finance: { getOwnerSettlements } } }))
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

import OwnerSettlementPanel from '../OwnerSettlementPanel.vue'

enableAutoUnmount(afterEach)

const totals = {
  active_accounts: 2,
  requests: 10,
  official_cost: 1,
  user_billed: 0.8,
  paid_cost: 0.6,
  gift_cost: 0.2,
  account_cost: 0.5,
  payout_base: 0.5,
  earnings: 0.375,
  gift_account_cost: 0.125,
  pending_requests: 4,
  pending_account_cost: 0.1,
  earnings_all_time: 5,
  paid_total: 2,
  unpaid: 3
}

function response(overrides: Record<string, unknown> = {}) {
  return {
    range: 'this_month',
    timezone: 'Asia/Shanghai',
    start_time: '2026-09-30T16:00:00Z',
    end_time: '2026-10-31T16:00:00Z',
    ledger: {
      processed_until: null,
      last_run_at: null,
      lag_seconds: 20,
      last_error: '',
      rebuild_pending: false,
      last_rebuild_at: null
    },
    items: [{ ...totals, owner_user_id: 5, email: 'owner-b@example.com', username: '' }],
    totals,
    ...overrides
  }
}

describe('OwnerSettlementPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getOwnerSettlements.mockResolvedValue(response())
  })

  it('loads this month for all owners and shows the table and window, without a pending note', async () => {
    const wrapper = mount(OwnerSettlementPanel, { props: { ownerUserId: null } })
    await flushPromises()

    expect(getOwnerSettlements).toHaveBeenCalledWith({ range: 'this_month' })
    expect(wrapper.get('[data-testid="settlement-row-5"]').text()).toContain('owner-b@example.com')
    expect(wrapper.get('[data-testid="settlement-totals"] [data-testid="settlement-cell-earnings"]').text()).toBe('$0.375000')
    expect(wrapper.get('[data-testid="finance-window"]').text()).toContain('Asia/Shanghai')
    // 4 unsplit requests and 20 s of lag are routine: no ledger note at all.
    expect(wrapper.find('[data-testid="ledger-status-note"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('"count":"4"')
    expect(wrapper.find('[data-testid="settlement-payout-5"]').exists()).toBe(false)
  })

  it('follows the page owner filter and its own range (including all and custom)', async () => {
    const wrapper = mount(OwnerSettlementPanel, { props: { ownerUserId: null } })
    await flushPromises()

    await wrapper.setProps({ ownerUserId: 5 })
    await flushPromises()
    expect(getOwnerSettlements).toHaveBeenLastCalledWith({ range: 'this_month' }, 5)

    await wrapper.get('[data-testid="finance-period-all"]').trigger('click')
    await flushPromises()
    expect(getOwnerSettlements).toHaveBeenLastCalledWith({ range: 'all' }, 5)

    await wrapper.get('[data-testid="finance-period-custom"]').trigger('click')
    await wrapper.get('[data-testid="finance-period-start"]').setValue('2026-01-01')
    await wrapper.get('[data-testid="finance-period-end"]').setValue('2026-10-30')
    await wrapper.get('[data-testid="finance-period-apply"]').trigger('click')
    await flushPromises()
    expect(getOwnerSettlements).toHaveBeenLastCalledWith({ range: 'custom', start: '2026-01-01', end: '2026-10-30' }, 5)

    await wrapper.setProps({ ownerUserId: null })
    await flushPromises()
    expect(getOwnerSettlements).toHaveBeenLastCalledWith({ range: 'custom', start: '2026-01-01', end: '2026-10-30' })
  })

  it('reports load failures through the app toast', async () => {
    getOwnerSettlements.mockRejectedValue(new Error('down'))
    mount(OwnerSettlementPanel, { props: { ownerUserId: null } })
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('down')
  })
})
