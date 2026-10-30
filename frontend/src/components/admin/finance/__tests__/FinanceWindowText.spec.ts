import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

import FinanceWindowText from '../FinanceWindowText.vue'

enableAutoUnmount(afterEach)

const text = (window: Record<string, string> | null) =>
  mount(FinanceWindowText, { props: { window } }).text()

describe('FinanceWindowText', () => {
  it('shows whole-day windows as inclusive dates in the server timezone', () => {
    // [2026-10-01 00:00, 2026-11-01 00:00) Asia/Shanghai
    expect(
      text({ range: 'this_month', timezone: 'Asia/Shanghai', start_time: '2026-09-30T16:00:00Z', end_time: '2026-10-31T16:00:00Z' })
    ).toBe('2026-10-01 - 2026-10-31 (Asia/Shanghai)')
    expect(
      text({ range: 'today', timezone: 'Asia/Shanghai', start_time: '2026-10-29T16:00:00Z', end_time: '2026-10-30T16:00:00Z' })
    ).toBe('2026-10-30 - 2026-10-30 (Asia/Shanghai)')
  })

  it('shows minutes when a boundary is not at midnight', () => {
    expect(
      text({ range: 'today', timezone: 'Asia/Shanghai', start_time: '2026-10-29T16:00:00Z', end_time: '2026-10-30T07:05:00Z' })
    ).toBe('2026-10-30 00:00 - 2026-10-30 15:05 (Asia/Shanghai)')
  })

  it('shows all time without the epoch start', () => {
    expect(
      text({ range: 'all', timezone: 'Asia/Shanghai', start_time: '1970-01-01T00:00:00Z', end_time: '2026-10-30T07:05:00Z' })
    ).toBe('admin.finance.period.allTime (Asia/Shanghai)')
  })

  it('renders nothing without a window', () => {
    expect(text(null)).toBe('')
    expect(text({ range: 'today', timezone: 'Asia/Shanghai', start_time: '', end_time: '' })).toBe('')
  })
})
