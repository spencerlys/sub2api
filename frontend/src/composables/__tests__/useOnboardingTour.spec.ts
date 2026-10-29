import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { Config, DriveStep } from 'driver.js'

import { getAdminSteps, getOwnerSteps, OWNER_TOUR_SELECTORS } from '@/components/Guide/steps'
import { useOnboardingTour } from '../useOnboardingTour'

const authState = vi.hoisted(() => ({
  user: { id: 1, role: 'owner' } as { id: number; role: string } | null,
  isSimpleMode: false
}))

// A minimal stand-in for driver.js: it keeps the active step and calls
// onHighlightStarted the way the real driver does (element undefined when missing).
const fakeDriver = vi.hoisted(() => {
  const created: Array<ReturnType<typeof make>> = []
  function make(config: Config) {
    const state: { active: boolean; index?: number; step?: DriveStep; element?: Element } = { active: false }
    const steps = () => config.steps ?? []
    const go = (index: number) => {
      const step = steps()[index]
      if (!step) {
        api.destroy()
        return
      }
      const element = typeof step.element === 'string' ? document.querySelector(step.element) ?? undefined : undefined
      state.index = index
      void config.onHighlightStarted?.(element, step, { config, state: { activeIndex: index }, driver: api as never })
      state.step = step
      state.element = element
    }
    const api = {
      config,
      drive: vi.fn((index = 0) => {
        state.active = true
        go(index)
      }),
      moveTo: vi.fn((index: number) => go(index)),
      moveNext: vi.fn(() => go((state.index ?? -1) + 1)),
      movePrevious: vi.fn(() => go((state.index ?? 0) - 1)),
      destroy: vi.fn(() => {
        if (!state.active) return
        state.active = false
        state.index = undefined
        state.step = undefined
        state.element = undefined
        config.onDestroyed?.(undefined, {}, { config, state: {}, driver: api as never })
      }),
      isActive: () => state.active,
      isLastStep: () => state.index === steps().length - 1,
      getActiveIndex: () => state.index,
      getActiveStep: () => state.step,
      getActiveElement: () => state.element,
      getConfig: () => config,
      refresh: vi.fn()
    }
    created.push(api)
    return api
  }
  return { make, created }
})

vi.mock('driver.js', () => ({ driver: (config: Config) => fakeDriver.make(config) }))
vi.mock('driver.js/dist/driver.css', () => ({}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }))

const identity = (key: string) => key
const ownerSteps = getOwnerSteps(identity)
const ownerIndex = (selector: string) => ownerSteps.findIndex((step) => step.element === selector)
const seenKey = (role: string) => `test_tour_1_${role}_v4_interactive`

const press = (key: string, target: Element = document.body) => {
  const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true })
  target.dispatchEvent(event)
  return event
}

const html = (markup: string) => {
  document.body.innerHTML = markup
}

const dialog = (inner: string) =>
  `<div role="dialog"><button aria-label="Close modal" id="close-modal"></button><form>${inner}</form></div>`

let wrapper: VueWrapper | null = null

const mountTour = () => {
  let tour!: ReturnType<typeof useOnboardingTour>
  wrapper = mount(
    defineComponent({
      setup() {
        tour = useOnboardingTour({ storageKey: 'test_tour', autoStart: false })
        return () => h('div')
      }
    })
  )
  return tour
}

const activeDriver = () => fakeDriver.created[fakeDriver.created.length - 1]

const clickNext = () => {
  const instance = activeDriver()
  const index = instance.getActiveIndex()
  return instance.config.onNextClick?.(undefined, instance.config.steps![index ?? 0], {
    config: instance.config,
    state: { activeIndex: index },
    driver: instance as never
  })
}

const clickPrevious = () => {
  const instance = activeDriver()
  const index = instance.getActiveIndex()
  return instance.config.onPrevClick?.(undefined, instance.config.steps![index ?? 0], {
    config: instance.config,
    state: { activeIndex: index },
    driver: instance as never
  })
}

