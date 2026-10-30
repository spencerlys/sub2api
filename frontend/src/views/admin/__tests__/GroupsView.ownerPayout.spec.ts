import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminGroup } from '@/types'
import GroupsView from '@/views/admin/GroupsView.vue'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

const {
  listGroups,
  createGroup,
  updateGroup,
  getModelAllowlistCandidates,
  getUsageSummary,
  getCapacitySummary,
  getLiveCapability,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  listGroups: vi.fn(),
  createGroup: vi.fn(),
  updateGroup: vi.fn(),
  getModelAllowlistCandidates: vi.fn(),
  getUsageSummary: vi.fn(),
  getCapacitySummary: vi.fn(),
  getLiveCapability: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

const authState = vi.hoisted(() => ({ isSimpleMode: false, isOwner: false }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list: listGroups,
      duplicate: vi.fn(),
      getModelAllowlistCandidates,
      getUsageSummary,
      getCapacitySummary,
      getLiveCapability,
      getAll: vi.fn().mockResolvedValue([]),
      create: createGroup,
      update: updateGroup,
      delete: vi.fn(),
      updateSortOrder: vi.fn()
    },
    accounts: {
      list: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 }),
      getById: vi.fn()
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep: vi.fn(() => false),
    nextStep: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const baseGroup: AdminGroup = {
  id: 42,
  name: 'Primary',
  description: null,
  platform: 'anthropic',
  rate_multiplier: 1,
  rpm_limit: 0,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  allow_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  default_mapped_model: '',
  messages_dispatch_model_config: undefined,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: '2026-07-16T00:00:00Z',
  updated_at: '2026-07-16T00:00:00Z',
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  account_count: 1,
  active_account_count: 1,
  rate_limited_account_count: 0,
  model_allowlist: undefined,
  sort_order: 10,
  owner_payout_multiplier: 0.8
} as AdminGroup

const AppLayoutStub = defineComponent({ template: '<main><slot /></main>' })

const TablePageLayoutStub = defineComponent({
  template: '<section><slot name="filters" /><slot name="table" /><slot name="pagination" /></section>'
})

const DataTableStub = defineComponent({
  props: {
    data: { type: Array, default: () => [] },
    columns: { type: Array, default: () => [] },
    loading: { type: Boolean, default: false }
  },
  template: `
    <div>
      <div data-test="columns">{{ columns.map((col) => col.key).join(',') }}</div>
      <div v-for="row in data" :key="row.id">
        <span data-test="owner-payout-cell">
          <slot name="cell-owner_payout_multiplier" :row="row" :value="row.owner_payout_multiplier" />
        </span>
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `
})

const BaseDialogStub = defineComponent({
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const GroupI18nFieldStub = defineComponent({
  props: { modelValue: { type: String, default: '' }, field: { type: String, default: '' } },
  emits: ['update:modelValue'],
  template:
    '<input :data-field="field" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
})

function mountView() {
  return mount(GroupsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        Select: true,
        PlatformIcon: true,
        Icon: true,
        GroupI18nField: GroupI18nFieldStub,
        GroupCapacityBadge: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        VueDraggable: true
      }
    }
  })
}

type Wrapper = ReturnType<typeof mountView>

const CREATE_INPUT = '#create-group-form [data-testid="group-owner-payout-multiplier"]'
const EDIT_INPUT = '#edit-group-form [data-testid="group-owner-payout-multiplier"]'

async function openCreate(wrapper: Wrapper) {
  const button = wrapper.findAll('button').find((b) => b.text() === 'admin.groups.createGroup')
  expect(button).toBeTruthy()
  await button!.trigger('click')
  await flushPromises()
  await wrapper.get('#create-group-form input[data-field="name"]').setValue('New group')
}

async function openEdit(wrapper: Wrapper) {
  const button = wrapper.findAll('button').find((b) => b.text() === 'common.edit')
  expect(button).toBeTruthy()
  await button!.trigger('click')
  await flushPromises()
}

const columnKeys = (wrapper: Wrapper) =>
  wrapper.get('[data-test="columns"]').text().split(',').filter(Boolean)

