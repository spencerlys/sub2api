import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import BalanceRechargeRateSettings from '../BalanceRechargeRateSettings.vue'
import { computeRechargeRate } from '@/utils/rechargeRate'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

const refreshRechargeRate = vi.hoisted(() => vi.fn())
const showSuccess = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key}:${JSON.stringify(params)}` : key),
    }),
  }
})

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: { refreshRechargeRate },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showSuccess, showError }),
}))

type Props = InstanceType<typeof BalanceRechargeRateSettings>['$props']

function mountSettings(props: Partial<Props> = {}) {
  return mount(BalanceRechargeRateSettings, {
    props: { multiplier: 1, ...props } as Props,
  })
}

describe('computeRechargeRate', () => {
  it('mirrors the backend formula and rounding', () => {
    expect(computeRechargeRate(7.2, 'amount', 0.3)).toEqual({ effective: 7.5, multiplier: 0.133333 })
    expect(computeRechargeRate(7.2, 'percent', 5)).toEqual({ effective: 7.56, multiplier: 0.132275 })
    expect(computeRechargeRate(0, 'percent', 5)).toBeNull()
    expect(computeRechargeRate(7.2, 'amount', 6)).toBeNull()
    expect(computeRechargeRate(7.2, 'percent', 101)).toBeNull()
  })
})

describe('BalanceRechargeRateSettings', () => {
  beforeEach(() => {
    refreshRechargeRate.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
  })

  it('keeps the manual multiplier editable while auto is off', async () => {
    const wrapper = mountSettings({ multiplier: 0.14, autoEnabled: false })
    expect(wrapper.find('[data-testid="recharge-rate-markup-mode"]').exists()).toBe(false)

    const input = wrapper.get('[data-testid="recharge-rate-multiplier-input"]')
    expect(input.attributes('disabled')).toBeUndefined()
    expect((input.element as HTMLInputElement).value).toBe('0.14')

    await input.setValue('0.1389')
    expect(wrapper.emitted('update:multiplier')?.at(-1)).toEqual([0.1389])

    await wrapper.get('[data-testid="recharge-rate-auto-toggle"]').trigger('click')
    expect(wrapper.emitted('update:autoEnabled')?.at(-1)).toEqual([true])
  })

  it('disables the manual field and shows the computed value while auto is on', () => {
    const wrapper = mountSettings({
      multiplier: 1,
      autoEnabled: true,
      markupMode: 'amount',
      markupValue: 0.3,
      market: 7.2,
      fetchedAt: '2026-10-24T08:00:00Z',
      source: 'frankfurter',
    })
    const input = wrapper.get('[data-testid="recharge-rate-multiplier-input"]')
    expect(input.attributes('disabled')).toBeDefined()
    expect((input.element as HTMLInputElement).value).toBe('0.133333')

    const preview = wrapper.get('[data-testid="recharge-rate-preview"]').text()
    expect(preview).toContain('admin.settings.payment.rechargeRate.perCny:{"usd":"0.1333"}')
    expect(preview).toContain('admin.settings.payment.rechargeRate.perUsd:{"cny":"7.50"}')
    expect(wrapper.get('[data-testid="recharge-rate-market"]').text()).toContain('"cny":"7.2000"')
    expect(wrapper.get('[data-testid="recharge-rate-market"]').text()).toContain('frankfurter')
    expect(wrapper.find('[data-testid="recharge-rate-fetched-at"]').exists()).toBe(true)
  })

  it('emits markup mode and value changes', async () => {
    const wrapper = mountSettings({ autoEnabled: true, markupMode: 'percent', markupValue: 0, market: 7.2 })
    await wrapper.get('[data-testid="recharge-rate-markup-mode"]').setValue('amount')
    expect(wrapper.emitted('update:markupMode')?.at(-1)).toEqual(['amount'])

    await wrapper.get('[data-testid="recharge-rate-markup-value"]').setValue('5')
    expect(wrapper.emitted('update:markupValue')?.at(-1)).toEqual([5])

    // percent mode preview: 7.20 × 1.05 = 7.56 → 0.132275
    await wrapper.setProps({ markupValue: 5 })
    expect((wrapper.get('[data-testid="recharge-rate-multiplier-input"]').element as HTMLInputElement).value).toBe('0.132275')
  })

  it('refresh button calls the endpoint and emits the new state', async () => {
    const state = {
      auto_enabled: true,
      markup_mode: 'amount',
      markup_value: 0.3,
      market_rate: 7.2,
      fetched_at: '2026-10-24T08:00:00Z',
      source: 'frankfurter',
      effective_rate: 7.5,
      computed_multiplier: 0.133333,
      balance_recharge_multiplier: 0.133333,
    }
    refreshRechargeRate.mockResolvedValue({ data: state })
    const wrapper = mountSettings({ autoEnabled: true })

    await wrapper.get('[data-testid="recharge-rate-refresh"]').trigger('click')
    await flushPromises()

    expect(refreshRechargeRate).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('refreshed')?.[0]).toEqual([state])
    expect(showSuccess).toHaveBeenCalledWith('admin.settings.payment.rechargeRate.refreshSuccess')
  })

  it('refresh failure shows an error and emits nothing', async () => {
    refreshRechargeRate.mockRejectedValue({ message: 'failed to fetch the USD/CNY market rate' })
    const wrapper = mountSettings()

    await wrapper.get('[data-testid="recharge-rate-refresh"]').trigger('click')
    await flushPromises()

    expect(wrapper.emitted('refreshed')).toBeUndefined()
    expect(showError).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="recharge-rate-refresh"]').attributes('disabled')).toBeUndefined()
  })

  it('has zh and en strings for the controls', () => {
    const zhRate = zh.admin.settings.payment.rechargeRate
    const enRate = en.admin.settings.payment.rechargeRate
    expect(zhRate.markupModeAmount).toBe('固定加价 ¥')
    expect(zhRate.markupModePercent).toBe('百分比加价 %')
    expect(zhRate.refresh).toBe('立即更新汇率')
    expect(Object.keys(enRate).sort()).toEqual(Object.keys(zhRate).sort())
  })
})
