import { useState, useEffect } from 'react'
import { getAlerts, getServers } from '../api/client'

const FILTERS = [
  { key: 'all',      label: '전체' },
  { key: 'warning',  label: '경고' },
  { key: 'critical', label: '위험' },
  { key: 'down',     label: '다운' },
]

const LEVEL_CONFIG = {
  warning:  { badge: 'bg-yellow-100 text-yellow-800', label: '경고' },
  critical: { badge: 'bg-orange-100 text-orange-800', label: '위험' },
  down:     { badge: 'bg-red-100 text-red-800',       label: '다운' },
}

export default function AlertHistory() {
  const [alerts, setAlerts]   = useState([])
  const [servers, setServers] = useState({})
  const [filter, setFilter]   = useState('all')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchAll = async () => {
      try {
        const [{ data: alertData }, { data: serverData }] = await Promise.all([
          getAlerts(200),
          getServers(),
        ])
        setAlerts(alertData)
        setServers(Object.fromEntries(serverData.map((s) => [s.id, s])))
      } finally {
        setLoading(false)
      }
    }
    fetchAll()
  }, [])

  const filtered =
    filter === 'all' ? alerts : alerts.filter((a) => a.level === filter)

  if (loading) return <div className="text-center py-20 text-gray-400">로딩 중...</div>

  return (
    <div>
      <h1 className="text-xl font-bold text-gray-800 mb-6">알림 히스토리</h1>

      {/* 필터 */}
      <div className="flex gap-2 mb-6">
        {FILTERS.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setFilter(key)}
            className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-colors ${
              filter === key
                ? 'bg-blue-600 text-white'
                : 'bg-white border text-gray-600 hover:bg-gray-50'
            }`}
          >
            {label}
            {key !== 'all' && (
              <span className="ml-1.5 text-xs opacity-70">
                {alerts.filter((a) => a.level === key).length}
              </span>
            )}
          </button>
        ))}
      </div>

      {filtered.length === 0 ? (
        <div className="text-center py-20 text-gray-400">알림이 없습니다</div>
      ) : (
        <div className="space-y-2">
          {filtered.map((alert) => {
            const serverName = servers[alert.server_id]?.name || alert.server_id
            const cfg = LEVEL_CONFIG[alert.level] || { badge: 'bg-gray-100 text-gray-700', label: alert.level }
            return (
              <div key={alert.id} className="bg-white border rounded-xl p-4">
                <div className="flex justify-between items-start">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className={`px-2 py-0.5 rounded-full text-xs font-semibold ${cfg.badge}`}>
                      {cfg.label}
                    </span>
                    <span className="font-medium text-gray-800">{serverName}</span>
                    <span className="text-sm text-gray-500">{alert.metric}</span>
                    <span className="text-sm font-semibold text-gray-700">
                      {alert.value?.toFixed(1)}%
                    </span>
                  </div>
                  <div className="text-right text-xs text-gray-400 shrink-0 ml-4">
                    <div>{new Date(alert.created_at).toLocaleString('ko-KR')}</div>
                    {alert.resolved_at && (
                      <div className="text-green-600 mt-0.5">
                        복구: {new Date(alert.resolved_at).toLocaleString('ko-KR')}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
