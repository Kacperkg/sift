import { ArrowIcon } from './Icons'
import { cx, styles } from '../styles'

export function DashboardNavigation() {
  return <nav className={styles.topbar} aria-label="Primary navigation">
    <a className={styles.brand} href="#top" aria-label="Job Inbox home">
      <span className={styles['brand-mark']}>JI</span><span>Job Inbox</span>
    </a>
    <div className={styles['privacy-state']}><span className={styles['live-dot']} /> Local vault · Gmail read-only</div>
  </nav>
}

export function DetailNavigation({ onBack }: { onBack: () => void }) {
  return <nav className={cx('topbar', 'detail-nav')}>
    <button className={styles['back-button']} onClick={onBack}><ArrowIcon direction="left" /> Back to index</button>
    <div className={styles['privacy-state']}><span className={styles['live-dot']} /> Local vault</div>
  </nav>
}
