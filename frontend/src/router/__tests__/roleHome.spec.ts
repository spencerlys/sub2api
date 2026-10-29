import { describe, it, expect } from 'vitest'
import { OWNER_HOME_PATH, isOwnerAllowedAdminPath, resolveRoleHomePath } from '@/router/roleHome'
import { resolveCompletedSetupRedirectPath } from '@/router/setupRedirect'

describe('roleHome', () => {
  it('resolves landing path per role', () => {
    expect(resolveRoleHomePath(true)).toBe('/admin/dashboard')
    expect(resolveRoleHomePath(true, true)).toBe('/admin/dashboard')
    expect(resolveRoleHomePath(false, true)).toBe(OWNER_HOME_PATH)
    expect(resolveRoleHomePath(false)).toBe('/dashboard')
    expect(resolveCompletedSetupRedirectPath(true, false, true)).toBe('/admin/accounts')
  })

  it('allows owners only their scoped admin pages', () => {
    for (const p of ['/admin/accounts', '/admin/proxies', '/admin/groups', '/admin/usage']) {
      expect(isOwnerAllowedAdminPath(p)).toBe(true)
    }
    for (const p of ['/admin', '/admin/dashboard', '/admin/users', '/admin/settings', '/admin/accounts-x', '/admin/channels/pricing']) {
      expect(isOwnerAllowedAdminPath(p)).toBe(false)
    }
  })
})
