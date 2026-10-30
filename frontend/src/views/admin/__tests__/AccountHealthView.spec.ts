import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AccountHealthItem, AccountHealthResponse } from '@/api/admin/accountHealth'

const { getAccountHealth, showError, listUsers, getMyOwnerSettlement, authState } = vi.hoisted(() => ({
  getAccountHealth: vi.fn(),
  showError: vi.fn(),
  listUsers: vi.fn(),
  getMyOwnerSettlement: vi.fn(),
  authState: { isOwner: false, isAdmin: true }
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accountHealth: { getAccountHealth },
    users: { list: listUsers },
    finance: { getMyOwnerSettlement }
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess: vi.fn() })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

import AccountHealthView from '../AccountHealthView.vue'

function healthItem(overrides: Partial<AccountHealthItem> = {}): AccountHealthItem {
  return {
    id: 1,
    name: 'claude-a',
    platform: 'anthropic',
    type: 'oauth',
    status: 'active',
    schedulable: true,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    rate_limited: false,
    overloaded: false,
    temp_unschedulable: false,
    error_message: '',
    requests_ok: 100,
    errors_total: 0,
    errors: { rate_limit: 0, overloaded: 0, auth: 0, upstream_5xx: 0, timeout: 0, other: 0 },
    error_rate: 0,
    ttft_p50_ms: 850,
    ttft_p95_ms: 2400,
    duration_p50_ms: 12000,
    tokens_in: 120000,
    tokens_out: 3400,
    tokens_cache: 5000,
    cost: 1.25,
    last_success_at: '2026-10-30T00:00:00Z',
    last_error_at: null,
    last_error_category: '',
    last_error_status: null,
    ...overrides
  }
}

function response(items: AccountHealthItem[], range: AccountHealthResponse['range'] = '24h'): AccountHealthResponse {
  return { range, start_time: '2026-10-29T00:00:00Z', end_time: '2026-10-30T00:00:00Z', items }
}

const wrappers: ReturnType<typeof mount>[] = []

function mountView() {
  const wrapper = mount(AccountHealthView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' },
        'router-link': { props: ['to'], template: '<a :data-to="to"><slot /></a>' }
      }
    }
  })
  wrappers.push(wrapper)
  return wrapper
}

