import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { AdminUser } from '@/types'
import UserBalanceHistoryModal from '../UserBalanceHistoryModal.vue'

const mocks = vi.hoisted(() => ({ getUserBalanceHistory: vi.fn(), updateBalanceRecordSource: vi.fn() }))
const toast = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: mocks } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => toast }))
vi.mock('@/utils/format', () => ({ formatDateTime: () => 'date' }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); vi.spyOn(console, 'error').mockImplementation(() => {}) })
afterEach(() => vi.restoreAllMocks())
function result(id: number) {
  return { items: [{ id, type: 'admin_balance', value: id, notes: `History ${id}` }], total: 1, total_recharged: id }
}
function deferred() {
  let resolve!: (value: ReturnType<typeof result>) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<ReturnType<typeof result>>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
async function openDialog() {
  const wrapper = mount(UserBalanceHistoryModal, {
    props: { show: false, user: { id: 1, email: 'one@example.com', balance: 1 } as AdminUser },
    global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }, Icon: true, Select: true } }
  })
  await wrapper.setProps({ show: true })
  return wrapper
}

describe('UserBalanceHistoryModal request ordering', () => {
  it('keeps the new user history when an old response finishes later', async () => {
    const old = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(old.promise).mockResolvedValueOnce(result(20))
    const wrapper = await openDialog()
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, user: { id: 2, email: 'two@example.com', balance: 2 } as AdminUser })
    await flushPromises()
    old.resolve(result(10))
    await flushPromises()
    expect(wrapper.text()).toContain('History 20')
    expect(wrapper.text()).not.toContain('History 10')
    expect(wrapper.text()).toContain('$20.00')
  })

  it('does not end the current filter loading state when an old request finishes', async () => {
    const old = deferred()
    const current = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const wrapper = await openDialog()
    wrapper.findComponent({ name: 'Select' }).vm.$emit('change', 'balance')
    await flushPromises()
    old.resolve(result(10))
    await flushPromises()
    expect(wrapper.find('svg.animate-spin').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('History 10')
    current.resolve(result(20))
    await flushPromises()
    expect(wrapper.find('svg.animate-spin').exists()).toBe(false)
    expect(wrapper.text()).toContain('History 20')
  })

  it.each(['close', 'unmount'])('ignores failures after %s', async (action) => {
    const pending = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(pending.promise)
    const wrapper = await openDialog()
    if (action === 'close') await wrapper.setProps({ show: false })
    else wrapper.unmount()
    pending.reject(new Error('Stale error'))
    await flushPromises()
    expect(console.error).not.toHaveBeenCalled()
  })

  it('still reports current failures and ends loading', async () => {
    const error = new Error('Current error')
    mocks.getUserBalanceHistory.mockRejectedValueOnce(error)
    const wrapper = await openDialog()
    await flushPromises()
    expect(console.error).toHaveBeenCalledWith('Failed to load balance history:', error)
    expect(wrapper.find('svg.animate-spin').exists()).toBe(false)
  })
})

describe('UserBalanceHistoryModal balance source', () => {
  function sourceHistory() {
    return {
      items: [
        { id: 11, type: 'admin_balance', value: 5, notes: 'legacy grant', created_at: '' },
        { id: 12, type: 'admin_balance', value: 9, notes: 'bank transfer', balance_source: 'offline', created_at: '' },
        { id: 13, type: 'balance', value: 3, code: 'ABCDEFGHIJ', notes: '', created_at: '' }
      ],
      total: 3,
      total_recharged: 17
    }
  }
  const select = (wrapper: Awaited<ReturnType<typeof openDialog>>, id: number) =>
    wrapper.find(`[data-testid="balance-source-${id}"]`)

  it('shows a source select only on admin balance rows, defaulting to gift', async () => {
    mocks.getUserBalanceHistory.mockResolvedValueOnce(sourceHistory())
    const wrapper = await openDialog()
    await flushPromises()
    expect((select(wrapper, 11).element as HTMLSelectElement).value).toBe('gift')
    expect((select(wrapper, 12).element as HTMLSelectElement).value).toBe('offline')
    expect(select(wrapper, 13).exists()).toBe(false)
  })

  it('updates the record source and the row on change', async () => {
    mocks.getUserBalanceHistory.mockResolvedValueOnce(sourceHistory())
    mocks.updateBalanceRecordSource.mockResolvedValueOnce({ id: 11, balance_source: 'correction', notes: 'fixed typo' })
    const wrapper = await openDialog()
    await flushPromises()
    await select(wrapper, 11).setValue('correction')
    await flushPromises()
    expect(mocks.updateBalanceRecordSource).toHaveBeenCalledWith(1, 11, 'correction')
    expect(toast.showSuccess).toHaveBeenCalledWith('admin.finance.balanceSource.updated')
    expect((select(wrapper, 11).element as HTMLSelectElement).value).toBe('correction')
    expect(wrapper.text()).toContain('fixed typo')
  })

  it('reverts the select and reports the error when the update fails', async () => {
    mocks.getUserBalanceHistory.mockResolvedValueOnce(sourceHistory())
    mocks.updateBalanceRecordSource.mockRejectedValueOnce({ message: 'Only admin balance records' })
    const wrapper = await openDialog()
    await flushPromises()
    await select(wrapper, 12).setValue('gift')
    await flushPromises()
    expect(mocks.updateBalanceRecordSource).toHaveBeenCalledWith(1, 12, 'gift')
    expect(toast.showError).toHaveBeenCalledWith('Only admin balance records')
    expect((select(wrapper, 12).element as HTMLSelectElement).value).toBe('offline')
  })
})
