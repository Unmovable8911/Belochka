const INITIAL_DELAY_MS = 1000
const MAX_DELAY_MS = 30000

/**
 * Calculate reconnection delay using exponential backoff.
 * attempt=0 returns INITIAL_DELAY_MS, each subsequent attempt doubles it, capped at MAX_DELAY_MS.
 */
export function getReconnectDelay(attempt: number): number {
  return Math.min(INITIAL_DELAY_MS * 2 ** attempt, MAX_DELAY_MS)
}
