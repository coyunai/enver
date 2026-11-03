import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'
import { api, streamJob } from './api'
import type { Job, JobEvent, JobStatus } from './types'

const iconPaths: Record<string, string> = {
  dashboard: 'M3 3h7v9H3zM14 3h7v5h-7zM14 12h7v9h-7zM3 16h7v5H3z',
  box: 'M21 8l-9-5-9 5 9 5 9-5zM3 8v8l9 5 9-5V8M12 13v8',
  layers: 'M12 2l9 5-9 5-9-5 9-5zM3 12l9 5 9-5M3 17l9 5 9-5',
  terminal: 'M4 4h16v16H4zM8 9l3 3-3 3M13 15h4',
  package: 'M12 2l9 5v10l-9 5-9-5V7l9-5zM3 7l9 5 9-5M12 12v10',
  activity: 'M22 12h-4l-3 9L9 3l-3 9H2',
  settings:
    'M12 15a3 3 0 100-6 3 3 0 000 6zM19.4 15a1.7 1.7 0 00.3 1.9l.1.1a2 2 0 11-2.8 2.8l-.1-.1a1.7 1.7 0 00-1.9-.3 1.7 1.7 0 00-1 1.5V21a2 2 0 11-4 0v-.1a1.7 1.7 0 00-1.1-1.5 1.7 1.7 0 00-1.9.3l-.1.1a2 2 0 11-2.8-2.8l.1-.1a1.7 1.7 0 00.3-1.9 1.7 1.7 0 00-1.5-1H3a2 2 0 110-4h.1a1.7 1.7 0 001.5-1.1 1.7 1.7 0 00-.3-1.9l-.1-.1a2 2 0 112.8-2.8l.1.1a1.7 1.7 0 001.9.3H9a1.7 1.7 0 001-1.5V3a2 2 0 114 0v.1a1.7 1.7 0 001 1.5 1.7 1.7 0 001.9-.3l.1-.1a2 2 0 112.8 2.8l-.1.1a1.7 1.7 0 00-.3 1.9V9a1.7 1.7 0 001.5 1H21a2 2 0 110 4h-.1a1.7 1.7 0 00-1.5 1z',
  plus: 'M12 5v14M5 12h14',
  refresh:
    'M23 4v6h-6M1 20v-6h6M3.5 9a9 9 0 0114.9-3.4L23 10M1 14l4.6 4.4A9 9 0 0020.5 15',
  trash: 'M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6M10 11v6M14 11v6',
  play: 'M5 3l14 9-14 9V3z',
  download: 'M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M7 10l5 5 5-5M12 15V3',
  folder: 'M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z',
  check: 'M20 6L9 17l-5-5',
  alert: 'M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0zM12 9v4M12 17h.01',
  x: 'M18 6L6 18M6 6l12 12',
  external: 'M18 13v6a2 2 0 01-2 2H5a2 2 0 01-2-2V8a2 2 0 012-2h6M15 3h6v6M10 14L21 3',
  search: 'M11 19a8 8 0 100-16 8 8 0 000 16zM21 21l-4.3-4.3',
  cpu: 'M9 9h6v6H9zM4 9h2M4 15h2M18 9h2M18 15h2M9 4v2M15 4v2M9 18v2M15 18v2M6 6h12v12H6z',
  disk: 'M22 12H2M5.5 5h13l3.5 7v7a2 2 0 01-2 2H5a2 2 0 01-2-2v-7l2.5-7zM7 16h.01M11 16h.01',
  arrow: 'M5 12h14M13 6l6 6-6 6',
  clock: 'M12 22a10 10 0 100-20 10 10 0 000 20zM12 6v6l4 2',
  spark: 'M13 2L3 14h8l-1 8 10-12h-8l1-8z',
}

export function Icon({ name, size = 17 }: { name: keyof typeof iconPaths | string; size?: number }) {
  return (
    <svg
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={iconPaths[name] || iconPaths.box} />
    </svg>
  )
}

export function Card({
  title,
  subtitle,
  actions,
  children,
  className = '',
}: {
  title?: ReactNode
  subtitle?: ReactNode
  actions?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <section className={`card ${className}`}>
      {(title || actions) && (
        <div className="card-head">
          <div>
            {title && <h3 className="card-title">{title}</h3>}
            {subtitle && <p className="card-sub">{subtitle}</p>}
          </div>
          {actions && <div className="row">{actions}</div>}
        </div>
      )}
      {children}
    </section>
  )
}

export function Badge({
  children,
  tone = '',
}: {
  children: ReactNode
  tone?: '' | 'ok' | 'warn' | 'bad' | 'blue'
}) {
  return <span className={`badge ${tone}`}>{children}</span>
}

export function Field({ label, children, hint }: { label: string; children: ReactNode; hint?: string }) {
  return (
    <div className="field">
      <label>{label}</label>
      {children}
      {hint && <span className="faint small">{hint}</span>}
    </div>
  )
}

