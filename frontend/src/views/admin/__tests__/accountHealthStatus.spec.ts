import { describe, expect, it } from 'vitest'

import { ACCOUNT_HEALTH_THRESHOLDS, deriveAccountHealth } from '../accountHealthStatus'

type Input = Parameters<typeof deriveAccountHealth>[0]

function item(overrides: Partial<Input> = {}): Input {
  return {
    status: 'active',
    schedulable: true,
    rate_limited: false,
    overloaded: false,
    temp_unschedulable: false,
    requests_ok: 100,
    errors_total: 0,
    error_rate: 0,
    ...overrides
  }
}

// ok + errors with the matching error_rate, as the backend computes it.
function traffic(ok: number, errors: number): Partial<Input> {
  const total = ok + errors
  return { requests_ok: ok, errors_total: errors, error_rate: total > 0 ? errors / total : 0 }
}

describe('deriveAccountHealth', () => {
  it('pins the documented thresholds', () => {
    expect(ACCOUNT_HEALTH_THRESHOLDS).toEqual({
      MIN_SAMPLE: 5,
      FAILING_ERROR_RATE: 0.2,
      DEGRADED_ERROR_RATE: 0.05
    })
  })

  it('is healthy with traffic and a low error rate', () => {
    expect(deriveAccountHealth(item(traffic(96, 4)))).toBe('healthy')
  })

  it('is degraded from 5% up to (not including) 20%', () => {
    expect(deriveAccountHealth(item(traffic(95, 5)))).toBe('degraded')
    expect(deriveAccountHealth(item(traffic(81, 19)))).toBe('degraded')
  })

  it('is failing at 20% or more', () => {
    expect(deriveAccountHealth(item(traffic(80, 20)))).toBe('failing')
    expect(deriveAccountHealth(item(traffic(0, 10)))).toBe('failing')
  })

  it('ignores the error rate below the minimum sample', () => {
    expect(deriveAccountHealth(item(traffic(0, 4)))).toBe('healthy')
    expect(deriveAccountHealth(item(traffic(3, 1)))).toBe('healthy')
    expect(deriveAccountHealth(item(traffic(0, 5)))).toBe('failing')
  })

  it('is idle without any traffic', () => {
    expect(deriveAccountHealth(item(traffic(0, 0)))).toBe('idle')
  })

  it('maps account status "error" to failing, ahead of everything else', () => {
    expect(deriveAccountHealth(item({ status: 'error', ...traffic(0, 0) }))).toBe('failing')
    expect(deriveAccountHealth(item({ status: 'error', schedulable: false, rate_limited: true }))).toBe('failing')
  })

  it('is disabled when inactive or not schedulable', () => {
    expect(deriveAccountHealth(item({ status: 'inactive' }))).toBe('disabled')
    expect(deriveAccountHealth(item({ schedulable: false, ...traffic(0, 50) }))).toBe('disabled')
    expect(deriveAccountHealth(item({ schedulable: false, rate_limited: true }))).toBe('disabled')
  })

  it('is rate_limited while any cooldown is active, even with a high error rate', () => {
    expect(deriveAccountHealth(item({ rate_limited: true }))).toBe('rate_limited')
    expect(deriveAccountHealth(item({ overloaded: true }))).toBe('rate_limited')
    expect(deriveAccountHealth(item({ temp_unschedulable: true, ...traffic(0, 0) }))).toBe('rate_limited')
    expect(deriveAccountHealth(item({ rate_limited: true, ...traffic(10, 90) }))).toBe('rate_limited')
  })
})
