import { Link } from 'react-router-dom'
import StatusBadge from './StatusBadge'

const BORDER_COLOR = {
  normal:   'border-green-400',
  warning:  'border-yellow-400',
  critical: 'border-orange-400',
  down:     'border-red-400',
}

export default function ServerCard({ server }) {
  const borderColor = BORDER_COLOR[server.status] || 'border-gray-200'
  const lastSeen = server.last_seen
    ? new Date(server.last_seen).toLocaleString('ko-KR')
    : '없음'

  return (
    <Link to={`/servers/${server.id}`}>
      <div className={`bg-white rounded-xl border-l-4 ${borderColor} shadow-sm p-4 hover:shadow-md transition-shadow cursor-pointer`}>
        <div className="flex justify-between items-start mb-1">
          <h3 className="font-semibold text-gray-800 truncate">{server.name}</h3>
          <StatusBadge status={server.status} />
        </div>
        <p className="text-sm text-gray-500 truncate">{server.host}</p>
        <p className="text-xs text-gray-400 mt-2">마지막 수신: {lastSeen}</p>
      </div>
    </Link>
  )
}
