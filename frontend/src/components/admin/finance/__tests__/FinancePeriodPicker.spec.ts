import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

import FinancePeriodPicker from '../FinancePeriodPicker.vue'

enableAutoUnmount(afterEach)

function mountPicker() {
  return mount(FinancePeriodPicker, {
    props: { modelValue: { start: '2026-10-01', end: '2026-10-30' } }
  })
}

describe('FinancePeriodPicker', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 9, 30, 12))
  })
  afterEach(() => vi.useRealTimers())

  it('starts on this month and emits last month when picked', async () => {
    const wrapper = mountPicker()
    expect(wrapper.get('[data-testid="finance-period-thisMonth"]').attributes('aria-selected')).toBe('true')
    expect(wrapper.find('[data-testid="finance-period-start"]').exists()).toBe(false)

    await wrapper.get('[data-testid="finance-period-lastMonth"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([{ start: '2026-09-01', end: '2026-09-30' }])

    await wrapper.get('[data-testid="finance-period-thisMonth"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([{ start: '2026-10-01', end: '2026-10-30' }])
  })

  it('emits a valid custom range only when applied', async () => {
    const wrapper = mountPicker()
    await wrapper.get('[data-testid="finance-period-custom"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()

    await wrapper.get('[data-testid="finance-period-start"]').setValue('2026-08-15')
    await wrapper.get('[data-testid="finance-period-end"]').setValue('2026-09-14')
    await wrapper.get('[data-testid="finance-period-apply"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([{ start: '2026-08-15', end: '2026-09-14' }])
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
