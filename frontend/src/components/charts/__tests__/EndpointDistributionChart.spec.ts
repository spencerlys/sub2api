import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import EndpointDistributionChart from '../EndpointDistributionChart.vue'

const messages: Record<string, string> = {
  'usage.endpoint': 'Endpoint',
  'admin.dashboard.requests': 'Requests',
  'admin.dashboard.tokens': 'Tokens',
  'admin.dashboard.actual': 'Actual',
  'admin.dashboard.standard': 'Standard',
  'admin.dashboard.metricTokens': 'By Tokens',
  'admin.dashboard.metricActualCost': 'By Actual Cost',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

vi.mock('vue-chartjs', () => ({
  Doughnut: {
    props: ['data'],
    template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>',
  },
}))

const endpointStats = [
  { endpoint: '/v1/messages', requests: 3, total_tokens: 100, cost: 0.25, actual_cost: 0.123456 },
  { endpoint: '/v1/chat/completions', requests: 1, total_tokens: 900, cost: 0.5, actual_cost: 0.654321 },
]

describe('EndpointDistributionChart', () => {
  it('shows the user-billed (actual) column by default', () => {
    const wrapper = mount(EndpointDistributionChart, {
      props: { endpointStats, enableBreakdown: false },
    })
    expect(wrapper.text()).toContain('Actual')
    expect(wrapper.text()).toContain('$0.123')
  })

  it('hides user-billed amounts and ignores the actual-cost metric when showActualCost is false', () => {
    const wrapper = mount(EndpointDistributionChart, {
      props: { endpointStats, enableBreakdown: false, showActualCost: false, metric: 'actual_cost', showMetricToggle: true },
    })
    const text = wrapper.text()
    expect(text).not.toContain('Actual')
    expect(text).not.toContain('$0.123')
    expect(text).not.toContain('$0.654')
    expect(text).not.toContain('By Actual Cost')
    expect(text).toContain('Standard')
    // chart falls back to token metric, never plotting billed amounts
    const data = JSON.parse(wrapper.get('.chart-data').text())
    expect(data.datasets[0].data).toEqual([900, 100])
  })
})
