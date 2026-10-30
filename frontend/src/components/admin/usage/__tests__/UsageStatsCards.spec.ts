import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UsageStatsCards from '../UsageStatsCards.vue'

const messages: Record<string, string> = {
  'usage.totalRequests': 'Total Requests',
  'usage.inSelectedRange': 'in selected range',
  'usage.totalTokens': 'Total Tokens',
  'usage.in': 'In',
  'usage.out': 'Out',
  'usage.cacheTotal': 'Cache',
  'usage.cacheBreakdown': 'Cache Token Breakdown',
  'usage.cacheCreationTokensLabel': 'Cache Creation',
  'usage.cacheReadTokensLabel': 'Cache Read',
  'usage.totalCost': 'Total Cost',
  'usage.accountCost': 'Cost',
  'usage.standardCost': 'Standard',
  'usage.avgDuration': 'Avg Duration',
  'usage.officialCost': 'Official price',
  'usage.discountRate': 'Discount rate',
  'admin.usage.receivable': 'Receivable',
  'admin.usage.paidPortion': 'Paid use',
  'admin.usage.giftPortion': 'Internal use',
  'admin.usage.accountCostTotal': 'Account Cost',
  'admin.usage.ownerPayable': 'Owner payable',
  'admin.usage.giftAccountCostRef': 'Internal-use cost',
  'admin.usage.officialCostTotal': 'Official-price Cost',
  'admin.usage.totalEarnings': 'My Earnings',
  'admin.usage.earningsGiftExcluded': 'Internal use excluded',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        (messages[key] ?? key).replace(/\{(\w+)\}/g, (_, name) => String(params?.[name] ?? `{${name}}`)),
    }),
  }
})

const stats = {
  total_requests: 1,
  total_input_tokens: 100,
  total_output_tokens: 50,
  total_cache_tokens: 34,
  total_cache_creation_tokens: 12,
  total_cache_read_tokens: 22,
  total_tokens: 184,
  total_cost: 0.001,
  total_actual_cost: 0.001,
  total_account_cost: 0.001,
  average_duration_ms: 250,
}

describe('UsageStatsCards', () => {
  it('shows cache token breakdown values', () => {
    const wrapper = mount(UsageStatsCards, {
      props: {
        stats,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('Cache: 34')
    expect(text).toContain('Cache Token Breakdown')
    expect(text).toContain('Cache Creation')
    expect(text).toContain('12')
    expect(text).toContain('Cache Read')
    expect(text).toContain('22')
  })

  it('keeps the cache tooltip out of the layout while it is hidden', () => {
    const wrapper = mount(UsageStatsCards, {
      props: {
        stats,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    const tooltip = wrapper.findAll('span').find((el) => el.classes().includes('group-hover:block'))

    expect(tooltip).toBeDefined()
    // `opacity-0` hides the tooltip visually but keeps it in the layout, so its
    // fixed width still widens the document and causes horizontal scrolling on
    // narrow screens. `hidden` (display: none) takes it out of the flow.
    expect(tooltip?.classes()).toContain('hidden')
    expect(tooltip?.classes()).not.toContain('opacity-0')
  })

  const mountCards = (props: Record<string, unknown>) =>
    mount(UsageStatsCards, {
      props: props as any,
      global: { stubs: { Icon: true } },
    })

  const adminStats = {
    ...stats,
    total_requests: 12,
    total_cost: 2.5,
    total_actual_cost: 2,
    total_account_cost: 1.234567,
    total_paid_cost: 1.5,
    total_gift_cost: 0.5,
    total_earnings: 0.876543,
    total_gift_account_cost: 0.123456,
    pending_requests: 3,
  }

  it('admin variant shows official cost, discount, receivable split, account cost and owner payable', () => {
    const wrapper = mountCards({ stats: adminStats, variant: 'admin' })

    expect(wrapper.get('[data-testid="stats-receivable"]').text()).toBe('$2.0000')
    expect(wrapper.get('[data-testid="stats-official-cost"]').text()).toBe('$2.5000')
    // discount = total_actual_cost / total_cost
    expect(wrapper.get('[data-testid="stats-discount"]').text()).toBe('0.80x')
    expect(wrapper.get('[data-testid="stats-paid"]').text()).toBe('$1.5000')
    expect(wrapper.get('[data-testid="stats-gift"]').text()).toBe('$0.5000')
    expect(wrapper.get('[data-testid="stats-account-cost"]').text()).toBe('$1.2346')
    expect(wrapper.get('[data-testid="stats-owner-payable"]').text()).toBe('$0.8765')
    expect(wrapper.get('[data-testid="stats-gift-account-cost"]').text()).toBe('$0.1235')
    // Routine unsplit requests (pending_requests: 3) are never surfaced.
    expect(wrapper.find('[data-testid="stats-pending"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('pending')
    expect(wrapper.find('[data-testid="stats-earnings"]').exists()).toBe(false)
  })

  it('admin variant shows no discount when the official cost is zero', () => {
    const wrapper = mountCards({
      stats: { ...adminStats, total_cost: 0, total_actual_cost: 0, pending_requests: 0 },
      variant: 'admin',
    })
    expect(wrapper.get('[data-testid="stats-discount"]').text()).toBe('-')
    expect(wrapper.find('[data-testid="stats-pending"]').exists()).toBe(false)
  })

  it('owner variant shows official cost and earnings, never billed amounts or multipliers', () => {
    // even if the backend leaks admin-only totals, the owner cards must not render them
    const wrapper = mountCards({ stats: adminStats, variant: 'owner' })

    expect(wrapper.get('[data-testid="stats-official-cost"]').text()).toBe('$2.5000')
    expect(wrapper.get('[data-testid="stats-earnings"]').text()).toBe('$0.8765')
    expect(wrapper.find('[data-testid="stats-pending"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Internal use excluded')

    for (const id of ['stats-receivable', 'stats-discount', 'stats-paid', 'stats-gift', 'stats-account-cost', 'stats-owner-payable', 'stats-gift-account-cost']) {
      expect(wrapper.find(`[data-testid="${id}"]`).exists()).toBe(false)
    }
    const text = wrapper.text()
    expect(text).not.toContain('$2.0000') // total_actual_cost
    expect(text).not.toContain('$1.2346') // total_account_cost
    expect(text).not.toContain('$1.5000') // paid
    expect(text).not.toContain('Receivable')
    expect(text).not.toContain('Total Cost')
    expect(text).not.toContain('Discount rate')
  })

  it('owner variant falls back to zero earnings when the field is missing', () => {
    const wrapper = mountCards({
      stats: { total_requests: 1, total_tokens: 1, total_cost: 0.5, average_duration_ms: 1 },
      variant: 'owner',
    })
    expect(wrapper.get('[data-testid="stats-earnings"]').text()).toBe('$0.0000')
  })

  it('user variant shows charged total with official price and discount rate only', () => {
    const wrapper = mountCards({
      stats: { ...stats, total_cost: 1, total_actual_cost: 0.75 },
      variant: 'user',
    })
    expect(wrapper.get('[data-testid="stats-charged"]').text()).toBe('$0.7500')
    expect(wrapper.get('[data-testid="stats-official-cost"]').text()).toBe('$1.0000')
    expect(wrapper.get('[data-testid="stats-discount"]').text()).toBe('0.75x')
    for (const id of ['stats-paid', 'stats-gift', 'stats-account-cost', 'stats-owner-payable', 'stats-earnings', 'stats-pending']) {
      expect(wrapper.find(`[data-testid="${id}"]`).exists()).toBe(false)
    }
  })
})
