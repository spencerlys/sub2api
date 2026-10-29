/**
 * Role-aware landing paths.
 *
 * AI account owners (role "owner") reuse the admin panel, but the backend only
 * grants them their own accounts / proxies / groups / usage. Every other admin
 * route (and the user dashboard) sends them to OWNER_HOME_PATH.
 */
export const OWNER_HOME_PATH = '/admin/accounts'

export const OWNER_ADMIN_PATHS = ['/admin/accounts', '/admin/proxies', '/admin/groups', '/admin/usage']

export function isOwnerAllowedAdminPath(path: string): boolean {
  return OWNER_ADMIN_PATHS.some((allowed) => path === allowed || path.startsWith(allowed + '/'))
}

export function resolveRoleHomePath(isAdmin: boolean, isOwner = false): string {
  if (isAdmin) return '/admin/dashboard'
  if (isOwner) return OWNER_HOME_PATH
  return '/dashboard'
}
