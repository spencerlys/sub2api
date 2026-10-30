import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

import FinancePeriodPicker from '../FinancePeriodPicker.vue'
import type { FinanceRange } from '@/api/admin/finance'

enableAutoUnmount(afterEach)

function mountPicker(modelValue: FinanceRange = { range: 'this_month' }) {
  return mount(FinancePeriodPicker, { props: { modelValue } })
}

describe('FinancePeriodPicker', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 9, 30, 12))
  })
  afterEach(() => vi.useRealTimers())

  it('marks the current preset and emits just the preset name for every fixed preset', async () => {
    const wrapper = mountPicker()
    expect(wrapper.get('[data-testid="finance-period-this_month"]').attributes('aria-selected')).toBe('true')
    expect(wrapper.find('[data-testid="finance-period-start"]').exists()).toBe(false)

    for (const preset of ['today', 'yesterday', 'this_month', 'last_month', 'last_7d', 'last_30d', 'all']) {
      await wrapper.get(`[data-testid="finance-period-${preset}"]`).trigger('click')
      expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([{ range: preset }])
    }
  })

  it('follows modelValue changes from the parent', async () => {
    const wrapper = mountPicker()
    await wrapper.setProps({ modelValue: { range: 'all' } })
    expect(wrapper.get('[data-testid="finance-period-all"]').attributes('aria-selected')).toBe('true')
    expect(wrapper.get('[data-testid="finance-period-this_month"]').attributes('aria-selected')).toBe('false')
  })

  it('emits a custom range only when applied, prefilled with this month', async () => {
    const wrapper = mountPicker()
    await wrapper.get('[data-testid="finance-period-custom"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect((wrapper.get('[data-testid="finance-period-start"]').element as HTMLInputElement).value).toBe('2026-10-01')
    expect((wrapper.get('[data-testid="finance-period-end"]').element as HTMLInputElement).value).toBe('2026-10-30')

    // Longer than a year is fine: the backend no longer caps the range.
    await wrapper.get('[data-testid="finance-period-start"]').setValue('2024-08-15')
    await wrapper.get('[data-testid="finance-period-end"]').setValue('2026-09-14')
    await wrapper.get('[data-testid="finance-period-apply"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([
      { range: 'custom', start: '2024-08-15', end: '2026-09-14' }
    ])
  })

  it('opens on custom with the given dates', () => {
    const wrapper = mountPicker({ range: 'custom', start: '2026-05-01', end: '2026-05-31' })
    expect(wrapper.get('[data-testid="finance-period-custom"]').attributes('aria-selected')).toBe('true')
    expect((wrapper.get('[data-testid="finance-period-start"]').element as HTMLInputElement).value).toBe('2026-05-01')
    expect((wrapper.get('[data-testid="finance-period-end"]').element as HTMLInputElement).value).toBe('2026-05-31')
  })

  it('rejects an inverted custom range', async () => {
    const wrapper = mountPicker()
    await wrapper.get('[data-testid="finance-period-custom"]').trigger('click')
    await wrapper.get('[data-testid="finance-period-start"]').setValue('2026-10-20')
    await wrapper.get('[data-testid="finance-period-end"]').setValue('2026-10-01')

    expect(wrapper.text()).toContain('admin.finance.period.invalidRange')
    expect(wrapper.get('[data-testid="finance-period-apply"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="finance-period-apply"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
})
