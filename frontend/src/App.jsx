import { BrowserRouter, Routes, Route } from 'react-router-dom'
import Navbar from './components/Navbar'
import ServerList from './pages/ServerList'
import ServerDetail from './pages/ServerDetail'
import AlertHistory from './pages/AlertHistory'

export default function App() {
  return (
    <BrowserRouter>
      <div className="min-h-screen bg-gray-50">
        <Navbar />
        <main className="container mx-auto px-4 py-6 max-w-7xl">
          <Routes>
            <Route path="/" element={<ServerList />} />
            <Route path="/servers/:id" element={<ServerDetail />} />
            <Route path="/alerts" element={<AlertHistory />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  )
}
