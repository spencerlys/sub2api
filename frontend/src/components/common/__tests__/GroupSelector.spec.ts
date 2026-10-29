import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import GroupSelector from '../GroupSelector.vue'

const authState = { isSimpleMode: false }

vi.mock('@/stores', () => ({ useAuthStore: () => authState }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const groups = [
  { id: 1, name: 'Basic', platform: 'anthropic', status: 'active' },
  { id: 2, name: 'Composite', platform: 'composite', status: 'active' }
] as any

const mountSelector = (modelValue: number[] = []) => mount(GroupSelector, {
  props: { modelValue, groups },
  global: { stubs: { GroupBadge: { props: ['name'], template: '<span>{{ name }}</span>' }, Icon: true } }
})

describe('GroupSelector simple-mode binding policy', () => {
  beforeEach(() => { authState.isSimpleMode = false })

  it('hides composite groups in simple mode and preserves basic groups', () => {
    authState.isSimpleMode = true
    const wrapper = mountSelector()
    expect(wrapper.text()).toContain('Basic')
    expect(wrapper.text()).not.toContain('Composite')
  })

  it('keeps composite groups available in advanced mode', () => {
    const wrapper = mountSelector()
    expect(wrapper.text()).toContain('Composite')
  })

  it('cleans hidden historical composite IDs while preserving visible selections', () => {
    authState.isSimpleMode = true
    const wrapper = mountSelector([1, 2])
    expect(wrapper.emitted('update:modelValue')).toEqual([[[1]]])
  })
})

describe('GroupSelector locked groups', () => {
  beforeEach(() => { authState.isSimpleMode = false })

  it('keeps a locked group selected and labels it as admin-managed', async () => {
    const wrapper = mount(GroupSelector, {
      props: { modelValue: [1], groups: [groups[0], { id: 3, name: 'Shared', platform: 'anthropic', status: 'active' }], lockedIds: [3] },
      global: { stubs: { GroupBadge: { props: ['name'], template: '<span>{{ name }}</span>' }, Icon: true } }
    })
    await wrapper.setProps({ modelValue: [1, 3] })
    const boxes = wrapper.findAll('input[type="checkbox"]')
    expect(boxes[1].attributes('disabled')).toBeDefined()
    expect(boxes[0].attributes('disabled')).toBeUndefined()
    expect(wrapper.text()).toContain('admin.accounts.groupManagedByAdmin')

    await boxes[1].setValue(false)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await boxes[0].setValue(false)
    expect(wrapper.emitted('update:modelValue')).toEqual([[[3]]])
  })
})
