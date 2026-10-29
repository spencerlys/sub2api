import { describe, expect, it } from 'vitest'

import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'
import {
  OWNER_TOUR_SELECTORS,
  getAdminSteps,
  getOwnerSteps,
  getTourSteps,
  getUserSteps,
  isManualAdvanceStep,
  resolveTourRole,
  shouldAutoStartTour
} from '../steps'

const identity = (key: string) => key

const lookup = (messages: unknown, key: string): unknown =>
  key.split('.').reduce<unknown>((node, segment) => {
    if (node && typeof node === 'object') return (node as Record<string, unknown>)[segment]
    return undefined
  }, messages)

const elementsOf = (steps: ReturnType<typeof getOwnerSteps>) =>
  steps.map((step) => (typeof step.element === 'string' ? step.element : undefined))

describe('onboarding tour role selection', () => {
  it('maps each role to its own tour; owners never fall into admin or user', () => {
    expect(resolveTourRole('admin')).toBe('admin')
    expect(resolveTourRole('owner')).toBe('owner')
    expect(resolveTourRole('user')).toBe('user')
    expect(resolveTourRole(undefined)).toBe('user')
    expect(resolveTourRole(null)).toBe('user')
    expect(resolveTourRole('something-else')).toBe('user')
  })

  it('returns the owner steps for owners and leaves admin/user tours unchanged', () => {
    expect(getTourSteps('owner', identity)).toEqual(getOwnerSteps(identity))
    expect(getTourSteps('admin', identity)).toEqual(getAdminSteps(identity))
    expect(getTourSteps('admin', identity, true)).toEqual(getAdminSteps(identity, true))
    expect(getTourSteps('user', identity)).toEqual(getUserSteps(identity))

    const ownerTitles = getTourSteps('owner', identity).map((s) => s.popover?.title)
    expect(ownerTitles.every((title) => String(title).startsWith('onboarding.owner.'))).toBe(true)
  })

  it('auto-starts for admins and owners only', () => {
    expect(shouldAutoStartTour('admin')).toBe(true)
    expect(shouldAutoStartTour('owner')).toBe(true)
    expect(shouldAutoStartTour('user')).toBe(false)
  })
})

describe('owner tour steps', () => {
  const steps = getOwnerSteps(identity)
  const elements = elementsOf(steps)

  it('walks proxy -> group -> account -> usage in order', () => {
    const order = [
      OWNER_TOUR_SELECTORS.sidebarProxies,
      OWNER_TOUR_SELECTORS.proxySubmit,
      OWNER_TOUR_SELECTORS.sidebarGroups,
      OWNER_TOUR_SELECTORS.groupSubmit,
      OWNER_TOUR_SELECTORS.sidebarAccounts,
      OWNER_TOUR_SELECTORS.accountSubmit,
      OWNER_TOUR_SELECTORS.accountAuthorize,
      OWNER_TOUR_SELECTORS.accountRowMore,
      OWNER_TOUR_SELECTORS.sidebarUsage,
      OWNER_TOUR_SELECTORS.userMenu
    ].map((selector) => elements.indexOf(selector))

    expect(order.every((index) => index >= 0)).toBe(true)
    expect([...order].sort((a, b) => a - b)).toEqual(order)
    expect(steps[0].element).toBeUndefined()
    expect(elements[elements.length - 1]).toBe(OWNER_TOUR_SELECTORS.userMenu)
  })

  it('never targets admin-only or user-only controls', () => {
    for (const element of elements) {
      if (!element) continue
      expect(element).not.toContain('key-form')
      expect(element).not.toContain('sidebar-my-keys')
      expect(element).not.toContain('group-form-multiplier')
      expect(element).not.toContain('group-form-exclusive')
    }
  })

  it('waits for the user on submit/navigation steps and explains form fields with next/previous', () => {
    const byElement = (selector: string) => steps.find((step) => step.element === selector)
    for (const selector of [
      OWNER_TOUR_SELECTORS.sidebarProxies,
      OWNER_TOUR_SELECTORS.proxySubmit,
      OWNER_TOUR_SELECTORS.groupSubmit,
      OWNER_TOUR_SELECTORS.accountSubmit,
      OWNER_TOUR_SELECTORS.accountAuthorize
    ]) {
      expect(byElement(selector)?.popover?.showButtons).toEqual(['close'])
    }
    expect(byElement('[data-tour="proxy-form-fallback"]')?.popover?.showButtons).toEqual(['next', 'previous'])
    expect(byElement(OWNER_TOUR_SELECTORS.userMenu)?.popover?.showButtons).toEqual(['next', 'previous'])
  })

  it('marks only the authorization dialog step as manually advanced', () => {
    const manual = steps.filter((step) => isManualAdvanceStep(step))
    expect(manual.map((step) => step.element)).toEqual([OWNER_TOUR_SELECTORS.accountAuthorize])
    expect(isManualAdvanceStep(undefined)).toBe(false)
    expect(getAdminSteps(identity).some((step) => isManualAdvanceStep(step))).toBe(false)
  })

  it('has a non-empty EN and ZH message for every key it uses', () => {
    const keys = steps.flatMap((step) =>
      [step.popover?.title, step.popover?.description, step.popover?.nextBtnText].filter(
        (value): value is string => typeof value === 'string'
      )
    )
    expect(keys.length).toBeGreaterThan(steps.length)
    for (const key of keys) {
      for (const messages of [en, zh]) {
        const value = lookup(messages, key)
        expect(typeof value, key).toBe('string')
        expect(String(value).trim(), key).not.toBe('')
      }
    }
  })

  it('quotes the real UI labels in its step texts', () => {
    const text = (key: string, messages: unknown) => String(lookup(messages, `onboarding.owner.${key}.description`))
    const label = (key: string, messages: unknown) => String(lookup(messages, key))

    for (const messages of [en, zh]) {
      expect(text('proxyManage', messages)).toContain(label('nav.proxies', messages))
      expect(text('createProxy', messages)).toContain(label('admin.proxies.createProxy', messages))
      expect(text('proxyFallback', messages)).toContain(label('admin.proxies.fallbackNone', messages))
      expect(text('proxyFallback', messages)).toContain(label('admin.proxies.fallbackProxy', messages))
      expect(text('proxyFallback', messages)).toContain(label('admin.proxies.fallbackDirect', messages))
      expect(text('groupManage', messages)).toContain(label('nav.groups', messages))
      expect(text('createGroup', messages)).toContain(label('admin.groups.createGroup', messages))
      expect(text('accountManage', messages)).toContain(label('nav.accounts', messages))
      expect(text('createAccount', messages)).toContain(label('admin.accounts.createAccount', messages))
      expect(text('accountAuthorize', messages)).toContain(label('admin.accounts.oauth.generateAuthUrl', messages))
      expect(text('accountAuthorize', messages)).toContain(label('admin.accounts.oauth.completeAuth', messages))
      expect(text('accountTest', messages)).toContain(label('admin.accounts.testConnection', messages))
      expect(text('usageManage', messages)).toContain(label('nav.usage', messages))
      expect(text('usageBilling', messages)).toContain(label('usage.userBilled', messages))
      expect(text('usageBilling', messages)).toContain(label('usage.accountBilled', messages))
      expect(text('finish', messages)).toContain(label('onboarding.restartTour', messages))
      expect(text('finish', messages)).toContain(label('nav.ownerGuide', messages))
    }
  })
})
