import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import PaymentProviderDialog from '@/components/payment/PaymentProviderDialog.vue'
import { STRIPE_SDK_API_VERSION } from '@/components/payment/providerConfig'
import type { ProviderInstance } from '@/types/payment'

const messages: Record<string, string> = {
  'admin.settings.payment.providerConfig': 'Credentials',
  'admin.settings.payment.easypayCustomMethods': 'Custom EasyPay methods',
  'admin.settings.payment.easypayCustomMethodsHint': 'Add provider-specific EasyPay type values.',
  'admin.settings.payment.addCustomMethod': 'Add method',
  'admin.settings.payment.customMethodType': 'Payment type',
  'admin.settings.payment.customMethodUpstreamType': 'Upstream type',
  'admin.settings.payment.customMethodDisplayName': 'Display name',
  'admin.settings.payment.customMethodDisplayNamePlaceholder': '信用卡',
  'admin.settings.payment.paymentGuideTrigger': 'View payment guide',
  'admin.settings.payment.alipayGuideSummary': 'Desktop prefers QR precreate and falls back to cashier; mobile prefers WAP checkout.',
  'admin.settings.payment.wxpayGuideSummary': 'Desktop prefers Native QR; mobile routes to JSAPI or H5 based on browser context.',
  'admin.settings.payment.airwallexGuideSummary': 'Use Payment Acceptance read/write only.',
  'admin.settings.payment.stripeWebhookHint': 'Configure Stripe webhook.',
  'admin.settings.payment.stripeWebhookApiVersionHint': 'Use Stripe API version {version}.',
  'admin.settings.payment.airwallexWebhookHint': 'Select payment_intent.succeeded and use the latest stable API version.',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string>) => {
      const message = messages[key] ?? key
      if (!params) return message
      return Object.entries(params).reduce(
        (value, [name, replacement]) => value.replaceAll(`{${name}}`, replacement),
        message,
      )
    },
  }),
}))

function providerFactory(overrides: Partial<ProviderInstance> = {}): ProviderInstance {
  return {
    id: 1,
    provider_key: 'airwallex',
    name: 'Airwallex',
    config: {},
    supported_types: ['airwallex'],
    enabled: true,
    payment_mode: '',
    refund_enabled: false,
    allow_user_refund: false,
    limits: '',
    sort_order: 0,
    ...overrides,
  }
}

function mountDialog(options: { editing?: ProviderInstance | null } = {}) {
  return mount(PaymentProviderDialog, {
    props: {
      show: true,
      saving: false,
      editing: options.editing ?? null,
      allKeyOptions: [
        { value: 'easypay', label: 'EasyPay' },
        { value: 'alipay', label: 'Alipay' },
        { value: 'wxpay', label: 'WeChat Pay' },
        { value: 'stripe', label: 'Stripe' },
        { value: 'airwallex', label: 'Airwallex' },
      ],
      enabledKeyOptions: [
        { value: 'easypay', label: 'EasyPay' },
        { value: 'alipay', label: 'Alipay' },
        { value: 'wxpay', label: 'WeChat Pay' },
        { value: 'airwallex', label: 'Airwallex' },
      ],
      allPaymentTypes: [
        { value: 'alipay', label: 'Alipay' },
        { value: 'wxpay', label: 'WeChat Pay' },
      ],
      redirectLabel: 'Redirect',
    },
    global: {
      stubs: {
        BaseDialog: {
          template: '<div><slot /><slot name="footer" /></div>',
        },
        Select: {
          props: ['modelValue', 'options', 'disabled'],
          template: '<div />',
        },
        ToggleSwitch: {
          template: '<div />',
        },
      },
    },
  })
}

