import type { SyncProgress } from '../types/application'
import { cx, styles } from '../styles'

export function ScanToast({ progress }: { progress: SyncProgress | null }) {
  if (!progress) return null
  const percentage = progress.total > 0
    ? Math.min(100, Math.round((progress.processed / progress.total) * 100))
    : 0
  return <div
    className={cx('scan-toast', progress.running ? 'running' : 'complete', progress.total === 0 && 'indeterminate')}
    role="status"
    aria-live="polite"
  >
    <div className={styles['scan-toast-heading']}>
      <span className={styles['scan-indicator']} />
      <div>
        <strong>{progress.phase}</strong>
        <small>{progress.total > 0 ? `${progress.processed} of ${progress.total} emails` : 'Preparing your private inbox scan'}</small>
      </div>
      <b>{progress.total > 0 ? `${percentage}%` : '•••'}</b>
    </div>
    <div className={styles['scan-track']}><span style={{ width: progress.total > 0 ? `${percentage}%` : '18%' }} /></div>
  </div>
}
