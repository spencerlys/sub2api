import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key)
    })
  }
})

import OwnerSettlementTable from '../OwnerSettlementTable.vue'
import enFinance from '@/i18n/locales/en/admin/finance'
import zhFinance from '@/i18n/locales/zh/admin/finance'
import type { OwnerSettlementRow, OwnerSettlementTotals } from '@/api/admin/finance'

enableAutoUnmount(afterEach)

const totals: OwnerSettlementTotals = {
  active_accounts: 3,
  requests: 900,
  official_cost: 200,
  user_billed: 150,
  paid_cost: 100,
  gift_cost: 50,
  account_cost: 130,
  payout_base: 120,
  earnings: 80,
  gift_account_cost: 40,
  pending_requests: 0,
  pending_account_cost: 0,
  earnings_all_time: 400,
  paid_total: 300,
  unpaid: 100
}

function row(overrides: Partial<OwnerSettlementRow> = {}): OwnerSettlementRow {
  return { ...totals, owner_user_id: 3, email: 'owner-a@example.com', username: 'alice', ...overrides }
}

const COLUMNS = [
  'active_accounts',
  'requests',
  'official_cost',
  'user_billed',
  'paid_cost',
  'gift_cost',
  'payout_base',
  'gift_account_cost',
  'earnings',
  'earnings_all_time',
  'paid_total',
  'unpaid'
]

