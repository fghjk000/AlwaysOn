import { useState, useEffect } from 'react'
import { getServers } from '../api/client'
import ServerCard from '../components/ServerCard'

const SUMMARY_ITEMS = [
  { key: 'normal',   label: '정상', style: 'bg-green-50 border-green-200 text-green-700' },
  { key: 'warning',  label: '경고', style: 'bg-yellow-50 border-yellow-200 text-yellow-700' },
  { key: 'critical', label: '위험', style: 'bg-orange-50 border-orange-200 text-orange-700' },
  { key: 'down',     label: '다운', style: 'bg-red-50 border-red-200 text-red-700' },
]

export default function ServerList() {
  const [servers, setServers] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const fetchServers = async () => {
    try {
      const { data } = await getServers()
      setServers(data)
      setError(null)
    } catch {
      setError('서버 목록을 불러오지 못했습니다.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchServers()
    const id = setInterval(fetchServers, 10000)
    return () => clearInterval(id)
  }, [])

  const counts = SUMMARY_ITEMS.reduce((acc, { key }) => {
    acc[key] = servers.filter((s) => s.status === key).length
    return acc
  }, {})

  if (loading) {
    return <div className="text-center py-20 text-gray-400">로딩 중...</div>
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-bold text-gray-800">서버 모니터링</h1>
        <span className="text-sm text-gray-400">10초마다 자동 갱신</span>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 text-red-700 rounded-lg text-sm">
          {error}
        </div>
      )}

      <div className="flex gap-3 mb-6">
        {SUMMARY_ITEMS.map(({ key, label, style }) => (
          <div key={key} className={`border rounded-lg px-4 py-2 text-sm ${style}`}>
            {label} <span className="font-bold ml-1">{counts[key]}</span>
          </div>
        ))}
      </div>

      {servers.length === 0 ? (
        <div className="text-center py-20 text-gray-400">
          <p className="text-lg mb-2">등록된 서버가 없습니다</p>
          <p className="text-sm">에이전트를 설치하면 자동으로 등록됩니다</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {servers.map((server) => (
            <ServerCard key={server.id} server={server} />
          ))}
        </div>
      )}
    </div>
  )
}
