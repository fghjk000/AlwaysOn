import { Link, useLocation } from 'react-router-dom'

export default function Navbar() {
  const { pathname } = useLocation()

  const linkClass = (path) =>
    `text-sm font-medium transition-colors ${
      pathname === path
        ? 'text-white'
        : 'text-gray-400 hover:text-white'
    }`

  return (
    <nav className="bg-gray-900 border-b border-gray-800 px-6 py-4 flex items-center gap-6">
      <Link to="/" className="text-lg font-bold text-green-400 tracking-tight">
        AlwaysOn
      </Link>
      <Link to="/" className={linkClass('/')}>서버 목록</Link>
      <Link to="/alerts" className={linkClass('/alerts')}>알림 히스토리</Link>
    </nav>
  )
}