export function PageHeader({
  title,
  subtitle,
  actions,
}: {
  title: string
  subtitle?: string
  actions?: ReactNode
}) {
  return (
    <div className="topbar">
      <div>
        <h1>{title}</h1>
        {subtitle && <p>{subtitle}</p>}
      </div>
      {actions && <div className="topbar-actions">{actions}</div>}
    </div>
  )
}

export function Modal({
  open,
  title,
  children,
  footer,
  onClose,
  wide = false,
}: {
  open: boolean
  title: ReactNode
  children: ReactNode
  footer?: ReactNode
  onClose: () => void
  wide?: boolean
}) {
  if (!open) return null
  return createPortal(
    <div className="overlay" onMouseDown={onClose}>
      <div className={`modal ${wide ? 'wide' : ''}`} onMouseDown={(e) => e.stopPropagation()}>
        <div className="modal-head">
          <div className="modal-title">{title}</div>
          <button className="btn ghost icon-only small" onClick={onClose} aria-label="关闭">
            <Icon name="x" size={15} />
          </button>
        </div>
        <div className="modal-body">{children}</div>
        {footer && <div className="modal-foot">{footer}</div>}
      </div>
    </div>,
    document.body,
  )
}

type ToastKind = 'ok' | 'error'
type ToastItem = { id: number; message: string; kind: ToastKind }

const ToastContext = createContext<(message: string, kind?: ToastKind) => void>(() => {})

export function useToast() {
  return useContext(ToastContext)
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])
  const push = useCallback((message: string, kind: ToastKind = 'ok') => {
    const id = Date.now() + Math.random()
    setItems((list) => [...list, { id, message, kind }])
    window.setTimeout(() => {
      setItems((list) => list.filter((item) => item.id !== id))
    }, 5000)
  }, [])
  return (
    <ToastContext.Provider value={push}>
      {children}
      <div className="toasts">
        {items.map((item) => (
          <div key={item.id} className={`toast ${item.kind}`}>
            {item.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}

const statusTone: Record<JobStatus, 'ok' | 'warn' | 'bad' | 'blue'> = {
  running: 'blue',
  success: 'ok',
  failed: 'bad',
  canceled: 'warn',
}

export function JobConsole({
  job,
  onClose,
  onDone,
}: {
  job: Job | null
  onClose: () => void
  onDone?: (event: JobEvent) => void
}) {
  const [lines, setLines] = useState<string[]>([])
  const [status, setStatus] = useState<JobStatus>('running')
  const [error, setError] = useState('')
  const logRef = useRef<HTMLPreElement | null>(null)

  useEffect(() => {
    if (!job) return
    setLines(job.lines || [])
    setStatus(job.status)
    setError(job.error || '')
    const stop = streamJob(
      job.id,
      (event) => {
        if (event.type === 'log' && event.line) {
          setLines((current) => [...current, event.line as string])
        }
        if (event.type === 'done') {
          setStatus(event.status || 'failed')
          setError(event.error || '')
        }
      },
      (event) => {
        setStatus(event.status || 'failed')
        setError(event.error || '')
        onDone?.(event)
      },
    )
    return stop
  }, [job?.id])

  useEffect(() => {
    const node = logRef.current
    if (node) node.scrollTop = node.scrollHeight
  }, [lines])

  if (!job) return null

  return (
    <Modal
      open
      wide
      title={
        <div className="row">
          <span>{job.title}</span>
          <Badge tone={statusTone[status]}>{statusLabel(status)}</Badge>
        </div>
      }
      onClose={onClose}
      footer={
        <>
          {status === 'running' && (
            <button className="btn danger" onClick={() => api.cancelJob(job.id)}>
              取消
            </button>
          )}
          <button className="btn" onClick={onClose}>
            {status === 'running' ? '后台运行' : '关闭'}
          </button>
        </>
      }
    >
      <pre className="log" ref={logRef}>
        {lines.length === 0 ? '等待输出…' : lines.join('\n')}
        {error ? `\n${error}` : ''}
      </pre>
    </Modal>
  )
}

function statusLabel(status: JobStatus) {
  switch (status) {
    case 'running':
      return '运行中'
    case 'success':
      return '成功'
    case 'failed':
      return '失败'
    case 'canceled':
      return '已取消'
  }
}

export function Loader({ label = '加载中…' }: { label?: string }) {
  return (
    <div className="empty">
      <span className="spinner" />
      <span>{label}</span>
    </div>
  )
}

export function formatBytes(bytes: number): string {
  if (!bytes) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

export function Empty({ title, hint, action }: { title: string; hint?: string; action?: ReactNode }) {
  return (
    <div className="empty">
      <strong>{title}</strong>
      {hint && <span>{hint}</span>}
      {action}
    </div>
  )
}
