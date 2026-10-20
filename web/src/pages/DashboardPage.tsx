import { useMemo, useState } from 'react'
import { ApplicationIndex } from '../components/ApplicationIndex'
import { BlockedPanel } from '../components/BlockedPanel'
import { Hero } from '../components/Hero'
import { Metrics } from '../components/Metrics'
import { DashboardNavigation } from '../components/Navigation'
import { PrivacyFooter } from '../components/PrivacyFooter'
import { ScanToast } from '../components/ScanToast'
import type {
  AIStatus,
  Application,
  ApplicationFilter,
  BlockedItem,
  BlockedRole,
  SyncProgress,
} from '../types/application'
import { styles } from '../styles'
import { matchesApplication } from '../utils/application'

type DashboardPageProps = {
  connected: boolean
  applications: Application[]
  ai: AIStatus
  blockedRoles: BlockedRole[]
  scanProgress: SyncProgress | null
  syncing: boolean
  aiBusy: boolean
  blockBusy: boolean
  error: string
  onSync: (mode: 'update' | 'full') => void
  onToggleAI: () => void
  onOpen: (applicationID: string) => void
  onUnblock: (item: BlockedItem) => void
}

export function DashboardPage({
  connected,
  applications,
  ai,
  blockedRoles,
  scanProgress,
  syncing,
  aiBusy,
  blockBusy,
  error,
  onSync,
  onToggleAI,
  onOpen,
  onUnblock,
}: DashboardPageProps) {
  const [filter, setFilter] = useState<ApplicationFilter>('All')
  const [query, setQuery] = useState('')
  const visibleApplications = useMemo(() => applications.filter(application => (
    (filter === 'All' || application.status === filter) && matchesApplication(application, query)
  )), [applications, filter, query])

  return <main className={styles.shell}>
    <DashboardNavigation />
    <Hero
      connected={connected}
      syncing={syncing}
      ai={ai}
      aiBusy={aiBusy}
      onSync={onSync}
      onToggleAI={onToggleAI}
    />
    <Metrics applications={applications} filter={filter} onFilter={setFilter} />
    {error && <div className={styles.error} role="alert"><span>!</span>{error}</div>}
    <ApplicationIndex
      applications={visibleApplications}
      hasApplications={applications.length > 0}
      connected={connected}
      filter={filter}
      query={query}
      onFilter={setFilter}
      onQuery={setQuery}
      onOpen={onOpen}
    />
    <BlockedPanel blockedRoles={blockedRoles} busy={blockBusy} onUnblock={onUnblock} />
    <ScanToast progress={scanProgress} />
    <PrivacyFooter />
  </main>
}
