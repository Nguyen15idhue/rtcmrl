import { useState, useEffect, useCallback } from 'react'
import { api, StationsResponse } from '../api'

function timeAgo(s: string): string {
  const diff = Date.now() - new Date(s).getTime()
  if (diff < 60000) return Math.floor(diff / 1000) + 's ago'
  if (diff < 3600000) return Math.floor(diff / 60000) + 'm ago'
  return Math.floor(diff / 3600000) + 'h ago'
}

function fmtBytes(n: number): string {
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  return (n / 1024 / 1024).toFixed(1) + ' MB'
}

export default function Stations() {
  const [data, setData] = useState<StationsResponse | null>(null)

  const load = useCallback(async () => {
    try {
      const d = await api.get<StationsResponse>('/api/v1/stations')
      setData(d)
    } catch (e) {
      console.error(e)
    }
  }, [])

  useEffect(() => {
    load()
    const id = setInterval(load, 5000)
    return () => clearInterval(id)
  }, [load])

  if (!data) return <div className="loading">Loading stations...</div>

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2>Stations ({data.total} active)</h2>
        <button className="btn" onClick={load}>Refresh</button>
      </div>

      {data.total === 0 ? (
        <div className="section">
          <p style={{ color: 'var(--text2)' }}>No active stations. Waiting for incoming RTCM data...</p>
        </div>
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table>
            <thead>
              <tr>
                <th>Station ID</th>
                <th>Mount</th>
                <th>Source IP</th>
                <th>Frames Out</th>
                <th>Bytes Out</th>
                <th>Last Seen</th>
              </tr>
            </thead>
            <tbody>
              {data.stations.map((s, i) => (
                <tr key={i}>
                  <td><span className="badge green">{s.station_id}</span></td>
                  <td style={{ fontFamily: 'monospace' }}>{s.mount}</td>
                  <td style={{ fontFamily: 'monospace', fontSize: '12px' }}>{s.source_ip}</td>
                  <td>{s.frames_out.toLocaleString()}</td>
                  <td>{fmtBytes(s.bytes_out)}</td>
                  <td>{timeAgo(s.last_seen)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
