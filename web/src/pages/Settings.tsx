import { useState } from 'react'
import { api } from '../api'
import {
  Badge,
  Card,
  Field,
  Icon,
  JobConsole,
  Modal,
  PageHeader,
  formatBytes,
  useToast,
} from '../components'
import type { Job } from '../types'
import type { PageProps } from './types'

export default function Settings({ overview, refresh }: PageProps) {
  const toast = useToast()
  const [path, setPath] = useState(overview.storage.sdkRoot || '')
  const [deleteOld, setDeleteOld] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [starting, setStarting] = useState(false)
  const [job, setJob] = useState<Job | null>(null)
  const [bin, setBin] = useState(overview.vfox?.bin || '')
  const [savingBin, setSavingBin] = useState(false)

  const startMigrate = async () => {
    if (!path.trim()) {
      toast('请输入目标路径', 'error')
      return
    }
    setStarting(true)
    try {
      const created = await api.migrate(path.trim(), deleteOld)
      setJob(created)
      setConfirming(false)
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setStarting(false)
    }
  }

  const saveBin = async () => {
    setSavingBin(true)
    try {
      await api.setVfoxBin(bin.trim())
      toast('vfox 路径已更新')
      await refresh()
    } catch (error) {
      toast(messageOf(error), 'error')
    } finally {
      setSavingBin(false)
    }
  }

  return (
    <>
      <PageHeader
        title="设置"
        subtitle="存储位置、vfox 可执行文件与应用信息"
        actions={
          <button className="btn" onClick={() => void refresh()}>
            <Icon name="refresh" size={15} /> 刷新
          </button>
        }
      />

      <div className="grid cols-2">
        <Card title="SDK 存储位置" subtitle="修改后需要迁移已安装的运行时">
          <div className="kv">
            <div className="kv-row">
              <span>vfox Home</span>
              <span>{overview.storage.home}</span>
            </div>
            <div className="kv-row">
              <span>当前 SDK 根目录</span>
              <span>{overview.storage.sdkRoot || '—'}</span>
            </div>
            <div className="kv-row">
              <span>可用空间</span>
              <span>{formatBytes(overview.storage.freeBytes)}</span>
            </div>
            <div className="kv-row">
              <span>配置文件</span>
              <span>{overview.storage.configPath || '—'}</span>
            </div>
          </div>
          <div className="hr" />
          <Field label="新的 SDK 根目录" hint="必须是绝对路径。迁移采用复制 → 校验 → 改配置 → 重建软链接。">
            <input
              className="input"
              value={path}
              placeholder={overview.storage.sdkRoot || 'D:\\vfox-sdks'}
              onChange={(event) => setPath(event.target.value)}
            />
          </Field>
          <label className="row small muted" style={{ marginTop: 10, gap: 8 }}>
            <input
              type="checkbox"
              checked={deleteOld}
              onChange={(event) => setDeleteOld(event.target.checked)}
            />
            校验通过后删除旧目录
          </label>
          <div className="actions" style={{ marginTop: 12 }}>
            <button className="btn primary" onClick={() => setConfirming(true)} disabled={starting}>
              <Icon name="disk" size={15} /> 迁移 SDK
            </button>
          </div>
          <div className="notice warn" style={{ marginTop: 12 }}>
            <Icon name="alert" />
            <div>
              迁移会移动全部已安装 SDK。执行前请关闭正在运行的 Node/Python 进程，避免文件占用。
            </div>
          </div>
        </Card>

        <div className="grid">
          <Card title="vfox" subtitle="enver 只通过 CLI 调用 vfox，不复制其状态">
            <div className="kv">
              <div className="kv-row">
                <span>版本</span>
                <span>{overview.vfox?.version || '未检测到'}</span>
              </div>
              <div className="kv-row">
                <span>可执行文件</span>
                <span>{overview.vfox?.bin || '—'}</span>
              </div>
              <div className="kv-row">
                <span>最低要求</span>
                <span>v{overview.vfox?.minVersion || '1.0.13'}</span>
              </div>
              <div className="kv-row">
                <span>状态</span>
                <span>
                  {overview.vfox?.compatible ? (
                    <Badge tone="ok">兼容</Badge>
                  ) : (
                    <Badge tone="warn">需要升级</Badge>
                  )}
                </span>
              </div>
            </div>
            <div className="hr" />
            <Field label="自定义 vfox 路径" hint="留空则自动从 PATH 与常见安装目录查找">
              <input
                className="input"
                value={bin}
                placeholder="C:\\Program Files\\vfox\\vfox.exe"
                onChange={(event) => setBin(event.target.value)}
              />
            </Field>
            <div className="actions" style={{ marginTop: 10 }}>
              <button className="btn" onClick={() => void saveBin()} disabled={savingBin}>
                保存路径
              </button>
              <a className="btn ghost" href="https://vfox.dev" target="_blank" rel="noreferrer">
                <Icon name="external" size={14} /> vfox 文档
              </a>
            </div>
          </Card>

          <Card title="关于" subtitle="本地优先，不上传任何环境数据">
            <div className="kv">
              <div className="kv-row">
                <span>enver 版本</span>
                <span>{overview.enverVersion || '—'}</span>
              </div>
              <div className="kv-row">
                <span>平台</span>
                <span>{overview.platform}</span>
              </div>
              <div className="kv-row">
                <span>UI</span>
                <span>127.0.0.1 · 会话令牌保护</span>
              </div>
              <div className="kv-row">
                <span>数据</span>
                <span>预设与 venv 元数据仅存本机</span>
              </div>
            </div>
            <div className="notice" style={{ marginTop: 12 }}>
              <Icon name="activity" />
              <div>
                想贡献代码或反馈问题？项目按 MIT 协议开源，欢迎提交 issue 与 PR。
              </div>
            </div>
          </Card>
        </div>
      </div>

      <Modal
        open={confirming}
        title="确认迁移 SDK 存储目录"
        onClose={() => setConfirming(false)}
        footer={
          <>
            <button className="btn" onClick={() => setConfirming(false)}>
              取消
            </button>
            <button className="btn primary" disabled={starting} onClick={() => void startMigrate()}>
              开始迁移
            </button>
          </>
        }
      >
        <div className="notice warn">
          <Icon name="alert" />
          <div>
            <strong>从</strong>
            <div className="mono small">{overview.storage.sdkRoot}</div>
            <strong style={{ display: 'block', marginTop: 8 }}>到</strong>
            <div className="mono small">{path || '(未填写)'}</div>
          </div>
        </div>
        <div className="muted small">
          迁移期间 vfox 写操作会被串行化。完成后 enver 会重新建立全局版本软链接并校验新目录。
          {deleteOld ? ' 校验通过后将删除旧目录。' : ' 旧目录会保留，确认无误后可手动删除。'}
        </div>
      </Modal>

      <JobConsole
        job={job}
        onClose={() => {
          setJob(null)
          void refresh()
        }}
        onDone={() => {
          void refresh()
        }}
      />
    </>
  )
}

function messageOf(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}