describe('OwnerSettlementTable', () => {
  it('orders columns: usage, billed to users, payout basis − internal use = payable, settlement', () => {
    const wrapper = mount(OwnerSettlementTable, { props: { items: [row()], totals } })
    const order = wrapper
      .findAll('[data-testid^="settlement-col-"]')
      .map((h) => h.attributes('data-testid')!.replace('settlement-col-', ''))
    expect(order).toEqual(COLUMNS)
  })

  it('shows every settlement column with precise amounts and a totals row', () => {
    const wrapper = mount(OwnerSettlementTable, {
      props: { items: [row({ official_cost: 0.012345, earnings: 0.5 })], totals }
    })

    for (const key of COLUMNS) expect(wrapper.find(`[data-testid="settlement-col-${key}"]`).exists()).toBe(true)

    const r = wrapper.get('[data-testid="settlement-row-3"]')
    expect(r.text()).toContain('owner-a@example.com')
    expect(r.text()).toContain('alice')
    expect(r.get('[data-testid="settlement-cell-active_accounts"]').text()).toBe('3')
    expect(r.get('[data-testid="settlement-cell-official_cost"]').text()).toBe('$0.012345')
    expect(r.get('[data-testid="settlement-cell-user_billed"]').text()).toBe('$150.00')
    expect(r.get('[data-testid="settlement-cell-paid_cost"]').text()).toBe('$100.00')
    expect(r.get('[data-testid="settlement-cell-gift_cost"]').text()).toBe('$50.0000')
    expect(r.get('[data-testid="settlement-cell-earnings"]').text()).toBe('$0.500000')
    expect(r.get('[data-testid="settlement-cell-unpaid"]').text()).toBe('$100.00')

    const foot = wrapper.get('[data-testid="settlement-totals"]')
    expect(foot.text()).toContain('admin.finance.settlement.totals')
    expect(foot.get('[data-testid="settlement-cell-requests"]').text()).toBe('900')
    expect(foot.get('[data-testid="settlement-cell-earnings"]').text()).toBe('$80.0000')
  })

  it('shows internal-use cost as a plain column and never folds it into payable', () => {
    const wrapper = mount(OwnerSettlementTable, { props: { items: [row()], totals } })

    const header = wrapper.get('[data-testid="settlement-col-gift_account_cost"]')
    expect(header.text()).toBe('admin.finance.settlement.giftAccountCost')
    expect(header.attributes('data-reference')).toBeUndefined()

    const cell = wrapper.get('[data-testid="settlement-row-3"] [data-testid="settlement-cell-gift_account_cost"]')
    expect(cell.text()).toBe('$40.0000')
    expect(cell.classes()).not.toContain('italic')

    // Payable is exactly `earnings` (80), not earnings + gift_account_cost (120).
    const payable = wrapper.get('[data-testid="settlement-totals"] [data-testid="settlement-cell-earnings"]')
    expect(payable.text()).toBe('$80.0000')
    expect(payable.text()).not.toContain('120')
  })

  it('shows the payout basis instead of account cost, and basis − internal use = payable', () => {
    const wrapper = mount(OwnerSettlementTable, { props: { items: [row()], totals } })
    expect(wrapper.find('[data-testid="settlement-col-account_cost"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('$130')

    const header = wrapper.get('[data-testid="settlement-col-payout_base"]')
    expect(header.text()).toBe('admin.finance.settlement.payoutBase')
    expect(header.attributes('title')).toBe('admin.finance.settlement.hints.payoutBase')

    const r = wrapper.get('[data-testid="settlement-row-3"]')
    const basis = r.get('[data-testid="settlement-cell-payout_base"]').text()
    expect(basis).toBe('$120.00')
    const parse = (s: string) => Number(s.replace(/[$,]/g, ''))
    const gift = parse(r.get('[data-testid="settlement-cell-gift_account_cost"]').text())
    const payable = parse(r.get('[data-testid="settlement-cell-earnings"]').text())
    expect(parse(basis) - gift).toBeCloseTo(payable, 9)
    const foot = wrapper.get('[data-testid="settlement-totals"]')
    expect(foot.get('[data-testid="settlement-cell-payout_base"]').text()).toBe('$120.00')
  })

  it('has bilingual payout-basis copy', () => {
    expect(zhFinance.finance.settlement.payoutBase).toBe('分成基数')
    expect(zhFinance.finance.settlement.hints.payoutBase).toBe('官方价 × 号主分成倍率（按请求时的分组倍率）')
    expect(enFinance.finance.settlement.payoutBase).toBe('Payout basis')
    expect(enFinance.finance.settlement.hints.payoutBase).toContain('owner payout rate')
  })

  it('never shows routine pending requests under the request count', () => {
    const wrapper = mount(OwnerSettlementTable, { props: { items: [row({ pending_requests: 12 })], totals } })
    expect(wrapper.find('[data-testid="settlement-pending"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="settlement-row-3"] [data-testid="settlement-cell-requests"]').text()).not.toContain('12')
  })

  it('emits record-payout only when the action column is enabled', async () => {
    const plain = mount(OwnerSettlementTable, { props: { items: [row()], totals } })
    expect(plain.find('[data-testid="settlement-payout-3"]').exists()).toBe(false)

    const withAction = mount(OwnerSettlementTable, { props: { items: [row()], totals, payoutAction: true } })
    await withAction.get('[data-testid="settlement-payout-3"]').trigger('click')
    expect(withAction.emitted('record-payout')?.[0]?.[0]).toMatchObject({ owner_user_id: 3, email: 'owner-a@example.com' })
  })

  it('shows the empty state without a totals row', () => {
    const wrapper = mount(OwnerSettlementTable, { props: { items: [], totals } })
    expect(wrapper.text()).toContain('admin.finance.settlement.empty')
    expect(wrapper.find('[data-testid="settlement-totals"]').exists()).toBe(false)
  })
  it('gives phones one card per owner with payable and unpaid up front, the rest under details', async () => {
    const wrapper = mount(OwnerSettlementTable, { props: { items: [row({ earnings: 0.5 })], totals, payoutAction: true } })
    expect(wrapper.get('[data-testid="settlement-cards"]').classes()).toContain('md:hidden')
    expect(wrapper.get('table').element.parentElement!.classList).toContain('md:block')

    const card = wrapper.get('[data-testid="settlement-card-3"]')
    expect(card.text()).toContain('owner-a@example.com')
    const shown = card.findAll(':scope > dl dd').map((d) => d.text())
    expect(shown).toEqual(['$0.500000', '$100.00'])
    const details = card.get('details')
    for (const key of COLUMNS) expect(details.text()).toContain(`admin.finance.settlement.${key === 'gift_account_cost' ? 'giftAccountCost' : key.replace(/_([a-z])/g, (_, c) => c.toUpperCase())}`)

    await card.get('[data-testid="settlement-card-payout-3"]').trigger('click')
    expect(wrapper.emitted('record-payout')?.[0]?.[0]).toMatchObject({ owner_user_id: 3 })
    expect(wrapper.get('[data-testid="settlement-card-totals"]').text()).toContain('$80.0000')
  })

  it('has bilingual details label', () => {
    expect(zhFinance.finance.settlement.details).toBe('明细')
    expect(enFinance.finance.settlement.details).toBe('Details')
  })
})
