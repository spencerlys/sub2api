export default {
  finance: {
    title: 'Finance',
    description: 'Money received by source, usage consumption, owner settlements and owner payouts',
    loadFailed: 'Failed to load finance data',
    period: {
      label: 'Period',
      presets: {
        today: 'Today',
        yesterday: 'Yesterday',
        this_month: 'This month',
        last_month: 'Last month',
        last_7d: 'Last 7 days',
        last_30d: 'Last 30 days',
        all: 'All',
        custom: 'Custom'
      },
      start: 'Start date',
      end: 'End date',
      apply: 'Apply',
      invalidRange: 'Pick a start date on or before the end date',
      allTime: 'All time'
    },
    ownerSelect: {
      label: 'Owner',
      all: 'All owners',
      loadFailed: 'Failed to load owners'
    },
    balanceSource: {
      label: 'Source',
      gift: 'Gift',
      offline: 'Offline payment',
      correction: 'Correction',
      help: {
        gift: 'Free credit given to the user; counted as a gift, not income.',
        offline: 'Real money the user paid outside online payment (e.g. bank transfer); counted as income.',
        correction: 'Fixes a previous mistake; counted as neither income nor gift.'
      },
      updated: 'Source updated',
      updateFailed: 'Failed to update the source'
    },
    moneyIn: {
      title: 'Money in by source',
      online: 'Online payments',
      onlineOrders: '{count} orders',
      credited: 'Credited',
      bonus: 'Bonus',
      noPayments: 'No payments',
      offline: 'Offline payments',
      offlineCount: '{count} records',
      gift: 'Gifts',
      giftAdmin: 'Admin gifts',
      giftPaymentBonus: 'Payment bonus',
      giftPromo: 'Promo codes',
      giftRedeem: 'Redeem codes',
      giftAffiliate: 'Affiliate rebates',
      corrections: 'Corrections (net)',
      correctionsCount: '{count} records',
      deductions: 'Deductions',
      deductionsCount: '{count} records'
    },
    consumption: {
      title: 'Usage consumed',
      requests: 'Requests',
      userBilled: 'Receivable from users',
      userBilledHint: 'Total billed to users for usage in this period (paid use + internal use)',
      paidCost: 'Paid use',
      paidCostHint: 'Part of the receivable funded by money users paid (paid use)',
      giftCost: 'Internal use',
      giftCostHint: 'Part of the receivable funded by gifted credit (internal use)'
    },
    ledger: {
      title: 'Settlement ledger',
      description: 'Background job that splits every request into paid use and internal use',
      processedUntil: 'Processed until',
      lag: 'Lag',
      lastRebuild: 'Last rebuild',
      lastError: 'Last error',
      noError: 'None',
      neverRanShort: 'Never ran',
      warning: {
        lag: 'Settlement ledger is {minutes} min behind; figures may be low',
        rebuild: 'Settlement ledger is rebuilding; figures may be low for a moment',
        error: 'Settlement ledger hit an error; figures may be low'
      },
      lagUnits: {
        seconds: '{n} s',
        minutes: '{n} min',
        hours: '{n} h',
        days: '{n} d'
      },
      rebuild: 'Rebuild ledger',
      rebuildTitle: 'Rebuild settlement ledger',
      rebuildConfirm:
        'Recompute the paid / internal-use split for all usage from scratch? It runs in the background and usually finishes within a minute; figures may change afterwards.',
      rebuildStarted: 'Ledger rebuild scheduled',
      rebuildFailed: 'Failed to schedule the ledger rebuild',
      rebuildPending: 'Rebuild in progress'
    },
    settlement: {
      title: 'Owner settlement',
      owner: 'Owner',
      activeAccounts: 'Active accounts',
      requests: 'Requests',
      officialCost: 'Official price',
      userBilled: 'Receivable from users',
      paidCost: 'Paid use',
      giftCost: 'Internal use',
      accountCost: 'Account cost',
      payoutBase: 'Payout basis',
      earnings: 'Payable (period)',
      giftAccountCost: 'Internal-use cost',
      earningsAllTime: 'Payable (all time)',
      paidTotal: 'Paid out',
      unpaid: 'Unpaid',
      actions: 'Actions',
      totals: 'Total',
      empty: 'No owner activity in this period',
      recordPayout: 'Record payout',
      details: 'Details',
      hints: {
        activeAccounts: 'Accounts that are active and schedulable right now',
        officialCost: 'Usage at official list price',
        userBilled: 'What users were billed for this usage',
        paidCost: 'Part of the receivable funded by paid balance (paid use)',
        giftCost: 'Part of the receivable funded by gift balance (internal use)',
        accountCost: 'Official price × account multiplier',
        payoutBase: "Official price × owner payout rate (the group's rate at request time)",
        earnings: "Owed to the owner: account cost of paid use × the group's owner payout rate (rate in effect at request time)",
        giftAccountCost: "Account cost of internal use × the group's owner payout rate",
        unpaid: 'Payable (all time) minus paid out'
      }
    },
    payout: {
      title: 'Record payout',
      owner: 'Owner',
      amount: 'Amount (USD)',
      paidAt: 'Paid on',
      notes: 'Notes',
      amountRequired: 'Enter an amount greater than 0',
      success: 'Payout recorded',
      failed: 'Failed to record the payout',
      historyTitle: 'Payout history',
      empty: 'No payouts in this period',
      delete: 'Delete',
      deleteTitle: 'Delete payout',
      deleteConfirm: 'Delete the {amount} payout to {owner}? This cannot be undone.',
      deleted: 'Payout deleted',
      deleteFailed: 'Failed to delete the payout',
      columns: {
        owner: 'Owner',
        amount: 'Amount',
        paidAt: 'Paid on',
        notes: 'Notes',
        createdAt: 'Recorded at',
        actions: 'Actions'
      }
    },
    ownerEarnings: {
      title: 'My earnings',
      requests: 'Requests',
      officialCost: 'Total cost (official price)',
      earnings: 'Earnings this period',
      paidTotal: 'Paid out (all time)',
      unpaid: 'Unpaid (all time)',
      loadFailed: 'Failed to load earnings'
    }
  }
}
