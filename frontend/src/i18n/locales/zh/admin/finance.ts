export default {
  finance: {
    title: '财务对账',
    description: '按来源统计入账、用量消耗、号主结算与号主打款',
    loadFailed: '加载财务数据失败',
    period: {
      label: '统计周期',
      presets: {
        today: '今天',
        yesterday: '昨天',
        this_month: '本月',
        last_month: '上月',
        last_7d: '近 7 天',
        last_30d: '近 30 天',
        all: '全部',
        custom: '自定义'
      },
      start: '开始日期',
      end: '结束日期',
      apply: '应用',
      invalidRange: '开始日期不能晚于结束日期',
      allTime: '全部时间'
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
    consumption: {
      title: '用量消耗',
      requests: '请求数',
      userBilled: '用户应收',
      userBilledHint: '本期用量向用户计费的总额（付费调用 + 内部调用）',
      paidCost: '付费调用',
      paidCostHint: '应收中由用户真实充值的余额支付的部分（付费调用）',
      giftCost: '内部调用',
      giftCostHint: '应收中由赠送额度支付的部分（内部调用）'
    },
    ledger: {
      title: '结算账本',
      description: '后台任务，将每个请求精确拆分为付费调用与内部调用',
      processedUntil: '已处理至',
      lag: '延迟',
      lastRebuild: '上次重建',
      lastError: '最近错误',
      noError: '无',
      neverRanShort: '尚未运行',
      warning: {
        lag: '结算账本延迟 {minutes} 分钟，数字可能偏低',
        rebuild: '结算账本重建中，数字可能暂时偏低',
        error: '结算账本运行异常，数字可能偏低'
      },
      lagUnits: {
        seconds: '{n} 秒',
        minutes: '{n} 分钟',
        hours: '{n} 小时',
        days: '{n} 天'
      },
      rebuild: '重建账本',
      rebuildTitle: '重建结算账本',
      rebuildConfirm: '确定从头重新计算全部用量的付费调用 / 内部调用拆分吗？任务在后台运行，通常一分钟内完成，完成后数据可能变化。',
      rebuildStarted: '已安排重建账本',
      rebuildFailed: '安排重建账本失败',
      rebuildPending: '重建中'
    },
    settlement: {
      title: '号主结算',
      owner: '号主',
      activeAccounts: '可用账号',
      requests: '请求数',
      officialCost: '官方价',
      userBilled: '用户应收',
      paidCost: '付费调用',
      giftCost: '内部调用',
      accountCost: '账号成本',
      payoutBase: '分成基数',
      earnings: '本期应付',
      giftAccountCost: '内部调用成本',
      earningsAllTime: '累计应付',
      paidTotal: '已打款',
      unpaid: '未打款',
      actions: '操作',
      totals: '合计',
      empty: '本期没有号主数据',
      recordPayout: '记录打款',
      details: '明细',
      hints: {
        activeAccounts: '当前处于启用且可调度状态的账号',
        officialCost: '按官方价计算的用量',
        userBilled: '这些用量向用户计费的金额',
        paidCost: '应收中由充值余额支付的部分（付费调用）',
        giftCost: '应收中由赠送额度支付的部分（内部调用）',
        accountCost: '官方价 × 账号倍率',
        payoutBase: '官方价 × 号主分成倍率（按请求时的分组倍率）',
        earnings: '应付号主：付费调用对应的账号成本 × 分组号主分成倍率（按请求时的倍率）',
        giftAccountCost: '内部调用对应的账号成本 × 分组号主分成倍率',
        unpaid: '累计应付减去已打款'
      }
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
      unpaid: '未打款（累计）',
      loadFailed: '加载收益失败'
    }
  }
}
