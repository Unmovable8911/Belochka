export type AuthType = "password" | "key"

export interface Server {
  id: string
  name: string
  host: string
  port: number
  auth_type: AuthType
  username: string
  key_path?: string
  host_key_fingerprint?: string
  created_at: string
  updated_at: string
}

export interface ServerFormData {
  name: string
  host: string
  port: number
  username: string
  authType: AuthType
  password: string
  keyPath: string
}

export interface TestResult {
  fingerprint: string
}
