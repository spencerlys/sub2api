export default {
  finance: {
    title: '财务对账',
    description: '按来源统计入账、号主结算与号主打款',
    loadFailed: '加载财务数据失败',
    period: {
      label: '统计周期',
      thisMonth: '本月',
      lastMonth: '上月',
      custom: '自定义',
      start: '开始日期',
      end: '结束日期',
      apply: '应用',
      invalidRange: '开始日期不能晚于结束日期，且跨度不超过 366 天'
    },
    ownerSelect: {
      label: '号主',
      all: '全部号主',
      loadFailed: '加载号主列表失败'
    },
    balanceSource: {
      label: '来源',
      gift: '赠送',
      offline: '线下收款',
      correction: '更正',
      help: {
        gift: '免费赠送给用户的额度，计入赠送，不计入收入。',
        offline: '用户通过线上支付以外方式（如转账）实际付的钱，计入收入。',
        correction: '修正之前的错误操作，既不计入收入也不计入赠送。'
      },
      updated: '来源已更新',
      updateFailed: '更新来源失败'
    },
    moneyIn: {
      title: '入账来源',
      online: '线上支付',
      onlineOrders: '{count} 笔订单',
      credited: '到账',
      bonus: '赠送',
      noPayments: '暂无支付',
      offline: '线下收款',
      offlineCount: '{count} 笔',
      gift: '赠送',
      giftAdmin: '管理员赠送',
      giftPaymentBonus: '充值赠送',
      giftPromo: '优惠码',
      giftRedeem: '兑换码',
      giftAffiliate: '邀请返利',
      corrections: '更正（净额）',
      correctionsCount: '{count} 笔',
      deductions: '扣减',
      deductionsCount: '{count} 笔'
    },
    settlement: {
      title: '号主结算',
      owner: '号主',
      requests: '请求数',
      officialCost: '官方价',
      userBilled: '用户实付',
      accountCost: '账号成本',
      earnings: '本期收益',
      earningsAllTime: '累计收益',
      paidTotal: '已打款',
      unpaid: '未打款',
      actions: '操作',
      totals: '合计',
      empty: '本期没有号主数据',
      recordPayout: '记录打款'
    },
    payout: {
      title: '记录打款',
      owner: '号主',
      amount: '金额（美元）',
      paidAt: '打款日期',
      notes: '备注',
      amountRequired: '请输入大于 0 的金额',
      success: '打款已记录',
      failed: '记录打款失败',
      historyTitle: '打款记录',
      empty: '本期没有打款记录',
      delete: '删除',
      deleteTitle: '删除打款记录',
      deleteConfirm: '确定删除给 {owner} 的 {amount} 打款记录吗？此操作不可撤销。',
      deleted: '打款记录已删除',
      deleteFailed: '删除打款记录失败',
      columns: {
        owner: '号主',
        amount: '金额',
        paidAt: '打款日期',
        notes: '备注',
        createdAt: '记录时间',
        actions: '操作'
      }
    },
    ownerEarnings: {
      title: '我的收益',
      requests: '请求数',
      officialCost: '总成本（官方价）',
      earnings: '本期收益',
      paidTotal: '已打款（累计）',
      unpaid: '未打款',
      loadFailed: '加载收益失败'
    }
  }
}
