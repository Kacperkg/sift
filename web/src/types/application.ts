export const statuses = ['Applied', 'No response', 'Interviewing', 'Offer', 'Accepted', 'Declined'] as const
export const filters = ['All', ...statuses] as const

export type ApplicationStatus = (typeof statuses)[number]
export type ApplicationFilter = (typeof filters)[number]
export type InterviewOutcome = 'Accepted' | 'Declined'

export type Application = {
  id: string
  company: string
  role: string
  status: ApplicationStatus
  closed?: boolean
  lastEvent: string
  updatedAt: string
}

export type AIStatus = {
  enabled: boolean
  available: boolean
  modelInstalled: boolean
  model: string
}

export type BlockedRole = {
  role?: string
  messageId?: string
  label?: string
  createdAt: string
}

export type BlockedItem = Pick<BlockedRole, 'role' | 'messageId' | 'label'>

export type SyncProgress = {
  running: boolean
  phase: string
  processed: number
  total: number
}
