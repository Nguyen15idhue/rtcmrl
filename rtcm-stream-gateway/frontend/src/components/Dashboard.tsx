import { useState, useEffect, useCallback } from 'react'
import { api, Stats, Health, WorkerInfo } from '../api'

export default function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [health, setHealth] = useState<Health | null>(null)
  const [workers, setWorkers] = useState<WorkerInfo | null>(null)

  const load = useCallback(async () => {
    try {
      const [s, h, w] = await Promise.all([
        api.get<Stats>('/api/v1/stats'),
        api.get<Health>('/api/v1/health'),
        api.get<WorkerInfo>('/api/v1/workers'),
      ])
      setStats(s)
      setHealth(h)
      setWorkers(w)
    } catch (e) {
      console.error(e)
    }
  }, [])

  useEffect(() => {
    load()
    const id = setInterval(load, 3000)
    return () => clearInterval(id)
  }, [load])

  if (!stats || !health) return <div className="loading">Loading...</div>

  const queuePct = (stats.queue_depth / 4096) * 100

  return (
    <div>
      <div className="cards">
        <div className="card">
          <div className="card-label">Status</div>
          <div className={`card-value ${health.status === 'healthy' ? 'green' : 'yellow'}`}>
            {health.status}
          </div>
        </div>
        <div className="card">
          <div className="card-label">Active Stations</div>
          <div className="card-value green">{stats.stations}</div>
        </div>
        <div className="card">
          <div className="card-label">Workers (active/desired)</div>
          <div className="card-value blue">{stats.workers_active}/{stats.workers_desired}</div>
        </div>
        <div className="card">
          <div className="card-label">Frames Forwarded</div>
          <div className="card-value green">{stats.forwarded.toLocaleString()}</div>
        </div>
        <div className="card">
          <div className="card-label">Frames Dropped</div>
          <div className={`card-value ${stats.drops > 0 ? 'red' : ''}`}>{stats.drops.toLocaleString()}</div>
        </div>
        <div className="card">
          <div className="card-label">Queue Depth</div>
          <div className={`card-value ${queuePct > 80 ? 'red' : queuePct > 50 ? 'yellow' : ''}`}>
            {stats.queue_depth} ({queuePct.toFixed(0)}%)
          </div>
        </div>
        <div className="card">
          <div className="card-label">Sources</div>
          <div className="card-value">{stats.sources}</div>
        </div>
        <div className="card">
          <div className="card-label">Uptime</div>
          <div className="card-value" style={{ fontSize: '20px' }}>{stats.uptime}</div>
        </div>
      </div>

      <div className="section">
        <h2>Runtime Info</h2>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '16px' }}>
          <div>
            <div className="card-label">Goroutines</div>
            <div className="card-value" style={{ fontSize: '20px' }}>{stats.goroutines}</div>
          </div>
          <div>
            <div className="card-label">Memory (MB)</div>
            <div className="card-value" style={{ fontSize: '20px' }}>{stats.mem_alloc_mb}</div>
          </div>
          <div>
            <div className="card-label">Auto-scale</div>
            <div className="card-value" style={{ fontSize: '20px' }}>{workers?.auto ? 'ON' : 'OFF'}</div>
          </div>
          <div>
            <div className="card-label">Worker Range</div>
            <div className="card-value" style={{ fontSize: '20px' }}>{workers?.min} - {workers?.max}</div>
          </div>
        </div>
      </div>

      <div className="section">
        <h2>Frame Counters</h2>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '16px' }}>
          <div>
            <div className="card-label">Unknown (no StationID)</div>
            <div className="card-value">{stats.unknown.toLocaleString()}</div>
          </div>
          <div>
            <div className="card-label">Ambiguous</div>
            <div className="card-value">{stats.ambiguous.toLocaleString()}</div>
          </div>
          <div>
            <div className="card-label">Total Drops</div>
            <div className={`card-value ${stats.drops > 0 ? 'red' : ''}`}>{stats.drops.toLocaleString()}</div>
          </div>
        </div>
      </div>
    </div>
  )
}
