import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8080/api',
})

export const getServers = () => api.get('/servers')
export const getServer = (id) => api.get(`/servers/${id}`)
export const getServerMetrics = (id, hours = 1) =>
  api.get(`/servers/${id}/metrics?hours=${hours}`)
export const getAlerts = (limit = 100) =>
  api.get(`/alerts?limit=${limit}`)
export const updateThresholds = (id, data) =>
  api.put(`/servers/${id}/thresholds`, data)
