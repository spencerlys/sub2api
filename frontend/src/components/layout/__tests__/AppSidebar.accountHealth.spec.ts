import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

const { routeState, authState } = vi.hoisted(() => ({
  routeState: { path: '/admin/accounts' },
  authState: {
    role: 'admin',
    isSimpleMode: false,
    get isAdmin() {
      return this.role === 'admin'
    },
    get isOwner() {
      return this.role === 'owner'
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({ push: vi.fn() })
}))

const authStore = reactive(authState)
const appStore = reactive({
  sidebarCollapsed: false,
  mobileOpen: false,
  backendModeEnabled: false,
  siteName: 'Sub2API',
  siteLogo: '',
  siteVersion: '',
  publicSettingsLoaded: true,
  cachedPublicSettings: null,
  sidebarScrollTop: 0,
  toggleSidebar: vi.fn(),
  setMobileOpen: vi.fn()
})

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => authStore,
  useOnboardingStore: () => ({ isCurrentStep: () => false, nextStep: vi.fn() }),
  useAdminSettingsStore: () => ({
    customMenuItems: [],
    opsMonitoringEnabled: true,
    paymentEnabled: false,
    fetch: vi.fn()
  })
}))

vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: new Proxy({}, { get: (_target, key) => key }),
  makeSidebarFlag: () => () => true
}))

vi.mock('@/utils/siteBillingMode', () => ({
  resolveSiteBillingMode: () => 'standard'
}))

vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: { value: false }, refreshBatchImageAccess: vi.fn() })
}))

vi.mock('@/components/common/VersionBadge.vue', () => ({ default: { template: '<span />' } }))

import AppSidebar from '../AppSidebar.vue'

const wrappers: ReturnType<typeof mount>[] = []

async function mountSidebar(role: 'admin' | 'owner', path: string) {
  authStore.role = role
  routeState.path = path
  const wrapper = mount(AppSidebar, {
    global: {
      stubs: {
        Icon: true,
        transition: false,
        RouterLink: { props: ['to'], template: '<a :data-to="to"><slot /></a>' },
        'router-link': { props: ['to'], template: '<a :data-to="to"><slot /></a>' }
      }
    }
  })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

describe('AppSidebar account health entry', () => {
  afterEach(() => {
    wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  })

  it.each(['owner', 'admin'] as const)('shows Account Health to %s right after Accounts', async (role) => {
    const wrapper = await mountSidebar(role, '/admin/accounts')

    const link = wrapper.find('a[data-to="/admin/accounts/health"]')
    expect(link.exists()).toBe(true)
    expect(link.text()).toContain('nav.accountHealth')

    const paths = wrapper.findAll('a.sidebar-link[data-to^="/admin/"]').map((a) => a.attributes('data-to'))
    expect(paths.indexOf('/admin/accounts/health')).toBe(paths.indexOf('/admin/accounts') + 1)
  })

  it('highlights only Account Health (not its Accounts parent) on the health page', async () => {
    const wrapper = await mountSidebar('owner', '/admin/accounts/health')

    expect(wrapper.find('a[data-to="/admin/accounts/health"]').classes()).toContain('sidebar-link-active')
    expect(wrapper.find('a[data-to="/admin/accounts"].sidebar-link').classes()).not.toContain('sidebar-link-active')
  })

  it('keeps Accounts highlighted on the accounts page', async () => {
    const wrapper = await mountSidebar('admin', '/admin/accounts')

    expect(wrapper.find('a[data-to="/admin/accounts"].sidebar-link').classes()).toContain('sidebar-link-active')
    expect(wrapper.find('a[data-to="/admin/accounts/health"]').classes()).not.toContain('sidebar-link-active')
  })
})
