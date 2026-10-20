import { ArrowIcon, LockIcon } from '../components/Icons'
import { DetailNavigation } from '../components/Navigation'
import { StatusPill } from '../components/StatusPill'
import { cx, styles } from '../styles'
import type { Application, InterviewOutcome } from '../types/application'
import { formatDate, gmailMessageURL, initials } from '../utils/application'

type ApplicationDetailPageProps = {
  application?: Application
  blockBusy: boolean
  outcomeBusy: boolean
  error: string
  onBack: () => void
  onBlock: (application: Application) => void
  onCloseInterview: (application: Application, status: InterviewOutcome) => void
}

export function ApplicationDetailPage({
  application,
  blockBusy,
  outcomeBusy,
  error,
  onBack,
  onBlock,
  onCloseInterview,
}: ApplicationDetailPageProps) {
  if (!application) {
    return <main className={cx('detail-shell', 'missing')}>
      <button className={styles['back-button']} onClick={onBack}><ArrowIcon direction="left" /> Back to index</button>
      <h1>Application not found.</h1>
      <p>It may have been removed by a more recent Gmail scan.</p>
    </main>
  }

  return <main className={styles['detail-shell']}>
    <DetailNavigation onBack={onBack} />
    <section className={styles['detail-hero']}>
      <div className={styles['detail-avatar']}>{initials(application.company)}</div>
      <div className={styles['detail-title']}>
        <span className={styles.eyebrow}>APPLICATION RECORD / {application.id.slice(-6).toUpperCase()}</span>
        <h1>{application.company}</h1>
        <p>{application.role}</p>
      </div>
      <StatusPill status={application.status} detail />
    </section>
    <section className={styles['detail-grid']}>
      <article className={styles['email-card']}>
        <span className={styles['section-kicker']}>Latest matched email</span>
        <h2>{application.lastEvent || 'Application update'}</h2>
        <div className={styles['email-meta']}>
          <span>Gmail message</span>
          <time dateTime={application.updatedAt}>{formatDate(application.updatedAt, true)}</time>
        </div>
        <a className={styles['gmail-button']} href={gmailMessageURL(application.id)} target="_blank" rel="noreferrer">
          Open original in Gmail <ArrowIcon />
        </a>
        <p className={styles['link-note']}>Opens the exact message in your signed-in Gmail account.</p>
      </article>
      <aside className={styles['record-card']}>
        <span className={styles['section-kicker']}>Record details</span>
        <dl>
          <div><dt>Company</dt><dd>{application.company}</dd></div>
          <div><dt>Role</dt><dd>{application.role}</dd></div>
          <div><dt>Status</dt><dd>{application.status}</dd></div>
          <div><dt>Last activity</dt><dd>{formatDate(application.updatedAt, true)}</dd></div>
        </dl>
        {application.status === 'Interviewing' && !application.closed && <div className={styles['outcome-action']}>
          <span className={styles['section-kicker']}>Close interview</span>
          <div>
            <button className={styles.accept} onClick={() => onCloseInterview(application, 'Accepted')} disabled={outcomeBusy}>{outcomeBusy ? 'Saving…' : 'Accepted'}</button>
            <button className={styles.decline} onClick={() => onCloseInterview(application, 'Declined')} disabled={outcomeBusy}>{outcomeBusy ? 'Saving…' : 'Declined'}</button>
          </div>
          <p>Sets a private manual outcome. Nothing is changed in Gmail.</p>
          {error && <small role="alert">{error}</small>}
        </div>}
        <div className={styles['security-note']}>
          <LockIcon />
          <p><strong>Stored privately</strong><br />This record is encrypted on your Mac. Job Inbox only has read-only Gmail access.</p>
        </div>
        <div className={styles['block-action']}>
          <button onClick={() => onBlock(application)} disabled={blockBusy}>
            {blockBusy ? 'Removing…' : application.role === 'Role not detected' ? 'Not my application · Hide email' : 'Not my application · Block role'}
          </button>
          <p>{application.role === 'Role not detected'
            ? 'Hides this Gmail message permanently from future scans.'
            : `Removes every exact “${application.role}” match and prevents it returning after a scan.`}</p>
        </div>
      </aside>
    </section>
  </main>
}
