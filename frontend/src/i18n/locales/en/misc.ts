export default {

  // Subscription Progress (Header component)
  subscriptionProgress: {
    title: 'My Subscriptions',
    viewDetails: 'View subscription details',
    activeCount: '{count} active subscription(s)',
    daily: 'Daily',
    weekly: 'Weekly',
    monthly: 'Monthly',
    daysRemaining: '{days} days left',
    expired: 'Expired',
    expiresToday: 'Expires today',
    expiresTomorrow: 'Expires tomorrow',
    viewAll: 'View all subscriptions',
    noSubscriptions: 'No active subscriptions',
    unlimited: 'Unlimited'
  },

  // Version Badge
  version: {
    currentVersion: 'Current Version',
    latestVersion: 'Latest Version',
    upToDate: "You're running the latest version.",
    updateAvailable: 'A new version is available!',
    releaseNotes: 'Release Notes',
    noReleaseNotes: 'No release notes',
    viewUpdate: 'View Update',
    viewRelease: 'View Release',
    viewChangelog: 'View Changelog',
    refresh: 'Refresh',
    sourceMode: 'Source Build',
    sourceModeHint: 'Source build, use git pull to update',
    updateNow: 'Update Now',
    updating: 'Updating...',
    updateComplete: 'Update Complete',
    updateFailed: 'Update Failed',
    restartRequired: 'Please restart the service to apply the update',
    restartNow: 'Restart Now',
    restarting: 'Restarting...',
    retry: 'Retry',
    rollback: 'Version Rollback',
    rollbackSelectVersion: 'Select a version to roll back to (last 3 versions)',
    rollbackConfirm: 'Roll back to {version}',
    rollbackWarning:
      'Rollback downloads the selected version and replaces the current binary. A service restart is required afterwards.',
    rollingBack: 'Rolling back...',
    rollbackComplete: 'Rollback Complete',
    rollbackFailed: 'Rollback Failed',
    manualRollbackCommand: 'Manual rollback',
    copyCommand: 'Copy',
    copied: 'Copied',
    noRollbackVersions: 'No versions available for rollback',
    loadVersionsFailed: 'Failed to load versions',
    rollbackSourceHint: 'Online rollback is not available for source builds',
    deployScript: 'Script',
    deployDocker: 'Docker',
    dockerEditCompose: 'Edit the image tag in docker-compose.yml',
    dockerRecreate: 'Recreate the container'
  },

  // Recharge / Subscription Page
  purchase: {
    title: 'Recharge / Subscription',
    description: 'Recharge balance or purchase subscription via the embedded page',
    rechargeDescription: 'Recharge balance via the embedded page',
    subscriptionDescription: 'Purchase subscription via the embedded page',
    openInNewTab: 'Open in new tab',
    notEnabledTitle: 'Feature not enabled',
    notEnabledDesc: 'The administrator has not enabled the recharge/subscription entry. Please contact admin.',
    notConfiguredTitle: 'Recharge / Subscription URL not configured',
    notConfiguredDesc:
      'The administrator enabled the entry but has not configured a recharge/subscription URL. Please contact admin.'
  },

  // Custom Page (iframe embed)
  customPage: {
    title: 'Custom Page',
    openInNewTab: 'Open in new tab',
    notFoundTitle: 'Page not found',
    notFoundDesc: 'This custom page does not exist or has been removed.',
    notConfiguredTitle: 'Page URL not configured',
    notConfiguredDesc: 'The URL for this custom page has not been properly configured.',
    tableOfContents: 'Contents',
    copyCode: 'Copy',
    copiedCode: 'Copied',
    copyCodeFailed: 'Failed'
  },

  // Announcements Page
  announcements: {
    title: 'Announcements',
    description: 'View system announcements',
    unreadOnly: 'Show unread only',
    markRead: 'Mark as read',
    markAllRead: 'Mark all as read',
    viewAll: 'View all announcements',
    markedAsRead: 'Marked as read',
    allMarkedAsRead: 'All announcements marked as read',
    newCount: '{count} new announcement | {count} new announcements',
    readAt: 'Read at',
    read: 'Read',
    unread: 'Unread',
    startsAt: 'Starts at',
    endsAt: 'Ends at',
    empty: 'No announcements',
    emptyUnread: 'No unread announcements',
    total: 'announcements',
    emptyDescription: 'There are no system announcements at this time',
    readStatus: 'You have read this announcement',
    markReadHint: 'Click "Mark as read" to mark this announcement'
  },

  // User Subscriptions Page
  userSubscriptions: {
    title: 'My Subscriptions',
    description: 'View your subscription plans and usage',
    noActiveSubscriptions: 'No Active Subscriptions',
    noActiveSubscriptionsDesc:
      "You don't have any active subscriptions. Contact administrator to get one.",
    failedToLoad: 'Failed to load subscriptions',
    status: {
      active: 'Active',
      expired: 'Expired',
      revoked: 'Revoked'
    },
    usage: 'Usage',
    expires: 'Expires',
    noExpiration: 'No expiration',
    unlimited: 'Unlimited',
    unlimitedDesc: 'No usage limits on this subscription',
    daily: 'Daily',
    weekly: 'Weekly',
    monthly: 'Monthly',
    daysRemaining: '{days} days remaining',
    expiresOn: 'Expires on {date}',
    resetIn: 'Resets in {time}',
    quotaEndsIn: 'Quota ends in {time}',
    windowNotActive: 'Awaiting first use',
    usageOf: '{used} of {limit}'
  },

  // Onboarding Tour
  onboarding: {
    restartTour: 'Restart Onboarding Tour',
    dontShowAgain: "Don't show again",
    dontShowAgainTitle: 'Permanently close onboarding guide',
    confirmDontShow: "Are you sure you don't want to see the onboarding guide again?\n\nYou can restart it anytime from the user menu in the top right corner.",
    confirmExit: 'Are you sure you want to exit the onboarding guide? You can restart it anytime from the top right menu.',
    interactiveHint: 'Press Enter or Click to continue',
    navigation: {
      flipPage: 'Flip Page',
      exit: 'Exit'
    },
    // Admin tour steps
    admin: {
      welcome: {
        title: '👋 Welcome to Sub2API',
        description: '<div style="line-height: 1.8;"><p style="margin-bottom: 16px;">Sub2API is a powerful AI service gateway platform that helps you easily manage and distribute AI services.</p><p style="margin-bottom: 12px;"><b>🎯 Core Features:</b></p><ul style="margin-left: 20px; margin-bottom: 16px;"><li>📦 <b>Group Management</b> - Create service tiers (VIP, Free Trial, etc.)</li><li>🔗 <b>Account Pool</b> - Connect multiple upstream AI service accounts</li><li>🔑 <b>Key Distribution</b> - Generate independent API Keys for users</li><li>💰 <b>Billing Control</b> - Flexible rate and quota management</li></ul><p style="color: #10b981; font-weight: 600;">Let\'s complete the initial setup in 3 minutes →</p></div>',
        nextBtn: 'Start Setup 🚀',
        prevBtn: 'Skip'
      },
      groupManage: {
        title: '📦 Step 1: Group Management',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;"><b>What is a Group?</b></p><p style="margin-bottom: 12px;">Groups are the core concept of Sub2API, like a "service package":</p><ul style="margin-left: 20px; margin-bottom: 12px; font-size: 13px;"><li>🎯 Each group can contain multiple upstream accounts</li><li>💰 Each group has independent billing multiplier</li><li>👥 Can be set as public or exclusive</li></ul><p style="margin-top: 12px; padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>💡 Example:</b> You can create "VIP Premium" (high rate) and "Free Trial" (low rate) groups</p><p style="margin-top: 16px; color: #10b981; font-weight: 600;">👉 Click "Group Management" on the left sidebar</p></div>'
      },
      createGroup: {
        title: '➕ Create New Group',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Let\'s create your first group.</p><p style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>📝 Tip:</b> Recommend creating a test group first to familiarize yourself with the process</p><p style="color: #10b981; font-weight: 600;">👉 Click the "Create Group" button</p></div>'
      },
      groupName: {
        title: '✏️ 1. Group Name',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Give your group an easy-to-identify name.</p><div style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>💡 Naming Suggestions:</b><ul style="margin: 8px 0 0 16px;"><li>"Test Group" - For testing</li><li>"VIP Premium" - High-quality service</li><li>"Free Trial" - Trial version</li></ul></div><p style="font-size: 13px; color: #6b7280;">Click "Next" when done</p></div>',
        nextBtn: 'Next'
      },
      groupPlatform: {
        title: '🤖 2. Select Platform',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Choose the AI platform this group supports.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>📌 Platform Guide:</b><ul style="margin: 8px 0 0 16px;"><li><b>Anthropic</b> - Claude models</li><li><b>OpenAI</b> - GPT models</li><li><b>Google</b> - Gemini models</li></ul></div><p style="font-size: 13px; color: #6b7280;">One group can only have one platform</p></div>',
        nextBtn: 'Next'
      },
      groupMultiplier: {
        title: '💰 3. Rate Multiplier',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Set the billing multiplier to control user charges.</p><div style="padding: 8px 12px; background: #fef3c7; border-left: 3px solid #f59e0b; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>⚙️ Billing Rules:</b><ul style="margin: 8px 0 0 16px;"><li><b>1.0</b> - Original price (cost price)</li><li><b>1.5</b> - User consumes $1, charged $1.5</li><li><b>2.0</b> - User consumes $1, charged $2</li><li><b>0.8</b> - Subsidy mode (loss-making)</li></ul></div><p style="font-size: 13px; color: #6b7280;">Recommend setting test group to 1.0</p></div>',
        nextBtn: 'Next'
      },
      groupExclusive: {
        title: '🔒 4. Exclusive Group (Optional)',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Control group visibility and access permissions.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>🔐 Permission Guide:</b><ul style="margin: 8px 0 0 16px;"><li><b>Off</b> - Public group, visible to all users</li><li><b>On</b> - Exclusive group, only for specified users</li></ul></div><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>💡 Use Cases:</b> VIP exclusive, internal testing, special customers</p></div>',
        nextBtn: 'Next'
      },
      groupSubmit: {
        title: '✅ Save Group',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Confirm the information and click create to save the group.</p><p style="padding: 8px 12px; background: #fef3c7; border-left: 3px solid #f59e0b; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>⚠️ Note:</b> Platform type cannot be changed after creation, but other settings can be edited anytime</p><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>📌 Next Step:</b> After creation, we\'ll add upstream accounts to this group</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">👉 Click "Create" button</p></div>'
      },
      accountManage: {
        title: '🔗 Step 2: Add Account',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;"><b>Great! Group created successfully 🎉</b></p><p style="margin-bottom: 12px;">Now add upstream AI service accounts to enable actual service delivery.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>🔑 Account Purpose:</b><ul style="margin: 8px 0 0 16px;"><li>Connect to upstream AI services (Claude, GPT, etc.)</li><li>One group can contain multiple accounts (load balancing)</li><li>Supports OAuth and Session Key methods</li></ul></div><p style="margin-top: 16px; color: #10b981; font-weight: 600;">👉 Click "Account Management" on the left sidebar</p></div>'
      },
      createAccount: {
        title: '➕ Add New Account',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Click the button to start adding your first upstream account.</p><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>💡 Tip:</b> Recommend using OAuth method - more secure and no manual key extraction needed</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">👉 Click "Add Account" button</p></div>'
      },
      accountName: {
        title: '✏️ 1. Account Name',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Set an easy-to-identify name for the account.</p><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>💡 Naming Suggestions:</b> "Claude Main", "GPT Backup 1", "Test Account", etc.</p></div>',
        nextBtn: 'Next'
      },
      accountPlatform: {
        title: '🤖 2. Select Platform',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Choose the service provider platform for this account.</p><p style="padding: 8px 12px; background: #fef3c7; border-left: 3px solid #f59e0b; border-radius: 4px; font-size: 13px;"><b>⚠️ Important:</b> Platform must match the group you just created</p></div>',
        nextBtn: 'Next'
      },
      accountType: {
        title: '🔐 3. Authorization Method',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Choose the account authorization method.</p><div style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>✅ Recommended: OAuth Method</b><ul style="margin: 8px 0 0 16px;"><li>No manual key extraction needed</li><li>More secure with auto-refresh support</li><li>Works with Claude Code, ChatGPT OAuth</li></ul></div><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px;"><b>📌 Session Key Method</b><ul style="margin: 8px 0 0 16px;"><li>Requires manual extraction from browser</li><li>May need periodic updates</li><li>For platforms without OAuth support</li></ul></div></div>',
        nextBtn: 'Next'
      },
      accountPriority: {
        title: '⚖️ 4. Priority (Optional)',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Set the account call priority.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>📊 Priority Rules:</b><ul style="margin: 8px 0 0 16px;"><li>Lower number = higher priority</li><li>System uses low-value accounts first</li><li>Same priority = random selection</li></ul></div><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>💡 Use Case:</b> Set main account to lower value, backup accounts to higher value</p></div>',
        nextBtn: 'Next'
      },
      accountGroups: {
        title: '🎯 5. Assign Groups',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;"><b>Key Step!</b> Assign the account to the group you just created.</p><div style="padding: 8px 12px; background: #fee2e2; border-left: 3px solid #ef4444; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>⚠️ Important Reminder:</b><ul style="margin: 8px 0 0 16px;"><li>Must select at least one group</li><li>Unassigned accounts cannot be used</li><li>One account can be assigned to multiple groups</li></ul></div><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>💡 Tip:</b> Select the test group you just created</p></div>',
        nextBtn: 'Next'
      },
      accountSubmit: {
        title: '✅ Save Account',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Confirm the information and click save.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>📌 OAuth Flow:</b><ul style="margin: 8px 0 0 16px;"><li>Will redirect to service provider page after clicking save</li><li>Complete login and authorization on provider page</li><li>Auto-return after successful authorization</li></ul></div><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>📌 Next Step:</b> After adding account, we\'ll create an API key</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">👉 Click "Save" button</p></div>'
      },
      keyManage: {
        title: '🔑 Step 3: Generate Key',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;"><b>Congratulations! Account setup complete 🎉</b></p><p style="margin-bottom: 12px;">Final step: generate an API Key to test if the service works properly.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>🔑 API Key Purpose:</b><ul style="margin: 8px 0 0 16px;"><li>Credential for calling AI services</li><li>Each key is bound to one group</li><li>Can set quota and expiration</li><li>Supports independent usage statistics</li></ul></div><p style="margin-top: 16px; color: #10b981; font-weight: 600;">👉 Click "API Keys" on the left sidebar</p></div>'
      },
      createKey: {
        title: '➕ Create Key',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Click the button to create your first API Key.</p><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>💡 Tip:</b> Copy and save immediately after creation - key is only shown once</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">👉 Click "Create Key" button</p></div>'
      },
      keyName: {
        title: '✏️ 1. Key Name',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Set an easy-to-manage name for the key.</p><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>💡 Naming Suggestions:</b> "Test Key", "Production", "Mobile", etc.</p></div>',
        nextBtn: 'Next'
      },
      keyGroup: {
        title: '🎯 2. Select Group',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Select the group you just configured.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>📌 Group Determines:</b><ul style="margin: 8px 0 0 16px;"><li>Which accounts this key can use</li><li>What billing multiplier applies</li><li>Whether it\'s an exclusive key</li></ul></div><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>💡 Tip:</b> Select the test group you just created</p></div>',
        nextBtn: 'Next'
      },
      keySubmit: {
        title: '🎉 Generate and Copy',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">System will generate a complete API Key after clicking create.</p><div style="padding: 8px 12px; background: #fee2e2; border-left: 3px solid #ef4444; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>⚠️ Important Reminder:</b><ul style="margin: 8px 0 0 16px;"><li>Key is only shown once, copy immediately</li><li>Need to regenerate if lost</li><li>Keep it safe, don\'t share with others</li></ul></div><div style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>🚀 Next Steps:</b><ul style="margin: 8px 0 0 16px;"><li>Copy the generated sk-xxx key</li><li>Use in any OpenAI-compatible client</li><li>Start experiencing AI services!</li></ul></div><p style="margin-top: 12px; color: #10b981; font-weight: 600;">👉 Click "Create" button</p></div>'
      }
    },
    // AI account owner tour steps
    owner: {
      welcome: {
        title: 'Welcome, AI Account Owner',
        description: '<div style="line-height: 1.8;"><p style="margin-bottom: 16px;">As an account owner you bring your own Claude accounts to this gateway and manage them yourself.</p><p style="margin-bottom: 12px;"><b>What you can do:</b></p><ul style="margin-left: 20px; margin-bottom: 16px;"><li><b>Proxies</b> - give each account its own proxy</li><li><b>Groups</b> - organise your accounts in your own group</li><li><b>Accounts</b> - add and authorize your Claude accounts</li><li><b>Usage</b> - see per-account usage and billing</li></ul><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Scope:</b> you only see your own accounts, proxies, groups and their usage. Other users, their API keys and other owners\' resources are not visible to you.</div><p style="color: #10b981; font-weight: 600;">Let\'s set up proxy, group and account in that order →</p></div>',
        nextBtn: 'Start'
      },
      proxyManage: {
        title: 'Step 1: Proxies',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Every account should reach the upstream through its own proxy, so start by adding the proxy for your first account.</p><div style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Tip:</b> one proxy per account keeps accounts isolated from each other.</div><p style="margin-top: 12px; color: #10b981; font-weight: 600;">→ Click "Proxies" in the left sidebar</p></div>'
      },
      createProxy: {
        title: 'Create a Proxy',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Add the proxy your first account will use.</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">→ Click "Create Proxy"</p></div>'
      },
      proxyName: {
        title: '1. Proxy Name',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Use a name that tells you which account the proxy belongs to.</p><div style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Example:</b> "claude-main-proxy"</div><p style="font-size: 13px; color: #6b7280;">Click "Next" when done</p></div>',
        nextBtn: 'Next'
      },
      proxyProtocol: {
        title: '2. Protocol',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Select the protocol your proxy speaks.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Recommended:</b><ul style="margin: 8px 0 0 16px;"><li><b>SOCKS5H (Remote DNS)</b> - domain names are resolved on the proxy side</li><li><b>HTTP</b> - widely supported</li></ul></div></div>',
        nextBtn: 'Next'
      },
      proxyAddress: {
        title: '3. Host and Port',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Enter the proxy server address and port. If the proxy needs authentication, fill in "Username (Optional)" and "Password (Optional)" below.</p><div style="padding: 8px 12px; background: #fef3c7; border-left: 3px solid #f59e0b; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Public address only:</b> private, loopback, link-local or unresolvable hosts are rejected.</div></div>',
        nextBtn: 'Next'
      },
      proxyFallback: {
        title: '4. Expiry Fallback',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Choose what happens to accounts on this proxy once it passes its "Validity" date.</p><div style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Recommended:</b><ul style="margin: 8px 0 0 16px;"><li><b>No fallback</b> - accounts stay on this proxy; renew it in time</li><li><b>Backup proxy</b> - accounts switch to another proxy of yours</li></ul></div><div style="padding: 8px 12px; background: #fee2e2; border-left: 3px solid #ef4444; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Do not use "Direct connection":</b> accounts would then reach the upstream without any proxy. Your accounts are expected to always go through a proxy.</div></div>',
        nextBtn: 'Next'
      },
      proxySubmit: {
        title: 'Save Proxy',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Check the details and create the proxy.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Next:</b> after the proxy is created we will create your group.</div><p style="margin-top: 12px; color: #10b981; font-weight: 600;">→ Click "Create"</p></div>'
      },
      groupManage: {
        title: 'Step 2: Groups',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;"><b>Proxy created.</b> Now create a group to hold your accounts.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>About owner groups:</b><ul style="margin: 8px 0 0 16px;"><li>Requests sent to a group are served by the accounts in it</li><li>Your groups are always exclusive</li><li>The administrator decides which users may use your group</li></ul></div><p style="margin-top: 12px; color: #10b981; font-weight: 600;">→ Click "Groups" in the left sidebar</p></div>'
      },
      createGroup: {
        title: 'Create a Group',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Create the group your accounts will belong to.</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">→ Click "Create Group"</p></div>'
      },
      groupName: {
        title: '1. Group Name',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Give the group an easy-to-identify name.</p><p style="font-size: 13px; color: #6b7280;">Click "Next" when done</p></div>',
        nextBtn: 'Next'
      },
      groupPlatform: {
        title: '2. Platform',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Choose <b>Anthropic</b> for Claude accounts.</p><div style="padding: 8px 12px; background: #fef3c7; border-left: 3px solid #f59e0b; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Note:</b> the platform cannot be changed after the group is created.</div></div>',
        nextBtn: 'Next'
      },
      groupSubmit: {
        title: 'Save Group',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Create the group.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Exclusive group:</b> only users the administrator assigns can use it. Ask your administrator to assign users once your accounts are ready.</div><p style="margin-top: 12px; color: #10b981; font-weight: 600;">→ Click "Create"</p></div>'
      },
      accountManage: {
        title: 'Step 3: Accounts',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;"><b>Group created.</b> Now add your Claude account, using the proxy and group you just created.</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">→ Click "Accounts" in the left sidebar</p></div>'
      },
      createAccount: {
        title: 'Add an Account',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Start adding your first Claude account.</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">→ Click "Create Account"</p></div>'
      },
      accountName: {
        title: '1. Account Name',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Set an easy-to-identify name, e.g. matching the proxy name.</p><p style="font-size: 13px; color: #6b7280;">Click "Next" when done</p></div>',
        nextBtn: 'Next'
      },
      accountPlatform: {
        title: '2. Platform',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Select <b>Anthropic</b>.</p><div style="padding: 8px 12px; background: #fef3c7; border-left: 3px solid #f59e0b; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Note:</b> the platform must match your group.</div></div>',
        nextBtn: 'Next'
      },
      accountType: {
        title: '3. Account Type',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Select <b>Claude Code</b> (OAuth / Setup Token) for a Claude subscription account.</p></div>',
        nextBtn: 'Next'
      },
      accountAddMethod: {
        title: '4. Add Method',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Keep <b>OAuth</b> selected. You will authorize the account in the next step of the dialog.</p></div>',
        nextBtn: 'Next'
      },
      accountWarmup: {
        title: '5. Intercept Warmup Requests',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Warmup requests such as title generation get a mock response instead of using upstream quota.</p><div style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Recommended:</b> turn it on.</div></div>',
        nextBtn: 'Next'
      },
      accountTlsFingerprint: {
        title: '6. TLS Fingerprint Simulation',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Makes upstream connections present a client-like TLS fingerprint.</p><div style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Recommended:</b> turn it on and keep the default profile.</div></div>',
        nextBtn: 'Next'
      },
      accountCustomBaseUrl: {
        title: '7. Custom Relay URL',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Sends this account\'s requests to another relay instead of the official upstream.</p><div style="padding: 8px 12px; background: #fef3c7; border-left: 3px solid #f59e0b; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Keep it off:</b> requests should go to the official upstream through your proxy.</div></div>',
        nextBtn: 'Next'
      },
      accountProxy: {
        title: '8. Proxy (required)',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Select the proxy you created for this account.</p><div style="padding: 8px 12px; background: #fee2e2; border-left: 3px solid #ef4444; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Important:</b> do not leave this on "No Proxy". Every account is expected to go through its own proxy.</div></div>',
        nextBtn: 'Next'
      },
      accountConcurrency: {
        title: '9. Concurrency',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">The maximum number of requests this account serves at the same time.</p><div style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Recommended:</b> 3-5 per account.</div></div>',
        nextBtn: 'Next'
      },
      accountPriority: {
        title: '10. Priority',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Lower value accounts are used first.</p><div style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Tip:</b> give all accounts in one pool the same value so load is spread evenly between them.</div></div>',
        nextBtn: 'Next'
      },
      accountGroups: {
        title: '11. Groups',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Select <b>your group</b> so the account can serve requests.</p><div style="padding: 8px 12px; background: #fef3c7; border-left: 3px solid #f59e0b; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Note:</b> an account that is not in any group is never used.</div></div>',
        nextBtn: 'Next'
      },
      accountSubmit: {
        title: 'Continue to Authorization',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Check the settings and continue to authorization.</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">→ Click "Next"</p></div>'
      },
      accountAuthorize: {
        title: 'Authorize the Account',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 10px;">In the dialog: "Generate Auth URL" → sign in → paste the code → "Complete Authorization".</p><div style="padding: 8px 12px; background: #fef3c7; border-left: 3px solid #f59e0b; border-radius: 4px; font-size: 13px; margin-bottom: 10px;">Open the auth URL in a browser that uses <b>the same proxy</b> as this account, so it signs in and serves requests from the same address.</div><p style="font-size: 13px; color: #6b7280;">The tour continues once the account has been created.</p></div>'
      },
      accountTest: {
        title: 'Test the Connection',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Your account is in the list. Check that it works before it serves traffic.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>How:</b> in the account\'s row click "More" and choose "Test Connection". "View Stats" in the same menu shows this account\'s usage.</div><p style="font-size: 13px; color: #6b7280;">Repeat proxy → account for each further account.</p></div>',
        nextBtn: 'Next'
      },
      usageManage: {
        title: 'Step 4: Usage',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">See how your accounts are used and billed.</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">→ Click "Usage" in the left sidebar</p></div>'
      },
      usageOverview: {
        title: 'Usage Overview',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Totals for <b>your accounts only</b> in the selected time range: requests, tokens, cost and average duration.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Cost:</b> "Total Cost" is what users were charged; the orange "Cost" is billed to your accounts.</div></div>',
        nextBtn: 'Next'
      },
      usageBilling: {
        title: 'Per-request Billing',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Each row is one request served by one of your accounts.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Hover the info icon in the cost column:</b><ul style="margin: 8px 0 0 16px;"><li><b>User billed</b> - what the user was charged</li><li><b>Account billed</b> - what is billed to your account (account rate set by the administrator)</li></ul></div><div style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>Per account:</b> filter by account here, or use "More" → "View Stats" on the Accounts page.</div></div>',
        nextBtn: 'Next'
      },
      finish: {
        title: 'All Set',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">You have set up a proxy, a group and an account.</p><div style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>In this user menu:</b><ul style="margin: 8px 0 0 16px;"><li><b>Restart Onboarding Tour</b> - run this tour again</li><li><b>Owner Guide</b> - the detailed owner guide, if your administrator has provided one</li></ul></div></div>',
        nextBtn: 'Done'
      }
    },
    // User tour steps
    user: {
      welcome: {
        title: '👋 Welcome to Sub2API',
        description: '<div style="line-height: 1.8;"><p style="margin-bottom: 16px;">Hello! Welcome to the Sub2API AI service platform.</p><p style="margin-bottom: 12px;"><b>🎯 Quick Start:</b></p><ul style="margin-left: 20px; margin-bottom: 16px;"><li>🔑 Create API Key</li><li>📋 Copy key to your application</li><li>🚀 Start using AI services</li></ul><p style="color: #10b981; font-weight: 600;">Just 1 minute, let\'s get started →</p></div>',
        nextBtn: 'Start 🚀',
        prevBtn: 'Skip'
      },
      keyManage: {
        title: '🔑 API Key Management',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Manage all your API access keys here.</p><p style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px;"><b>📌 What is an API Key?</b><br/>An API key is your credential for accessing AI services, like a key that allows your application to call AI capabilities.</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">👉 Click to enter key page</p></div>'
      },
      createKey: {
        title: '➕ Create New Key',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Click the button to create your first API key.</p><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>💡 Tip:</b> Key is only shown once after creation, make sure to copy and save</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">👉 Click "Create Key"</p></div>'
      },
      keyName: {
        title: '✏️ Key Name',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Give your key an easy-to-identify name.</p><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>💡 Examples:</b> "My First Key", "For Testing", etc.</p></div>',
        nextBtn: 'Next'
      },
      keyGroup: {
        title: '🎯 Select Group',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Select the service group assigned by the administrator.</p><p style="padding: 8px 12px; background: #eff6ff; border-left: 3px solid #3b82f6; border-radius: 4px; font-size: 13px;"><b>📌 Group Info:</b><br/>Different groups may have different service quality and billing rates, choose according to your needs.</p></div>',
        nextBtn: 'Next'
      },
      keySubmit: {
        title: '🎉 Complete Creation',
        description: '<div style="line-height: 1.7;"><p style="margin-bottom: 12px;">Click to confirm and create your API key.</p><div style="padding: 8px 12px; background: #fee2e2; border-left: 3px solid #ef4444; border-radius: 4px; font-size: 13px; margin-bottom: 12px;"><b>⚠️ Important:</b><ul style="margin: 8px 0 0 16px;"><li>Copy the key (sk-xxx) immediately after creation</li><li>Key is only shown once, need to regenerate if lost</li></ul></div><p style="padding: 8px 12px; background: #f0fdf4; border-left: 3px solid #10b981; border-radius: 4px; font-size: 13px;"><b>🚀 How to Use:</b><br/>Configure the key in any OpenAI-compatible client (like ChatBox, OpenCat, etc.) and start using!</p><p style="margin-top: 12px; color: #10b981; font-weight: 600;">👉 Click "Create" button</p></div>'
      }
    }
  },

  // Payment System
  payment: {
    title: 'Recharge / Subscription',
    amountLabel: 'Amount',
    paymentAmount: 'Payment Amount',
    creditedBalance: 'Credited Balance',
    rechargeBonus: {
      creditedShort: 'Get {amount}',
      payShort: 'Pay {amount}',
      amountLabel: 'Bonus',
      amountLabelWithPercent: 'Bonus (+{percent}%)',
      discountLabelWithPercent: 'Discount ({percent}% OFF)',
    },
    quickAmounts: 'Quick Amounts',
    customAmount: 'Custom Amount',
    enterAmount: 'Enter amount',
    paymentMethod: 'Payment Method',
    fee: 'Fee',
    actualPay: 'Actual Payment',
    createOrder: 'Confirm Payment',
    methods: {
      easypay: 'EasyPay',
      alipay: 'Alipay',
      wxpay: 'WeChat Pay',
      stripe: 'Stripe',
      airwallex: 'Airwallex',
      card: 'Card',
      link: 'Link',
      alipay_direct: 'Alipay (Direct)',
      wxpay_direct: 'WeChat Pay (Direct)',
    },
    status: {
      pending: 'Pending',
      paid: 'Paid',
      recharging: 'Recharging',
      completed: 'Completed',
      expired: 'Expired',
      cancelled: 'Cancelled',
      failed: 'Failed',
      refund_requested: 'Refund Requested',
      refunding: 'Refunding',
      refund_pending: 'Refund Pending',
      refunded: 'Refunded',
      partially_refunded: 'Partially Refunded',
      refund_failed: 'Refund Failed',
    },
    qr: {
      scanToPay: 'Scan to Pay',
      scanAlipay: 'Alipay QR Payment',
      scanWxpay: 'WeChat QR Payment',
      scanAlipayHint: 'Open Alipay on your phone and scan the QR code to pay',
      scanWxpayHint: 'Open WeChat on your phone and scan the QR code to pay',
      payInNewWindow: 'Complete Payment in New Window',
      payInNewWindowHint: 'The payment page has opened in a new window. Please complete the payment there and return to this page.',
      openPayWindow: 'Reopen Payment Page',
      expiresIn: 'Expires in',
      expired: 'Order Expired',
      expiredDesc: 'This order has expired. Please create a new one.',
      cancelled: 'Order Cancelled',
      cancelledDesc: 'You have cancelled this payment.',
      waitingPayment: 'Waiting for payment...',
      cancelOrder: 'Cancel Order',
      alipayOpening: 'Opening Alipay',
      alipayContinueInApp: 'Complete payment in Alipay',
      alipayWaitingHint: 'The server will confirm the payment and update this page automatically',
      alipayFallbackTitle: 'Alipay did not open',
      alipayFallbackHint: 'Try opening Alipay again, or save the QR code and scan it from your Alipay photo album',
      reopenAlipay: 'Open Alipay Again',
      saveQRCode: 'Save QR Code',
      alipaySaveAndScanHint: 'Save the QR code, open Alipay Scan, then select it from your photo album',
    },
    orders: {
      title: 'My Orders',
      empty: 'No orders yet',
      orderId: 'Order ID',
      orderNo: 'Order No.',
      amount: 'Amount',
      payAmount: 'Paid',
      creditedAmount: 'Credited Amount',
      bonusAmount: 'Bonus',
      bonusIncluded: 'incl. bonus {amount}',
      fee: 'Fee',
      baseAmount: 'Base Amount',
      includedInPayAmount: 'included in paid amount',
      status: 'Status',
      paymentMethod: 'Payment Method',
      createdAt: 'Created',
      cancel: 'Cancel Order',
      userId: 'User ID',
      orderType: 'Order Type',
      actions: 'Actions',
      requestRefund: 'Request Refund',
    },
    result: {
      success: 'Payment Successful',
      subscriptionSuccess: 'Subscription Successful',
      processing: 'Payment Processing',
      processingHint: 'Payment confirmation is still pending. This page will refresh automatically.',
      failed: 'Payment Failed',
      backToRecharge: 'Back to Recharge',
      viewOrders: 'View Orders',
    },
    currentBalance: 'Current Balance',
    groupFallback: 'Group #{id}',
    rechargeAccount: 'Recharge Account',
    activeSubscription: 'Active Subscription',
    noActiveSubscription: 'No active subscription',
    tabTopUp: 'Top Up',
    tabSubscribe: 'Subscribe',
    noPlans: 'No subscription plans available',
    notAvailable: 'Top-up is currently unavailable',
    billingUnavailable: 'Neither top-up nor subscriptions are currently available. Please contact the administrator.',
    confirmSubscription: 'Confirm Subscription',
    confirmCancel: 'Are you sure you want to cancel this order?',
    amountTooLow: 'Minimum amount is {min}',
    amountTooHigh: 'Maximum amount is {max}',
    amountNoMethod: 'No payment method available for this amount',
    rechargeRatePreview: 'Current rate: 1 {currency} = {usd} USD',
    refundReason: 'Refund Reason',
    refundReasonPlaceholder: 'Please describe your refund reason',
    stripeLoadFailed: 'Failed to load payment component. Please refresh and try again.',
    stripeMissingParams: 'Missing order ID or client secret',
    stripeNotConfigured: 'Stripe is not configured',
    airwallexLoadFailed: 'Failed to load Airwallex payment component. Please refresh and try again.',
    airwallexMissingParams: 'Missing Airwallex payment parameters',
    errors: {
      tooManyPending: 'Too many pending orders (max {max}). Please complete or cancel existing orders first.',
      cancelRateLimited: 'Too many cancellations. Please try again later.',
      wechatH5NotAuthorized: 'This merchant has not enabled WeChat H5 payment. Open this page in WeChat to continue.',
      wechatPaymentMpNotConfigured: 'This site has not completed WeChat MP/JSAPI payment setup, so in-app WeChat payment is unavailable right now.',
      wechatJsapiUnavailable: 'WeChat payment could not be invoked in the current environment. Reopen this page inside WeChat and try again.',
      wechatJsapiFailed: 'WeChat payment did not complete. Try invoking it again or switch to QR payment.',
      wechatUnavailable: 'WeChat payment is temporarily unavailable. Please try again later.',
      wechatOpenInWeChatHint: 'Open the current page inside WeChat, or switch to desktop WeChat QR payment.',
      wechatScanOnDesktopHint: 'On desktop, use WeChat Scan to pay; on mobile, reopen the current page inside WeChat.',
      wechatSwitchBrowserHint: 'Switch to desktop WeChat QR payment, or reopen this page in an external browser and retry.',
      mobilePaymentFallbackToQr: 'This merchant has not enabled mobile payment. The flow has been switched to QR payment automatically.',
      alipayDesktopUnavailable: 'The desktop Alipay flow could not generate a QR code.',
      alipayDesktopQrHint: 'Desktop Alipay should render a QR code. Refresh and retry, or make sure the payment page was not blocked.',
      alipayMobileUnavailable: 'This page could not hand off to Alipay.',
      alipayMobileOpenHint: 'Allow the current page to open the Alipay app, or retry from the system browser.',
      // Structured error codes (reason strings from backend ApplicationError)
      PAYMENT_DISABLED: 'Payment system is disabled.',
      USER_INACTIVE: 'Your account is disabled.',
      BALANCE_PAYMENT_DISABLED: 'Balance recharge has been disabled.',
      INVALID_AMOUNT: 'Invalid amount.',
      INVALID_INPUT: 'Invalid request.',
      PLAN_NOT_AVAILABLE: 'Plan not found or no longer available.',
      GROUP_NOT_FOUND: 'Subscription group is no longer available.',
      GROUP_TYPE_MISMATCH: 'Group is not a subscription type.',
      TOO_MANY_PENDING: 'Too many pending orders (max {max}). Please complete or cancel existing orders first.',
      DAILY_LIMIT_EXCEEDED: 'Daily recharge limit reached. Remaining: {remaining}.',
      PAYMENT_GATEWAY_ERROR: 'Payment method is unavailable.',
      NO_AVAILABLE_INSTANCE: 'No payment channel available right now.',
      PAYMENT_PROVIDER_MISCONFIGURED: 'Payment provider misconfigured. Please contact an administrator.',
      WXPAY_CONFIG_MISSING_KEY: 'WeChat Pay config missing required key: {key}.',
      WXPAY_CONFIG_INVALID_KEY_LENGTH: 'WeChat Pay {key} length is invalid (expected {expected} bytes, got {actual}).',
      WXPAY_CONFIG_INVALID_KEY: 'WeChat Pay {key} is malformed. Make sure you copied the full PEM content.',
      PENDING_ORDERS: 'This provider has pending orders. Please wait for them to complete before making changes.',
      PAYMENT_PROVIDER_CONFLICT: 'Another enabled provider instance is already serving this payment method. Disable it before continuing.',
      CANCEL_RATE_LIMITED: 'Too many cancellations. Please try again later.',
      NOT_FOUND: 'Order not found.',
      FORBIDDEN: 'No permission for this order.',
      CONFLICT: 'Order status has changed. Please refresh.',
      INVALID_ORDER_TYPE: 'Only balance orders can request a refund.',
      INVALID_STATUS: 'The current order status does not allow this operation.',
      BALANCE_NOT_ENOUGH: 'Refund amount exceeds balance.',
      REFUND_AMOUNT_EXCEEDED: 'Refund amount exceeds the recharge amount.',
      REFUND_FAILED: 'Refund failed.',
    },
    airwallexPay: 'Airwallex Payment',
    stripePay: 'Pay Now',
    stripeSuccessProcessing: 'Payment successful, processing your order...',
    stripePopup: {
      redirecting: 'Redirecting to payment page...',
      loadingQr: 'Loading WeChat Pay QR code...',
      timeout: 'Timed out waiting for payment credentials, please retry',
      qrFailed: 'Failed to get WeChat Pay QR code',
    },
    subscribeNow: 'Subscribe Now',
    renewNow: 'Renew',
    selectPlan: 'Select Plan',
    planFeatures: 'Features',
    planCard: {
      rate: 'Rate',
      peakRate: 'Peak Rate',
      dailyLimit: 'Daily',
      weeklyLimit: 'Weekly',
      monthlyLimit: 'Monthly',
      quota: 'Quota',
      unlimited: 'Unlimited',
      models: 'Models',
    },
    days: 'days',
    weeks: 'weeks',
    months: 'months',
    years: 'years',
    oneMonth: '1 Month',
    oneYear: '1 Year',
    perMonth: 'month',
    perYear: 'year',
    admin: {
      tabs: {
        overview: 'Overview',
        orders: 'Orders',
        channels: 'Channels',
        plans: 'Plans',
      },
      todayRevenue: 'Today Revenue',
      totalRevenue: 'Total Revenue',
      todayOrders: 'Today Orders',
      orderCount: 'Order Count',
      avgAmount: 'Average Amount',
      revenue: 'Revenue',
      dailyRevenue: 'Daily Revenue',
      paymentDistribution: 'Payment Distribution',
      colUser: 'User',
      topUsers: 'Top Users',
      noData: 'No data',
      days: 'days',
      weeks: 'weeks',
      months: 'months',
      searchOrders: 'Search orders...',
      allStatuses: 'All Statuses',
      allPaymentTypes: 'All Payment Types',
      allOrderTypes: 'All Order Types',
      orderDetail: 'Order Detail',
      orderType: 'Order Type',
      orders: 'Orders',
      balanceOrder: 'Balance Top-Up',
      subscriptionOrder: 'Subscription',
      paidAt: 'Paid At',
      completedAt: 'Completed At',
      expiresAt: 'Expires At',
      feeRate: 'Fee Rate',
      refund: 'Refund',
      refundOrder: 'Refund Order',
      refundAmount: 'Refund Amount',
      maxRefundable: 'Max Refundable',
      refundReason: 'Refund Reason',
      refundReasonPlaceholder: 'Please enter refund reason',
      confirmRefund: 'Confirm Refund',
      refundSuccess: 'Refund successful',
      refundPending: 'Refund pending gateway confirmation',
      queryRefundStatus: 'Query refund status',
      refundInfo: 'Refund Info',
      refundEnabled: 'Refund Enabled',
      allowUserRefund: 'Allow User Refund',
      alreadyRefunded: 'Already Refunded',
      deductBalance: 'Deduct Balance',
      deductBalanceHint: 'Subtract recharged amount from user balance',
      userBalance: 'User Balance',
      orderAmount: 'Order Amount',
      insufficientBalance: 'Insufficient balance — will deduct to $0',
      noDeduction: 'Will NOT deduct user balance',
      forceRefund: 'Force refund (ignore balance check)',
      orderCancelled: 'Order Cancelled',
      retry: 'Retry',
      retrySuccess: 'Retry successful',
      approveRefund: 'Approve Refund',
      retryRefund: 'Retry Refund',
      refundRequestInfo: 'Refund Request Info',
      refundRequestedAt: 'Requested At',
      refundRequestedBy: 'Requested By',
      refundRequestReason: 'Request Reason',
      auditLogs: 'Audit Logs',
      operator: 'Operator',
      channelName: 'Channel Name',
      channelDescription: 'Channel Description',
      createChannel: 'Create Channel',
      editChannel: 'Edit Channel',
      deleteChannel: 'Delete Channel',
      deleteChannelConfirm: 'Are you sure you want to delete this channel?',
      planName: 'Plan Name',
      planDescription: 'Plan Description',
      createPlan: 'Create Plan',
      editPlan: 'Edit Plan',
      deletePlan: 'Delete Plan',
      deletePlanConfirm: 'Are you sure you want to delete this plan?',
      originalPrice: 'Original Price',
      price: 'Price',
      currency: 'Currency Label',
      currencyPlaceholder: 'e.g. USD / NZD / CNY',
      currencyHint: 'Display-only 3-letter ISO currency code shown next to the price; leave empty to hide, does not affect billing',
      subscriptionCnyPayPreview: 'CNY channel charge preview: {amount}',
      subscriptionCnyPayPreviewWithFee: '({feeRate}% fee included: {total})',
      validity: 'Validity',
      validityUnit: 'Validity Unit',
      sortOrder: 'Sort Order',
      forSale: 'For Sale',
      onSale: 'On Sale',
      offSale: 'Off Sale',
      group: 'Group',
      groupId: 'Group ID',
      features: 'Features',
      featuresHint: 'One feature per line',
      featuresPlaceholder: 'Enter plan features...',
      providerManagement: 'Provider Management',
      providerManagementDesc: 'Manage payment provider instances',
      createProvider: 'Create Provider',
      editProvider: 'Edit Provider',
      deleteProvider: 'Delete Provider',
      deleteProviderConfirm: 'Are you sure you want to delete this provider?',
      providerName: 'Provider Name',
      providerKey: 'Provider Key',
      selectProviderKey: 'Select Provider Key',
      providerConfig: 'Provider Config',
      noProviders: 'No providers configured',
      noProvidersHint: 'Create a provider instance to start accepting payments',
      supportedTypes: 'Supported Payment Types',
      supportedTypesHint: 'Select the payment types this provider supports',
      rateMultiplier: 'Rate Multiplier',
      dashboardTitle: 'Payment Dashboard',
      dashboardDesc: 'Recharge order analytics and insights',
      daySuffix: 'd',
      paymentConfigTitle: 'Payment Config',
      paymentConfigDesc: 'Configure payment providers and settings',
      plansPageTitle: 'Subscription Plans',
      plansPageDesc: 'Manage subscription plan configuration',
      tabPlanConfig: 'Plan Configuration',
      tabUserSubs: 'User Subscriptions',
      selectGroup: 'Select a group',
      groupRequired: 'Please select a subscription group',
      priceRequired: 'Price must be greater than 0',
      validityRequired: 'Validity must be greater than 0',
      groupMissing: 'Missing',
      groupInfo: 'Group Info',
      platform: 'Platform',
      rateMultiplierLabel: 'Rate',
      dailyLimit: 'Daily Limit',
      weeklyLimit: 'Weekly Limit',
      monthlyLimit: 'Monthly Limit',
      unlimited: 'Unlimited',
      searchUserSubs: 'Search user subscriptions...',
      daily: 'D',
      weekly: 'W',
      monthly: 'M',
      subsStatus: {
        active: 'Active',
        expired: 'Expired',
        revoked: 'Revoked',
      },
    },
  },

}