describe('PaymentProviderDialog callback URLs', () => {
  it.each([
    ['https://notify.example.com/', 'https://return.example.com///', 'https://notify.example.com', 'https://return.example.com'],
    [' https://notify.example.com/sub/ ', ' https://return.example.com/site/ ', 'https://notify.example.com/sub', 'https://return.example.com/site'],
    ['https://notify.example.com', 'https://return.example.com', 'https://notify.example.com', 'https://return.example.com'],
    ['', '', window.location.origin, window.location.origin],
  ])('joins callback paths to %s and %s', async (notify, returnUrl, expectedNotify, expectedReturn) => {
    const provider = providerFactory({
      provider_key: 'easypay', name: 'EasyPay',
      config: { pid: 'pid-1', apiBase: 'https://pay.example.com' },
      supported_types: ['alipay'], payment_mode: 'qrcode',
    })
    const wrapper = mountDialog({ editing: provider })
    ;(wrapper.vm as unknown as { loadProvider: (provider: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()
    const bases = wrapper.findAll('input').filter(input => input.classes().includes('!rounded-r-none'))
    await bases[0].setValue(notify)
    await bases[1].setValue(returnUrl)
    await wrapper.find('form').trigger('submit')
    const payload = wrapper.emitted('save')?.[0]?.[0] as { config: Record<string, string> }
    expect(payload.config.notifyUrl).toBe(expectedNotify + '/api/v1/payment/webhook/easypay')
    expect(payload.config.returnUrl).toBe(expectedReturn + '/payment/result')
    wrapper.unmount()
  })
})

describe('PaymentProviderDialog shop01', () => {
  type DialogApi = { reset: (key: string) => void; loadProvider: (provider: ProviderInstance) => void }
  type SavePayload = { provider_key: string; supported_types: string[]; payment_mode: string; config: Record<string, string> }

  function shop01Provider(config: Record<string, string>): ProviderInstance {
    return providerFactory({
      provider_key: 'shop01',
      name: 'Shop01',
      config,
      supported_types: ['alipay'],
    })
  }

  it('renders merchantKey as a single-line password input, not a textarea', async () => {
    const wrapper = mountDialog()
    ;(wrapper.vm as unknown as DialogApi).reset('shop01')
    await nextTick()

    expect(wrapper.find('textarea').exists()).toBe(false)
    const passwordInputs = wrapper.findAll('input[type="password"]')
    expect(passwordInputs).toHaveLength(1)
    expect(wrapper.text()).toContain('admin.settings.payment.field_merchantKey')
    expect(wrapper.text()).toContain('/api/v1/payment/webhook/shop01')
    const apiBaseInput = wrapper
      .findAll('input[type="text"]')
      .find(input => (input.element as HTMLInputElement).placeholder === 'http://host:5000')
    expect(apiBaseInput).toBeDefined()
  })

  it('emits all four config keys on create', async () => {
    const wrapper = mountDialog()
    ;(wrapper.vm as unknown as DialogApi).reset('shop01')
    await nextTick()

    // Text inputs in DOM order: provider name, merchantNo, apiBase, notify base URL.
    const [nameInput, merchantNoInput] = wrapper.findAll('input[type="text"]')
    await nameInput.setValue('Shop01 main')
    await merchantNoInput.setValue('M10001')
    await wrapper.find('input[type="password"]').setValue('secret-key')
    const apiBaseInput = wrapper
      .findAll('input[type="text"]')
      .find(input => (input.element as HTMLInputElement).placeholder === 'http://host:5000')!
    await apiBaseInput.setValue('http://10.0.0.5:5000')
    const notifyBase = wrapper.findAll('input').find(input => input.classes().includes('!rounded-r-none'))!
    await notifyBase.setValue('https://cc.example.com/')
    await wrapper.find('form').trigger('submit.prevent')

    const payload = wrapper.emitted('save')?.[0]?.[0] as SavePayload
    expect(payload.provider_key).toBe('shop01')
    expect(payload.supported_types).toEqual(['alipay'])
    expect(payload.payment_mode).toBe('qrcode')
    expect(payload.config).toEqual({
      merchantNo: 'M10001',
      merchantKey: 'secret-key',
      apiBase: 'http://10.0.0.5:5000',
      notifyUrl: 'https://cc.example.com/api/v1/payment/webhook/shop01',
    })
  })

  it('never echoes a stored/masked merchantKey on edit and omits it when left empty', async () => {
    const provider = shop01Provider({
      merchantNo: 'M10001',
      merchantKey: '********',
      apiBase: 'http://10.0.0.5:5000',
      notifyUrl: 'https://cc.example.com/api/v1/payment/webhook/shop01',
    })
    const wrapper = mountDialog({ editing: provider })
    ;(wrapper.vm as unknown as DialogApi).loadProvider(provider)
    await nextTick()

    const secretInput = wrapper.find('input[type="password"]')
    expect((secretInput.element as HTMLInputElement).value).toBe('')
    expect((secretInput.element as HTMLInputElement).placeholder).toBe('admin.accounts.leaveEmptyToKeep')
    expect(wrapper.html()).not.toContain('********')

    await wrapper.find('form').trigger('submit.prevent')

    const payload = wrapper.emitted('save')?.[0]?.[0] as SavePayload
    expect(payload.config).not.toHaveProperty('merchantKey')
    expect(payload.config).toEqual({
      merchantNo: 'M10001',
      apiBase: 'http://10.0.0.5:5000',
      notifyUrl: 'https://cc.example.com/api/v1/payment/webhook/shop01',
    })
  })

  it('sends a newly typed merchantKey on edit', async () => {
    const provider = shop01Provider({
      merchantNo: 'M10001',
      apiBase: 'http://10.0.0.5:5000',
      notifyUrl: 'https://cc.example.com/api/v1/payment/webhook/shop01',
    })
    const wrapper = mountDialog({ editing: provider })
    ;(wrapper.vm as unknown as DialogApi).loadProvider(provider)
    await nextTick()

    await wrapper.find('input[type="password"]').setValue('rotated-key')
    await wrapper.find('form').trigger('submit.prevent')

    const payload = wrapper.emitted('save')?.[0]?.[0] as SavePayload
    expect(payload.config.merchantKey).toBe('rotated-key')
  })

  it.each(['qrcode', ''])('re-saves shop01 with payment_mode "qrcode" (stored %j), never resetting it to ""', async (stored) => {
    const provider = { ...shop01Provider({
      merchantNo: 'M10001',
      apiBase: 'http://10.0.0.5:5000',
      notifyUrl: 'https://cc.example.com/api/v1/payment/webhook/shop01',
    }), payment_mode: stored }
    const wrapper = mountDialog({ editing: provider })
    ;(wrapper.vm as unknown as DialogApi).loadProvider(provider)
    await nextTick()

    await wrapper.find('form').trigger('submit.prevent')

    const payload = wrapper.emitted('save')?.[0]?.[0] as SavePayload
    expect(payload.payment_mode).toBe('qrcode')
    wrapper.unmount()
  })

  it('keeps alipay private/public keys as textareas', async () => {
    const wrapper = mountDialog()
    ;(wrapper.vm as unknown as DialogApi).reset('alipay')
    await nextTick()

    expect(wrapper.findAll('textarea')).toHaveLength(2)
    expect(wrapper.findAll('input[type="password"]')).toHaveLength(0)
  })
})

describe('PaymentProviderDialog payment guide', () => {
  it('shows no payment guide for providers without a flow guide', () => {
    const wrapper = mountDialog()

    expect(wrapper.text()).not.toContain(messages['admin.settings.payment.alipayGuideSummary'])
    expect(wrapper.text()).not.toContain(messages['admin.settings.payment.wxpayGuideSummary'])
    expect(wrapper.find('button[title="View payment guide"]').exists()).toBe(false)
  })

  it.each([
    ['alipay', 'admin.settings.payment.alipayGuideSummary'],
    ['wxpay', 'admin.settings.payment.wxpayGuideSummary'],
    ['airwallex', 'admin.settings.payment.airwallexGuideSummary'],
  ])('shows the payment guide summary for %s', async (providerKey, summaryKey) => {
    const wrapper = mountDialog()

    ;(wrapper.vm as unknown as { reset: (key: string) => void }).reset(providerKey)
    await nextTick()

    expect(wrapper.text()).toContain(messages[summaryKey])
    expect(wrapper.find('button[title="View payment guide"]').exists()).toBe(true)
  })

  it('shows Airwallex webhook event and API version guidance with the webhook URL', async () => {
    const wrapper = mountDialog()

    ;(wrapper.vm as unknown as { reset: (key: string) => void }).reset('airwallex')
    await nextTick()

    expect(wrapper.text()).toContain(messages['admin.settings.payment.airwallexWebhookHint'])
    expect(wrapper.text()).toContain('/api/v1/payment/webhook/airwallex')
  })

  it('shows Stripe webhook API version guidance with the integrated SDK version', async () => {
    const wrapper = mountDialog()

    ;(wrapper.vm as unknown as { reset: (key: string) => void }).reset('stripe')
    await nextTick()

    expect(wrapper.text()).toContain(messages['admin.settings.payment.stripeWebhookHint'])
    expect(wrapper.text()).toContain(`Use Stripe API version ${STRIPE_SDK_API_VERSION}.`)
    expect(wrapper.text()).toContain('/api/v1/payment/webhook/stripe')
  })

  it('emits an empty Airwallex accountId when the admin clears it', async () => {
    const provider = providerFactory({
      config: {
        clientId: 'cid_123',
        apiBase: 'https://api.airwallex.com/api/v1',
        countryCode: 'CN',
        currency: 'CNY',
        accountId: 'acct_123',
      },
    })
    const wrapper = mountDialog({ editing: provider })

    ;(wrapper.vm as unknown as { loadProvider: (provider: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()

    const accountIdInput = wrapper
      .findAll('input[type="text"]')
      .find(input => (input.element as HTMLInputElement).value === 'acct_123')
    if (!accountIdInput) throw new Error('accountId input not found')

    await accountIdInput.setValue('')
    await wrapper.find('form').trigger('submit.prevent')

    const payload = wrapper.emitted('save')?.[0]?.[0] as { config: Record<string, string> }
    expect(payload.config.accountId).toBe('')
  })

  it.each(['epay', 'usdt.trc20'])('serializes EasyPay upstream type %s and adds the local type to supported_types', async (upstreamType) => {
    const provider = providerFactory({
      provider_key: 'easypay',
      name: 'EasyPay',
      config: {
        pid: 'pid-1',
        apiBase: 'https://pay.example.com',
        notifyUrl: 'https://example.com/api/v1/payment/webhook/easypay',
        returnUrl: 'https://example.com/payment/result',
      },
      supported_types: ['alipay', 'wxpay'],
      payment_mode: 'qrcode',
    })
    const wrapper = mountDialog({ editing: provider })

    ;(wrapper.vm as unknown as { loadProvider: (provider: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()

    await wrapper.find('button.btn-sm').trigger('click')
    await nextTick()

    const inputs = wrapper.findAll('input[type="text"]')
    const customTypeInputs = inputs.filter(input => (input.element as HTMLInputElement).placeholder === 'credit_card')
    const ldcTypeInput = customTypeInputs[0]
    const upstreamTypeInput = customTypeInputs[1]
    const displayNameInput = inputs.find(input => (input.element as HTMLInputElement).placeholder === '信用卡')
    if (!ldcTypeInput || !upstreamTypeInput || !displayNameInput) {
      throw new Error('custom method inputs not found')
    }

    await ldcTypeInput.setValue('ldc')
    await upstreamTypeInput.setValue(upstreamType)
    await displayNameInput.setValue('LDC')
    await wrapper.find('form').trigger('submit.prevent')

    const payload = wrapper.emitted('save')?.[0]?.[0] as {
      config: Record<string, string>
      supported_types: string[]
    }
    expect(JSON.parse(payload.config.customMethods)).toEqual([{ type: 'ldc', upstreamType, displayName: 'LDC' }])
    expect(payload.supported_types).toEqual(['alipay', 'wxpay', 'ldc'])
  })

  it.each([
    ['alipay_hk', 'hkpay'],
    ['usdt.trc20', 'usdt.trc20'],
    ['usdt_trc20', 'usdt/trc20'],
  ])('rejects invalid EasyPay mapping %s to %s', async (type, upstreamType) => {
    const provider = providerFactory({
      provider_key: 'easypay',
      name: 'EasyPay',
      config: {
        pid: 'pid-1',
        apiBase: 'https://pay.example.com',
        notifyUrl: 'https://example.com/api/v1/payment/webhook/easypay',
        returnUrl: 'https://example.com/payment/result',
      },
      supported_types: ['alipay', 'wxpay'],
      payment_mode: 'qrcode',
    })
    const wrapper = mountDialog({ editing: provider })

    ;(wrapper.vm as unknown as { loadProvider: (provider: ProviderInstance) => void }).loadProvider(provider)
    await nextTick()

    await wrapper.find('button.btn-sm').trigger('click')
    await nextTick()

    const inputs = wrapper.findAll('input[type="text"]')
    const customTypeInputs = inputs.filter(input => (input.element as HTMLInputElement).placeholder === 'credit_card')
    const typeInput = customTypeInputs[0]
    const upstreamTypeInput = customTypeInputs[1]
    const displayNameInput = inputs.find(input => (input.element as HTMLInputElement).placeholder === '信用卡')
    if (!typeInput || !upstreamTypeInput || !displayNameInput) {
      throw new Error('custom method inputs not found')
    }

    await typeInput.setValue(type)
    await upstreamTypeInput.setValue(upstreamType)
    await displayNameInput.setValue('Custom payment')
    await wrapper.find('form').trigger('submit.prevent')

    expect(wrapper.emitted('save')).toBeUndefined()
  })
})
