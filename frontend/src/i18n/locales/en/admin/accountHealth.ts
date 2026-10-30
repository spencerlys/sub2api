export default {
  accountHealth: {
    title: 'Account Health',
    description: 'Recent requests, errors, latency and cost per account, to spot accounts that are failing, rate-limited or slow',
    rangeLabel: 'Time range',
    loadFailed: 'Failed to load account health',
    emptyTitle: 'No accounts',
    emptyDescription: 'There are no accounts to report on yet',
    columns: {
      account: 'Account',
      health: 'Health',
      requests: 'Requests',
      errorRate: 'Error Rate',
      ttft: 'TTFT p50 / p95',
      duration: 'Duration p50',
      tokens: 'Tokens In / Out',
      cost: 'Cost',
      lastSuccess: 'Last Success',
      lastError: 'Last Error'
    },
    status: {
      healthy: 'Healthy',
      degraded: 'Degraded',
      failing: 'Failing',
      rate_limited: 'Rate Limited',
      disabled: 'Disabled',
      idle: 'Idle'
    },
    statusHint: {
      healthy: 'Error rate is below 5%',
      degraded: 'Error rate is 5% or higher (at least 5 requests)',
      failing: 'Account is in error state, or error rate is 20% or higher (at least 5 requests)',
      rate_limited: 'Currently rate-limited, overloaded or temporarily unschedulable',
      disabled: 'Account is inactive or scheduling is turned off',
      idle: 'No requests in this time range'
    },
    rateLimitedUntil: 'Rate limited until {time}',
    overloadedUntil: 'Overloaded until {time}',
    tempUnschedulableUntil: 'Temporarily unschedulable until {time}',
    accountError: 'Account error',
    errorBreakdown: '{errors} errors of {total} requests',
    tokensTitle: 'Input {input} / Output {output} / Cache {cache}',
    totals: {
      title: 'Total (shown accounts)',
      requests: 'Requests',
      errors: 'Errors',
      tokensIn: 'Input tokens',
      tokensOut: 'Output tokens',
      tokensCache: 'Cache tokens',
      cost: 'Cost'
    },
    errorCategory: {
      rate_limit: 'Rate limit',
      overloaded: 'Overloaded',
      auth: 'Auth',
      upstream_5xx: 'Upstream 5xx',
      timeout: 'Timeout',
      other: 'Other'
    }
  }
}
