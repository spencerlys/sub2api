import { describe, expect, it, vi } from 'vitest'

import { isOwnerAllowedAdminPath } from '@/router/roleHome'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: false,
  isAdmin: false,
  isOwner: false,
  isSimpleMode: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  cachedPublicSettings: null as null | Record<string, unknown>,
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({
    customMenuItems: [],
  }),
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

describe('router account health route', () => {
  it('resolves /admin/accounts/health to its own admin route (not shadowed by /admin/accounts)', async () => {
    const { default: router } = await import('@/router')
    const resolved = router.resolve('/admin/accounts/health')

    expect(resolved.name).toBe('AdminAccountHealth')
    expect(resolved.meta.requiresAuth).toBe(true)
    expect(resolved.meta.requiresAdmin).toBe(true)
    expect(resolved.meta.titleKey).toBe('admin.accountHealth.title')
    expect(router.resolve('/admin/accounts').name).toBe('AdminAccounts')
  })

  it('is inside the owner-scoped admin area', () => {
    expect(isOwnerAllowedAdminPath('/admin/accounts/health')).toBe(true)
  })
})
