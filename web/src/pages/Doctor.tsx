import { useCallback, useEffect, useState } from 'react'
import { api } from '../api'
import { Badge, Card, Icon, Loader, PageHeader, useToast } from '../components'
import type { Check, DoctorReport } from '../types'
import type { PageProps } from './types'

export default function Doctor({ refresh }: PageProps) {
  const toast = useToast()
  const [report, setReport] = useState<DoctorReport | null>(null)
  const [loading, setLoading] = useState(true)

  const run = useCallback(async () => {
    setLoading(true)
    try {
      const data = await api.doctor()
      setReport(data)
      await refresh()
    } catch (error) {
      toast(error instanceof Error ? error.message : String(error), 'error')
    } finally {
      setLoading(false)
    }
  }, [refresh, toast])

  useEffect(() => {
    void run()
  }, [run])

  const summary = report?.summary || {}

  return (
    <>
      <PageHeader
        title="诊断"
        subtitle="检查 vfox、PATH 冲突、磁盘空间、软链接与虚拟环境健康度"
        actions={
          <button className="btn primary" onClick={() => void run()} disabled={loading}>
            <Icon name="refresh" size={15} /> 重新检查
          </button>
        }
      />

      {loading && !report ? (
        <Loader label="正在体检…" />
      ) : (
        <>
          <div className="grid stats">
            <Card>
              <div className="stat-label">通过</div>
              <div className="stat-value">{summary.ok || 0}</div>
            </Card>
            <Card>
              <div className="stat-label">警告</div>
              <div className="stat-value">{summary.warn || 0}</div>
            </Card>
            <Card>
              <div className="stat-label">问题</div>
              <div className="stat-value">{summary.error || 0}</div>
            </Card>
            <Card>
              <div className="stat-label">提示</div>
              <div className="stat-value">{summary.info || 0}</div>
            </Card>
          </div>

          <Card
            title="检查项"
            subtitle={report ? `检查时间 ${new Date(report.checkedAt).toLocaleString()}` : ''}
          >
            <div className="checks">
              {(report?.checks || []).map((check) => (
                <CheckRow key={check.id} check={check} />
              ))}
            </div>
          </Card>
        </>
      )}
    </>
  )
}

function CheckRow({ check }: { check: Check }) {
  const icon = check.status === 'ok' ? 'check' : check.status === 'info' ? 'activity' : 'alert'
  return (
    <div className="check">
      <span className={`check-icon ${check.status}`}>
        <Icon name={icon} size={15} />
      </span>
      <div className="check-body" style={{ flex: 1 }}>
        <div className="row" style={{ justifyContent: 'space-between' }}>
          <span className="check-title">{check.title}</span>
          <Badge
            tone={
              check.status === 'ok'
                ? 'ok'
                : check.status === 'warn'
                  ? 'warn'
                  : check.status === 'error'
                    ? 'bad'
                    : 'blue'
            }
          >
            {statusText(check.status)}
          </Badge>
        </div>
        <div className="check-detail">{check.detail}</div>
        {check.hint && <div className="check-hint">{check.hint}</div>}
      </div>
    </div>
  )
}

function statusText(status: Check['status']) {
  switch (status) {
    case 'ok':
      return '通过'
    case 'warn':
      return '警告'
    case 'error':
      return '问题'
    default:
      return '提示'
  }
}
