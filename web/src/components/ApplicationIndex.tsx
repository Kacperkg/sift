import type { Application, ApplicationFilter } from '../types/application'
import { filters } from '../types/application'
import { cx, styles } from '../styles'
import { formatDate, initials } from '../utils/application'
import { ArrowIcon, SearchIcon } from './Icons'
import { StatusPill } from './StatusPill'

type ApplicationIndexProps = {
  applications: Application[]
  hasApplications: boolean
  connected: boolean
  filter: ApplicationFilter
  query: string
  onFilter: (filter: ApplicationFilter) => void
  onQuery: (query: string) => void
  onOpen: (applicationID: string) => void
}

export function ApplicationIndex({
  applications,
  hasApplications,
  connected,
  filter,
  query,
  onFilter,
  onQuery,
  onOpen,
}: ApplicationIndexProps) {
  return <section className={styles.workspace}>
    <div className={styles['workspace-heading']}>
      <div><span className={styles['section-kicker']}>Application index</span><h2>Every opportunity,<br />in one place.</h2></div>
      <div className={styles['workspace-tools']}>
        <label className={styles.search}>
          <SearchIcon />
          <span className={styles['sr-only']}>Search applications</span>
          <input value={query} onChange={event => onQuery(event.target.value)} placeholder="Search company, role, email…" />
        </label>
        <div className={styles.filters} aria-label="Filter by status">
          {filters.map(item => <button key={item} className={filter === item ? styles.active : ''} onClick={() => onFilter(item)}>{item}</button>)}
        </div>
      </div>
    </div>

    {!hasApplications
      ? <div className={styles.empty}>
          <span className={styles['empty-star']}>✦</span><h3>Your index is ready.</h3>
          <p>{connected ? 'Scan Gmail to find your first application.' : 'Connect Gmail, then run your first private scan.'}</p>
        </div>
      : applications.length === 0
        ? <div className={cx('empty', 'compact')}>
            <h3>No matches found.</h3>
            <button className={styles['text-button']} onClick={() => { onQuery(''); onFilter('All') }}>Clear filters</button>
          </div>
        : <div className={styles['application-list']}>
            <div className={styles['list-labels']} aria-hidden="true">
              <span>Company / role</span><span>Latest email</span><span>Status</span><span>Updated</span><span />
            </div>
            {applications.map((application, index) => <button
              className={styles['application-row']}
              key={application.id}
              onClick={() => onOpen(application.id)}
              aria-label={`Open ${application.role} at ${application.company}`}
            >
              <span className={styles['company-cell']}>
                <span className={styles['company-avatar']}>{initials(application.company)}</span>
                <span><strong>{application.company}</strong><small>{application.role}</small></span>
              </span>
              <span className={styles['event-cell']}>{application.lastEvent || 'Application email'}</span>
              <span><StatusPill status={application.status} /></span>
              <time dateTime={application.updatedAt}>{formatDate(application.updatedAt)}</time>
              <span className={styles['row-arrow']}><ArrowIcon /></span>
              <span className={styles['row-number']}>{String(index + 1).padStart(2, '0')}</span>
            </button>)}
          </div>}
  </section>
}
