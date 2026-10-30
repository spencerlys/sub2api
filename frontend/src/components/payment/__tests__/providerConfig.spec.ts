import { describe, expect, it } from 'vitest'
import {
  PAYMENT_CURRENCY_OPTIONS,
  PROVIDER_CALLBACK_PATHS,
  PROVIDER_CONFIG_FIELDS,
  PROVIDER_SUPPORTED_TYPES,
  WEBHOOK_PATHS,
  isBuiltInAlipayMethod,
  isSensitiveConfigField,
  resolveFieldInputType,
  isBuiltInWxpayMethod,
  parseEasyPayCustomMethods,
  serializeEasyPayCustomMethods,
} from '@/components/payment/providerConfig'

function findField(providerKey: string, key: string) {
  const fields = PROVIDER_CONFIG_FIELDS[providerKey] || []
  return fields.find(field => field.key === key)
}

describe('PROVIDER_CONFIG_FIELDS.wxpay', () => {
  it('keeps admin form validation aligned with backend-required credentials', () => {
    expect(findField('wxpay', 'publicKeyId')?.optional).toBeFalsy()
    expect(findField('wxpay', 'certSerial')?.optional).toBeFalsy()
  })

  it('only keeps the simplified visible credential set in the admin form', () => {
    expect(findField('wxpay', 'mpAppId')).toBeUndefined()
    expect(findField('wxpay', 'h5AppName')).toBeUndefined()
    expect(findField('wxpay', 'h5AppUrl')).toBeUndefined()
  })
})

describe('PROVIDER_CONFIG_FIELDS.airwallex', () => {
  it('adds currency config with CNY as the default', () => {
    const currency = findField('airwallex', 'currency')

    expect(currency?.defaultValue).toBe('CNY')
    expect(currency?.hintKey).toBe('admin.settings.payment.field_paymentCurrencyHint')
    expect(currency?.options).toBe(PAYMENT_CURRENCY_OPTIONS)
  })

  it('marks accountId as optional and explains when it can be left blank', () => {
    const accountId = findField('airwallex', 'accountId')

    expect(accountId?.optional).toBe(true)
    expect(accountId?.clearable).toBe(true)
    expect(accountId?.hintKey).toBe('admin.settings.payment.field_accountIdHint')
  })

  it('explains that apiBase must match the Airwallex key environment', () => {
    expect(findField('airwallex', 'apiBase')?.hintKey).toBe('admin.settings.payment.field_airwallexApiBaseHint')
  })
})

describe('PROVIDER_CONFIG_FIELDS.stripe', () => {
  it('adds currency config with CNY as the default', () => {
    const currency = findField('stripe', 'currency')

    expect(currency?.defaultValue).toBe('CNY')
    expect(currency?.hintKey).toBe('admin.settings.payment.field_paymentCurrencyHint')
    expect(currency?.options).toBe(PAYMENT_CURRENCY_OPTIONS)
  })
})

describe('shop01 provider config', () => {
  it('declares merchantNo, merchantKey, apiBase as form fields and notifyUrl as callback path', () => {
    expect(PROVIDER_CONFIG_FIELDS.shop01.map(f => f.key)).toEqual(['merchantNo', 'merchantKey', 'apiBase'])
    expect(PROVIDER_CALLBACK_PATHS.shop01).toEqual({ notifyUrl: '/api/v1/payment/webhook/shop01' })
  })

  it('marks only merchantKey as sensitive and renders it as a password input', () => {
    const merchantKey = findField('shop01', 'merchantKey')
    expect(merchantKey?.sensitive).toBe(true)
    expect(merchantKey?.inputType).toBe('password')
    expect(resolveFieldInputType(merchantKey!)).toBe('password')
    expect(PROVIDER_CONFIG_FIELDS.shop01.filter(f => f.sensitive).map(f => f.key)).toEqual(['merchantKey'])
    expect(isSensitiveConfigField('shop01', 'merchantKey')).toBe(true)
    expect(isSensitiveConfigField('shop01', 'merchantNo')).toBe(false)
  })

  it('supports alipay only and uses the shop01 webhook path', () => {
    expect(PROVIDER_SUPPORTED_TYPES.shop01).toEqual(['alipay'])
    expect(WEBHOOK_PATHS.shop01).toBe('/api/v1/payment/webhook/shop01')
  })

  it('uses a placeholder for apiBase without applying it as a default value', () => {
    const apiBase = findField('shop01', 'apiBase')
    expect(apiBase?.placeholder).toBe('http://host:5000')
    expect(apiBase?.defaultValue).toBeUndefined()
  })
})

describe('resolveFieldInputType legacy heuristic', () => {
  it('keeps existing providers rendering unchanged', () => {
    expect(resolveFieldInputType(findField('alipay', 'privateKey')!)).toBe('textarea')
    expect(resolveFieldInputType(findField('alipay', 'publicKey')!)).toBe('textarea')
    expect(resolveFieldInputType(findField('easypay', 'pkey')!)).toBe('password')
    expect(resolveFieldInputType(findField('stripe', 'webhookSecret')!)).toBe('password')
    expect(resolveFieldInputType(findField('stripe', 'secretKey')!)).toBe('textarea')
    expect(resolveFieldInputType(findField('stripe', 'currency')!)).toBe('select')
    expect(resolveFieldInputType(findField('alipay', 'appId')!)).toBe('text')
  })
})

describe('EasyPay custom methods config', () => {
  it('parses customMethods from the JSON string stored in provider config', () => {
    expect(parseEasyPayCustomMethods(
      '[{"type":"ldc","upstreamType":"epay","displayName":"LDC"},{"type":"usdt_trc20","upstreamType":"usdt","displayName":"USDT-TRC20"}]',
    )).toEqual([
      { type: 'ldc', upstreamType: 'epay', displayName: 'LDC' },
      { type: 'usdt_trc20', upstreamType: 'usdt', displayName: 'USDT-TRC20' },
    ])
  })

  it('serializes non-empty custom methods into the config string format', () => {
    expect(serializeEasyPayCustomMethods([
      { type: 'ldc', upstreamType: 'epay', displayName: 'LDC' },
      { type: '  ', upstreamType: 'ignored', displayName: 'Ignored' },
      { type: 'usdt_trc20', upstreamType: 'usdt', displayName: '' },
    ])).toBe('[{"type":"ldc","upstreamType":"epay","displayName":"LDC"},{"type":"usdt_trc20","upstreamType":"usdt","displayName":""}]')
  })

  it('returns an empty string for invalid or empty custom methods', () => {
    expect(parseEasyPayCustomMethods('not-json')).toEqual([])
    expect(serializeEasyPayCustomMethods([{ type: '', upstreamType: 'epay', displayName: 'LDC' }])).toBe('')
  })
})

describe('built-in payment method helpers', () => {
  it('only treats exact built-in aliases as Alipay or WeChat Pay', () => {
    expect(isBuiltInAlipayMethod('alipay')).toBe(true)
    expect(isBuiltInAlipayMethod('alipay_direct')).toBe(true)
    expect(isBuiltInAlipayMethod('card_alipay')).toBe(false)

    expect(isBuiltInWxpayMethod('wxpay')).toBe(true)
    expect(isBuiltInWxpayMethod('wxpay_direct')).toBe(true)
    expect(isBuiltInWxpayMethod('card_wxpay')).toBe(false)
  })
})
