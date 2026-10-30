/**
 * Where an admin balance adjustment's money came from.
 * gift = free credit, offline = real money received outside the payment system,
 * correction = fixing a mistake (neither income nor gift).
 *
 * Kept dependency-free so components can import the list without the API client.
 */
export type BalanceSource = 'gift' | 'offline' | 'correction'

export const BALANCE_SOURCES: BalanceSource[] = ['gift', 'offline', 'correction']
