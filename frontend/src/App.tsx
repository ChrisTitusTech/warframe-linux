import {useEffect, useState} from 'react'
import {ListCatalog} from '../wailsjs/go/main/App'
import type {services} from '../wailsjs/go/models'

export default function App() {
  const [items, setItems] = useState<services.Item[]>([])
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let active = true
    ListCatalog().then(result => {
      if (active) { setItems(result); setStatus('ready') }
    }).catch(() => {
      if (active) setStatus('error')
    })
    return () => { active = false }
  }, [attempt])

  return <main>
    <header>
      <p className="eyebrow">Warframe Linux</p>
      <h1>Catalog</h1>
      <p>A starting point for your companion.</p>
    </header>
    <section aria-labelledby="catalog-heading">
      <div className="section-title">
        <h2 id="catalog-heading">Mock catalog</h2>
        <span className="badge">Demo data</span>
      </div>
      <p>These examples are fictional. Live catalog, prices and inventory are not connected.</p>
      {status === 'loading' && <p role="status">Loading examples...</p>}
      {status === 'error' && <div role="alert">
        <p>The examples could not be loaded.</p>
        <button onClick={() => { setStatus('loading'); setAttempt(n => n + 1) }}>Retry</button>
      </div>}
      {status === 'ready' && (items.length === 0
        ? <p role="status">No examples available.</p>
        : <ul aria-label="Catalog examples">{items.map(item => <li key={item.id}>
          <strong>{item.name}</strong><span>{item.category}</span>
        </li>)}</ul>)}
    </section>
    <footer>Offline mock preview. No account required.</footer>
  </main>
}
