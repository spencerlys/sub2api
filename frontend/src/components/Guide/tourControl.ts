import { getActivePinia } from 'pinia'
import { useOnboardingStore } from '@/stores/onboarding'

/**
 * Advance the onboarding tour only when it is currently waiting on `selector`
 * (e.g. a submit button whose step must not move on until the API call succeeds).
 *
 * Safe to call from components that are unit-tested without an active Pinia:
 * it then does nothing.
 */
export function advanceTourIfCurrent(selector: string, delay = 500): boolean {
  if (!getActivePinia()) return false
  const onboardingStore = useOnboardingStore()
  if (!onboardingStore.isCurrentStep(selector)) return false
  void onboardingStore.nextStep(delay)
  return true
}
