import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'

function formatTime(timeStr) {
  return new Date(timeStr).toLocaleTimeString('ko-KR', {
    hour: '2-digit',
    minute: '2-digit',
  })
}

const LINES = [
  { key: 'CPU',    color: '#3b82f6' },
  { key: '메모리', color: '#10b981' },
  { key: '디스크', color: '#f59e0b' },
]

export default function MetricChart({ data }) {
  const chartData = data.map((m) => ({
    time:   formatTime(m.time),
    CPU:    parseFloat(m.cpu?.toFixed(1) ?? 0),
    메모리: parseFloat(m.memory?.toFixed(1) ?? 0),
    디스크: parseFloat(m.disk?.toFixed(1) ?? 0),
  }))

  return (
    <div className="bg-white rounded-xl border p-4">
      <h3 className="text-sm font-semibold text-gray-500 mb-4">메트릭 추이 (%)</h3>
      {chartData.length === 0 ? (
        <div className="flex items-center justify-center h-48 text-gray-400 text-sm">
          데이터 없음
        </div>
      ) : (
        <ResponsiveContainer width="100%" height={220}>
          <LineChart data={chartData} margin={{ top: 4, right: 8, bottom: 0, left: -8 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
            <XAxis dataKey="time" tick={{ fontSize: 11 }} />
            <YAxis domain={[0, 100]} tick={{ fontSize: 11 }} unit="%" />
            <Tooltip formatter={(v) => `${v}%`} />
            <Legend />
            {LINES.map(({ key, color }) => (
              <Line
                key={key}
                type="monotone"
                dataKey={key}
                stroke={color}
                dot={false}
                strokeWidth={2}
              />
            ))}
          </LineChart>
        </ResponsiveContainer>
      )}
    </div>
  )
}
