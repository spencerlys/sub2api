import { DriveStep } from 'driver.js'

/**
 * 管理员完整引导流程
 * 交互式引导：指引用户实际操作
 * @param t 国际化函数
 * @param isSimpleMode 是否为简易模式（简易模式下会过滤分组相关步骤）
 */
export const getAdminSteps = (t: (key: string) => string, isSimpleMode = false): DriveStep[] => {
  const allSteps: DriveStep[] = [
  // ========== 欢迎介绍 ==========
  {
    popover: {
      title: t('onboarding.admin.welcome.title'),
      description: t('onboarding.admin.welcome.description'),
      align: 'center',
      nextBtnText: t('onboarding.admin.welcome.nextBtn'),
      prevBtnText: t('onboarding.admin.welcome.prevBtn')
    }
  },

  // ========== 第一部分：创建分组 ==========
  {
    element: '#sidebar-group-manage',
    popover: {
      title: t('onboarding.admin.groupManage.title'),
      description: t('onboarding.admin.groupManage.description'),
      side: 'right',
      align: 'center',
      showButtons: ['close'],
    }
  },
  {
    element: '[data-tour="groups-create-btn"]',
    popover: {
      title: t('onboarding.admin.createGroup.title'),
      description: t('onboarding.admin.createGroup.description'),
      side: 'bottom',
      align: 'end',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="group-form-name"]',
    popover: {
      title: t('onboarding.admin.groupName.title'),
      description: t('onboarding.admin.groupName.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="group-form-platform"]',
    popover: {
      title: t('onboarding.admin.groupPlatform.title'),
      description: t('onboarding.admin.groupPlatform.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="group-form-multiplier"]',
    popover: {
      title: t('onboarding.admin.groupMultiplier.title'),
      description: t('onboarding.admin.groupMultiplier.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="group-form-exclusive"]',
    popover: {
      title: t('onboarding.admin.groupExclusive.title'),
      description: t('onboarding.admin.groupExclusive.description'),
      side: 'top',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="group-form-submit"]',
    popover: {
      title: t('onboarding.admin.groupSubmit.title'),
      description: t('onboarding.admin.groupSubmit.description'),
      side: 'left',
      align: 'center',
      showButtons: ['close']
    }
  },

  // ========== 第二部分：创建账号授权 ==========
  {
    element: '#sidebar-channel-manage',
    popover: {
      title: t('onboarding.admin.accountManage.title'),
      description: t('onboarding.admin.accountManage.description'),
      side: 'right',
      align: 'center',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="accounts-create-btn"]',
    popover: {
      title: t('onboarding.admin.createAccount.title'),
      description: t('onboarding.admin.createAccount.description'),
      side: 'bottom',
      align: 'end',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="account-form-name"]',
    popover: {
      title: t('onboarding.admin.accountName.title'),
      description: t('onboarding.admin.accountName.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="account-form-platform"]',
    popover: {
      title: t('onboarding.admin.accountPlatform.title'),
      description: t('onboarding.admin.accountPlatform.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="account-form-type"]',
    popover: {
      title: t('onboarding.admin.accountType.title'),
      description: t('onboarding.admin.accountType.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="account-form-priority"]',
    popover: {
      title: t('onboarding.admin.accountPriority.title'),
      description: t('onboarding.admin.accountPriority.description'),
      side: 'top',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="account-form-groups"]',
    popover: {
      title: t('onboarding.admin.accountGroups.title'),
      description: t('onboarding.admin.accountGroups.description'),
      side: 'top',
      align: 'center',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="account-form-submit"]',
    popover: {
      title: t('onboarding.admin.accountSubmit.title'),
      description: t('onboarding.admin.accountSubmit.description'),
      side: 'left',
      align: 'center',
      showButtons: ['close']
    }
  },

  // ========== 第三部分：创建API密钥 ==========
  {
    element: '[data-tour="sidebar-my-keys"]',
    popover: {
      title: t('onboarding.admin.keyManage.title'),
      description: t('onboarding.admin.keyManage.description'),
      side: 'right',
      align: 'center',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="keys-create-btn"]',
    popover: {
      title: t('onboarding.admin.createKey.title'),
      description: t('onboarding.admin.createKey.description'),
      side: 'bottom',
      align: 'end',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="key-form-name"]',
    popover: {
      title: t('onboarding.admin.keyName.title'),
      description: t('onboarding.admin.keyName.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="key-form-group"]',
    popover: {
      title: t('onboarding.admin.keyGroup.title'),
      description: t('onboarding.admin.keyGroup.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="key-form-submit"]',
    popover: {
      title: t('onboarding.admin.keySubmit.title'),
      description: t('onboarding.admin.keySubmit.description'),
      side: 'left',
      align: 'center',
      showButtons: ['close']
    }
  }
  ]

  // 简易模式下过滤分组相关步骤
  if (isSimpleMode) {
    return allSteps.filter(step => {
      const element = step.element as string | undefined
      // 过滤掉分组管理和账号分组选择相关步骤
      return !element || (
        !element.includes('sidebar-group-manage') &&
        !element.includes('groups-create-btn') &&
        !element.includes('group-form-') &&
        !element.includes('account-form-groups')
      )
    })
  }

  return allSteps
}

/**
 * 普通用户引导流程
 */
export const getUserSteps = (t: (key: string) => string): DriveStep[] => [
  {
    popover: {
      title: t('onboarding.user.welcome.title'),
      description: t('onboarding.user.welcome.description'),
      align: 'center',
      nextBtnText: t('onboarding.user.welcome.nextBtn'),
      prevBtnText: t('onboarding.user.welcome.prevBtn')
    }
  },
  {
    element: '[data-tour="sidebar-my-keys"]',
    popover: {
      title: t('onboarding.user.keyManage.title'),
      description: t('onboarding.user.keyManage.description'),
      side: 'right',
      align: 'center',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="keys-create-btn"]',
    popover: {
      title: t('onboarding.user.createKey.title'),
      description: t('onboarding.user.createKey.description'),
      side: 'bottom',
      align: 'end',
      showButtons: ['close']
    }
  },
  {
    element: '[data-tour="key-form-name"]',
    popover: {
      title: t('onboarding.user.keyName.title'),
      description: t('onboarding.user.keyName.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="key-form-group"]',
    popover: {
      title: t('onboarding.user.keyGroup.title'),
      description: t('onboarding.user.keyGroup.description'),
      side: 'right',
      align: 'start',
      showButtons: ['next', 'previous']
    }
  },
  {
    element: '[data-tour="key-form-submit"]',
    popover: {
      title: t('onboarding.user.keySubmit.title'),
      description: t('onboarding.user.keySubmit.description'),
      side: 'left',
      align: 'center',
      showButtons: ['close']
    }
  }
]

/**
 * Selectors shared between the owner tour and the views that advance it after
 * an async action succeeds (the tour cannot observe API results by itself).
 */
export const OWNER_TOUR_SELECTORS = {
  sidebarProxies: '[data-tour="sidebar-proxies"]',
  proxiesCreate: '[data-tour="proxies-create-btn"]',
  proxySubmit: '[data-tour="proxy-form-submit"]',
  sidebarGroups: '#sidebar-group-manage',
  groupSubmit: '[data-tour="group-form-submit"]',
  sidebarAccounts: '#sidebar-channel-manage',
  accountsCreate: '[data-tour="accounts-create-btn"]',
  accountSubmit: '[data-tour="account-form-submit"]',
  // The whole create-account dialog panel while it shows the authorization step,
  // so the owner can still reach the dialog footer ("Complete Authorization").
  accountAuthorize: '.modal-content:has([data-tour="account-oauth-flow"])',
  accountRowMore: '[data-tour="account-row-more"]',
  sidebarUsage: '[data-tour="sidebar-usage"]',
  userMenu: '[data-tour="header-user-menu"]'
} as const

/**
 * Steps whose target is a container the user works inside (not a single
 * button). The engine must not auto-advance on the first click inside it;
 * business code calls onboardingStore.nextStep() once the action succeeds.
 */
const MANUAL_ADVANCE_SELECTORS: ReadonlySet<string> = new Set([OWNER_TOUR_SELECTORS.accountAuthorize])

export const isManualAdvanceStep = (step: DriveStep | undefined): boolean => {
  return typeof step?.element === 'string' && MANUAL_ADVANCE_SELECTORS.has(step.element)
}

type StepSide = 'top' | 'right' | 'bottom' | 'left'
type StepAlign = 'start' | 'center' | 'end'

/**
 * AI 账号所有者（role=owner）引导流程：
 * 代理 → 分组 → 账号（含授权）→ 测试连接 → 用量 → 结束。
 * 只引用所有者可见的页面与控件；交互方式与管理员引导一致：
 * - 只有 close 按钮的步骤需要用户实际点击目标（或由业务代码在成功后推进）
 * - next/previous 步骤为表单字段说明
 */
export const getOwnerSteps = (t: (key: string) => string): DriveStep[] => {
  const click = (element: string, key: string, side: StepSide, align: StepAlign): DriveStep => ({
    element,
    popover: {
      title: t(`onboarding.owner.${key}.title`),
      description: t(`onboarding.owner.${key}.description`),
      side,
      align,
      showButtons: ['close']
    }
  })
  const info = (element: string, key: string, side: StepSide, align: StepAlign): DriveStep => ({
    element,
    popover: {
      title: t(`onboarding.owner.${key}.title`),
      description: t(`onboarding.owner.${key}.description`),
      side,
      align,
      showButtons: ['next', 'previous']
    }
  })
  const withPopoverClass = (step: DriveStep, popoverClass: string): DriveStep => ({
    ...step,
    popover: { ...step.popover, popoverClass }
  })
  const S = OWNER_TOUR_SELECTORS

  return [
    {
      popover: {
        title: t('onboarding.owner.welcome.title'),
        description: t('onboarding.owner.welcome.description'),
        align: 'center',
        nextBtnText: t('onboarding.owner.welcome.nextBtn')
      }
    },

    // ========== 第一部分：代理 ==========
    click(S.sidebarProxies, 'proxyManage', 'right', 'center'),
    click(S.proxiesCreate, 'createProxy', 'bottom', 'end'),
    info('[data-tour="proxy-form-name"]', 'proxyName', 'right', 'start'),
    info('[data-tour="proxy-form-protocol"]', 'proxyProtocol', 'right', 'start'),
    info('[data-tour="proxy-form-address"]', 'proxyAddress', 'right', 'start'),
    info('[data-tour="proxy-form-fallback"]', 'proxyFallback', 'right', 'start'),
    click(S.proxySubmit, 'proxySubmit', 'left', 'center'),

    // ========== 第二部分：分组 ==========
    click(S.sidebarGroups, 'groupManage', 'right', 'center'),
    click('[data-tour="groups-create-btn"]', 'createGroup', 'bottom', 'end'),
    info('[data-tour="group-form-name"]', 'groupName', 'right', 'start'),
    info('[data-tour="group-form-platform"]', 'groupPlatform', 'right', 'start'),
    click(S.groupSubmit, 'groupSubmit', 'left', 'center'),

    // ========== 第三部分：账号 ==========
    click(S.sidebarAccounts, 'accountManage', 'right', 'center'),
    click(S.accountsCreate, 'createAccount', 'bottom', 'end'),
    info('[data-tour="account-form-name"]', 'accountName', 'right', 'start'),
    info('[data-tour="account-form-platform"]', 'accountPlatform', 'right', 'start'),
    info('[data-tour="account-form-type"]', 'accountType', 'right', 'start'),
    info('[data-tour="account-form-add-method"]', 'accountAddMethod', 'right', 'start'),
    info('[data-tour="account-form-intercept-warmup"]', 'accountWarmup', 'top', 'start'),
    info('[data-tour="account-form-tls-fingerprint"]', 'accountTlsFingerprint', 'top', 'start'),
    info('[data-tour="account-form-custom-base-url"]', 'accountCustomBaseUrl', 'top', 'start'),
    info('[data-tour="account-form-proxy"]', 'accountProxy', 'top', 'start'),
    info('[data-tour="account-form-concurrency"]', 'accountConcurrency', 'top', 'start'),
    info('[data-tour="account-form-priority"]', 'accountPriority', 'top', 'start'),
    info('[data-tour="account-form-groups"]', 'accountGroups', 'top', 'center'),
    click(S.accountSubmit, 'accountSubmit', 'left', 'center'),
    // The whole dialog is highlighted here, so a positioned popover would cover the
    // authorization code field; dock it in the corner instead (onboarding.css).
    withPopoverClass(click(S.accountAuthorize, 'accountAuthorize', 'left', 'start'), 'theme-tour-popover tour-popover-docked'),
    info(S.accountRowMore, 'accountTest', 'left', 'center'),

    // ========== 第四部分：用量 ==========
    click(S.sidebarUsage, 'usageManage', 'right', 'center'),
    info('[data-tour="usage-stats-cards"]', 'usageOverview', 'bottom', 'start'),
    info('[data-tour="usage-table"]', 'usageBilling', 'top', 'start'),

    // ========== 结束 ==========
    info(S.userMenu, 'finish', 'bottom', 'end')
  ]
}

export type TourRole = 'admin' | 'owner' | 'user'

/** Map a user role to its tour. Owners get their own tour, never the admin or user one. */
export const resolveTourRole = (role: string | null | undefined): TourRole => {
  if (role === 'admin') return 'admin'
  if (role === 'owner') return 'owner'
  return 'user'
}

/** Roles whose tour starts automatically on first visit (standard mode only). */
export const shouldAutoStartTour = (role: TourRole): boolean => role === 'admin' || role === 'owner'

export const getTourSteps = (
  role: TourRole,
  t: (key: string) => string,
  isSimpleMode = false
): DriveStep[] => {
  switch (role) {
    case 'admin':
      return getAdminSteps(t, isSimpleMode)
    case 'owner':
      return getOwnerSteps(t)
    default:
      return getUserSteps(t)
  }
}
