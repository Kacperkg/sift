import type { Application } from '../types/application'

export function formatDate(value: string, long = false) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Unknown date'
  return date.toLocaleDateString(
    'en-GB',
    long
      ? { day: 'numeric', month: 'long', year: 'numeric' }
      : { day: '2-digit', month: 'short', year: '2-digit' },
  )
}

export function initials(company: string) {
  return company
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map(word => word[0])
    .join('')
    .toUpperCase() || '—'
}

export function statusClass(status: string) {
  return status.toLowerCase().replaceAll(' ', '-')
}

export function matchesApplication(application: Application, query: string) {
  const needle = query.trim().toLowerCase()
  if (!needle) return true
  return [application.company, application.role, application.lastEvent]
    .some(value => value.toLowerCase().includes(needle))
}

export function gmailMessageURL(messageID: string) {
  return `https://mail.google.com/mail/u/0/#all/${encodeURIComponent(messageID)}`
}
