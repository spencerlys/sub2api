export default {
  accountHealth: {
    title: '账号健康',
    description: '按账号查看近期请求、错误、延迟与费用，及时发现失败、限流或变慢的账号',
    rangeLabel: '时间范围',
    loadFailed: '加载账号健康数据失败',
    emptyTitle: '暂无账号',
    emptyDescription: '当前没有可统计的账号',
    columns: {
      account: '账号',
      health: '健康状态',
      requests: '成功请求',
      errorRate: '错误率',
      ttft: '首字 p50 / p95',
      duration: '耗时 p50',
      tokens: '输入 / 输出 Token',
      billed: '用户计费',
      officialCost: '官方价',
      earnings: '应付',
      lastSuccess: '最近成功',
      lastError: '最近错误'
    },
    status: {
      healthy: '健康',
      degraded: '降级',
      failing: '失败',
      rate_limited: '限流中',
      disabled: '已停用',
      idle: '空闲'
    },
    statusHint: {
      healthy: '错误率低于 5%',
      degraded: '错误率不低于 5%（至少 5 次请求）',
      failing: '账号处于错误状态，或错误率不低于 20%（至少 5 次请求）',
      rate_limited: '当前处于限流、过载或临时不可调度状态',
      disabled: '账号未启用或已关闭调度',
      idle: '该时间范围内没有请求'
    },
    rateLimitedUntil: '限流至 {time}',
    overloadedUntil: '过载至 {time}',
    tempUnschedulableUntil: '临时不可调度至 {time}',
    accountError: '账号错误',
    errorBreakdown: '{total} 次请求中 {errors} 次错误',
    tokensTitle: '输入 {input} / 输出 {output} / 缓存 {cache}',
    totals: {
      title: '合计（当前显示账号）',
      requests: '请求数',
      errors: '错误数',
      tokensIn: '输入 Token',
      tokensOut: '输出 Token',
      tokensCache: '缓存 Token',
      billed: '用户计费',
      officialCost: '官方价',
      earnings: '应付'
    },
    unowned: '无号主',
    errorCategory: {
      rate_limit: '限流',
      overloaded: '过载',
      auth: '认证',
      upstream_5xx: '上游 5xx',
      timeout: '超时',
      other: '其他'
    }
  }
}
