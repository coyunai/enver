import { Badge, Card, Empty, Icon, PageHeader, formatBytes } from '../components'
import type { PageProps } from './types'

export default function Dashboard({ overview, refresh, onNavigate, onOpenTerminal }: PageProps) {
  const vfox = overview.vfox
  const sdks = overview.sdks
  const versionCount = sdks.reduce((sum, sdk) => sum + sdk.versions.length, 0)
  const hasPython = sdks.some((sdk) => sdk.name === 'python' && sdk.versions.length > 0)

  return (
    <>
      <PageHeader
        title="环境编排台"
        subtitle="vfox 负责版本，enver 负责可切换、可复现的环境组合"
        actions={
          <>
            <button className="btn" onClick={() => void refresh()}>
              <Icon name="refresh" size={15} />
              刷新
            </button>
            <button className="btn primary" onClick={() => onNavigate('sdks')}>
              <Icon name="plus" size={15} />
              安装运行时
            </button>
          </>
        }
      />

      {overview.vfoxError && (
        <div className="notice bad" style={{ marginBottom: 16 }}>
          <Icon name="alert" />
          <div>
            <strong>vfox 尚未就绪</strong>
            <div>{overview.vfoxError}</div>
            <div className="faint small" style={{ marginTop: 4 }}>
              enver 依赖 vfox 管理运行时。安装后可在设置中指定自定义 vfox 路径。
            </div>
            <button className="btn small" style={{ marginTop: 8 }} onClick={() => onNavigate('settings')}>
              前往设置
            </button>
          </div>
        </div>
      )}

      <div className="grid stats">
        <Card>
          <div className="stat-label">
            <Icon name="box" size={14} /> vfox
          </div>
          <div className="stat-value small">{vfox ? `v${vfox.version}` : '未就绪'}</div>
          <div className="stat-foot">
            {vfox ? (vfox.compatible ? '版本兼容' : `需要 v${vfox.minVersion}+`) : '未检测到 vfox'}
          </div>
        </Card>
        <Card>
          <div className="stat-label">
            <Icon name="cpu" size={14} /> 已安装运行时
          </div>
          <div className="stat-value">{versionCount}</div>
          <div className="stat-foot">{sdks.length} 个 SDK</div>
        </Card>
        <Card>
          <div className="stat-label">
            <Icon name="layers" size={14} /> 预设
          </div>
          <div className="stat-value">{overview.presetCount}</div>
          <div className="stat-foot">一键切换的组合环境</div>
        </Card>
        <Card>
          <div className="stat-label">
            <Icon name="cpu" size={14} /> 虚拟环境
          </div>
          <div className="stat-value">{overview.venvCount}</div>
          <div className="stat-foot">Python 隔离环境</div>
        </Card>
      </div>

      <div className="grid cols-2">
        <Card
          title="已安装运行时"
          subtitle="当前全局选中的版本"
          actions={
            <button className="btn small" onClick={() => onNavigate('sdks')}>
              管理
            </button>
          }
        >
          {sdks.length === 0 ? (
            <Empty
              title="还没有安装任何运行时"
              hint="进入运行时页面，从 SDK 商店选择版本安装"
              action={
                <button className="btn primary small" onClick={() => onNavigate('sdks')}>
                  去安装
                </button>
              }
            />
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>SDK</th>
                    <th>当前版本</th>
                    <th>已装版本</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {sdks.map((sdk) => (
                    <tr key={sdk.name}>
                      <td className="mono">{sdk.name}</td>
                      <td>
                        {sdk.current ? (
                          <Badge tone="ok">v{sdk.current}</Badge>
                        ) : (
                          <span className="faint">未选择</span>
                        )}
                      </td>
                      <td className="muted">{sdk.versions.length}</td>
                      <td style={{ textAlign: 'right' }}>
                        <button className="btn small ghost" onClick={() => onNavigate('sdks')}>
                          切换
                          <Icon name="arrow" size={13} />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>

        <Card
          title="存储"
          subtitle="SDK 安装位置与可用空间"
          actions={
            <button className="btn small" onClick={() => onNavigate('settings')}>
              设置
            </button>
          }
        >
          <div className="kv">
            <div className="kv-row">
              <span>vfox Home</span>
              <span>{overview.storage.home}</span>
            </div>
            <div className="kv-row">
              <span>SDK 根目录</span>
              <span>{overview.storage.sdkRoot || '—'}</span>
            </div>
            <div className="kv-row">
              <span>可用空间</span>
              <span>{formatBytes(overview.storage.freeBytes)}</span>
            </div>
            <div className="kv-row">
              <span>平台</span>
              <span>{overview.platform}</span>
            </div>
          </div>
        </Card>
      </div>

      <div className="grid cols-2 section">
        <Card title="快捷操作" subtitle="常用入口">
          <div className="actions">
            <button className="btn" onClick={() => onNavigate('sdks')}>
              <Icon name="plus" size={15} /> 安装运行时
            </button>
            <button className="btn" onClick={() => onNavigate('presets')}>
              <Icon name="layers" size={15} /> 新建预设
            </button>
            <button className="btn" onClick={() => onOpenTerminal({ cwd: '' })}>
              <Icon name="terminal" size={15} /> 打开终端
            </button>
            <button className="btn" onClick={() => onNavigate('doctor')}>
              <Icon name="activity" size={15} /> 运行诊断
            </button>
          </div>
        </Card>

        <Card title="就绪清单" subtitle="把环境补齐到可直接开工">
          <div className="kv">
            <ReadyRow ok={Boolean(vfox)} text="检测到可用的 vfox" action={() => onNavigate('settings')} />
            <ReadyRow
              ok={versionCount > 0}
              text="至少安装一个运行时"
              action={() => onNavigate('sdks')}
            />
            <ReadyRow ok={hasPython} text="安装 Python（用于 venv / uv）" action={() => onNavigate('sdks')} />
            <ReadyRow
              ok={overview.presetCount > 0}
              text="创建可一键切换的预设"
              action={() => onNavigate('presets')}
            />
          </div>
        </Card>
      </div>
    </>
  )
}

function ReadyRow({ ok, text, action }: { ok: boolean; text: string; action: () => void }) {
  return (
    <div className="row" style={{ justifyContent: 'space-between' }}>
      <span className="row" style={{ gap: 8 }}>
        <span className={`check-icon ${ok ? 'ok' : 'info'}`} style={{ width: 22, height: 22 }}>
          <Icon name={ok ? 'check' : 'arrow'} size={13} />
        </span>
        <span style={{ color: 'var(--text)' }}>{text}</span>
      </span>
      {!ok && (
        <button className="btn small ghost" onClick={action}>
          去处理
        </button>
      )}
    </div>
  )
}
