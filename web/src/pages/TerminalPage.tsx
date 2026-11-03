import { useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { api, terminalURL } from '../api'
import { Badge, Card, Icon, PageHeader, useToast } from '../components'
import type { Preset, Venv } from '../types'
import type { PageProps, TerminalTarget } from './types'

export default function TerminalPage({
  onOpenTerminal,
  target,
}: PageProps & { target: TerminalTarget }) {
  const toast = useToast()
  const containerRef = useRef<HTMLDivElement | null>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const observerRef = useRef<ResizeObserver | null>(null)

  const [presets, setPresets] = useState<Preset[]>([])
  const [venvs, setVenvs] = useState<Venv[]>([])
  const [presetName, setPresetName] = useState('')
  const [venvID, setVenvID] = useState('')
  const [cwd, setCwd] = useState('')
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    void api.presets().then((data) => setPresets(data.presets)).catch(() => undefined)
    void api.venvs().then((data) => setVenvs(data.venvs)).catch(() => undefined)
  }, [])

  useEffect(() => {
    if (target.preset !== undefined) setPresetName(target.preset || '')
    if (target.venv !== undefined) setVenvID(target.venv || '')
    if (target.cwd !== undefined) setCwd(target.cwd || '')
  }, [target])

  const cleanup = () => {
    const term = termRef.current
    const observer = observerRef.current
    const socket = wsRef.current
    termRef.current = null
    fitRef.current = null
    observerRef.current = null
    wsRef.current = null
    observer?.disconnect()
    if (socket && socket.readyState === WebSocket.OPEN) socket.close()
    term?.dispose()
    setConnected(false)
  }

  useEffect(() => cleanup, [])

  const connect = () => {
    if (wsRef.current || !containerRef.current) return
    setError('')
    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: '"JetBrains Mono", ui-monospace, Consolas, monospace',
      scrollback: 5000,
      theme: {
        background: '#06090d',
        foreground: '#d8e4ef',
        cursor: '#3fd07f',
        cursorAccent: '#06090d',
        selectionBackground: '#1f3b2d',
        black: '#0b1017',
        red: '#ff6b6b',
        green: '#3fd07f',
        yellow: '#e3b341',
        blue: '#5aa7ff',
        magenta: '#c792ea',
        cyan: '#4dd0e1',
        white: '#d8e4ef',
      },
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(containerRef.current)
    termRef.current = term
    fitRef.current = fit
    requestAnimationFrame(() => {
      try {
        fit.fit()
      } catch {
        // container may still be sizing
      }
    })

    const socket = new WebSocket(
      terminalURL({
        preset: presetName || undefined,
        venv: venvID || undefined,
        cwd,
        cols: term.cols,
        rows: term.rows,
      }),
    )
    socket.binaryType = 'arraybuffer'
    wsRef.current = socket

    socket.onopen = () => {
      setConnected(true)
      setError('')
      socket.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
      term.focus()
    }
    socket.onmessage = (event) => {
      if (typeof event.data === 'string') {
        term.write(event.data)
      } else {
        term.write(new Uint8Array(event.data as ArrayBuffer))
      }
    }
    socket.onerror = () => {
      setError('终端连接失败，请确认服务仍在运行')
    }
    socket.onclose = () => {
      if (wsRef.current === socket) {
        wsRef.current = null
        setConnected(false)
        const current = termRef.current
        if (current) {
          try {
            current.write('\r\n\x1b[90m[连接已关闭]\x1b[0m\r\n')
          } catch {
            // terminal already disposed
          }
        }
      }
    }

    term.onData((data) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: 'input', data }))
      }
    })
    term.onResize(({ cols, rows }) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: 'resize', cols, rows }))
      }
    })

    const observer = new ResizeObserver(() => {
      try {
        fit.fit()
      } catch {
        // ignore transient layout
      }
    })
    observer.observe(containerRef.current)
    observerRef.current = observer
  }

  return (
    <>
      <PageHeader
        title="终端"
        subtitle="环境由 enver 直接注入，不依赖外部 shell hook；切换预设或 venv 后重新连接即可"
        actions={
          connected ? (
            <button className="btn danger" onClick={cleanup}>
              <Icon name="x" size={15} /> 断开
            </button>
          ) : (
            <button className="btn primary" onClick={connect}>
              <Icon name="play" size={14} /> 连接
            </button>
          )
        }
      />

      <Card>
        <div className="row wrap" style={{ marginBottom: 12 }}>
          <select
            className="select"
            style={{ width: 190 }}
            value={presetName}
            disabled={connected}
            onChange={(event) => setPresetName(event.target.value)}
          >
            <option value="">不套用预设</option>
            {presets.map((preset) => (
              <option key={preset.name} value={preset.name}>
                {preset.name}
              </option>
            ))}
          </select>
          <select
            className="select"
            style={{ width: 200 }}
            value={venvID}
            disabled={connected}
            onChange={(event) => setVenvID(event.target.value)}
          >
            <option value="">不激活 venv</option>
            {venvs.map((venv) => (
              <option key={venv.id} value={venv.id}>
                {venv.name} · {venv.pythonVersion}
              </option>
            ))}
          </select>
          <input
            className="input"
            style={{ flex: 1, minWidth: 220 }}
            placeholder="工作目录（留空使用用户主目录）"
            value={cwd}
            disabled={connected}
            onChange={(event) => setCwd(event.target.value)}
          />
          <Badge tone={connected ? 'ok' : ''}>{connected ? '已连接' : '未连接'}</Badge>
          {venvs.length > 0 && (
            <button
              className="btn small ghost"
              onClick={() => onOpenTerminal({ venv: venvs[0].id, cwd: venvs[0].projectDir || '' })}
            >
              快捷打开
            </button>
          )}
        </div>

        {error && (
          <div className="notice bad" style={{ marginBottom: 12 }}>
            <Icon name="alert" />
            <div>{error}</div>
          </div>
        )}

        <div className="terminal-shell">
          <div ref={containerRef} style={{ height: '100%' }} />
        </div>

        <div className="faint small" style={{ marginTop: 10 }}>
          {connected
            ? '提示：终端内的 vfox 切换只影响当前 shell 会话。'
            : '选择一个预设可同时注入多个运行时版本、环境变量与 venv。'}
        </div>
      </Card>
    </>
  )
}
