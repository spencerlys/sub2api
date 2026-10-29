import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { Driver, DriveStep } from 'driver.js'

import { useOnboardingStore } from '@/stores/onboarding'
import { OWNER_TOUR_SELECTORS, getOwnerSteps } from '../steps'
import { advanceTourPastIfCurrent } from '../tourControl'

const steps = getOwnerSteps((key) => key)
const indexOf = (selector: string) => steps.findIndex((step) => step.element === selector)
const ACCOUNT_STEPS = [OWNER_TOUR_SELECTORS.accountSubmit, OWNER_TOUR_SELECTORS.accountAuthorize] as const

const installDriver = (activeSelector: string) => {
  const state = { active: true, step: steps[indexOf(activeSelector)] as DriveStep | undefined }
  const fake = {
    isActive: () => state.active,
    getConfig: () => ({ steps }),
    getActiveStep: () => state.step,
    moveTo: vi.fn((index: number) => {
      state.step = steps[index]
    })
  }
  const store = useOnboardingStore()
  store.setDriverInstance(fake as unknown as Driver)
  store.setControlMethods({
    nextStep: vi.fn(async () => {}),
    isCurrentStep: (selector) => state.active && state.step?.element === selector
  })
  return { fake, state }
}

describe('advanceTourPastIfCurrent', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    document.body.innerHTML = '<button data-tour="account-row-more"></button>'
  })

  afterEach(() => {
    document.body.innerHTML = ''
    vi.useRealTimers()
  })

  it.each(ACCOUNT_STEPS)('moves from %s past the authorization step to the account row', async (selector) => {
    const { fake } = installDriver(selector)

    expect(advanceTourPastIfCurrent(ACCOUNT_STEPS, 300)).toBe(true)
    expect(fake.moveTo).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(300)
    expect(fake.moveTo).toHaveBeenCalledWith(indexOf(OWNER_TOUR_SELECTORS.accountRowMore))
  })

  it('does nothing when the tour is on another step', async () => {
    const { fake } = installDriver(OWNER_TOUR_SELECTORS.proxySubmit)

    expect(advanceTourPastIfCurrent(ACCOUNT_STEPS, 300)).toBe(false)
    await vi.advanceTimersByTimeAsync(300)
    expect(fake.moveTo).not.toHaveBeenCalled()
  })

  it('does not revive a tour that was closed during the delay', async () => {
    const { fake, state } = installDriver(OWNER_TOUR_SELECTORS.accountSubmit)

    expect(advanceTourPastIfCurrent(ACCOUNT_STEPS, 300)).toBe(true)
    state.active = false
    await vi.advanceTimersByTimeAsync(300)
    expect(fake.moveTo).not.toHaveBeenCalled()
  })

  it('moves to the step right after when its target is not rendered yet', async () => {
    document.body.innerHTML = ''
    const { fake } = installDriver(OWNER_TOUR_SELECTORS.accountAuthorize)

    advanceTourPastIfCurrent(ACCOUNT_STEPS, 0)
    await vi.advanceTimersByTimeAsync(0)
    expect(fake.moveTo).toHaveBeenCalledWith(indexOf(OWNER_TOUR_SELECTORS.accountAuthorize) + 1)
  })

  it('is a no-op without an active Pinia', () => {
    setActivePinia(undefined as never)
    expect(advanceTourPastIfCurrent(ACCOUNT_STEPS)).toBe(false)
  })
})