describe('GroupsView owner payout multiplier', () => {
  beforeEach(() => {
    authState.isSimpleMode = false
    authState.isOwner = false
    localStorage.clear()
    vi.spyOn(console, 'error').mockImplementation(() => {})
    for (const fn of [
      listGroups,
      createGroup,
      updateGroup,
      getModelAllowlistCandidates,
      getUsageSummary,
      getCapacitySummary,
      getLiveCapability,
      showSuccess,
      showError
    ]) {
      fn.mockReset()
    }
    listGroups.mockResolvedValue({ items: [baseGroup], total: 1, page: 1, page_size: 20, pages: 1 })
    createGroup.mockResolvedValue({ ...baseGroup, id: 43 })
    updateGroup.mockResolvedValue(baseGroup)
    getModelAllowlistCandidates.mockResolvedValue([])
    getUsageSummary.mockResolvedValue([])
    getCapacitySummary.mockResolvedValue([])
    getLiveCapability.mockResolvedValue({ supported: false })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    localStorage.clear()
  })

  describe('admin', () => {
    it('shows the field on create with default 1 and sends it in the create payload', async () => {
      const wrapper = mountView()
      await flushPromises()
      await openCreate(wrapper)

      const input = wrapper.get<HTMLInputElement>(CREATE_INPUT)
      expect(input.element.value).toBe('1')
      expect(input.attributes('step')).toBe('0.01')
      expect(input.attributes('min')).toBe('0')
      expect(input.attributes('max')).toBe('10')
      expect(wrapper.text()).toContain('admin.groups.form.ownerPayoutMultiplierHint')

      await wrapper.get('#create-group-form').trigger('submit')
      await flushPromises()
      expect(createGroup).toHaveBeenCalledTimes(1)
      expect(createGroup.mock.calls[0][0]).toMatchObject({ owner_payout_multiplier: 1 })
      wrapper.unmount()
    })

    it('sends an edited value on create', async () => {
      const wrapper = mountView()
      await flushPromises()
      await openCreate(wrapper)

      await wrapper.get(CREATE_INPUT).setValue('0.65')
      await wrapper.get('#create-group-form').trigger('submit')
      await flushPromises()
      expect(createGroup.mock.calls[0][0]).toMatchObject({ owner_payout_multiplier: 0.65 })
      wrapper.unmount()
    })

    it('prefills the field on edit and sends the new value in the update payload', async () => {
      const wrapper = mountView()
      await flushPromises()
      await openEdit(wrapper)

      const input = wrapper.get<HTMLInputElement>(EDIT_INPUT)
      expect(input.element.value).toBe('0.8')
      expect(input.attributes('step')).toBe('0.01')
      expect(input.attributes('min')).toBe('0')
      expect(input.attributes('max')).toBe('10')

      await input.setValue('1.25')
      await wrapper.get('#edit-group-form').trigger('submit')
      await flushPromises()
      expect(updateGroup).toHaveBeenCalledTimes(1)
      expect(updateGroup.mock.calls[0][0]).toBe(42)
      expect(updateGroup.mock.calls[0][1]).toMatchObject({ owner_payout_multiplier: 1.25 })
      wrapper.unmount()
    })

    it('leaves the multiplier unchanged (omits it) when the edit input is cleared', async () => {
      const wrapper = mountView()
      await flushPromises()
      await openEdit(wrapper)

      await wrapper.get(EDIT_INPUT).setValue('')
      await wrapper.get('#edit-group-form').trigger('submit')
      await flushPromises()
      expect(updateGroup).toHaveBeenCalledTimes(1)
      expect(updateGroup.mock.calls[0][1]).not.toHaveProperty('owner_payout_multiplier')
      wrapper.unmount()
    })

    it('shows the owner payout column next to the rate multiplier and renders the value', async () => {
      const wrapper = mountView()
      await flushPromises()

      const keys = columnKeys(wrapper)
      expect(keys).toContain('owner_payout_multiplier')
      expect(keys.indexOf('owner_payout_multiplier')).toBe(keys.indexOf('rate_multiplier') + 1)
      expect(wrapper.get('[data-test="owner-payout-cell"]').text()).toBe('0.8x')
      wrapper.unmount()
    })
  })

  describe('owner', () => {
    beforeEach(() => {
      authState.isOwner = true
      // Backend omits the field for owner requests.
      const { owner_payout_multiplier: _omit, ...ownerView } = baseGroup
      listGroups.mockResolvedValue({ items: [ownerView], total: 1, page: 1, page_size: 20, pages: 1 })
    })

    it('hides the field on create and never sends it', async () => {
      const wrapper = mountView()
      await flushPromises()
      await openCreate(wrapper)

      expect(wrapper.find(CREATE_INPUT).exists()).toBe(false)
      await wrapper.get('#create-group-form').trigger('submit')
      await flushPromises()
      expect(createGroup).toHaveBeenCalledTimes(1)
      expect(createGroup.mock.calls[0][0]).not.toHaveProperty('owner_payout_multiplier')
      wrapper.unmount()
    })

    it('hides the field on edit and never sends it', async () => {
      const wrapper = mountView()
      await flushPromises()
      await openEdit(wrapper)

      expect(wrapper.find(EDIT_INPUT).exists()).toBe(false)
      await wrapper.get('#edit-group-form').trigger('submit')
      await flushPromises()
      expect(updateGroup).toHaveBeenCalledTimes(1)
      expect(updateGroup.mock.calls[0][1]).not.toHaveProperty('owner_payout_multiplier')
      wrapper.unmount()
    })

    it('does not show the owner payout column', async () => {
      const wrapper = mountView()
      await flushPromises()

      expect(columnKeys(wrapper)).not.toContain('owner_payout_multiplier')
      wrapper.unmount()
    })
  })

  it('defines label, hint and column header in both locales', () => {
    const enGroups = (en as any).admin.groups
    const zhGroups = (zh as any).admin.groups
    expect(enGroups.form.ownerPayoutMultiplier).toBe('Owner payout rate')
    expect(zhGroups.form.ownerPayoutMultiplier).toBe('号主分成倍率')
    expect(enGroups.columns.ownerPayoutMultiplier).toBe('Owner payout rate')
    expect(zhGroups.columns.ownerPayoutMultiplier).toBe('号主分成倍率')
    expect(enGroups.form.ownerPayoutMultiplierHint).toBe(
      'Owner payable = official price × owner payout rate × paid-call share; changes apply only to requests after the change'
    )
    expect(zhGroups.form.ownerPayoutMultiplierHint).toBe(
      '应付号主 = 官方价 × 号主分成倍率 × 付费调用部分；修改只影响之后的请求'
    )
  })
})