describe('useOnboardingTour', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    authState.user = { id: 1, role: 'owner' }
    authState.isSimpleMode = false
    localStorage.clear()
    fakeDriver.created.length = 0
    vi.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0, y: 0, top: 0, left: 0, right: 20, bottom: 20, width: 20, height: 20, toJSON: () => ({})
    } as DOMRect)
  })

  afterEach(() => {
    activeDriver()?.destroy()
    wrapper?.unmount()
    wrapper = null
    document.body.innerHTML = ''
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  describe('keyboard', () => {
    it('turns off driver keyboard control so the composable handler is the only keyboard path', async () => {
      const tour = mountTour()
      await tour.startTour()
      expect(activeDriver().config.allowKeyboardControl).toBe(false)
    })

    it('ignores arrow keys while the user types in an editable element', async () => {
      html(
        dialog(
          '<input data-tour="proxy-form-name" /><select data-tour="proxy-form-protocol"><option>http</option></select>' +
            '<div contenteditable="true" id="editable"></div><button type="submit" data-tour="proxy-form-submit"></button>'
        )
      )
      const closeClick = vi.fn()
      document.getElementById('close-modal')!.addEventListener('click', closeClick)
      const tour = mountTour()
      await tour.startTour(ownerIndex('[data-tour="proxy-form-protocol"]'))
      const instance = activeDriver()

      const select = document.querySelector('select')!
      const editable = document.getElementById('editable')!
      for (const target of [document.querySelector('input')!, select, editable]) {
        for (const key of ['ArrowRight', 'ArrowLeft']) {
          expect(press(key, target).defaultPrevented, `${key} in ${target.tagName}`).toBe(false)
        }
      }
      expect(instance.moveNext).not.toHaveBeenCalled()
      expect(instance.movePrevious).not.toHaveBeenCalled()
      expect(instance.moveTo).not.toHaveBeenCalled()
      expect(closeClick).not.toHaveBeenCalled()
    })

    it('never skips a submit step (closing the dialog) from the arrow keys', async () => {
      html(dialog('<input data-tour="proxy-form-name" /><button type="submit" data-tour="proxy-form-submit"></button>'))
      const closeClick = vi.fn()
      document.getElementById('close-modal')!.addEventListener('click', closeClick)
      const tour = mountTour()
      await tour.startTour(ownerIndex(OWNER_TOUR_SELECTORS.proxySubmit))

      press('ArrowRight')
      await vi.advanceTimersByTimeAsync(2000)
      expect(closeClick).not.toHaveBeenCalled()
      expect(activeDriver().getActiveIndex()).toBe(ownerIndex(OWNER_TOUR_SELECTORS.proxySubmit))
    })

    it('moves forward once with ArrowRight on an info step', async () => {
      html(dialog('<input data-tour="proxy-form-name" /><select data-tour="proxy-form-protocol"></select>'))
      const tour = mountTour()
      await tour.startTour(ownerIndex('[data-tour="proxy-form-name"]'))

      expect(press('ArrowRight').defaultPrevented).toBe(true)
      expect(activeDriver().getActiveIndex()).toBe(ownerIndex('[data-tour="proxy-form-protocol"]'))
    })

    it('does not end the tour with ArrowLeft on the first step', async () => {
      const tour = mountTour()
      await tour.startTour()
      press('ArrowLeft')
      expect(activeDriver().isActive()).toBe(true)
      expect(activeDriver().getActiveIndex()).toBe(0)
    })

    it('goes back to the nearest step whose target is still on screen', async () => {
      html(
        '<a id="sidebar-channel-manage"></a><button data-tour="accounts-create-btn"></button>' +
          '<button data-tour="account-row-more"></button>'
      )
      const tour = mountTour()
      await tour.startTour(ownerIndex(OWNER_TOUR_SELECTORS.accountRowMore))

      press('ArrowLeft')
      expect(activeDriver().getActiveIndex()).toBe(ownerIndex(OWNER_TOUR_SELECTORS.accountsCreate))

      await tour.startTour(ownerIndex(OWNER_TOUR_SELECTORS.accountRowMore))
      await clickPrevious()
      expect(activeDriver().getActiveIndex()).toBe(ownerIndex(OWNER_TOUR_SELECTORS.accountsCreate))
    })

    it('closes the tour and marks it seen on Escape', async () => {
      const tour = mountTour()
      await tour.startTour()
      const event = press('Escape')
      expect(event.defaultPrevented).toBe(true)
      expect(activeDriver().isActive()).toBe(false)
      expect(localStorage.getItem(seenKey('owner'))).toBe('true')
    })

    it('leaves Enter on the popover buttons to the browser and never presses a submit target', async () => {
      html(dialog('<input data-tour="proxy-form-name" /><button type="submit" data-tour="proxy-form-submit"></button>'))
      const submit = document.querySelector<HTMLButtonElement>('[data-tour="proxy-form-submit"]')!
      const submitClick = vi.fn((event: Event) => event.preventDefault())
      submit.addEventListener('click', submitClick)
      const popover = document.createElement('div')
      popover.className = 'driver-popover'
      popover.innerHTML = '<button class="driver-popover-next-btn">Skip</button>'
      document.body.appendChild(popover)

      const tour = mountTour()
      await tour.startTour(ownerIndex(OWNER_TOUR_SELECTORS.proxySubmit))

      const onPopoverButton = press('Enter', popover.querySelector('button')!)
      expect(onPopoverButton.defaultPrevented).toBe(false)
      expect(submitClick).not.toHaveBeenCalled()

      press('Enter')
      expect(submitClick).not.toHaveBeenCalled()
      expect(activeDriver().getActiveIndex()).toBe(ownerIndex(OWNER_TOUR_SELECTORS.proxySubmit))
    })

    it('lets Enter reach the buttons inside the authorization dialog', async () => {
      html(
        '<div role="dialog"><div class="modal-content"><div data-tour="account-oauth-flow">' +
          '<button id="generate">Generate Auth URL</button></div></div></div>'
      )
      const tour = mountTour()
      await tour.startTour(ownerIndex(OWNER_TOUR_SELECTORS.accountAuthorize))

      const event = press('Enter', document.getElementById('generate')!)
      expect(event.defaultPrevented).toBe(false)
      expect(activeDriver().getActiveIndex()).toBe(ownerIndex(OWNER_TOUR_SELECTORS.accountAuthorize))
    })
  })

  describe('skipping a step that needs a real save', () => {
    const proxyDialog = () => {
      html(
        '<a id="sidebar-group-manage"></a>' +
          dialog('<input data-tour="proxy-form-name" /><button type="submit" data-tour="proxy-form-submit"></button>')
      )
      const closeClick = vi.fn()
      document.getElementById('close-modal')!.addEventListener('click', closeClick)
      return closeClick
    }

    it('closes the dialog and moves to the next step that is on screen', async () => {
      const closeClick = proxyDialog()
      const tour = mountTour()
      await tour.startTour(ownerIndex(OWNER_TOUR_SELECTORS.proxySubmit))

      void clickNext()
      await vi.advanceTimersByTimeAsync(1000)
      expect(closeClick).toHaveBeenCalledTimes(1)
      expect(activeDriver().getActiveIndex()).toBe(ownerIndex(OWNER_TOUR_SELECTORS.sidebarGroups))
    })

    it('ignores a second Skip while the first one is still running', async () => {
      proxyDialog()
      const tour = mountTour()
      await tour.startTour(ownerIndex(OWNER_TOUR_SELECTORS.proxySubmit))

      void clickNext()
      void clickNext()
      await vi.advanceTimersByTimeAsync(1000)
      expect(activeDriver().moveTo).toHaveBeenCalledTimes(1)
    })

    it('does not revive the tour when it was closed during the skip', async () => {
      proxyDialog()
      const tour = mountTour()
      await tour.startTour(ownerIndex(OWNER_TOUR_SELECTORS.proxySubmit))

      void clickNext()
      press('Escape')
      await vi.advanceTimersByTimeAsync(1000)
      expect(activeDriver().moveTo).not.toHaveBeenCalled()
      expect(activeDriver().isActive()).toBe(false)
    })

    it('closes the dialog before finishing when the last step is skipped (admin/user tours)', async () => {
      authState.user = { id: 1, role: 'admin' }
      html(dialog('<input data-tour="key-form-name" /><button type="submit" data-tour="key-form-submit"></button>'))
      const closeClick = vi.fn()
      document.getElementById('close-modal')!.addEventListener('click', closeClick)
      const tour = mountTour()
      await tour.startTour(getAdminSteps(identity).length - 1)

      void clickNext()
      await vi.advanceTimersByTimeAsync(1000)
      expect(closeClick).toHaveBeenCalledTimes(1)
      expect(activeDriver().isActive()).toBe(false)
      expect(localStorage.getItem(seenKey('admin'))).toBe('true')
    })
  })

  describe('steps whose target is not on screen yet', () => {
    it('does not bind listeners when the tour ended while waiting for the target', async () => {
      html('<a data-tour="sidebar-proxies"></a>')
      const tour = mountTour()
      await tour.startTour(ownerIndex(OWNER_TOUR_SELECTORS.sidebarProxies))
      activeDriver().moveNext() // proxies-create-btn is not rendered yet

      press('Escape')
      const target = document.createElement('div')
      target.setAttribute('data-tour', 'proxies-create-btn')
      document.body.appendChild(target)
      await vi.advanceTimersByTimeAsync(500)

      expect(target.hasAttribute('tabindex')).toBe(false)
      expect(activeDriver().moveTo).not.toHaveBeenCalled()
    })

    const accountForm = () =>
      html(
        dialog(
          '<select data-tour="account-form-type"></select>' +
            '<div data-tour="account-form-proxy"></div><button type="submit" data-tour="account-form-submit"></button>'
        )
      )

    it('skips info steps whose field is not rendered for the chosen platform (forward)', async () => {
      accountForm()
      const tour = mountTour()
      await tour.startTour(ownerIndex('[data-tour="account-form-type"]'))

      void clickNext()
      await vi.advanceTimersByTimeAsync(2000)
      expect(activeDriver().getActiveIndex()).toBe(ownerIndex('[data-tour="account-form-proxy"]'))
    })

    it('skips missing info steps backwards when navigating back', async () => {
      accountForm()
      const tour = mountTour()
      await tour.startTour(ownerIndex('[data-tour="account-form-proxy"]'))

      activeDriver().moveTo(ownerIndex('[data-tour="account-form-custom-base-url"]'))
      await vi.advanceTimersByTimeAsync(2000)
      expect(activeDriver().getActiveIndex()).toBe(ownerIndex('[data-tour="account-form-type"]'))
    })
  })

  it('marks the tour seen when business code finishes it from the last step', async () => {
    authState.user = { id: 1, role: 'admin' }
    html(dialog('<button type="submit" data-tour="key-form-submit"></button>'))
    const tour = mountTour()
    await tour.startTour(getAdminSteps(identity).length - 1)

    await tour.nextStep(0)
    expect(activeDriver().isActive()).toBe(false)
    expect(localStorage.getItem(seenKey('admin'))).toBe('true')
  })
})
