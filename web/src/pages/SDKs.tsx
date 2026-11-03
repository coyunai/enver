import { useCallback, useEffect, useMemo, useState } from 'react'
import { api } from '../api'
import { Badge, Card, Empty, Icon, JobConsole, Loader, PageHeader, useToast } from '../components'
import type { CatalogPlugin, Job, Version } from '../types'
import type { PageProps } from './types'

export default function SDKs({ overview, refresh }: PageProps) {
  const toast = useToast()
  const [catalog, setCatalog] = useState<CatalogPlugin[]>([])
  const [source, setSource] = useState('')
  const [selected, setSelected] = useState('')
  const [versions, setVersions] = useState<Version[]>([])
  const [loadingVersions, setLoadingVersions] = useState(false)
  const [versionError, setVersionError] = useState('')
  const [filter, setFilter] = useState('')
  const [versionFilter, setVersionFilter] = useState('')
  const [onlyInstalled, setOnlyInstalled] = useState(false)
  const [customPlugin, setCustomPlugin] = useState('')
  const [busy, setBusy] = useState('')
  const [job, setJob] = useState<Job | null>(null)

  const loadCatalog = useCallback(async () => {
    const data = await api.catalog()
    setCatalog(data.items)
    setSource(data.source)
  }, [])

  const loadVersions = useCallback(async (sdk: string) => {
    if (!sdk) return
    setLoadingVersions(true)
    setVersionError('')
    try {
      const data = await api.versions(sdk)
      setVersions(data.versions)
    } catch (error) {
      setVersions([])
      setVersionError(error instanceof Error ? error.message : String(error))
    } finally {
      setLoadingVersions(false)
    }
  }, [])

  useEffect(() => {
    void loadCatalog().catch((error) => toast(messageOf(error), 'error'))
  }, [loadCatalog, toast])

  useEffect(() => {
    if (selected || catalog.length === 0) return
    const withVersions = overview.sdks.find((sdk) => sdk.versions.length > 0)?.name
    const preferred = catalog.find((item) => item.name === withVersions)
    const first = preferred || catalog.find((item) => item.installed) || catalog[0]
    setSelected(first.name)
  }, [catalog, selected, overview.sdks])

  const selectedPlugin = useMemo(
    () => catalog.find((item) => item.name === selected),
    [catalog, selected],
  )

  useEffect(() => {
    if (!selected) return
    if (selectedPlugin && !selectedPlugin.installed) {
      setVersions([])
      setVersionError('')
      return
    }
    void loadVersions(selected)
  }, [selected, selectedPlugin, loadVersions])

  const filteredCatalog = useMemo(() => {
    const query = filter.trim().toLowerCase()
    if (!query) return catalog
    return catalog.filter(
      (item) =>
        item.name.toLowerCase().includes(query) ||
        item.desc.toLowerCase().includes(query) ||
        item.homepage.toLowerCase().includes(query),
    )
  }, [catalog, filter])

  const shownVersions = useMemo(() => {
    const query = versionFilter.trim().toLowerCase()
    return versions.filter((version) => {
      if (onlyInstalled && !version.installed) return false
      if (!query) return true
      return version.version.toLowerCase().includes(query)
    })
  }, [versions, onlyInstalled, versionFilter])

  const installedCount = versions.filter((version) => version.installed).length

  const afterJob = useCallback(async () => {
    await loadCatalog()
    await refresh()
    if (selected) await loadVersions(selected)
  }, [loadCatalog, refresh, selected, loadVersions])

  const startInstall = async (version: string) => {
    setBusy(`install:${version}`)
    try {
      setJob(await api.install(selected, version))
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setBusy('')
    }
  }

  const startUninstall = async (version: string) => {
    if (!window.confirm(`确定卸载 ${selected}@${version}？`)) return
    setBusy(`uninstall:${version}`)
    try {
      setJob(await api.uninstall(selected, version))
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setBusy('')
    }
  }

  const switchVersion = async (version: string) => {
    setBusy(`use:${version}`)
    try {
      await api.use({ sdk: selected, version, scope: 'global' })
      toast(`已全局切换到 ${selected}@${version}`)
      await loadVersions(selected)
      await refresh()
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setBusy('')
    }
  }

  const addPlugin = async (name: string) => {
    const plugin = name.trim()
    if (!plugin) return
    setBusy(`plugin:${plugin}`)
    try {
      setJob(await api.addPlugin(plugin))
      setCustomPlugin('')
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setBusy('')
    }
  }

  return (
    <>
      <PageHeader
        title="运行时"
        subtitle="安装 vfox 插件、切换版本；所有下载与插件逻辑由 vfox 负责"
        actions={
          <>
            <button className="btn" onClick={() => void loadCatalog()}>
              <Icon name="refresh" size={15} /> 刷新插件
            </button>
            <button className="btn primary" onClick={() => void loadVersions(selected)} disabled={!selected}>
              <Icon name="refresh" size={15} /> 刷新版本
            </button>
          </>
        }
      />

      <div className="split">
        <Card
          title="SDK 插件"
          subtitle={`${catalog.length} 个可用 · 来源 ${sourceLabel(source)}`}
        >
          <div style={{ position: 'relative', marginBottom: 10 }}>
            <span style={{ position: 'absolute', left: 10, top: 9, color: 'var(--faint)' }}>
              <Icon name="search" size={15} />
            </span>
            <input
              className="input"
              style={{ paddingLeft: 32 }}
              placeholder="搜索插件"
              value={filter}
              onChange={(event) => setFilter(event.target.value)}
            />
          </div>
          <div className="list">
            {filteredCatalog.map((item) => (
              <button
                key={item.name}
                className={`list-item ${selected === item.name ? 'active' : ''}`}
                onClick={() => setSelected(item.name)}
              >
                <span className={`dot ${item.installed ? 'ok' : ''}`} />
                <span className="list-main">
                  <span className="list-name">{item.name}</span>
                  <span className="list-desc">{item.desc || item.homepage}</span>
                </span>
                {item.installed && <Badge tone="ok">已装</Badge>}
              </button>
            ))}
            {filteredCatalog.length === 0 && <Empty title="没有匹配的插件" />}
          </div>
          <div className="hr" />
          <div className="row">
            <input
              className="input"
              placeholder="添加其他插件名"
              value={customPlugin}
              onChange={(event) => setCustomPlugin(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void addPlugin(customPlugin)
              }}
            />
            <button
              className="btn"
              disabled={!customPlugin.trim() || busy.startsWith('plugin:')}
              onClick={() => void addPlugin(customPlugin)}
            >
              添加
            </button>
          </div>
        </Card>

        <Card
          title={selected ? selected : '选择一个 SDK'}
          subtitle={selectedPlugin?.desc || selectedPlugin?.homepage || '从左侧插件列表开始'}
          actions={
            selectedPlugin?.installed ? (
              <div className="row">
                <div className="seg">
                  <button className={!onlyInstalled ? 'active' : ''} onClick={() => setOnlyInstalled(false)}>
                    全部
                  </button>
                  <button className={onlyInstalled ? 'active' : ''} onClick={() => setOnlyInstalled(true)}>
                    已安装 {installedCount}
                  </button>
                </div>
                <input
                  className="input"
                  style={{ width: 140 }}
                  placeholder="筛选版本"
                  value={versionFilter}
                  onChange={(event) => setVersionFilter(event.target.value)}
                />
              </div>
            ) : null
          }
        >
          {!selected && <Empty title="尚未选择 SDK" hint="从左侧选择一个插件查看可用版本" />}

          {selected && selectedPlugin && !selectedPlugin.installed && (
            <div className="notice warn">
              <Icon name="alert" />
              <div>
                <strong>{selected} 插件尚未安装</strong>
                <div>安装插件后才能查询版本并安装运行时。</div>
                <button
                  className="btn primary small"
                  style={{ marginTop: 8 }}
                  disabled={busy.startsWith('plugin:')}
                  onClick={() => void addPlugin(selected)}
                >
                  安装插件
                </button>
              </div>
            </div>
          )}

          {selected && (!selectedPlugin || selectedPlugin.installed) && (
            <>
              {loadingVersions ? (
                <Loader label="读取可用版本…" />
              ) : versionError ? (
                <div className="notice bad">
                  <Icon name="alert" />
                  <div>
                    <strong>版本列表获取失败</strong>
                    <div>{versionError}</div>
                  </div>
                </div>
              ) : shownVersions.length === 0 ? (
                <Empty
                  title="没有匹配的版本"
                  hint={onlyInstalled ? '当前没有已安装版本' : '尝试清空筛选条件'}
                />
              ) : (
                <div className="table-wrap" style={{ maxHeight: 590, overflow: 'auto' }}>
                  <table>
                    <thead>
                      <tr>
                        <th>版本</th>
                        <th>标签</th>
                        <th>说明</th>
                        <th style={{ textAlign: 'right' }}>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {shownVersions.map((version) => (
                        <tr key={version.version}>
                          <td className="mono">
                            {version.version}
                            {version.current && (
                              <span style={{ marginLeft: 8 }}>
                                <Badge tone="ok">当前</Badge>
                              </span>
                            )}
                          </td>
                          <td>
                            <div className="row wrap">
                              {version.installed && <Badge tone="blue">已安装</Badge>}
                              {version.lts && <Badge>LTS</Badge>}
                              {version.stable && <Badge>Stable</Badge>}
                            </div>
                          </td>
                          <td className="faint small">{version.meta || '—'}</td>
                          <td>
                            <div className="row" style={{ justifyContent: 'flex-end' }}>
                              {version.installed ? (
                                <>
                                  <button
                                    className="btn small"
                                    disabled={version.current || busy === `use:${version.version}`}
                                    onClick={() => void switchVersion(version.version)}
                                  >
                                    切换
                                  </button>
                                  <button
                                    className="btn small danger"
                                    disabled={busy === `uninstall:${version.version}`}
                                    onClick={() => void startUninstall(version.version)}
                                  >
                                    卸载
                                  </button>
                                </>
                              ) : (
                                <button
                                  className="btn small primary"
                                  disabled={busy === `install:${version.version}`}
                                  onClick={() => void startInstall(version.version)}
                                >
                                  安装
                                </button>
                              )}
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </>
          )}
        </Card>
      </div>

      <JobConsole
        job={job}
        onClose={() => setJob(null)}
        onDone={() => {
          void afterJob()
        }}
      />
    </>
  )
}

function sourceLabel(source: string) {
  switch (source) {
    case 'network':
      return '网络'
    case 'cache':
      return '缓存'
    case 'cache-stale':
      return '过期缓存'
    case 'embedded':
      return '内置快照'
    default:
      return source || '未知'
  }
}

function messageOf(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}
