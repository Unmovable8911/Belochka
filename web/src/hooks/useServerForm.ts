import { useState } from "react"
import { useTranslation } from "react-i18next"
import * as api from "@/api/client"

/**
 * useServerForm encapsulates the connection-test / host-key-fingerprint /
 * save state machine shared by the Add and Edit server dialogs. Each dialog
 * owns its own form data and change-detection logic; this hook owns the
 * test/trust/save lifecycle.
 */
export function useServerForm() {
  const { t } = useTranslation()
  const [testing, setTesting] = useState(false)
  const [testError, setTestError] = useState<string | null>(null)
  const [fingerprint, setFingerprint] = useState<string | null>(null)
  const [fingerprintTrusted, setFingerprintTrusted] = useState(false)
  const [saving, setSaving] = useState(false)

  /** Clears the connection-test outcome (error, fingerprint, trust). */
  function resetTestState() {
    setTestError(null)
    setFingerprint(null)
    setFingerprintTrusted(false)
  }

  /** Resets the entire test/save state machine. */
  function reset() {
    setTesting(false)
    resetTestState()
    setSaving(false)
  }

  /**
   * Runs a stateless connection test with the given request body, recording
   * the returned fingerprint or the error. Returns true on success.
   */
  async function runTest(body: Record<string, unknown>): Promise<boolean> {
    setTesting(true)
    resetTestState()
    try {
      const result = await api.testConnection(body)
      setFingerprint(result.fingerprint)
      return true
    } catch (err) {
      setTestError(err instanceof Error ? err.message : t("addServer.connectionTestFailed"))
      return false
    } finally {
      setTesting(false)
    }
  }

  return {
    testing,
    testError,
    fingerprint,
    fingerprintTrusted,
    saving,
    setTestError,
    setSaving,
    trust: () => setFingerprintTrusted(true),
    resetTestState,
    reset,
    runTest,
  }
}
