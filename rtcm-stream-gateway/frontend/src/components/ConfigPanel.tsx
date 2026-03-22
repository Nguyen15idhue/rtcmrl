import { useState, useEffect, useCallback } from 'react'
import { api, Config, WorkerInfo } from '../api'

export default function ConfigPanel() {
  const [config, setConfig] = useState<Config | null>(null)
  const [workers, setWorkers] = useState<WorkerInfo | null>(null)
  const [msg, setMsg] = useState<{ type: 'ok' | 'err'; text: string } | null>(null)
  const [autoScale, setAutoScale] = useState(false)
  const [desiredWorkers, setDesiredWorkers] = useState(0)
  const [saving, setSaving] = useState(false)

  // Form state
  const [casterHost, setCasterHost] = useState('')
  const [casterPort, setCasterPort] = useState('')
  const [casterPrefix, setCasterPrefix] = useState('')
  const [casterPass, setCasterPass] = useState('')
  const [casterUser, setCasterUser] = useState('')
  const [ntripVersion, setNtripVersion] = useState(2)
  const [captureDevice, setCaptureDevice] = useState('')
  const [capturePort, setCapturePort] = useState('')

  const load = useCallback(async () => {
    try {
      const [c, w] = await Promise.all([
        api.get<Config>('/api/v1/config'),
        api.get<WorkerInfo>('/api/v1/workers'),
      ])
      setConfig(c)
      setWorkers(w)
      setAutoScale(c.worker.auto_scale)
      setDesiredWorkers(w.desired)
      
      // Set form values
      const caster = c.caster as Record<string, unknown>
      const capture = c.capture as Record<string, unknown>
      setCasterHost(String(caster.host || caster.Host || ''))
      setCasterPort(String(caster.port || caster.Port || '1509'))
      setCasterPrefix(String(caster.mount_prefix || caster.MountPrefix || 'STN'))
      setCaptureDevice(String(capture.device || capture.Device || 'any'))
      setCapturePort(String(capture.listen_port || capture.ListenPort || '12101'))
      setNtripVersion(Number(caster.ntrip_version || caster.NtripVersion || 1))
      setCasterUser(String(caster.user || caster.User || ''))
      setCasterPass(String(caster.pass || caster.Pass || ''))
    } catch (e) {
      console.error('Load error:', e)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const handleAutoScale = async () => {
    try {
      await api.post('/api/v1/workers/auto-scale', { enabled: !autoScale })
      setAutoScale(!autoScale)
      setMsg({ type: 'ok', text: `Auto-scale ${!autoScale ? 'enabled' : 'disabled'}` })
      load()
    } catch {
      setMsg({ type: 'err', text: 'Failed to update' })
    }
    setTimeout(() => setMsg(null), 3000)
  }

  const handleSetWorkers = async () => {
    try {
      await api.post('/api/v1/workers', { count: desiredWorkers })
      setMsg({ type: 'ok', text: `Workers set to ${desiredWorkers}` })
      load()
    } catch {
      setMsg({ type: 'err', text: 'Failed to set workers' })
    }
    setTimeout(() => setMsg(null), 3000)
  }

  const handleSaveConfig = async () => {
    setSaving(true)
    setMsg(null)
    try {
      await api.post('/api/v1/config', {
        caster: {
          host: casterHost,
          port: parseInt(casterPort) || 2101,
          mount_prefix: casterPrefix,
          pass: casterPass || undefined,
          user: casterUser || undefined,
          ntrip_version: ntripVersion,
        },
        capture: {
          device: captureDevice,
          listen_port: parseInt(capturePort) || 12101,
        },
      })
      setMsg({ type: 'ok', text: 'Config saved! Restart to apply changes.' })
      load()
    } catch (e) {
      console.error('Save error:', e)
      setMsg({ type: 'err', text: 'Failed to save config: ' + String(e) })
    }
    setSaving(false)
    setTimeout(() => setMsg(null), 5000)
  }

  const handleRestart = async () => {
    if (!confirm('Restart gateway now?')) return
    try {
      await api.post('/api/v1/restart', {})
      setMsg({ type: 'ok', text: 'Restarting...' })
    } catch {
      setMsg({ type: 'err', text: 'Restart endpoint failed. Restart container manually.' })
    }
    setTimeout(() => setMsg(null), 5000)
  }

  if (!config || !workers) return <div className="loading">Loading config...</div>

  return (
    <div>
      <h2>Configuration</h2>

      {msg && (
        <div style={{ 
          padding: '12px 16px', 
          borderRadius: '8px', 
          marginBottom: '16px',
          background: msg.type === 'ok' ? 'rgba(34,197,94,0.15)' : 'rgba(239,68,68,0.15)',
          border: `1px solid ${msg.type === 'ok' ? '#22c55e' : '#ef4444'}`,
          color: msg.type === 'ok' ? '#22c55e' : '#ef4444',
        }}>
          {msg.text}
        </div>
      )}

      <div className="section">
        <h2 style={{ fontSize: '16px', marginBottom: '12px' }}>Worker Management</h2>
        <div className="cards" style={{ marginBottom: '16px' }}>
          <div className="card">
            <div className="card-label">Active Workers</div>
            <div className="card-value blue">{workers.active}</div>
          </div>
          <div className="card">
            <div className="card-label">Desired Workers</div>
            <div className="card-value">{workers.desired}</div>
          </div>
          <div className="card">
            <div className="card-label">Min / Max</div>
            <div className="card-value">{workers.min} / {workers.max}</div>
          </div>
          <div className="card">
            <div className="card-label">Auto-scale</div>
            <div className="card-value" style={{ fontSize: '20px' }}>{workers.auto ? 'ON' : 'OFF'}</div>
          </div>
        </div>

        <div className="row">
          <label className="toggle">
            <input type="checkbox" checked={autoScale} onChange={handleAutoScale} />
            <span className="toggle-slider"></span>
          </label>
          <span>Auto-scale (scales {workers.min}-{workers.max} based on load)</span>
        </div>

        <div className="row mt-4">
          <input
            type="number"
            min={workers.min}
            max={workers.max}
            value={desiredWorkers}
            onChange={e => setDesiredWorkers(Number(e.target.value))}
            style={{ width: '80px', padding: '6px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
          />
          <button className="btn" onClick={handleSetWorkers}>Set Workers</button>
        </div>
      </div>

      <div className="section">
        <h2 style={{ fontSize: '16px', marginBottom: '12px' }}>Capture Settings</h2>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '16px' }}>
          <div className="form-group">
            <label>Device</label>
            <input 
              type="text" 
              value={captureDevice}
              onChange={e => setCaptureDevice(e.target.value)}
              style={{ width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
            />
          </div>
          <div className="form-group">
            <label>Listen Port</label>
            <input 
              type="number" 
              value={capturePort}
              onChange={e => setCapturePort(e.target.value)}
              style={{ width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
            />
          </div>
        </div>
      </div>

      <div className="section">
        <h2 style={{ fontSize: '16px', marginBottom: '12px' }}>Caster Settings</h2>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '16px' }}>
          <div className="form-group">
            <label>Caster Host</label>
            <input 
              type="text" 
              value={casterHost}
              onChange={e => setCasterHost(e.target.value)}
              style={{ width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
            />
          </div>
          <div className="form-group">
            <label>Caster Port</label>
            <input 
              type="number" 
              value={casterPort}
              onChange={e => setCasterPort(e.target.value)}
              style={{ width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
            />
          </div>
          <div className="form-group">
            <label>Mount Prefix</label>
            <input 
              type="text" 
              value={casterPrefix}
              onChange={e => setCasterPrefix(e.target.value)}
              style={{ width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
            />
          </div>
          <div className="form-group">
            <label>NTRIP Version</label>
            <select 
              value={ntripVersion}
              onChange={e => setNtripVersion(Number(e.target.value))}
              style={{ width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
            >
              <option value={1}>NTRIP v1</option>
              <option value={2}>NTRIP v2</option>
            </select>
          </div>
          <div className="form-group">
            <label>Username (for NTRIP v2)</label>
            <input 
              type="text" 
              value={casterUser}
              onChange={e => setCasterUser(e.target.value)}
              placeholder="Enter username"
              style={{ width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
            />
          </div>
          <div className="form-group">
            <label>Password</label>
            <input 
              type="password" 
              value={casterPass}
              onChange={e => setCasterPass(e.target.value)}
              placeholder="Enter to change"
              style={{ width: '100%', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
            />
          </div>
        </div>
      </div>

      <div style={{ display: 'flex', gap: '12px', marginTop: '16px' }}>
        <button 
          className="btn" 
          onClick={handleSaveConfig}
          disabled={saving}
          style={{ background: 'var(--accent)', borderColor: 'var(--accent)', color: 'white', padding: '10px 20px' }}
        >
          {saving ? 'Saving...' : 'Save Config'}
        </button>
        <button 
          className="btn" 
          onClick={handleRestart}
          style={{ background: '#eab308', borderColor: '#eab308', color: '#000', padding: '10px 20px' }}
        >
          Restart Gateway
        </button>
      </div>
    </div>
  )
}
