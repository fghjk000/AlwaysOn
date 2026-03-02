const STATUS_CONFIG = {
  normal:   { style: 'bg-green-100 text-green-800',   label: '정상' },
  warning:  { style: 'bg-yellow-100 text-yellow-800', label: '경고' },
  critical: { style: 'bg-orange-100 text-orange-800', label: '위험' },
  down:     { style: 'bg-red-100 text-red-800',       label: '다운' },
}

export default function StatusBadge({ status }) {
  const { style, label } = STATUS_CONFIG[status] || STATUS_CONFIG.normal
  return (
    <span className={`px-2 py-0.5 rounded-full text-xs font-semibold ${style}`}>
      {label}
    </span>
  )
}
