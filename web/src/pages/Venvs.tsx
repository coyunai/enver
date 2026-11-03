import { useCallback, useEffect, useMemo, useState } from 'react'
import { api } from '../api'
import {
  Badge,
  Card,
  Empty,
  Field,
  Icon,
  Loader,
  Modal,
  PageHeader,
  useToast,
} from '../components'
import type { Preset, Venv } from '../types'
import type { PageProps } from './types'

type FormState = {
  name: string
  path: string
  projectDir: string
  pythonVersion: string
  pythonPath: string
  tool: string
  preset: string
}

export default function Venvs({ overview, refresh, onNavigate, onOpenTerminal }: PageProps) {
  const toast = useToast()
  const [venvs, setVenvs] = useState<Venv[]>([])
  const [loading, setLoading] = useState(true)
  const [presets, setPresets] = useState<Preset[]>([])
  const [form, setForm] = useState<FormState | null>(null)
  const [busy, setBusy] = useState('')

  const pythonSDK = overview.sdks.find((sdk) => sdk.name === 'python')
  const pythonVersions = useMemo(() => pythonSDK?.versions || [], [pythonSDK])

  const load = useCallback(async () => {
    try {
      const data = await api.venvs()
      setVenvs(data.venvs)
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setLoading(false)
    }
  }, [toast])

  useEffect(() => {
    void load()
    void api
      .presets()
      .then((data) => setPresets(data.presets))
      .catch(() => undefined)
  }, [load])

  const openNew = () => {
    const version = pythonSDK?.current || pythonVersions[0]?.version || ''
    setForm({
      name: '',
      path: '',
      projectDir: '',
      pythonVersion: version,
      pythonPath: '',
      tool: 'auto',
      preset: '',
    })
  }

  const create = async () => {
    if (!form) return
    if (!form.pythonVersion && !form.pythonPath.trim()) {
      toast('请选择 vfox Python，或填写外部解释器路径', 'error')
      return
    }
    setBusy('create')
    try {
      const result = await api.createVenv({
        name: form.name.trim() || undefined,
        path: form.path.trim() || undefined,
        projectDir: form.projectDir.trim() || undefined,
        pythonVersion: form.pythonVersion || undefined,
        pythonPath: form.pythonPath.trim() || undefined,
        tool: form.tool,
        preset: form.preset || undefined,
      })
      toast(`虚拟环境 ${result.venv.name} 已创建`)
      setForm(null)
      await load()
      await refresh()
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setBusy('')
    }
  }

  const rebuild = async (venv: Venv) => {
    if (!window.confirm(`重建 ${venv.name}？现有环境会被删除。`)) return
    setBusy(venv.id)
    try {
      const result = await api.rebuildVenv(venv.id)
      toast(`已重建 ${result.venv.name}`)
      await load()
      await refresh()
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setBusy('')
    }
  }

  const remove = async (venv: Venv) => {
    if (!window.confirm(`删除 ${venv.name}？\n${venv.path}\n该操作会删除目录内容。`)) return
    setBusy(venv.id)
    try {
      await api.deleteVenv(venv.id)
      toast('虚拟环境已删除')
      await load()
      await refresh()
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setBusy('')
    }
  }

  return (
    <>
      <PageHeader
        title="虚拟环境"
        subtitle="用 vfox 管理的 Python 作为基础解释器，venv 或 uv 负责隔离"
        actions={
          <>
            <button className="btn" onClick={() => void load()}>
              <Icon name="refresh" size={15} /> 刷新
            </button>
            <button className="btn primary" onClick={openNew}>
              <Icon name="plus" size={15} /> 新建环境
            </button>
          </>
        }
      />

      {pythonVersions.length === 0 && (
        <div className="notice warn" style={{ marginBottom: 16 }}>
          <Icon name="alert" />
          <div>
            <strong>还没有 vfox 管理的 Python</strong>
            <div>
              推荐在运行时页面安装 Python。部分 Windows 环境下 vfox 的 Python 插件安装可能失败，
              此时可以直接使用系统解释器创建隔离环境（兼容模式）。
            </div>
            <div className="row" style={{ marginTop: 8 }}>
              <button className="btn small" onClick={() => onNavigate('sdks')}>
                去安装 Python
              </button>
              {overview.systemPython && (
                <button
                  className="btn small primary"
                  onClick={() =>
                    setForm({
                      name: '',
                      path: '',
                      projectDir: '',
                      pythonVersion: '',
                      pythonPath: overview.systemPython || '',
                      tool: 'auto',
                      preset: '',
                    })
                  }
                >
                  使用系统 Python
                </button>
              )}
            </div>
          </div>
        </div>
      )}

      {loading ? (
        <Loader />
      ) : venvs.length === 0 ? (
        <Card>
          <Empty
            title="还没有虚拟环境"
            hint="为项目创建独立环境，避免全局包互相污染"
            action={
              <button className="btn primary small" onClick={openNew}>
                创建环境
              </button>
            }
          />
        </Card>
      ) : (
        <Card>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>名称</th>
                  <th>Python</th>
                  <th>工具</th>
                  <th>路径</th>
                  <th>状态</th>
                  <th style={{ textAlign: 'right' }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {venvs.map((venv) => (
                  <tr key={venv.id}>
                    <td>
                      <div className="mono">{venv.name}</div>
                      {venv.preset && <div className="faint small">预设 {venv.preset}</div>}
                    </td>
                    <td className="mono">{venv.pythonVersion || '—'}</td>
                    <td>
                      <Badge tone="blue">{venv.tool}</Badge>
                    </td>
                    <td className="muted small" style={{ maxWidth: 300, wordBreak: 'break-all' }}>
                      {venv.path}
                    </td>
                    <td>
                      {venv.healthy ? (
                        <Badge tone="ok">正常</Badge>
                      ) : (
                        <Badge tone="warn">{venv.health || '异常'}</Badge>
                      )}
                    </td>
                    <td>
                      <div className="row" style={{ justifyContent: 'flex-end' }}>
                        <button
                          className="btn small"
                          onClick={() =>
                            onOpenTerminal({
                              venv: venv.id,
                              cwd: venv.projectDir || parentDir(venv.path),
                            })
                          }
                        >
                          终端
                        </button>
                        <button
                          className="btn small"
                          disabled={busy === venv.id}
                          onClick={() => void rebuild(venv)}
                        >
                          重建
                        </button>
                        <button
                          className="btn small danger"
                          disabled={busy === venv.id}
                          onClick={() => void remove(venv)}
                        >
                          删除
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      <Modal
        open={Boolean(form)}
        wide
        title="新建虚拟环境"
        onClose={() => setForm(null)}
        footer={
          <>
            <button className="btn" onClick={() => setForm(null)}>
              取消
            </button>
            <button className="btn primary" disabled={busy === 'create'} onClick={() => void create()}>
              创建
            </button>
          </>
        }
      >
        {form && (
          <>
            <div className="grid cols-2">
              <Field label="名称" hint="留空则使用目录名">
                <input
                  className="input"
                  value={form.name}
                  placeholder="my-api"
                  onChange={(event) => setForm({ ...form, name: event.target.value })}
                />
              </Field>
              <Field label="基础 Python 版本">
                <select
                  className="select"
                  value={form.pythonVersion}
                  onChange={(event) => setForm({ ...form, pythonVersion: event.target.value })}
                >
                  {pythonVersions.length === 0 && <option value="">没有 vfox 管理的 Python</option>}
                  {pythonVersions.map((version) => (
                    <option key={version.version} value={version.version}>
                      {version.version}
                      {version.current ? '（当前）' : ''}
                    </option>
                  ))}
                </select>
              </Field>
            </div>
            <Field
              label="外部解释器路径（兼容模式，可选）"
              hint="填写后优先于上面的 vfox 版本；适用于 vfox Python 插件不可用的环境"
            >
              <input
                className="input"
                value={form.pythonPath}
                placeholder={overview.systemPython || 'C:\\Python313\\python.exe'}
                onChange={(event) => setForm({ ...form, pythonPath: event.target.value })}
              />
              {overview.systemPython && (
                <button
                  className="btn small ghost"
                  style={{ alignSelf: 'flex-start' }}
                  onClick={() => setForm({ ...form, pythonPath: overview.systemPython || '' })}
                >
                  使用检测到的系统 Python：{overview.systemPython}
                </button>
              )}
            </Field>
            <div className="grid cols-2">
              <Field label="创建工具">
                <select
                  className="select"
                  value={form.tool}
                  onChange={(event) => setForm({ ...form, tool: event.target.value })}
                >
                  <option value="auto">自动（优先 uv）</option>
                  <option value="uv">uv</option>
                  <option value="venv">python -m venv</option>
                </select>
              </Field>
              <Field label="关联预设（可选）">
                <select
                  className="select"
                  value={form.preset}
                  onChange={(event) => setForm({ ...form, preset: event.target.value })}
                >
                  <option value="">不关联</option>
                  {presets.map((preset) => (
                    <option key={preset.name} value={preset.name}>
                      {preset.name}
                    </option>
                  ))}
                </select>
              </Field>
            </div>
            <Field label="环境路径（可选）" hint="留空时：有项目目录则使用 <项目>/.venv，否则放在应用数据目录">
              <input
                className="input"
                value={form.path}
                placeholder=".venv 或 D:\\work\\myapp\\.venv"
                onChange={(event) => setForm({ ...form, path: event.target.value })}
              />
            </Field>
            <Field label="项目目录（可选）">
              <input
                className="input"
                value={form.projectDir}
                placeholder="D:\\work\\myapp"
                onChange={(event) => setForm({ ...form, projectDir: event.target.value })}
              />
            </Field>
            <div className="notice">
              <Icon name="alert" />
              <div>
                enver 会强制使用 vfox 安装的解释器创建环境，并设置 <span className="mono">UV_PYTHON_DOWNLOADS=never</span>，
                避免 uv 自行下载 Python 造成版本分裂。
              </div>
            </div>
          </>
        )}
      </Modal>
    </>
  )
}

function parentDir(path: string) {
  const normalized = path.replace(/[\\/]+$/, '')
  const index = Math.max(normalized.lastIndexOf('/'), normalized.lastIndexOf('\\'))
  return index > 0 ? normalized.slice(0, index) : normalized
}

function messageOf(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}
