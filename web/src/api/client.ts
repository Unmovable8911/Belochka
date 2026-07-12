import type { Server, TestResult, CronResult, CronEntry, CronRunResult, AppConfig, PatchConfigResponse, AuthStatus } from "@/types/server"

export class ApiError extends Error {
  code: string

  constructor(code: string, message: string) {
    super(message)
    this.code = code
    this.name = "ApiError"
  }
}

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, options)

  if (res.status === 401 && !url.includes("/api/auth/")) {
    const redirect = encodeURIComponent(window.location.pathname + window.location.search)
    window.location.href = `/login?redirect=${redirect}`
    throw new ApiError("unauthorized", "Session expired")
  }

  if (!res.ok) {
    let code = "unknown"
    let message = `Request failed with status ${res.status}`
    try {
      const body = await res.json()
      if (body.error) {
        code = body.error.code || code
        message = body.error.message || message
      }
    } catch {
      // response wasn't JSON
    }
    throw new ApiError(code, message)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

export async function getServer(id: string): Promise<Server> {
  return request<Server>(`/api/servers/${id}`)
}

export async function createServer(data: Record<string, unknown>): Promise<Server> {
  return request<Server>("/api/servers", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
}

export async function updateServer(id: string, data: Record<string, unknown>): Promise<Server> {
  return request<Server>(`/api/servers/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
}

export async function deleteServer(id: string): Promise<void> {
  return request<void>(`/api/servers/${id}`, { method: "DELETE" })
}

export async function reconnectServer(id: string): Promise<void> {
  return request<void>(`/api/servers/${id}/reconnect`, { method: "POST" })
}

export async function testConnection(data: Record<string, unknown>): Promise<TestResult> {
  return request<TestResult>("/api/servers/test", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
}

export async function getCrons(serverId: string): Promise<CronResult> {
  return request<CronResult>(`/api/servers/${serverId}/crons`)
}

export interface CreateCronPayload {
  minute: string
  hour: string
  dayOfMonth: string
  month: string
  dayOfWeek: string
  command: string
}

export async function createCron(serverId: string, payload: CreateCronPayload): Promise<CronEntry> {
  return request<CronEntry>(`/api/servers/${serverId}/crons`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
}

export interface UpdateCronPayload {
  minute: string
  hour: string
  dayOfMonth: string
  month: string
  dayOfWeek: string
  command: string
  enabled: boolean
}

export async function updateCron(serverId: string, index: number, payload: UpdateCronPayload): Promise<CronEntry> {
  return request<CronEntry>(`/api/servers/${serverId}/crons/${index}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
}

export async function deleteCron(serverId: string, index: number): Promise<void> {
  return request<void>(`/api/servers/${serverId}/crons/${index}`, { method: "DELETE" })
}

export async function runCron(serverId: string, index: number): Promise<CronRunResult> {
  return request<CronRunResult>(`/api/servers/${serverId}/crons/${index}/run`, { method: "POST" })
}

export async function getConfig(): Promise<AppConfig> {
  return request<AppConfig>("/api/config")
}

export async function uploadKeyFile(file: File): Promise<{ path: string }> {
  const formData = new FormData()
  formData.append("keyfile", file)

  const res = await fetch("/api/files/key", {
    method: "POST",
    body: formData,
  })

  if (!res.ok) {
    let message = `Upload failed with status ${res.status}`
    try {
      const body = await res.json()
      if (body.error?.message) {
        message = body.error.message
      }
    } catch {
      // response wasn't JSON
    }
    throw new Error(message)
  }

  return res.json()
}

export async function patchConfig(data: Partial<AppConfig>): Promise<PatchConfigResponse> {
  return request<PatchConfigResponse>("/api/config", {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  })
}

// --- Auth ---

export async function getAuthStatus(): Promise<AuthStatus> {
  return request<AuthStatus>("/api/auth/status")
}

export async function login(password: string): Promise<void> {
  await fetch("/api/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password }),
  }).then(async (res) => {
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      const msg = body?.error?.message || `Login failed (${res.status})`
      throw new ApiError(body?.error?.code || "unknown", msg)
    }
  })
}

export async function setup(password: string, confirmPassword: string): Promise<void> {
  await fetch("/api/setup", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password, confirm_password: confirmPassword }),
  }).then(async (res) => {
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      const msg = body?.error?.message || `Setup failed (${res.status})`
      throw new ApiError(body?.error?.code || "unknown", msg)
    }
  })
}

export async function logout(): Promise<void> {
  await request<void>("/api/logout", { method: "POST" })
}

export async function changePassword(oldPassword: string, newPassword: string, confirmPassword: string): Promise<void> {
  await request<void>("/api/change-password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      old_password: oldPassword,
      new_password: newPassword,
      confirm_password: confirmPassword,
    }),
  })
}
