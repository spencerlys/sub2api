import { defineComponent, nextTick } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import MonitorFormDialog from '@/components/admin/monitor/MonitorFormDialog.vue'

const {
  listTemplates,
  accountsList,
  proxiesGetAll,
  monitorCreate,
  monitorUpdate,
} = vi.hoisted(() => ({
  listTemplates: vi.fn(),
  accountsList: vi.fn(),
  proxiesGetAll: vi.fn(),
  monitorCreate: vi.fn(),
  monitorUpdate: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channelMonitor: {
      create: monitorCreate,
      update: monitorUpdate,
    },
    channelMonitorTemplate: {
      list: listTemplates,
    },
    accounts: {
      list: (...args: unknown[]) => accountsList(...args),
      getById: vi.fn(),
    },
    proxies: {
      getAll: (...args: unknown[]) => proxiesGetAll(...args),
    },
  },
}))

vi.mock('@/api/keys', () => ({
  keysAPI: { list: vi.fn() },
}))

vi.mock('@/api/groups', () => ({
  userGroupsAPI: { getUserGroupRates: vi.fn() },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: null,
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const BaseDialogStub = defineComponent({
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

const PROXIES = [
  { id: 3, name: 'sg-egress', protocol: 'http', host: '10.0.0.3', port: 3128, username: null, status: 'active' },
  { id: 4, name: 'us-egress', protocol: 'socks5', host: '10.0.0.4', port: 1080, username: null, status: 'active' },
]

function makeMonitor(overrides: Partial<ChannelMonitor> = {}): ChannelMonitor {
  return {
    id: 42,
    name: 'primary',
    provider: 'openai',
    api_mode: 'chat_completions',
    endpoint: 'https://api.example.com',
    api_key_masked: 'sk-t***',
    primary_model: 'gpt-4o-mini',
    extra_models: [],
    group_name: '',
    enabled: true,
    interval_seconds: 60,
    jitter_seconds: 0,
    last_checked_at: null,
    created_by: 1,
    created_at: '2026-07-16T00:00:00Z',
    updated_at: '2026-07-16T00:00:00Z',
    primary_status: '',
    primary_latency_ms: null,
    availability_7d: 0,
    extra_models_status: [],
    template_id: null,
    extra_headers: {},
    body_override_mode: 'off',
    body_override: null,
    check_mode: 'probe',
    account_id: null,
    proxy_id: null,
    proxy_name: '',
    ...overrides,
  }
}

let unmountWrapper: (() => void) | undefined

function mountDialog(monitor: ChannelMonitor | null = null) {
  const wrapper = mount(MonitorFormDialog, {
    props: { show: true, monitor },
    attachTo: document.body,
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Toggle: true,
        ModelTagInput: true,
        MonitorKeyPickerDialog: true,
        MonitorAdvancedRequestConfig: true,
      },
    },
  })
  unmountWrapper = () => wrapper.unmount()
  return wrapper
}

async function pickProxy(wrapper: VueWrapper, label: string) {
  await wrapper.get('[data-testid="monitor-proxy"] button').trigger('click')
  await nextTick()
  const option = wrapper
    .findAll('[data-testid="monitor-proxy"] .select-option')
    .find((el) => el.text().includes(label))
  expect(option, `proxy option containing "${label}" not found`).toBeDefined()
  await option!.trigger('click')
  await nextTick()
}

async function fillRequiredCreateFields(wrapper: VueWrapper) {
  await wrapper.findAll('input[type="text"]')[0].setValue('my monitor')
  await wrapper.get('[data-testid="monitor-primary-model"]').setValue('claude-sonnet-4-5')
}

afterEach(() => {
  unmountWrapper?.()
  unmountWrapper = undefined
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

describe('MonitorFormDialog probe proxy selector', () => {
  beforeEach(() => {
    listTemplates.mockReset().mockResolvedValue({ items: [] })
    accountsList.mockReset().mockResolvedValue({ items: [] })
    proxiesGetAll.mockReset().mockResolvedValue(PROXIES)
    monitorCreate.mockReset().mockResolvedValue({})
    monitorUpdate.mockReset().mockResolvedValue({})
  })

  it('submits the picked proxy_id when creating a probe monitor', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    expect(proxiesGetAll).toHaveBeenCalledTimes(1)

    await fillRequiredCreateFields(wrapper)
    await pickProxy(wrapper, 'sg-egress')
    await wrapper.get('#channel-monitor-form').trigger('submit')
    await flushPromises()

    expect(monitorCreate).toHaveBeenCalledWith(expect.objectContaining({
      check_mode: 'probe',
      proxy_id: 3,
    }))
  })

  it('sends proxy_id null when no proxy is picked', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    await fillRequiredCreateFields(wrapper)
    await wrapper.get('#channel-monitor-form').trigger('submit')
    await flushPromises()

    expect(monitorCreate).toHaveBeenCalledWith(expect.objectContaining({ proxy_id: null }))
  })

  it('keeps the stored proxy on edit and sends null after switching to no proxy', async () => {
    const wrapper = mountDialog(makeMonitor({ proxy_id: 4, proxy_name: 'us-egress' }))
    await flushPromises()

    expect(wrapper.get('[data-testid="monitor-proxy"] button').text()).toContain('us-egress')
    await wrapper.get('#channel-monitor-form').trigger('submit')
    await flushPromises()
    expect(monitorUpdate).toHaveBeenLastCalledWith(42, expect.objectContaining({ proxy_id: 4 }))

    await pickProxy(wrapper, 'admin.accounts.noProxy')
    await wrapper.get('#channel-monitor-form').trigger('submit')
    await flushPromises()
    expect(monitorUpdate).toHaveBeenLastCalledWith(42, expect.objectContaining({ proxy_id: null }))
  })

  it('hides the proxy selector in quota-only mode', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    expect(wrapper.find('[data-testid="monitor-proxy"]').exists()).toBe(true)

    await wrapper.get('[data-testid="monitor-check-mode-quota"]').trigger('click')
    await nextTick()
    expect(wrapper.find('[data-testid="monitor-proxy"]').exists()).toBe(false)
  })

  it('does not block the form when loading proxies fails', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    proxiesGetAll.mockRejectedValue(new Error('boom'))
    const wrapper = mountDialog()
    await flushPromises()

    await fillRequiredCreateFields(wrapper)
    await wrapper.get('#channel-monitor-form').trigger('submit')
    await flushPromises()

    expect(monitorCreate).toHaveBeenCalledWith(expect.objectContaining({ proxy_id: null }))
  })
})
