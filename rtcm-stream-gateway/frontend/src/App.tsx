import { useState } from 'react'
import Dashboard from './components/Dashboard'
import Stations from './components/Stations'
import ConfigPanel from './components/ConfigPanel'

type Tab = 'dashboard' | 'stations' | 'config'

export default function App() {
  const [tab, setTab] = useState<Tab>('dashboard')

  return (
    <>
      <nav>
        <h1>RTCM Gateway v2.0</h1>
        <a className={tab === 'dashboard' ? 'active' : ''} onClick={() => setTab('dashboard')}>Dashboard</a>
        <a className={tab === 'stations' ? 'active' : ''} onClick={() => setTab('stations')}>Stations</a>
        <a className={tab === 'config' ? 'active' : ''} onClick={() => setTab('config')}>Config</a>
        <a href="/metrics" target="_blank" rel="noreferrer" style={{ marginLeft: 'auto' }}>Metrics</a>
        <a href="/health" target="_blank" rel="noreferrer">Health</a>
      </nav>
      <main>
        {tab === 'dashboard' && <Dashboard />}
        {tab === 'stations' && <Stations />}
        {tab === 'config' && <ConfigPanel />}
      </main>
    </>
  )
}