describe('admin AccountHealthView', () => {
  beforeEach(() => {
    getAccountHealth.mockReset()
    showError.mockReset()
    listUsers.mockReset()
    listUsers.mockResolvedValue({ items: [{ id: 4, email: 'owner@example.com', username: 'own' }], total: 1 })
    getMyOwnerSettlement.mockReset()
    getMyOwnerSettlement.mockResolvedValue({
      start_time: '',
      end_time: '',
      requests: 5,
      official_cost: 9,
      earnings: 3,
      earnings_all_time: 6,
      paid_total: 2,
      unpaid: 4
    })
    authState.isOwner = false
    authState.isAdmin = true
  })

  afterEach(() => {
    wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  })

  it('loads the default 24h range and renders a row per account', async () => {
    getAccountHealth.mockResolvedValue(
      response([healthItem({ id: 1, name: 'claude-a' }), healthItem({ id: 2, name: 'claude-b' })])
    )
    const wrapper = mountView()
    await flushPromises()

    expect(getAccountHealth).toHaveBeenCalledTimes(1)
    expect(getAccountHealth).toHaveBeenCalledWith('24h')
    expect(wrapper.text()).toContain('claude-a')
    expect(wrapper.text()).toContain('claude-b')
    expect(wrapper.find('[data-testid="account-health-badge-1"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="account-health-badge-2"]').exists()).toBe(true)
    expect(wrapper.find('a[data-to="/admin/accounts"]').text()).toBe('claude-a')
  })

  it('derives each health badge from the shared thresholds', async () => {
    getAccountHealth.mockResolvedValue(
      response([
        healthItem({ id: 1 }),
        healthItem({ id: 2, requests_ok: 90, errors_total: 10, error_rate: 0.1 }),
        healthItem({
          id: 3,
          requests_ok: 70,
          errors_total: 30,
          error_rate: 0.3,
          errors: { rate_limit: 0, overloaded: 0, auth: 25, upstream_5xx: 5, timeout: 0, other: 0 }
        }),
        healthItem({ id: 4, rate_limited: true, rate_limit_reset_at: '2026-10-30T01:00:00Z' }),
        healthItem({ id: 5, status: 'error', error_message: 'token revoked' }),
        healthItem({ id: 6, schedulable: false }),
        healthItem({ id: 7, requests_ok: 0, errors_total: 0, error_rate: 0 })
      ])
    )
    const wrapper = mountView()
    await flushPromises()

    const health = (id: number) => wrapper.find(`[data-testid="account-health-badge-${id}"]`)
    expect(health(1).attributes('data-health')).toBe('healthy')
    expect(health(2).attributes('data-health')).toBe('degraded')
    expect(health(3).attributes('data-health')).toBe('failing')
    expect(health(4).attributes('data-health')).toBe('rate_limited')
    expect(health(5).attributes('data-health')).toBe('failing')
    expect(health(6).attributes('data-health')).toBe('disabled')
    expect(health(7).attributes('data-health')).toBe('idle')

    expect(health(1).text()).toBe('admin.accountHealth.status.healthy')
    expect(health(5).attributes('title')).toContain('token revoked')
    expect(health(4).attributes('title')).toContain('admin.accountHealth.rateLimitedUntil')

    const errorRate = wrapper.find('[data-testid="account-health-error-rate-3"]')
    expect(errorRate.text()).toContain('30%')
    const breakdown = errorRate.attributes('title') ?? ''
    expect(breakdown).toContain('admin.accountHealth.errorCategory.auth: 25')
    expect(breakdown).toContain('admin.accountHealth.errorCategory.upstream_5xx: 5')
    expect(breakdown).not.toContain('admin.accountHealth.errorCategory.timeout')
  })

  it('refetches with the new range when the range changes', async () => {
    getAccountHealth.mockResolvedValue(response([healthItem()]))
    const wrapper = mountView()
    await flushPromises()

    getAccountHealth.mockResolvedValue(response([healthItem({ name: 'only-7d' })], '7d'))
    await wrapper.find('[data-testid="account-health-range-7d"]').trigger('click')
    await flushPromises()

    expect(getAccountHealth).toHaveBeenCalledTimes(2)
    expect(getAccountHealth).toHaveBeenLastCalledWith('7d')
    expect(wrapper.text()).toContain('only-7d')

    // Re-selecting the active range does not refetch; the refresh button does.
    await wrapper.find('[data-testid="account-health-range-7d"]').trigger('click')
    await wrapper.find('[data-testid="account-health-refresh"]').trigger('click')
    await flushPromises()
    expect(getAccountHealth).toHaveBeenCalledTimes(3)
    expect(getAccountHealth).toHaveBeenLastCalledWith('7d')
  })

  it('shows the empty state when there are no accounts', async () => {
    getAccountHealth.mockResolvedValue(response([]))
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.accountHealth.emptyTitle')
  })

  it('reports load failures through the app toast', async () => {
    getAccountHealth.mockRejectedValue(new Error('boom'))
    mountView()
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('boom')
  })

  it('lets admins filter by owner and shows totals for the visible rows', async () => {
    getAccountHealth.mockResolvedValue(
      response([
        healthItem({ id: 1, requests_ok: 100, errors_total: 2, tokens_in: 1000, tokens_out: 50, tokens_cache: 7, cost: 1.25 }),
        healthItem({ id: 2, requests_ok: 30, errors_total: 3, tokens_in: 500, tokens_out: 25, tokens_cache: 3, cost: 0.75 })
      ])
    )
    const wrapper = mountView()
    await flushPromises()

    expect(listUsers).toHaveBeenCalledWith(1, 1000, { role: 'owner' })
    expect(wrapper.find('[data-testid="owner-earnings-card"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="account-health-total-requests_ok"]').text()).toContain('130')
    expect(wrapper.get('[data-testid="account-health-total-errors_total"]').text()).toContain('5')
    expect(wrapper.get('[data-testid="account-health-total-tokens_in"]').text()).toContain('1.5K')
    expect(wrapper.get('[data-testid="account-health-total-tokens_cache"]').text()).toContain('10')
    expect(wrapper.get('[data-testid="account-health-total-cost"]').text()).toContain('2.00')

    await wrapper.get('[data-testid="owner-select"]').setValue('4')
    await flushPromises()
    expect(getAccountHealth).toHaveBeenLastCalledWith('24h', 4)

    await wrapper.get('[data-testid="account-health-range-7d"]').trigger('click')
    await flushPromises()
    expect(getAccountHealth).toHaveBeenLastCalledWith('7d', 4)

    await wrapper.get('[data-testid="owner-select"]').setValue('')
    await flushPromises()
    expect(getAccountHealth).toHaveBeenLastCalledWith('7d')
  })

  it('shows owners their earnings card instead of the owner filter and totals', async () => {
    authState.isOwner = true
    authState.isAdmin = false
    getAccountHealth.mockResolvedValue(response([healthItem()]))
    const wrapper = mountView()
    await flushPromises()

    expect(getAccountHealth).toHaveBeenCalledWith('24h')
    expect(getMyOwnerSettlement).toHaveBeenCalledTimes(1)
    expect(listUsers).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="owner-earnings-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="owner-select"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="account-health-totals"]').exists()).toBe(false)
  })
})
