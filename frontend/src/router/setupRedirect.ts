import { resolveRoleHomePath } from './roleHome'

export function resolveCompletedSetupRedirectPath(
  isAuthenticated: boolean,
  isAdmin: boolean,
  isOwner = false
): string {
  if (!isAuthenticated) {
    return '/login'
  }

  return resolveRoleHomePath(isAdmin, isOwner)
}
