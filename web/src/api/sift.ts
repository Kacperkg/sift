import type {
  AIStatus,
  Application,
  BlockedItem,
  BlockedRole,
  InterviewOutcome,
  SyncProgress,
} from '../types/application'

type AuthStatus = { connected: boolean }

export type DashboardData = {
  connected: boolean
  applications: Application[]
  ai?: AIStatus
  blockedRoles?: BlockedRole[]
}

async function readJSON<T>(response: Response, fallback: string): Promise<T> {
  if (!response.ok) {
    const message = (await response.text()).trim()
    throw new Error(message || fallback)
  }
  return response.json() as Promise<T>
}

export async function getDashboardData(): Promise<DashboardData> {
  const [authResponse, applicationsResponse, aiResponse, blocksResponse] = await Promise.all([
    fetch('/api/auth/status'),
    fetch('/api/applications'),
    fetch('/api/ai/status'),
    fetch('/api/blocked-roles'),
  ])
  if (!authResponse.ok || !applicationsResponse.ok) {
    throw new Error('Could not reach the local Job Inbox service.')
  }
  const [auth, applications] = await Promise.all([
    authResponse.json() as Promise<AuthStatus>,
    applicationsResponse.json() as Promise<Application[]>,
  ])
  return {
    connected: Boolean(auth.connected),
    applications: Array.isArray(applications) ? applications : [],
    ai: aiResponse.ok ? await aiResponse.json() as AIStatus : undefined,
    blockedRoles: blocksResponse.ok ? await blocksResponse.json() as BlockedRole[] : undefined,
  }
}

export async function syncApplications(mode: 'update' | 'full') {
  const response = await fetch(`/api/sync?mode=${mode}`, { method: 'POST' })
  return readJSON<Application[]>(response, 'Sync failed.')
}

export async function getSyncProgress() {
  const response = await fetch('/api/sync/status')
  return readJSON<SyncProgress>(response, 'Could not read scan progress.')
}

export async function setAIEnabled(enabled: boolean) {
  const response = await fetch('/api/ai/status', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  })
  return readJSON<AIStatus>(response, 'Could not update local AI.')
}

export async function setBlockedItem(item: BlockedItem, blocked: boolean) {
  const response = await fetch('/api/blocked-roles', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...item, blocked }),
  })
  return readJSON<BlockedRole[]>(response, 'Could not update blocked roles.')
}

export async function setInterviewOutcome(applicationID: string, status: InterviewOutcome) {
  const response = await fetch('/api/applications/status', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: applicationID, status }),
  })
  return readJSON<Application>(response, 'Could not close this interview.')
}
