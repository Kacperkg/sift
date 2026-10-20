import type { AIStatus } from '../types/application'
import { cx, styles } from '../styles'
import { ArrowIcon, RefreshIcon } from './Icons'

type HeroProps = {
  connected: boolean
  syncing: boolean
  ai: AIStatus
  aiBusy: boolean
  onSync: (mode: 'update' | 'full') => void
  onToggleAI: () => void
}

function aiDescription(ai: AIStatus) {
  if (!ai.available) return 'Ollama offline'
  if (!ai.modelInstalled) return `${ai.model} not installed`
  if (ai.enabled) return `${ai.model} · ambiguous emails only`
  return `${ai.model} · ready`
}

export function Hero({ connected, syncing, ai, aiBusy, onSync, onToggleAI }: HeroProps) {
  return <header className={styles.hero} id="top">
    <div>
      <span className={styles.eyebrow}>PRIVATE CAREER INTELLIGENCE / 01</span>
      <h1>Your job search,<br /><em>out of the inbox.</em></h1>
    </div>
    <div className={styles['hero-aside']}>
      <p>A calm, private view of every application signal found in your Gmail.</p>
      {connected
        ? <div className={styles['scan-actions']}>
            <button className={styles['primary-action']} onClick={() => onSync('update')} disabled={syncing}>
              <RefreshIcon /> {syncing ? 'Reading Gmail…' : 'Update inbox'}
            </button>
            <button className={styles['secondary-action']} onClick={() => onSync('full')} disabled={syncing}>Full rescan</button>
          </div>
        : <a className={styles['primary-action']} href="/auth/google">Connect Gmail <ArrowIcon /></a>}
      <div className={styles['ai-control']}>
        <div>
          <span className={cx('ai-dot', ai.enabled && 'on')} />
          <span><strong>Local AI assist</strong><small>{aiDescription(ai)}</small></span>
        </div>
        <button
          className={ai.enabled ? styles.enabled : undefined}
          onClick={onToggleAI}
          disabled={aiBusy || !ai.available || !ai.modelInstalled}
          aria-pressed={ai.enabled}
        >{aiBusy ? '…' : ai.enabled ? 'On' : 'Off'}</button>
      </div>
    </div>
  </header>
}
