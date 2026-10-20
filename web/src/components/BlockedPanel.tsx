import type { BlockedItem, BlockedRole } from '../types/application'
import { styles } from '../styles'

type BlockedPanelProps = {
  blockedRoles: BlockedRole[]
  busy: boolean
  onUnblock: (item: BlockedItem) => void
}

export function BlockedPanel({ blockedRoles, busy, onUnblock }: BlockedPanelProps) {
  if (blockedRoles.length === 0) return null
  return <section className={styles['blocked-panel']}>
    <div>
      <span className={styles['section-kicker']}>Blocked items</span>
      <p>Blocked role titles and hidden emails will stay out of future scans.</p>
    </div>
    <div className={styles['blocked-list']}>
      {blockedRoles.map(item => <button
        key={item.messageId || item.role?.toLowerCase() || item.createdAt}
        onClick={() => onUnblock({ role: item.role, messageId: item.messageId, label: item.label })}
        disabled={busy}
        title="Allow this item again"
      >{item.role || item.label || 'Hidden email'}<span>×</span></button>)}
    </div>
  </section>
}
