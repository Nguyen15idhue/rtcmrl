import { useState, useEffect, useCallback } from 'react'
import { api, ModeInfo, NetworkInfo, CaptureTestResult } from '../api'

export default function Network() {
  const [modeInfo, setModeInfo] = useState<ModeInfo | null>(null)
  const [networkInfo, setNetworkInfo] = useState<NetworkInfo | null>(null)
  const [testResult, setTestResult] = useState<CaptureTestResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [selectedMode, setSelectedMode] = useState('auto')
  const [device, setDevice] = useState('')
  const [saved, setSaved] = useState(false)

  const load = useCallback(async () => {
    try {
      const [m, n] = await Promise.all([
        api.get<ModeInfo>('/api/v1/mode'),
        api.get<NetworkInfo>('/api/v1/network'),
      ])
      setModeInfo(m)
      setNetworkInfo(n)
      setSelectedMode(m.mode || 'auto')
      setDevice(m.device || '')
    } catch (e) {
      console.error(e)
    }
  }, [])

  useEffect(() => {
    load()
    const id = setInterval(load, 5000)
    return () => clearInterval(id)
  }, [load])

  const handleTest = async () => {
    setLoading(true)
    try {
      const result = await api.get<CaptureTestResult>('/api/v1/mode/test')
      setTestResult(result)
    } catch (e) {
      console.error(e)
    }
    setLoading(false)
  }

  const handleSave = async () => {
    setLoading(true)
    try {
      await api.post('/api/v1/mode', { mode: selectedMode, device })
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
      load()
    } catch (e) {
      console.error(e)
    }
    setLoading(false)
  }

  return (
    <div>
      <h2>Network Configuration</h2>

      <div className="section">
        <h3>System Info</h3>
        {networkInfo && (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '12px' }}>
            <div className="info-card">
              <span className="label">Hostname</span>
              <span className="value">{networkInfo.hostname}</span>
            </div>
            <div className="info-card">
              <span className="label">Platform</span>
              <span className="value">{networkInfo.platform}</span>
            </div>
            <div className="info-card">
              <span className="label">Architecture</span>
              <span className="value">{networkInfo.arch}</span>
            </div>
            <div className="info-card">
              <span className="label">Go Version</span>
              <span className="value">{networkInfo.go_version}</span>
            </div>
          </div>
        )}
      </div>

      <div className="section">
        <h3>Capture Mode</h3>
        <p style={{ color: 'var(--text2)', marginBottom: '16px' }}>
          Select how gateway captures RTCM data from network
        </p>

        <div className="form-group">
          <label>Mode</label>
          <select 
            value={selectedMode} 
            onChange={e => setSelectedMode(e.target.value)}
            style={{ width: '200px', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
          >
            <option value="auto">Auto (Recommended)</option>
            <option value="tcp">TCP (Listen on port)</option>
            <option value="pcap">PCAP (Sniff traffic)</option>
            <option value="sniff">Sniff (Same as PCAP)</option>
          </select>
        </div>

        <div className="form-group" style={{ marginTop: '16px' }}>
          <label>Network Device (for PCAP mode)</label>
          <input 
            type="text" 
            value={device}
            onChange={e => setDevice(e.target.value)}
            placeholder="any, eth0, \\Device\NPF_Loopback, ..."
            style={{ width: '400px', padding: '8px', borderRadius: '6px', border: '1px solid var(--border)', background: 'var(--bg3)', color: 'var(--text)' }}
          />
          <div style={{ fontSize: '12px', color: 'var(--text2)', marginTop: '4px' }}>
            Windows: <code>\Device\NPF_Loopback</code> for localhost, <code>\Device\NPF_&#123;GUID&#125;</code> for network interface
          </div>
        </div>

        <div style={{ display: 'flex', gap: '12px', marginTop: '16px', alignItems: 'center' }}>
          <button 
            className="btn" 
            onClick={handleSave}
            disabled={loading}
            style={{ background: 'var(--green)', borderColor: 'var(--green)', color: 'white' }}
          >
            {loading ? 'Saving...' : 'Save Mode'}
          </button>
          <button 
            className="btn" 
            onClick={handleTest}
            disabled={loading}
          >
            {loading ? 'Testing...' : 'Test Capture'}
          </button>
          {saved && <span style={{ color: 'var(--green)' }}>Saved! Restart gateway to apply.</span>}
        </div>
      </div>

      <div className="section">
        <h3>Current Status</h3>
        {modeInfo && (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '12px' }}>
            <div className="info-card">
              <span className="label">Current Mode</span>
              <span className="value" style={{ color: 'var(--green)' }}>{modeInfo.mode || 'unknown'}</span>
            </div>
            <div className="info-card">
              <span className="label">Device</span>
              <span className="value" style={{ fontSize: '12px', wordBreak: 'break-all' }}>{modeInfo.device || 'any'}</span>
            </div>
            <div className="info-card">
              <span className="label">Listen Port</span>
              <span className="value">{modeInfo.port || 12101}</span>
            </div>
          </div>
        )}
      </div>

      {testResult && (
        <div className="section">
          <h3>Test Results</h3>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '12px' }}>
            <div className="info-card">
              <span className="label">Port Listening</span>
              <span className="value" style={{ color: testResult.port_listening ? 'var(--red)' : 'var(--green)' }}>
                {testResult.port_listening ? 'Yes (TCP mode)' : 'No (PCAP mode)'}
              </span>
            </div>
            <div className="info-card">
              <span className="label">Mode Used</span>
              <span className="value">{testResult.mode}</span>
            </div>
            <div className="info-card">
              <span className="label">Device</span>
              <span className="value" style={{ fontSize: '12px', wordBreak: 'break-all' }}>{testResult.device || 'any'}</span>
            </div>
          </div>
        </div>
      )}

      <div className="section">
        <h3>Mode Explanation</h3>
        <table style={{ width: '100%' }}>
          <thead>
            <tr>
              <th>Mode</th>
              <th>Port Binding</th>
              <th>Requires</th>
              <th>Best For</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td><strong>TCP</strong></td>
              <td style={{ color: 'var(--red)' }}>Yes (listen)</td>
              <td>Nothing</td>
              <td>When receiver can send to different port</td>
            </tr>
            <tr>
              <td><strong>PCAP/Sniff</strong></td>
              <td style={{ color: 'var(--green)' }}>No (sniff)</td>
              <td>libpcap/Npcap</td>
              <td>When caster already running on port</td>
            </tr>
            <tr>
              <td><strong>Auto</strong></td>
              <td>Auto-detect</td>
              <td>libpcap/Npcap</td>
              <td>Recommended for most cases</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  )
}
