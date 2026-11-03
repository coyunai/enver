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
import type { Preset } from '../types'
import type { PageProps } from './types'

type RuntimeRow = { sdk: string; version: string }
type EnvRow = { key: string; value: string }
type FormState = {
  name: string
  description: string
  runtimes: RuntimeRow[]
  env: EnvRow[]
  venv: string
  tool: string
  isNew: boolean
}

const commonSDKs = ['nodejs', 'python', 'golang', 'java', 'php', 'ruby', 'dotnet', 'flutter', 'rust']

export default function Presets({ overview, refresh }: PageProps) {
  const toast = useToast()
  const [presets, setPresets] = useState<Preset[]>([])
  const [loading, setLoading] = useState(true)
  const [form, setForm] = useState<FormState | null>(null)
  const [saving, setSaving] = useState(false)
  const [target, setTarget] = useState<Preset | null>(null)
  const [projectDir, setProjectDir] = useState(() => localStorage.getItem('enver-project-dir') || '')
  const [logs, setLogs] = useState<{ title: string; content: string } | null>(null)

  const load = useCallback(async () => {
    try {
      const data = await api.presets()
      setPresets(data.presets)
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setLoading(false)
    }
  }, [toast])

  useEffect(() => {
    void load()
  }, [load])

  const sdkNames = useMemo(() => {
    const names = new Set<string>(commonSDKs)
    overview.sdks.forEach((sdk) => names.add(sdk.name))
    presets.forEach((preset) => Object.keys(preset.runtimes).forEach((name) => names.add(name)))
    return Array.from(names).sort()
  }, [overview.sdks, presets])

  const openNew = () => {
    setForm({
      name: '',
      description: '',
      runtimes: [{ sdk: 'nodejs', version: '' }],
      env: [],
      venv: '',
      tool: 'auto',
      isNew: true,
    })
  }

  const openEdit = (preset: Preset) => {
    setForm({
      name: preset.name,
      description: preset.description || '',
      runtimes:
        Object.entries(preset.runtimes).length > 0
          ? Object.entries(preset.runtimes).map(([sdk, version]) => ({ sdk, version }))
          : [{ sdk: '', version: '' }],
      env: Object.entries(preset.env || {}).map(([key, value]) => ({ key, value })),
      venv: preset.python?.venv || '',
      tool: preset.python?.tool || 'auto',
      isNew: false,
    })
  }

  const save = async () => {
    if (!form) return
    const name = form.name.trim()
    if (!name) {
      toast('预设名称不能为空', 'error')
      return
    }
    const runtimes: Record<string, string> = {}
    for (const row of form.runtimes) {
      const sdk = row.sdk.trim()
      const version = row.version.trim()
      if (sdk && version) runtimes[sdk] = version
    }
    if (Object.keys(runtimes).length === 0) {
      toast('至少需要一个运行时', 'error')
      return
    }
    const env: Record<string, string> = {}
    for (const row of form.env) {
      const key = row.key.trim()
      if (key) env[key] = row.value
    }
    const preset: Preset = {
      name,
      description: form.description.trim() || undefined,
      runtimes,
      env: Object.keys(env).length > 0 ? env : undefined,
      python: form.venv.trim()
        ? { venv: form.venv.trim(), tool: form.tool === 'auto' ? undefined : form.tool }
        : undefined,
    }
    setSaving(true)
    try {
      await api.savePreset(preset, form.isNew)
      toast(`预设 ${name} 已保存`)
      setForm(null)
      await load()
      await refresh()
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setSaving(false)
    }
  }

  const remove = async (preset: Preset) => {
    if (!window.confirm(`删除预设 ${preset.name}？`)) return
    try {
      await api.deletePreset(preset.name)
      toast('预设已删除')
      await load()
      await refresh()
    } catch (error) {
      toast(messageOf(error), 'error')
    }
  }

  const apply = async (preset: Preset, scope: 'global' | 'project', dir = '') => {
    try {
      const result = await api.applyPreset(preset.name, scope, dir)
      setLogs({ title: `应用 ${preset.name}`, content: result.logs.join('\n') || '完成' })
      toast(scope === 'global' ? '已全局应用预设' : '已应用到项目')
      setTarget(null)
      await refresh()
    } catch (error) {
      setLogs({ title: `应用 ${preset.name} 失败`, content: messageOf(error) })
      toast(messageOf(error), 'error')
    }
  }

  return (
    <>
      <PageHeader
        title="预设"
        subtitle="把多个运行时版本、环境变量和 Python 隔离组合成一套可一键切换的环境"
        actions={
          <>
            <button className="btn" onClick={() => void load()}>
              <Icon name="refresh" size={15} /> 刷新
            </button>
            <button className="btn primary" onClick={openNew}>
              <Icon name="plus" size={15} /> 新建预设
            </button>
          </>
        }
      />

      {loading ? (
        <Loader />
      ) : presets.length === 0 ? (
        <Card>
          <Empty
            title="还没有预设"
            hint="预设非常适合全栈项目：例如 nodejs 20 + python 3.12 + 一组环境变量"
            action={
              <button className="btn primary small" onClick={openNew}>
                创建第一个预设
              </button>
            }
          />
        </Card>
      ) : (
        <div className="grid cols-2">
          {presets.map((preset) => (
            <Card
              key={preset.name}
              title={
                <div className="row">
                  <span className="mono">{preset.name}</span>
                  <Badge>{Object.keys(preset.runtimes).length} 个运行时</Badge>
                </div>
              }
              subtitle={preset.description || '未填写描述'}
              actions={
                <>
                  <button className="btn small ghost" onClick={() => openEdit(preset)}>
                    编辑
                  </button>
                  <button className="btn small danger" onClick={() => void remove(preset)}>
                    删除
                  </button>
                </>
              }
            >
              <div className="row wrap">
                {Object.entries(preset.runtimes).map(([sdk, version]) => (
                  <span key={sdk} className="chip">
                    {sdk}@{version}
                  </span>
                ))}
                {preset.python?.venv && <span className="chip">venv {preset.python.venv}</span>}
              </div>
              {preset.env && Object.keys(preset.env).length > 0 && (
                <div className="row wrap" style={{ marginTop: 8 }}>
                  {Object.entries(preset.env).map(([key, value]) => (
                    <span key={key} className="chip">
                      {key}={value}
                    </span>
                  ))}
                </div>
              )}
              <div className="hr" />
              <div className="actions">
                <button className="btn small primary" onClick={() => void apply(preset, 'global')}>
                  全局应用
                </button>
                <button className="btn small" onClick={() => setTarget(preset)}>
                  应用到项目
                </button>
                {preset.updatedAt && (
                  <span className="faint small" style={{ marginLeft: 'auto' }}>
                    {new Date(preset.updatedAt).toLocaleString()}
                  </span>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}

      <Modal
        open={Boolean(form)}
        wide
        title={form?.isNew ? '新建预设' : `编辑 ${form?.name}`}
        onClose={() => setForm(null)}
        footer={
          <>
            <button className="btn" onClick={() => setForm(null)}>
              取消
            </button>
            <button className="btn primary" disabled={saving} onClick={() => void save()}>
              保存
            </button>
          </>
        }
      >
        {form && (
          <>
            <div className="grid cols-2">
              <Field label="名称">
                <input
                  className="input"
                  value={form.name}
                  disabled={!form.isNew}
                  placeholder="fullstack"
                  onChange={(event) => setForm({ ...form, name: event.target.value })}
                />
              </Field>
              <Field label="描述">
                <input
                  className="input"
                  value={form.description}
                  placeholder="Web 全栈开发环境"
                  onChange={(event) => setForm({ ...form, description: event.target.value })}
                />
              </Field>
            </div>

            <Field label="运行时" hint="版本必须已经安装在对应 SDK 下">
              <div className="grid" style={{ gap: 8 }}>
                {form.runtimes.map((row, index) => (
                  <div className="row" key={index}>
                    <input
                      className="input"
                      list="enver-sdk-options"
                      placeholder="sdk 例如 nodejs"
                      value={row.sdk}
                      onChange={(event) => {
                        const runtimes = [...form.runtimes]
                        runtimes[index] = { ...row, sdk: event.target.value }
                        setForm({ ...form, runtimes })
                      }}
                    />
                    <input
                      className="input"
                      list={`enver-ver-${row.sdk}`}
                      placeholder="版本 例如 20.11.1"
                      value={row.version}
                      onChange={(event) => {
                        const runtimes = [...form.runtimes]
                        runtimes[index] = { ...row, version: event.target.value }
                        setForm({ ...form, runtimes })
                      }}
                    />
                    <button
                      className="btn ghost icon-only"
                      onClick={() =>
                        setForm({
                          ...form,
                          runtimes: form.runtimes.filter((_, i) => i !== index),
                        })
                      }
                      aria-label="删除运行时"
                    >
                      <Icon name="x" size={14} />
                    </button>
                  </div>
                ))}
                <datalist id="enver-sdk-options">
                  {sdkNames.map((name) => (
                    <option key={name} value={name} />
                  ))}
                </datalist>
                {overview.sdks.map((sdk) => (
                  <datalist key={sdk.name} id={`enver-ver-${sdk.name}`}>
                    {sdk.versions.map((version) => (
                      <option key={version.version} value={version.version} />
                    ))}
                  </datalist>
                ))}
                <div>
                  <button
                    className="btn small"
                    onClick={() => setForm({ ...form, runtimes: [...form.runtimes, { sdk: '', version: '' }] })}
                  >
                    <Icon name="plus" size={13} /> 添加运行时
                  </button>
                </div>
              </div>
            </Field>

            <Field label="环境变量" hint="应用预设后，内嵌终端会注入这些变量">
              <div className="grid" style={{ gap: 8 }}>
                {form.env.map((row, index) => (
                  <div className="row" key={index}>
                    <input
                      className="input"
                      placeholder="KEY"
                      value={row.key}
                      onChange={(event) => {
                        const env = [...form.env]
                        env[index] = { ...row, key: event.target.value }
                        setForm({ ...form, env })
                      }}
                    />
                    <input
                      className="input"
                      placeholder="value"
                      value={row.value}
                      onChange={(event) => {
                        const env = [...form.env]
                        env[index] = { ...row, value: event.target.value }
                        setForm({ ...form, env })
                      }}
                    />
                    <button
                      className="btn ghost icon-only"
                      onClick={() => setForm({ ...form, env: form.env.filter((_, i) => i !== index) })}
                      aria-label="删除环境变量"
                    >
                      <Icon name="x" size={14} />
                    </button>
                  </div>
                ))}
                <div>
                  <button
                    className="btn small"
                    onClick={() => setForm({ ...form, env: [...form.env, { key: '', value: '' }] })}
                  >
                    <Icon name="plus" size={13} /> 添加变量
                  </button>
                </div>
              </div>
            </Field>

            <div className="grid cols-2">
              <Field label="Python venv 路径（可选）" hint="相对路径按项目目录解析">
                <input
                  className="input"
                  value={form.venv}
                  placeholder=".venv"
                  onChange={(event) => setForm({ ...form, venv: event.target.value })}
                />
              </Field>
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
            </div>
          </>
        )}
      </Modal>

      <Modal
        open={Boolean(target)}
        title={`把 ${target?.name || ''} 应用到项目`}
        onClose={() => setTarget(null)}
        footer={
          <>
            <button className="btn" onClick={() => setTarget(null)}>
              取消
            </button>
            <button
              className="btn primary"
              onClick={() => {
                if (!target) return
                const dir = projectDir.trim()
                if (!dir) {
                  toast('请输入项目目录', 'error')
                  return
                }
                localStorage.setItem('enver-project-dir', dir)
                void apply(target, 'project', dir)
              }}
            >
              应用
            </button>
          </>
        }
      >
        <Field label="项目目录" hint="vfox 会在该目录写入 .vfox.toml 与 .vfox/">
          <input
            className="input"
            value={projectDir}
            placeholder="D:\\work\\myapp"
            onChange={(event) => setProjectDir(event.target.value)}
          />
        </Field>
        <div className="notice warn">
          <Icon name="alert" />
          <div>
            Windows 下项目作用域需要创建软链接。若失败，请开启开发者模式或以管理员身份运行，或改用全局应用。
          </div>
        </div>
      </Modal>

      <Modal
        open={Boolean(logs)}
        wide
        title={logs?.title || '日志'}
        onClose={() => setLogs(null)}
        footer={
          <button className="btn" onClick={() => setLogs(null)}>
            关闭
          </button>
        }
      >
        <pre className="log">{logs?.content}</pre>
      </Modal>
    </>
  )
}

function messageOf(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}
