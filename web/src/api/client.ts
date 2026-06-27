import type { Server, TestResult, CronResult } from "@/types/server"

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
