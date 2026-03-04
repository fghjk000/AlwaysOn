import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { getServers, getServerMetrics, getAlerts, getHealthChecks, createHealthCheck, deleteHealthCheck } from '../api/client'
import MetricChart from '../components/MetricChart'
import StatusBadge from '../components/StatusBadge'

const HOURS_OPTIONS = [
  { label: '1시간',  value: 1 },
  { label: '6시간',  value: 6 },
  { label: '24시간', value: 24 },
]

const LEVEL_STYLE = {
  warning:  'text-yellow-600 bg-yellow-50',
  critical: 'text-orange-600 bg-orange-50',
  down:     'text-red-600 bg-red-50',
}

function MetricCard({ label, value }) {
  const pct = value?.toFixed(1) ?? '—'
  const color =
    value >= 90 ? 'text-red-600' :
    value >= 75 ? 'text-orange-500' :
    'text-gray-800'
  return (
    <div className="bg-white rounded-xl border p-4 text-center">
      <p className="text-xs text-gray-400 mb-1">{label}</p>
      <p className={`text-2xl font-bold ${color}`}>{pct}%</p>
    </div>
  )
}

export default function ServerDetail() {
  const { id } = useParams()
  const [server, setServer]   = useState(null)
  const [metrics, setMetrics] = useState([])
  const [alerts, setAlerts]   = useState([])
  const [hours, setHours]     = useState(1)
  const [loading, setLoading] = useState(true)
  const [healthChecks, setHealthChecks] = useState([])
  const [showHCForm, setShowHCForm] = useState(false)
  const [hcForm, setHcForm] = useState({ name: '', type: 'http', target: '', expected_status: 200 })

  useEffect(() => {
    const fetchAll = async () => {
      try {
        const [{ data: serverList }, { data: m }, { data: a }] = await Promise.all([
          getServers(),
          getServerMetrics(id, hours),
          getAlerts(200),
        ])
        setServer(serverList.find((s) => s.id === id) || null)
        setMetrics(m)
        setAlerts(a.filter((al) => al.server_id === id))
      } finally {
        setLoading(false)
      }
    }
    fetchAll()
    const interval = setInterval(fetchAll, 5000)
    return () => clearInterval(interval)
  }, [id, hours])

  useEffect(() => {
    getHealthChecks(id).then(({ data }) => setHealthChecks(data))
  }, [id])

  if (loading) return <div className="text-center py-20 text-gray-400">로딩 중...</div>
  if (!server)  return <div className="text-center py-20 text-gray-400">서버를 찾을 수 없습니다</div>

  const latest = metrics[metrics.length - 1]

  return (
    <div>
      {/* 헤더 */}
      <div className="flex items-center gap-3 mb-6">
        <Link to="/" className="text-gray-400 hover:text-gray-600 text-sm">← 목록으로</Link>
        <h1 className="text-xl font-bold text-gray-800">{server.name}</h1>
        <StatusBadge status={server.status} />
        <span className="text-sm text-gray-400">{server.host}</span>
      </div>

      {/* 현재 메트릭 요약 */}
      {latest && (
        <div className="grid grid-cols-3 gap-4 mb-6">
          <MetricCard label="CPU"    value={latest.cpu} />
          <MetricCard label="메모리" value={latest.memory} />
          <MetricCard label="디스크" value={latest.disk} />
        </div>
      )}

      {/* 프로세스 상태 */}
      {latest?.processes?.length > 0 && (
        <div className="bg-white rounded-lg shadow p-4 mb-4">
          <h3 className="font-semibold text-gray-700 mb-2">프로세스</h3>
          <div className="flex flex-wrap gap-2">
            {latest.processes.map(p => (
              <span
                key={p.name}
                className={`px-3 py-1 rounded-full text-sm font-medium ${
                  p.running
                    ? 'bg-green-100 text-green-700'
                    : 'bg-red-100 text-red-700'
                }`}
              >
                {p.running ? '●' : '○'} {p.name}
              </span>
            ))}
          </div>
        </div>
      )}

      {/* 헬스체크 패널 */}
      <div className="bg-white rounded-lg shadow p-4 mb-4">
        <div className="flex justify-between items-center mb-2">
          <h3 className="font-semibold text-gray-700">헬스체크</h3>
          <button
            onClick={() => setShowHCForm(!showHCForm)}
            className="text-sm text-blue-600 hover:underline"
          >
            + 추가
          </button>
        </div>

        {showHCForm && (
          <form onSubmit={async (e) => {
            e.preventDefault()
            const { data: created } = await createHealthCheck(id, hcForm)
            setHealthChecks(prev => [...prev, created])
            setShowHCForm(false)
            setHcForm({ name: '', type: 'http', target: '', expected_status: 200 })
          }} className="mb-3 flex gap-2 flex-wrap">
            <input
              placeholder="이름 (예: Nginx)"
              value={hcForm.name}
              onChange={e => setHcForm(f => ({ ...f, name: e.target.value }))}
              className="border rounded px-2 py-1 text-sm flex-1"
              required
            />
            <select
              value={hcForm.type}
              onChange={e => setHcForm(f => ({ ...f, type: e.target.value }))}
              className="border rounded px-2 py-1 text-sm"
            >
              <option value="http">HTTP</option>
              <option value="tcp">TCP</option>
            </select>
            <input
              placeholder="http://... 또는 host:port"
              value={hcForm.target}
              onChange={e => setHcForm(f => ({ ...f, target: e.target.value }))}
              className="border rounded px-2 py-1 text-sm flex-1"
              required
            />
            <button type="submit" className="bg-blue-500 text-white px-3 py-1 rounded text-sm">
              저장
            </button>
          </form>
        )}

        {healthChecks.length === 0 ? (
          <p className="text-gray-400 text-sm">등록된 헬스체크 없음</p>
        ) : (
          <ul className="divide-y">
            {healthChecks.map(hc => (
              <li key={hc.id} className="flex justify-between items-center py-1 text-sm">
                <span>
                  <span className="font-mono text-gray-500 mr-2">[{hc.type.toUpperCase()}]</span>
                  {hc.name} — {hc.target}
                </span>
                <button
                  onClick={async () => {
                    await deleteHealthCheck(id, hc.id)
                    setHealthChecks(prev => prev.filter(h => h.id !== hc.id))
                  }}
                  className="text-red-400 hover:text-red-600 ml-2"
                >
                  삭제
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>

      {/* 조회 범위 선택 */}
      <div className="flex gap-2 mb-4">
        {HOURS_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            onClick={() => setHours(opt.value)}
            className={`px-3 py-1 rounded-lg text-sm font-medium transition-colors ${
              hours === opt.value
                ? 'bg-blue-600 text-white'
                : 'bg-white border text-gray-600 hover:bg-gray-50'
            }`}
          >
            {opt.label}
          </button>
        ))}
      </div>

      {/* 메트릭 차트 */}
      <MetricChart data={metrics} />

      {/* 알림 히스토리 */}
      {alerts.length > 0 && (
        <div className="mt-8">
          <h2 className="text-base font-semibold text-gray-700 mb-3">이 서버의 알림</h2>
          <div className="space-y-2">
            {alerts.slice(0, 10).map((alert) => (
              <div
                key={alert.id}
                className={`rounded-lg p-3 text-sm ${LEVEL_STYLE[alert.level] || 'bg-gray-50 text-gray-700'}`}
              >
                <div className="flex justify-between items-center">
                  <span className="font-semibold uppercase">{alert.level} — {alert.metric}</span>
                  <span className="text-xs opacity-70">
                    {new Date(alert.created_at).toLocaleString('ko-KR')}
                  </span>
                </div>
                <p className="mt-1 opacity-80">{alert.value?.toFixed(1)}% 감지</p>
                {alert.resolved_at && (
                  <p className="text-xs text-green-600 mt-1">
                    복구: {new Date(alert.resolved_at).toLocaleString('ko-KR')}
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
