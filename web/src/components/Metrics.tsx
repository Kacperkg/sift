import type { Application, ApplicationFilter } from '../types/application'
import { statuses } from '../types/application'
import { cx, styles } from '../styles'

type MetricsProps = {
  applications: Application[]
  filter: ApplicationFilter
  onFilter: (filter: ApplicationFilter) => void
}

export function Metrics({ applications, filter, onFilter }: MetricsProps) {
  return <section className={styles.metrics} aria-label="Application overview">
    <div className={cx('metric', 'metric-total')}>
      <span className={styles['metric-index']}>00</span>
      <strong>{String(applications.length).padStart(2, '0')}</strong>
      <span>Total tracked</span>
    </div>
    {statuses.map((status, index) => <button
      className={cx('metric', filter === status && 'is-active')}
      key={status}
      onClick={() => onFilter(filter === status ? 'All' : status)}
    >
      <span className={styles['metric-index']}>0{index + 1}</span>
      <strong>{String(applications.filter(application => application.status === status).length).padStart(2, '0')}</strong>
      <span>{status}</span>
    </button>)}
  </section>
}
