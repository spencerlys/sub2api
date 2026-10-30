import { describe, expect, it, vi } from 'vitest'

import { isOwnerAllowedAdminPath } from '@/router/roleHome'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: true,
  isAdmin: false,
  isOwner: true,
  isSimpleMode: false,
  hasPendingAuthSession: false,
  user: { role: 'owner' }
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  cachedPublicSettings: null as null | Record<string, unknown>,
  publicSettingsLoaded: true,
  fetchPublicSettings: vi.fn()
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({
    customMenuItems: []
  })
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false }
  })
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn()
  })
}))

describe('router finance route', () => {
  it('resolves /admin/finance to an admin-only route', async () => {
    const { default: router } = await import('@/router')
    const resolved = router.resolve('/admin/finance')

    expect(resolved.name).toBe('AdminFinance')
    expect(resolved.meta.requiresAuth).toBe(true)
    expect(resolved.meta.requiresAdmin).toBe(true)
    expect(resolved.meta.titleKey).toBe('admin.finance.title')
  })

  it('is outside the owner-scoped admin area, so owners are redirected away', () => {
    expect(isOwnerAllowedAdminPath('/admin/finance')).toBe(false)
  })
})
