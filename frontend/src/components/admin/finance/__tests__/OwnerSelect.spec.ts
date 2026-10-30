import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'

const { list, showError } = vi.hoisted(() => ({ list: vi.fn(), showError: vi.fn() }))

vi.mock('@/api/admin', () => ({ adminAPI: { users: { list } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

import OwnerSelect from '../OwnerSelect.vue'

enableAutoUnmount(afterEach)

describe('OwnerSelect', () => {
  beforeEach(() => vi.clearAllMocks())

  it('lists owners from the users API and emits the chosen id (empty = all)', async () => {
    list.mockResolvedValue({
      items: [
        { id: 3, email: 'owner-a@example.com', username: 'alice' },
        { id: 4, email: 'owner-b@example.com', username: '' }
      ],
      total: 2
    })
    const wrapper = mount(OwnerSelect, { props: { modelValue: null } })
    await flushPromises()

    expect(list).toHaveBeenCalledWith(1, 1000, { role: 'owner' })
    const options = wrapper.findAll('option')
    expect(options.map((o) => o.text())).toEqual([
      'admin.finance.ownerSelect.all',
      'owner-a@example.com (alice)',
      'owner-b@example.com'
    ])

    await wrapper.get('select').setValue('4')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([4])
    await wrapper.get('select').setValue('')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([null])
  })

  it('reports owner list failures', async () => {
    list.mockRejectedValue(new Error('down'))
    mount(OwnerSelect, { props: { modelValue: null } })
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('admin.finance.ownerSelect.loadFailed')
  })
})
