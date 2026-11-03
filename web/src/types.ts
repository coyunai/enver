export type VfoxInfo = {
  bin: string
  version: string
  home: string
  sdkRoot: string
  registry: string
  configPath: string
  compatible: boolean
  minVersion: string
}

export type VfoxState = {
  available: boolean
  info?: VfoxInfo
  error?: string
}

export type Version = {
  version: string
  path?: string
  installed: boolean
  current: boolean
  stable?: boolean
  lts?: boolean
  meta?: string
}

export type SDK = {
  name: string
  current?: string
  versions: Version[]
}

export type StorageInfo = {
  home: string
  sdkRoot: string
  registry: string
  configPath: string
  version: string
  freeBytes: number
  platform: string
}

export type Overview = {
  vfox?: VfoxInfo
  vfoxError?: string
  sdks: SDK[]
  presetCount: number
  venvCount: number
  storage: StorageInfo
  platform: string
  systemPython?: string
  enverVersion: string
}

export type CatalogPlugin = {
  name: string
  desc: string
  homepage: string
  installed: boolean
}

export type Preset = {
  name: string
  description?: string
  runtimes: Record<string, string>
  env?: Record<string, string>
  python?: { venv?: string; tool?: string; autoSync?: boolean }
  packages?: Record<string, string[]>
  updatedAt?: string
}

export type Venv = {
  id: string
  name: string
  path: string
  pythonPath: string
  pythonVersion: string
  tool: string
  projectDir?: string
  preset?: string
  createdAt: string
  healthy: boolean
  health?: string
  pythonExe: string
}

export type PackageItem = {
  name: string
  version: string
  manager: string
}

export type Check = {
  id: string
  title: string
  status: 'ok' | 'warn' | 'error' | 'info'
  detail: string
  hint?: string
}

export type DoctorReport = {
  checkedAt: string
  checks: Check[]
  summary: Record<string, number>
}

export type JobStatus = 'running' | 'success' | 'failed' | 'canceled'

export type Job = {
  id: string
  kind: string
  title: string
  status: JobStatus
  error?: string
  lines?: string[]
  startedAt: string
  endedAt?: string
}

export type JobEvent = {
  type: 'log' | 'done'
  line?: string
  status?: JobStatus
  error?: string
  time?: string
}

export type UseRequest = {
  sdk: string
  version: string
  scope: 'global' | 'project' | 'session'
  dir?: string
}

export type CreateVenvRequest = {
  name?: string
  path?: string
  projectDir?: string
  pythonVersion?: string
  pythonPath?: string
  tool?: string
  preset?: string
}
