import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, downloadPackages } from '../api'
import { Badge, Card, Empty, Icon, Loader, PageHeader, useToast } from '../components'
import type { PackageItem } from '../types'
import type { PageProps } from './types'

export default function Packages({ overview }: PageProps) {
  const toast = useToast()
  const [sdk, setSdk] = useState('')
  const [version, setVersion] = useState('')
  const [items, setItems] = useState<PackageItem[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [filter, setFilter] = useState('')

  const options = useMemo(() => {
    const list: { sdk: string; version: string; current: boolean }[] = []
    overview.sdks.forEach((runtime) => {
      runtime.versions.forEach((item) => {
        list.push({ sdk: runtime.name, version: item.version, current: item.current })
      })
    })
    return list
  }, [overview.sdks])

  const sdkNames = useMemo(() => Array.from(new Set(options.map((option) => option.sdk))), [options])
  const versions = useMemo(
    () => options.filter((option) => option.sdk === sdk).map((option) => option.version),
    [options, sdk],
  )

  useEffect(() => {
    if (options.length === 0) return
    const valid = options.some((option) => option.sdk === sdk && option.version === version)
    if (!valid) {
      const preferred = options.find((option) => option.current) || options[0]
      setSdk(preferred.sdk)
      setVersion(preferred.version)
    }
  }, [options, sdk, version])

  const load = useCallback(
    async (nextSDK: string, nextVersion: string) => {
      if (!nextSDK || !nextVersion) return
      setLoading(true)
      setError('')
      try {
        const data = await api.packages(nextSDK, nextVersion)
        setItems(data.packages)
      } catch (err) {
        setItems([])
        setError(messageOf(err))
      } finally {
        setLoading(false)
      }
    },
    [],
  )

  useEffect(() => {
    void load(sdk, version)
  }, [sdk, version, load])

  const filtered = useMemo(() => {
    const query = filter.trim().toLowerCase()
    if (!query) return items
    return items.filter((item) => item.name.toLowerCase().includes(query))
  }, [items, filter])

  const exportList = async () => {
    try {
      await downloadPackages(sdk, version)
      toast('已导出清单')
    } catch (err) {
      toast(messageOf(err), 'error')
    }
  }

  return (
    <>
      <PageHeader
        title="全局库"
        subtitle="按运行时版本查看全局安装的库，导出清单便于迁移到另一个版本"
        actions={
          <>
            <button className="btn" onClick={() => void load(sdk, version)} disabled={!sdk}>
              <Icon name="refresh" size={15} /> 刷新
            </button>
            <button className="btn primary" onClick={() => void exportList()} disabled={items.length === 0}>
              <Icon name="download" size={15} /> 导出清单
            </button>
          </>
        }
      />

      {options.length === 0 ? (
        <Card>
          <Empty title="还没有可查询的运行时" hint="先安装一个 Node.js 或 Python 版本" />
        </Card>
      ) : (
        <Card
          title={`${sdk || '运行时'} 全局包`}
          subtitle={
            items.length > 0
              ? `${items.length} 个包 · ${items[0]?.manager || ''}`
              : '选择一个已安装的运行时版本'
          }
          actions={
            <div className="row">
              <select
                className="select"
                style={{ width: 150 }}
                value={sdk}
                onChange={(event) => {
                  const name = event.target.value
                  const first = options.find((option) => option.sdk === name)
                  setSdk(name)
                  setVersion(first?.version || '')
                }}
              >
                {sdkNames.map((name) => (
                  <option key={name} value={name}>
                    {name}
                  </option>
                ))}
              </select>
              <select
                className="select"
                style={{ width: 150 }}
                value={version}
                onChange={(event) => setVersion(event.target.value)}
              >
                {versions.map((item) => (
                  <option key={item} value={item}>
                    {item}
                  </option>
                ))}
              </select>
              <input
                className="input"
                style={{ width: 170 }}
                placeholder="筛选包名"
                value={filter}
                onChange={(event) => setFilter(event.target.value)}
              />
            </div>
          }
        >
          {loading ? (
            <Loader label="读取全局包…" />
          ) : error ? (
            <div className="notice bad">
              <Icon name="alert" />
              <div>
                <strong>无法读取全局包</strong>
                <div>{error}</div>
                <div className="faint small" style={{ marginTop: 4 }}>
                  当前支持 Node.js（npm -g）与 Python（pip）。其他运行时会在后续版本补充。
                </div>
              </div>
            </div>
          ) : filtered.length === 0 ? (
            <Empty
              title={items.length === 0 ? '这个版本没有全局包' : '没有匹配的包'}
              hint={items.length === 0 ? '使用 npm -g 或 pip 安装后会自动出现在这里' : '清空筛选条件试试'}
            />
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>包名</th>
                    <th>版本</th>
                    <th>管理器</th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((item) => (
                    <tr key={item.name}>
                      <td className="mono">{item.name}</td>
                      <td className="mono muted">{item.version}</td>
                      <td>
                        <Badge tone="blue">{item.manager}</Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      )}
    </>
  )
}

function messageOf(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}
