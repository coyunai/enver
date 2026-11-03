import type {
  CatalogPlugin,
  CreateVenvRequest,
  DoctorReport,
  Job,
  JobEvent,
  Overview,
  PackageItem,
  Preset,
  SDK,
  StorageInfo,
  UseRequest,
  Venv,
  Version,
  VfoxState,
} from './types'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers || {}),
    },
  })
  const text = await res.text()
  let data: unknown = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = text
    }
  }
  if (!res.ok) {
    const body = data as { error?: { message?: string } } | string | null
    const message =
      typeof body === 'object' && body?.error?.message
        ? body.error.message
        : typeof body === 'string' && body
          ? body
          : `请求失败 (${res.status})`
    if (res.status === 401) {
      window.location.reload()
    }
    throw new Error(message)
  }
  return data as T
}

export const api = {
  overview: () => request<Overview>('/api/overview'),
  vfox: () => request<VfoxState>('/api/vfox'),
  setVfoxBin: (bin: string) =>
    request<VfoxState>('/api/vfox', { method: 'PUT', body: JSON.stringify({ bin }) }),
  catalog: () => request<{ source: string; items: CatalogPlugin[] }>('/api/catalog'),
  sdks: () => request<{ sdks: SDK[] }>('/api/sdks'),
  versions: (sdk: string) =>
    request<{ sdk: string; versions: Version[] }>(`/api/sdks/${encodeURIComponent(sdk)}/versions`),
  addPlugin: (sdk: string) =>
    request<Job>(`/api/sdks/${encodeURIComponent(sdk)}/plugins`, { method: 'POST' }),
  install: (sdk: string, version: string) =>
    request<Job>('/api/install', { method: 'POST', body: JSON.stringify({ sdk, version }) }),
  uninstall: (sdk: string, version: string) =>
    request<Job>('/api/uninstall', { method: 'POST', body: JSON.stringify({ sdk, version }) }),
  use: (body: UseRequest) =>
    request<{ logs: string[] }>('/api/use', { method: 'POST', body: JSON.stringify(body) }),
  presets: () => request<{ presets: Preset[] }>('/api/presets'),
  savePreset: (preset: Preset, isNew: boolean) =>
    request<Preset>(isNew ? '/api/presets' : `/api/presets/${encodeURIComponent(preset.name)}`, {
      method: isNew ? 'POST' : 'PUT',
      body: JSON.stringify(preset),
    }),
  deletePreset: (name: string) =>
    request<void>(`/api/presets/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  applyPreset: (name: string, scope: 'global' | 'project', dir: string) =>
    request<{ logs: string[] }>(`/api/presets/${encodeURIComponent(name)}/apply`, {
      method: 'POST',
      body: JSON.stringify({ scope, dir }),
    }),
  venvs: () => request<{ venvs: Venv[] }>('/api/venvs'),
  createVenv: (body: CreateVenvRequest) =>
    request<{ venv: Venv; logs: string[] }>('/api/venvs', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  deleteVenv: (id: string) => request<void>(`/api/venvs/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  rebuildVenv: (id: string) =>
    request<{ venv: Venv; logs: string[] }>(`/api/venvs/${encodeURIComponent(id)}/rebuild`, {
      method: 'POST',
    }),
  packages: (sdk: string, version: string) =>
    request<{ packages: PackageItem[] }>(
      `/api/packages?sdk=${encodeURIComponent(sdk)}&version=${encodeURIComponent(version)}`,
    ),
  doctor: () => request<DoctorReport>('/api/doctor'),
  storage: () => request<StorageInfo>('/api/storage'),
  migrate: (path: string, deleteOld: boolean) =>
    request<Job>('/api/storage/migrate', {
      method: 'POST',
      body: JSON.stringify({ path, deleteOld }),
    }),
  job: (id: string) => request<Job>(`/api/jobs/${encodeURIComponent(id)}`),
  cancelJob: (id: string) =>
    request<void>(`/api/jobs/${encodeURIComponent(id)}`, { method: 'DELETE' }),
}

export function streamJob(
  id: string,
  onEvent: (event: JobEvent) => void,
  onEnd: (event: JobEvent) => void,
): () => void {
  const source = new EventSource(`/api/jobs/${encodeURIComponent(id)}/events`)
  let closed = false
  const close = () => {
    if (!closed) {
      closed = true
      source.close()
    }
  }
  source.onmessage = (message) => {
    try {
      const event = JSON.parse(message.data) as JobEvent
      onEvent(event)
      if (event.type === 'done') {
        close()
        onEnd(event)
      }
    } catch {
      // ignore malformed frames
    }
  }
  source.onerror = () => {
    if (closed) return
    close()
    onEnd({ type: 'done', status: 'failed', error: '事件流连接中断' })
  }
  return close
}

export function terminalURL(params: {
  preset?: string
  venv?: string
  cwd: string
  cols?: number
  rows?: number
}): string {
  const query = new URLSearchParams()
  if (params.preset) query.set('preset', params.preset)
  if (params.venv) query.set('venv', params.venv)
  if (params.cwd) query.set('cwd', params.cwd)
  if (params.cols) query.set('cols', String(params.cols))
  if (params.rows) query.set('rows', String(params.rows))
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/api/terminal?${query.toString()}`
}

export async function downloadPackages(sdk: string, version: string): Promise<void> {
  const res = await fetch(
    `/api/packages/export?sdk=${encodeURIComponent(sdk)}&version=${encodeURIComponent(version)}`,
  )
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || '导出失败')
  }
  const blob = await res.blob()
  const disposition = res.headers.get('Content-Disposition') || ''
  const match = /filename="([^"]+)"/.exec(disposition)
  const filename = match?.[1] || (sdk === 'nodejs' ? 'global-packages.json' : 'requirements.txt')
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}
