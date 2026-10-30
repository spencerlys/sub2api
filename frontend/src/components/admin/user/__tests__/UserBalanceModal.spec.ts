import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import UserBalanceModal from '../UserBalanceModal.vue'
import type { AdminUser } from '@/types'
const mocks = vi.hoisted(() => ({ updateBalance: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: mocks } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); vi.spyOn(console, 'error').mockImplementation(() => {}) })
afterEach(() => vi.restoreAllMocks())
describe('balance operation error feedback', () => {
  it.each([
    [{ message: 'User balance changed; retry' }, 'User balance changed; retry'],
    [{ response: { data: { detail: 'Legacy detail' } } }, 'Legacy detail'],
    [{}, 'common.error'],
  ])('shows the available API error and keeps the dialog open', async (error, message) => {
    mocks.updateBalance.mockRejectedValueOnce(error)
    const w = mount(UserBalanceModal, { props: { show: true, operation: 'add', user: { id: 1, email: 'a@example.com', balance: 5 } as AdminUser },
      global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } } })
    await w.get('input[type="number"]').setValue('1')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.showError).toHaveBeenCalledWith(message)
    expect(w.emitted('success')).toBeUndefined(); expect(w.emitted('close')).toBeUndefined()
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })
})

describe('balance source', () => {
  const mountModal = (operation: 'add' | 'subtract' = 'add') => mount(UserBalanceModal, {
    props: { show: true, operation, user: { id: 1, email: 'a@example.com', balance: 5 } as AdminUser },
    global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } }
  })

  it('defaults to gift and explains the choice', async () => {
    mocks.updateBalance.mockResolvedValueOnce({})
    const w = mountModal()
    const select = w.get('[data-testid="balance-source-select"]')
    expect((select.element as HTMLSelectElement).value).toBe('gift')
    expect(select.findAll('option').map((o) => o.attributes('value'))).toEqual(['gift', 'offline', 'correction'])
    expect(w.text()).toContain('admin.finance.balanceSource.help.gift')
    await w.get('input[type="number"]').setValue('2')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.updateBalance).toHaveBeenCalledWith(1, 2, 'add', '', 'gift')
  })

  it.each(['offline', 'correction'])('sends the chosen %s source', async (source) => {
    mocks.updateBalance.mockResolvedValueOnce({})
    const w = mountModal('subtract')
    await w.get('[data-testid="balance-source-select"]').setValue(source)
    expect(w.text()).toContain(`admin.finance.balanceSource.help.${source}`)
    await w.get('input[type="number"]').setValue('1')
    await w.get('textarea').setValue('memo')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.updateBalance).toHaveBeenCalledWith(1, 1, 'subtract', 'memo', source)
    expect(w.emitted('success')).toHaveLength(1)
  })

  it('resets the source to gift when reopened', async () => {
    const w = mountModal()
    await w.get('[data-testid="balance-source-select"]').setValue('offline')
    await w.setProps({ show: false }); await w.setProps({ show: true })
    expect((w.get('[data-testid="balance-source-select"]').element as HTMLSelectElement).value).toBe('gift')
  })
})
