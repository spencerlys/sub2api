import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AccountHealthItem, AccountHealthResponse } from '@/api/admin/accountHealth'

const { getAccountHealth, showError } = vi.hoisted(() => ({
  getAccountHealth: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accountHealth: { getAccountHealth }
  }
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
})
