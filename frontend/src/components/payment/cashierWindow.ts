import { getPaymentPopupFeatures } from './providerConfig'

/**
 * Shared handle to the gateway cashier window we opened (popup or new tab).
 *
 * Gateway cashier pages (e.g. shop01) have no return URL, so once the order
 * reaches a terminal state we close the window ourselves instead of leaving
 * the user stranded there. Only one cashier window is tracked at a time.
 */
let cashierWindow: Window | null = null

export type CashierWindowMode = 'popup' | 'tab'

/**
 * Opens the cashier and keeps the handle. Deliberately no `noopener`: that
 * would make window.open return null and the window could never be closed.
 * Returns null when the browser blocked the window.
 */
export function openCashierWindow(url: string, mode: CashierWindowMode = 'popup'): Window | null {
  if (!url || typeof window === 'undefined') return null
  const win = mode === 'popup'
    ? window.open(url, 'paymentPopup', getPaymentPopupFeatures())
    : window.open(url, '_blank')
  if (!win || win.closed) return null
  if (cashierWindow && cashierWindow !== win) closeCashierWindow()
  cashierWindow = win
  return win
}

/** Closes the tracked cashier window, if any. Never throws. */
export function closeCashierWindow(): void {
  const win = cashierWindow
  cashierWindow = null
  if (!win) return
  try {
    if (!win.closed) win.close()
  } catch {
    // Cross-origin or already-gone windows may throw; nothing else to do.
  }
}

/** Test/diagnostic accessor for the tracked handle. */
export function getCashierWindow(): Window | null {
  return cashierWindow
}
