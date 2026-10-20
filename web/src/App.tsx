import { useApplicationRoute } from './hooks/useApplicationRoute'
import { useSift } from './hooks/useSift'
import { ApplicationDetailPage } from './pages/ApplicationDetailPage'
import { DashboardPage } from './pages/DashboardPage'
import type { Application } from './types/application'

export function App() {
  const sift = useSift()
  const route = useApplicationRoute()
  const selectedApplication = sift.applications.find(application => application.id === route.selectedID)

  const blockApplication = async (application: Application) => {
    const blocked = await sift.updateBlockedItem(
      application.role === 'Role not detected'
        ? { messageId: application.id, label: application.lastEvent || application.company }
        : { role: application.role },
      true,
    )
    if (blocked) route.closeApplication()
  }

  if (route.selectedID) {
    return <ApplicationDetailPage
      application={selectedApplication}
      blockBusy={sift.blockBusy}
      outcomeBusy={sift.outcomeBusy}
      error={sift.error}
      onBack={route.closeApplication}
      onBlock={blockApplication}
      onCloseInterview={sift.closeInterview}
    />
  }

  return <DashboardPage
    connected={sift.connected}
    applications={sift.applications}
    ai={sift.ai}
    blockedRoles={sift.blockedRoles}
    scanProgress={sift.scanProgress}
    syncing={sift.syncing}
    aiBusy={sift.aiBusy}
    blockBusy={sift.blockBusy}
    error={sift.error}
    onSync={sift.sync}
    onToggleAI={sift.toggleAI}
    onOpen={route.openApplication}
    onUnblock={item => { void sift.updateBlockedItem(item, false) }}
  />
}
