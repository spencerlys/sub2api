import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const pollOrderStatus = vi.hoisted(() => vi.fn())
const cancelOrder = vi.hoisted(() => vi.fn())
const verifyOrder = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const toCanvas = vi.hoisted(() => vi.fn())
const copyToClipboard = vi.hoisted(() => vi.fn())

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores/payment', () => ({
  usePaymentStore: () => ({
    pollOrderStatus,
  }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
  }),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    cancelOrder,
    verifyOrder,
  },
}))

vi.mock('qrcode', () => ({
  default: {
    toCanvas,
  },
}))

import PaymentStatusPanel from '../PaymentStatusPanel.vue'

const orderFactory = (status: string) => ({
  id: 42,
  user_id: 9,
  amount: 88,
  pay_amount: 88,
  fee_rate: 0,
  payment_type: 'alipay',
  out_trade_no: 'sub2_20260420abcd1234',
  status,
  order_type: 'balance',
  created_at: '2026-04-20T12:00:00Z',
  expires_at: '2099-01-01T12:30:00Z',
  refund_amount: 0,
})

describe('PaymentStatusPanel', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    pollOrderStatus.mockReset()
    cancelOrder.mockReset()
    verifyOrder.mockReset()
    showError.mockReset()
    toCanvas.mockReset().mockResolvedValue(undefined)
    copyToClipboard.mockReset().mockResolvedValue(true)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('treats RECHARGING as a successful terminal state', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('RECHARGING'))

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        qrCode: 'https://pay.example.com/qr/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(pollOrderStatus).toHaveBeenCalledWith(42)
    expect(wrapper.text()).toContain('payment.result.success')
    expect(wrapper.emitted('success')).toHaveLength(1)
  })

  it('shows reopen button in QR mode when payUrl is also available', async () => {
    const openSpy = vi.spyOn(window, 'open').mockReturnValue({ closed: false } as Window)

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        qrCode: 'https://pay.example.com/qr/42',
        payUrl: 'https://pay.example.com/session/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await flushPromises()
    expect(wrapper.text()).toContain('payment.qr.openPayWindow')

    await wrapper.get('button.btn.btn-secondary.text-sm').trigger('click')
    expect(openSpy).toHaveBeenCalledWith(
      'https://pay.example.com/session/42',
      'paymentPopup',
      expect.any(String),
    )

    openSpy.mockRestore()
  })

  it('uses generic QR copy for custom methods that contain built-in names', async () => {
    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        qrCode: 'https://pay.example.com/qr/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'card_alipay',
        orderType: 'balance',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('payment.qr.scanToPay')
    expect(wrapper.text()).not.toContain('payment.qr.scanAlipay')
  })

  it('actively verifies a stuck pending order and settles it when upstream confirms payment', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    verifyOrder.mockResolvedValue({
      data: orderFactory('COMPLETED'),
    })

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        qrCode: 'https://pay.example.com/qr/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'wxpay',
        orderType: 'balance',
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(pollOrderStatus).toHaveBeenCalledWith(42)
    expect(verifyOrder).toHaveBeenCalledWith('sub2_20260420abcd1234')
    expect(wrapper.text()).toContain('payment.result.success')
    expect(wrapper.emitted('success')).toHaveLength(1)
  })

  it('actively verifies a pending mobile Alipay precreate order', async () => {
    const originalLocation = window.location
    const originalHidden = Object.getOwnPropertyDescriptor(document, 'hidden')
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { assign: vi.fn() },
    })
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      get: () => false,
    })
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    verifyOrder.mockResolvedValue({ data: orderFactory('COMPLETED') })

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        amount: 88,
        payAmount: 88,
        qrCode: 'https://qr.alipay.com/dynamic-order-42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
        outTradeNo: 'sub2_20260420abcd1234',
        mobileAlipayDeepLink: true,
      },
      global: { stubs: { Icon: true } },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(verifyOrder).toHaveBeenCalledWith('sub2_20260420abcd1234')
    expect(wrapper.emitted('success')).toHaveLength(1)

    wrapper.unmount()
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
    if (originalHidden) Object.defineProperty(document, 'hidden', originalHidden)
  })

  it('actively verifies a pending desktop Alipay order', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    verifyOrder.mockResolvedValue({ data: orderFactory('COMPLETED') })

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        amount: 88,
        payAmount: 88,
        qrCode: 'https://qr.alipay.com/desktop-order-42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
        outTradeNo: 'sub2_20260420abcd1234',
      },
      global: { stubs: { Icon: true } },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(verifyOrder).toHaveBeenCalledWith('sub2_20260420abcd1234')
    expect(wrapper.emitted('success')).toHaveLength(1)

    wrapper.unmount()
  })

  it('keeps the QR fallback hidden until the Alipay app launch times out', async () => {
    const originalLocation = window.location
    const originalHidden = Object.getOwnPropertyDescriptor(document, 'hidden')
    const assign = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { assign },
    })
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      get: () => false,
    })

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        amount: 88,
        payAmount: 88,
        qrCode: 'https://qr.alipay.com/dynamic-order-42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
        outTradeNo: 'sub2_20260420abcd1234',
        mobileAlipayDeepLink: true,
      },
      global: { stubs: { Icon: true } },
    })

    await flushPromises()
    expect(assign).toHaveBeenCalledWith(expect.stringContaining('alipays://platformapi/startapp?saId=10000007&qrcode='))
    expect(wrapper.find('[data-test="alipay-qr-fallback"]').exists()).toBe(false)

    await vi.advanceTimersByTimeAsync(2200)
    await flushPromises()

    expect(wrapper.find('[data-test="alipay-qr-fallback"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('payment.qr.saveQRCode')
    expect(wrapper.text()).toContain('sub2_20260420abcd1234')
    expect(toCanvas).toHaveBeenCalledWith(expect.any(HTMLCanvasElement), 'https://qr.alipay.com/dynamic-order-42', expect.any(Object))

    wrapper.unmount()
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
    if (originalHidden) Object.defineProperty(document, 'hidden', originalHidden)
  })

  it('does not show the QR fallback after the page enters the background', async () => {
    const originalLocation = window.location
    const originalHidden = Object.getOwnPropertyDescriptor(document, 'hidden')
    let hidden = false
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { assign: vi.fn() },
    })
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      get: () => hidden,
    })

    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        amount: 88,
        payAmount: 88,
        qrCode: 'https://qr.alipay.com/dynamic-order-42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
        outTradeNo: 'sub2_20260420abcd1234',
        mobileAlipayDeepLink: true,
      },
      global: { stubs: { Icon: true } },
    })

    await flushPromises()
    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(2200)
    await flushPromises()

    expect(wrapper.find('[data-test="alipay-qr-fallback"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('payment.qr.alipayContinueInApp')

    wrapper.unmount()
    Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
    if (originalHidden) Object.defineProperty(document, 'hidden', originalHidden)
  })

  it('renders qr_code_img directly as an image instead of generating a QR', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        payAmount: 10.37,
        qrCode: '',
        qrCodeImg: 'https://shop01.example.com/qr/42.png',
        payUrl: 'https://shop01.example.com/cashier/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
      },
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    const img = wrapper.get('[data-test="qr-code-img"]')
    expect(img.attributes('src')).toBe('https://shop01.example.com/qr/42.png')
    expect(wrapper.find('canvas').exists()).toBe(false)
    expect(toCanvas).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  const SHOP01_DATA_QR = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAACklEQVR4nGNgAAAAAgABSK+kcQAAAABJRU5ErkJggg=='

  function mountShop01Panel(extra: Record<string, unknown> = {}) {
    return mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        payAmount: 10.37,
        qrCode: '',
        qrCodeImg: SHOP01_DATA_QR,
        payUrl: 'http://shop01.example.com/p/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
        ...extra,
      },
      global: { stubs: { Icon: true } },
    })
  }

  it('shows the exact pay amount with a copy button above a gateway QR image', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    const wrapper = mountShop01Panel({ currency: 'CNY' })
    await flushPromises()

    const block = wrapper.get('[data-test="exact-pay-amount"]')
    expect(block.get('[data-test="exact-pay-amount-value"]').text()).toContain('10.37')
    expect(block.text()).toContain('payment.qr.exactAmountNote')
    const html = wrapper.html()
    expect(html.indexOf('data-test="exact-pay-amount"')).toBeLessThan(html.indexOf('data-test="qr-code-img"'))

    await wrapper.get('[data-test="copy-pay-amount"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('10.37', 'payment.qr.amountCopied')
    wrapper.unmount()
  })

  it('does not show the exact-amount block for providers without a gateway QR image', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        payAmount: 10.37,
        qrCode: 'https://pay.example.com/qr/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
      },
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="exact-pay-amount"]').exists()).toBe(false)
    expect(wrapper.find('canvas').exists()).toBe(true)
    wrapper.unmount()
  })

  it('renders a data: URI QR image inline with the exact amount, save hint and a new-tab cashier link', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    const fakeWin = { closed: false, close: vi.fn() }
    const openSpy = vi.spyOn(window, 'open').mockReturnValue(fakeWin as unknown as Window)
    const wrapper = mountShop01Panel()
    await flushPromises()

    expect(wrapper.get('[data-test="qr-code-img"]').attributes('src')).toBe(SHOP01_DATA_QR)
    expect(wrapper.find('canvas').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('payment.qr.payInNewWindowHint')
    expect(wrapper.get('[data-test="exact-pay-amount-value"]').text()).toContain('10.37')
    expect(wrapper.get('[data-test="qr-image-save-hint"]').text()).toBe('payment.qr.qrImageSaveHint')

    const cashier = wrapper.get('[data-test="open-cashier-tab"]')
    expect(cashier.text()).toContain('payment.qr.openCashier')
    await cashier.trigger('click')
    expect(openSpy).toHaveBeenCalledWith('http://shop01.example.com/p/42', '_blank')
    // still on the inline QR; never navigated away
    expect(wrapper.find('[data-test="qr-code-img"]').exists()).toBe(true)
    openSpy.mockRestore()
    wrapper.unmount()
  })

  it.each(['PAID', 'COMPLETED', 'FAILED', 'CANCELLED'])('closes the opened cashier window when the order becomes %s', async (status) => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    const fakeWin = { closed: false, close: vi.fn() }
    const openSpy = vi.spyOn(window, 'open').mockReturnValue(fakeWin as unknown as Window)
    const wrapper = mountShop01Panel()
    await flushPromises()
    await wrapper.get('[data-test="open-cashier-tab"]').trigger('click')
    expect(fakeWin.close).not.toHaveBeenCalled()

    pollOrderStatus.mockResolvedValue(orderFactory(status))
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(fakeWin.close).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('settled')).toHaveLength(1)
    if (status === 'PAID' || status === 'COMPLETED') {
      expect(wrapper.emitted('success')).toHaveLength(1)
    }
    wrapper.unmount()
    expect(fakeWin.close).toHaveBeenCalledTimes(1)
    openSpy.mockRestore()
  })

  it('closes a popup-mode cashier on unmount and tolerates close() throwing', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    const fakeWin = { closed: false, close: vi.fn(() => { throw new Error('cross-origin') }) }
    const openSpy = vi.spyOn(window, 'open').mockReturnValue(fakeWin as unknown as Window)
    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        payAmount: 10.05,
        qrCode: '',
        payUrl: 'https://pay.example.com/cashier/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
      },
      global: { stubs: { Icon: true } },
    })
    await flushPromises()
    const reopen = wrapper.get('[data-test="open-cashier"]')
    expect(reopen.text()).toContain('payment.qr.openPayWindow')
    await reopen.trigger('click')
    expect(openSpy).toHaveBeenCalledWith('https://pay.example.com/cashier/42', 'paymentPopup', expect.any(String))

    expect(() => wrapper.unmount()).not.toThrow()
    expect(fakeWin.close).toHaveBeenCalledTimes(1)
    openSpy.mockRestore()
  })

  it('falls back to a cashier button instead of a broken QR when the QR image fails to load', async () => {
    pollOrderStatus.mockResolvedValue(orderFactory('PENDING'))
    const openSpy = vi.spyOn(window, 'open').mockReturnValue({ closed: false } as Window)
    const wrapper = mount(PaymentStatusPanel, {
      props: {
        orderId: 42,
        payAmount: 10.37,
        qrCode: '',
        qrCodeImg: 'https://shop01.example.com/qr/broken.png',
        payUrl: 'https://shop01.example.com/cashier/42',
        expiresAt: '2099-01-01T12:30:00Z',
        paymentType: 'alipay',
        orderType: 'balance',
      },
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    await wrapper.get('[data-test="qr-code-img"]').trigger('error')
    expect(wrapper.find('[data-test="qr-code-img"]').exists()).toBe(false)
    expect(wrapper.find('canvas').exists()).toBe(false)
    const cashier = wrapper.get('[data-test="open-cashier"]')
    expect(cashier.text()).toContain('payment.qr.openCashier')
    expect(wrapper.get('[data-test="exact-pay-amount-value"]').text()).toContain('10.37')
    await cashier.trigger('click')
    expect(openSpy).toHaveBeenCalledWith('https://shop01.example.com/cashier/42', 'paymentPopup', expect.any(String))
    openSpy.mockRestore()
    wrapper.unmount()
  })
})
