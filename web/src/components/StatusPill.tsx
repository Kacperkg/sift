import { cx } from '../styles'
import { statusClass } from '../utils/application'

export function StatusPill({ status, detail = false }: { status: string; detail?: boolean }) {
  return <i className={cx('pill', detail && 'detail-status', statusClass(status))}><span />{status}</i>
}
