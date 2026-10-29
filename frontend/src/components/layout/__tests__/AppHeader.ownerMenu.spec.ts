import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { replayMock, authState, appState } = vi.hoisted(() => {
  return {
    replayMock: vi.fn(),
    authState: {
      user: null as null | { id: number; role: string; email: string; username: string; balance: number },
      isSimpleMode: false,
      get isAdmin() {
        return this.user?.role === 'admin'
      },
      get isOwner() {
        return this.user?.role === 'owner'
      }
    },
    appState: {
      docUrl: '',
      ownerGuideUrl: '',
      contactInfo: '',
      cachedPublicSettings: null
    }
  }
})

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ name: 'Accounts', path: '/admin/accounts', params: {}, meta: {} })
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
  useOnboardingStore: () => ({ replay: replayMock })
}))

vi.mock('@/components/common/LocaleSwitcher.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/common/AnnouncementBell.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/common/SubscriptionProgressMini.vue', () => ({ default: { template: '<div />' } }))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] })
}))

vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: { modelPlaza: 'modelPlaza', subscription: 'subscription' },
  isFeatureFlagEnabled: () => false
}))

vi.mock('@/router/title', () => ({
  resolveRouteMetaKeys: () => ({ titleKey: '', descriptionKey: '' })
}))

vi.mock('@/utils/siteBillingMode', () => ({
  resolveSiteBillingMode: () => 'standard'
}))

const authStore = reactive(authState)
const appStore = reactive({ ...appState, toggleMobileSidebar: vi.fn() })

import AppHeader from '../AppHeader.vue'

const mountHeader = async () => {
  const wrapper = mount(AppHeader, {
    global: {
      mocks: { $t: (key: string) => key },
      stubs: {
        AnnouncementBell: true,
        LocaleSwitcher: true,
        SubscriptionProgressMini: true,
        Icon: true,
        RouterLink: { template: '<a><slot /></a>' },
        'router-link': { template: '<a><slot /></a>' },
        transition: false
      }
    }
  })
  await wrapper.find('[data-tour="header-user-menu"]').trigger('click')
  await flushPromises()
  return wrapper
}

const setUser = (role: string) => {
  authStore.user = { id: 7, role, email: `${role}.example`, username: role, balance: 0 }
}

describe('AppHeader owner menu items', () => {
  beforeEach(() => {
    replayMock.mockReset()
    authStore.isSimpleMode = false
    appStore.ownerGuideUrl = ''
    appStore.docUrl = ''
  })

  it('shows the owner guide link to owners when configured, opening in a new tab', async () => {
    setUser('owner')
    appStore.ownerGuideUrl = 'https://docs.example.com/owner-guide'
    const wrapper = await mountHeader()

    const link = wrapper.find('[data-testid="owner-guide-link"]')
    expect(link.exists()).toBe(true)
    expect(link.attributes('href')).toBe('https://docs.example.com/owner-guide')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
    expect(link.text()).toContain('nav.ownerGuide')
  })

  it('hides the owner guide link when the setting is empty', async () => {
    setUser('owner')
    const wrapper = await mountHeader()
    expect(wrapper.find('[data-testid="owner-guide-link"]').exists()).toBe(false)
  })

  it('hides the owner guide link when the URL is not http(s)', async () => {
    setUser('owner')
    appStore.ownerGuideUrl = 'javascript:alert(1)'
    const wrapper = await mountHeader()
    expect(wrapper.find('[data-testid="owner-guide-link"]').exists()).toBe(false)
  })

  it('never shows the owner guide link to admins or users', async () => {
    appStore.ownerGuideUrl = 'https://docs.example.com/owner-guide'
    for (const role of ['admin', 'user']) {
      setUser(role)
      const wrapper = await mountHeader()
      expect(wrapper.find('[data-testid="owner-guide-link"]').exists()).toBe(false)
      wrapper.unmount()
    }
  })

  it('shows Restart Onboarding Tour to owners and replays the tour', async () => {
    setUser('owner')
    const wrapper = await mountHeader()
    const button = wrapper.find('[data-testid="restart-onboarding-tour"]')
    expect(button.exists()).toBe(true)
    await button.trigger('click')
    expect(replayMock).toHaveBeenCalledTimes(1)
  })

  it('keeps Restart Onboarding Tour for admins and hides it for users and in simple mode', async () => {
    setUser('admin')
    let wrapper = await mountHeader()
    expect(wrapper.find('[data-testid="restart-onboarding-tour"]').exists()).toBe(true)
    wrapper.unmount()

    setUser('user')
    wrapper = await mountHeader()
    expect(wrapper.find('[data-testid="restart-onboarding-tour"]').exists()).toBe(false)
    wrapper.unmount()

    setUser('owner')
    authStore.isSimpleMode = true
    wrapper = await mountHeader()
    expect(wrapper.find('[data-testid="restart-onboarding-tour"]').exists()).toBe(false)
  })
})
