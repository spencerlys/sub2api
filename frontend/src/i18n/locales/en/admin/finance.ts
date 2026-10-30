export default {
  finance: {
    title: 'Finance',
    description: 'Money received by source, owner settlements and owner payouts',
    loadFailed: 'Failed to load finance data',
    period: {
      label: 'Period',
      thisMonth: 'This month',
      lastMonth: 'Last month',
      custom: 'Custom',
      start: 'Start date',
      end: 'End date',
      apply: 'Apply',
      invalidRange: 'Pick a start date on or before the end date, at most 366 days apart'
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
    settlement: {
      title: 'Owner settlement',
      owner: 'Owner',
      requests: 'Requests',
      officialCost: 'Official price',
      userBilled: 'Billed to users',
      accountCost: 'Account cost',
      earnings: 'Earnings (period)',
      earningsAllTime: 'Earnings (all time)',
      paidTotal: 'Paid out',
      unpaid: 'Unpaid',
      actions: 'Actions',
      totals: 'Total',
      empty: 'No owner activity in this period',
      recordPayout: 'Record payout'
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
      unpaid: 'Unpaid',
      loadFailed: 'Failed to load earnings'
    }
  }
}
