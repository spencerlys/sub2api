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

/**
 * When the tour is on any of `selectors`, move past the last of them to the first
 * later step whose target is on screen (or to the step right after, which then
 * waits for its target). Used when one action can finish a run of steps at once,
 * e.g. creating an API-key account skips the OAuth authorization step.
 */
export function advanceTourPastIfCurrent(selectors: readonly string[], delay = 500): boolean {
  if (!getActivePinia()) return false
  const onboardingStore = useOnboardingStore()
  const driverInstance = onboardingStore.getDriverInstance()
  const isOnSelectors = () =>
    !!driverInstance?.isActive() && selectors.some((selector) => onboardingStore.isCurrentStep(selector))
  if (!driverInstance || !isOnSelectors()) return false

  const steps = driverInstance.getConfig().steps ?? []
  const lastIndex = Math.max(...selectors.map((selector) => steps.findIndex((step) => step.element === selector)))
  if (lastIndex < 0 || lastIndex + 1 >= steps.length) return false

  setTimeout(() => {
    if (!isOnSelectors()) return
    for (let index = lastIndex + 1; index < steps.length; index++) {
      const target = steps[index].element
      if (typeof target !== 'string' || document.querySelector(target)) {
        driverInstance.moveTo(index)
        return
      }
    }
    driverInstance.moveTo(lastIndex + 1)
  }, delay)
  return true
}
