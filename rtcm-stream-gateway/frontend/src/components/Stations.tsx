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

interface GeneratorConfig {
  Host: string
  Port: number
  StationIDs: number[]
  IntervalMs: number
  FrameType: number
}

interface GeneratorStatus {
  running: boolean
  config: GeneratorConfig | null
}

interface StationQuality {
  station_id: number
  mount: string
  frames_in: number
  frames_out: number
  frames_dropped: number
  bytes_in: number
  bytes_out: number
  packet_loss_percent: number
  avg_latency_ms: number
  last_seen: string
  uptime_sec: number
}

export default function Stations() {
  const [data, setData] = useState<StationsResponse | null>(null)
  const [quality, setQuality] = useState<StationQuality[]>([])
  const [genStatus, setGenStatus] = useState<GeneratorStatus>({ running: false, config: null })
  
  const [genStations, setGenStations] = useState(5)
  const [genInterval, setGenInterval] = useState(1000)
  const [starting, setStarting] = useState(false)

  const load = useCallback(async () => {
    try {
      const [d, g, q] = await Promise.all([
        api.get<StationsResponse>('/api/v1/stations'),
        api.get<GeneratorStatus>('/api/v1/generator'),
        api.get<StationQuality[]>('/api/v1/stations/quality').catch(() => [] as StationQuality[])
      ])
      setData(d)
      setGenStatus(g)
      setQuality(q)
    } catch (e) {
      console.error(e)
    }
  }, [])

  useEffect(() => {
    load()
    const id = setInterval(load, 3000)
    return () => clearInterval(id)
  }, [load])

  const handleStartGenerator = async () => {
    setStarting(true)
    try {
      const stations: number[] = []
      if (genStations >= 1) stations.push(1)
      if (genStations >= 2) stations.push(2)
      if (genStations >= 3) stations.push(3)
      if (genStations >= 4) stations.push(1001)
      if (genStations >= 5) stations.push(2001)
      for (let i = 5; i < genStations; i++) {
        stations.push(3000 + i)
      }
      
      await api.post('/api/v1/generator/start', {
        stations,
        interval_ms: genInterval,
        frame_type: 1006
      })
      load()
    } catch (e) {
      console.error(e)
    }
    setStarting(false)
  }

  const handleStopGenerator = async () => {
    try {
      await api.post('/api/v1/generator/stop', {})
      load()
    } catch (e) {
      console.error(e)
    }
  }

  const getQualityForStation = (stationId: number): StationQuality | undefined => {
    return quality.find(q => q.station_id === stationId)
  }

  if (!data) return <div className="loading">Loading stations...</div>

  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <h2>Stations ({data.total} active)</h2>
        <button className="btn" onClick={load}>Refresh</button>
      </div>

      {/* Generator Controls */}
      <div className="section mb-4">
        <h2 style={{ fontSize: '16px', marginBottom: '12px' }}>RTCM Generator (Test Mode)</h2>
        <div style={{ display: 'flex', gap: '16px', alignItems: 'flex-end', flexWrap: 'wrap' }}>
          <div className="form-group" style={{ marginBottom: 0 }}>
            <label>Stations</label>
            <input
              type="number"
              min={1}
              max={20}
              value={genStations}
              onChange={e => setGenStations(Number(e.target.value))}
              disabled={genStatus.running}
              style={{ width: '80px', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
            />
          </div>
          <div className="form-group" style={{ marginBottom: 0 }}>
            <label>Interval (ms)</label>
            <input
              type="number"
              min={100}
              max={10000}
              value={genInterval}
              onChange={e => setGenInterval(Number(e.target.value))}
              disabled={genStatus.running}
              style={{ width: '100px', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
            />
          </div>
          <div className="form-group" style={{ marginBottom: 0 }}>
            <label>Stations List</label>
            <div style={{ fontSize: '12px', color: 'var(--text2)', padding: '4px 0' }}>
              {genStations >= 1 && '1, '}{genStations >= 2 && '2, '}{genStations >= 3 && '3, '}{genStations >= 4 && '1001, '}{genStations >= 5 && '2001'}{genStations > 5 && `, ... (${genStations - 5} more)`}
            </div>
          </div>
          {genStatus.running ? (
            <button className="btn" onClick={handleStopGenerator} style={{ background: 'var(--red)', borderColor: 'var(--red)', color: 'white' }}>
              Stop Generator
            </button>
          ) : (
            <button className="btn" onClick={handleStartGenerator} disabled={starting} style={{ background: 'var(--green)', borderColor: 'var(--green)', color: 'white' }}>
              {starting ? 'Starting...' : 'Start Generator'}
            </button>
          )}
        </div>
        {genStatus.running && genStatus.config && (
          <div style={{ marginTop: '12px', fontSize: '13px', color: 'var(--text2)' }}>
            Running: {genStatus.config.StationIDs.length} stations, {genStatus.config.IntervalMs}ms interval
          </div>
        )}
      </div>

      {data.total === 0 ? (
        <div className="section">
          <p style={{ color: 'var(--text2)' }}>No active stations. Start the generator or connect RTCM source to port 12101.</p>
        </div>
      ) : (
        <div style={{ overflowX: 'auto' }}>
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Mount</th>
                <th>Frames In</th>
                <th>Frames Out</th>
                <th>Dropped</th>
                <th>Loss %</th>
                <th>Latency</th>
                <th>Bytes</th>
                <th>Last Seen</th>
              </tr>
            </thead>
            <tbody>
              {data.stations.map((s, i) => {
                const q = getQualityForStation(s.station_id)
                return (
                  <tr key={i}>
                    <td><span className="badge green">{s.station_id}</span></td>
                    <td style={{ fontFamily: 'monospace' }}>{s.mount}</td>
                    <td>{q?.frames_in?.toLocaleString() || '-'}</td>
                    <td>{s.frames_out.toLocaleString()}</td>
                    <td style={{ color: (q?.frames_dropped || 0) > 0 ? 'var(--red)' : 'var(--text)' }}>
                      {q?.frames_dropped?.toLocaleString() || '0'}
                    </td>
                    <td style={{ color: (q?.packet_loss_percent || 0) > 1 ? 'var(--red)' : (q?.packet_loss_percent || 0) > 0 ? 'var(--yellow)' : 'var(--green)' }}>
                      {q?.packet_loss_percent?.toFixed(2) || '0.00'}%
                    </td>
                    <td style={{ color: (q?.avg_latency_ms || 0) > 100 ? 'var(--yellow)' : 'var(--green)' }}>
                      {q?.avg_latency_ms?.toFixed(1) || '0.0'} ms
                    </td>
                    <td>{fmtBytes(s.bytes_out)}</td>
                    <td>{timeAgo(s.last_seen)}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
