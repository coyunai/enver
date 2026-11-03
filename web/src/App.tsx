import { useCallback, useEffect, useState } from 'react'
import { api } from './api'
import { Icon, Loader, ToastProvider } from './components'
import type { Overview } from './types'
import type { TerminalTarget, View } from './pages/types'
import Dashboard from './pages/Dashboard'
import SDKs from './pages/SDKs'
import Presets from './pages/Presets'
import Venvs from './pages/Venvs'
import Packages from './pages/Packages'
import Doctor from './pages/Doctor'
import TerminalPage from './pages/TerminalPage'
import Settings from './pages/Settings'

const navItems: { id: View; label: string; icon: string }[] = [
  { id: 'dashboard', label: '总览', icon: 'dashboard' },
  { id: 'sdks', label: '运行时', icon: 'box' },
  { id: 'presets', label: '预设', icon: 'layers' },
  { id: 'venvs', label: '虚拟环境', icon: 'cpu' },
  { id: 'packages', label: '全局库', icon: 'package' },
  { id: 'doctor', label: '诊断', icon: 'activity' },
  { id: 'terminal', label: '终端', icon: 'terminal' },
  { id: 'settings', label: '设置', icon: 'settings' },
]

export default function App() {
  const [view, setView] = useState<View>('dashboard')
  const [overview, setOverview] = useState<Overview | null>(null)
  const [loadError, setLoadError] = useState('')
  const [terminalTarget, setTerminalTarget] = useState<TerminalTarget>({ cwd: '' })
  const [theme, setTheme] = useState<'dark' | 'light'>(
    () => (localStorage.getItem('enver-theme') as 'dark' | 'light') || 'dark',
  )

  const refresh = useCallback(async () => {
    try {
      const data = await api.overview()
      setOverview(data)
      setLoadError('')
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : String(error))
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    localStorage.setItem('enver-theme', theme)
  }, [theme])

  const openTerminal = (target: TerminalTarget) => {
    setTerminalTarget(target)
    setView('terminal')
  }

  const pageProps = overview
    ? { overview, refresh, onNavigate: setView, onOpenTerminal: openTerminal }
    : null

  return (
    <ToastProvider>
      <div className="app">
        <aside className="sidebar">
          <div className="brand">
            <div className="brand-mark">e</div>
            <div>
              <div className="brand-name">enver</div>
              <div className="brand-sub">environment studio</div>
            </div>
          </div>
          <nav className="nav">
            {navItems.map((item) => (
              <button
                key={item.id}
                className={`nav-item ${view === item.id ? 'active' : ''}`}
                onClick={() => setView(item.id)}
              >
                <Icon name={item.icon} />
                {item.label}
              </button>
            ))}
          </nav>
          <div className="sidebar-footer">
            <div className="status-line">
              <span className={`dot ${overview?.vfox ? 'ok' : overview ? 'bad' : ''}`} />
              <span>{overview?.vfox ? `vfox ${overview.vfox.version}` : 'vfox 未就绪'}</span>
            </div>
            <div className="status-line">
              <span className="dot" />
              <span>{overview ? platformLabel(overview.platform) : '检测中'}</span>
            </div>
            <div className="status-line">
              <span className="dot" />
              <span>enver {overview?.enverVersion || '—'}</span>
            </div>
            <button
              className="btn ghost small"
              onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
              style={{ marginTop: 4 }}
            >
              <Icon name="spark" size={14} />
              {theme === 'dark' ? '切换浅色' : '切换深色'}
            </button>
          </div>
        </aside>

        <main className="main">
          {loadError && (
            <div className="notice bad" style={{ marginBottom: 16 }}>
              <Icon name="alert" />
              <div>
                <strong>无法连接到 enver 服务</strong>
                <div>{loadError}</div>
                <button className="btn small" style={{ marginTop: 8 }} onClick={() => void refresh()}>
                  重试
                </button>
              </div>
            </div>
          )}
          {!pageProps ? (
            <Loader label="正在读取 vfox 状态…" />
          ) : (
            <>
              {view === 'dashboard' && <Dashboard {...pageProps} />}
              {view === 'sdks' && <SDKs {...pageProps} />}
              {view === 'presets' && <Presets {...pageProps} />}
              {view === 'venvs' && <Venvs {...pageProps} />}
              {view === 'packages' && <Packages {...pageProps} />}
              {view === 'doctor' && <Doctor {...pageProps} />}
              {view === 'terminal' && <TerminalPage {...pageProps} target={terminalTarget} />}
              {view === 'settings' && <Settings {...pageProps} />}
            </>
          )}
        </main>
      </div>
    </ToastProvider>
  )
}

function platformLabel(platform: string) {
  switch (platform) {
    case 'windows':
      return 'Windows'
    case 'darwin':
      return 'macOS'
    case 'linux':
      return 'Linux'
    default:
      return platform
  }
}
