import { useCallback, useEffect, useState } from 'react'
import {
  getDashboardData,
  getSyncProgress,
  setAIEnabled,
  setBlockedItem,
  setInterviewOutcome,
  syncApplications,
} from '../api/sift'
import type {
  AIStatus,
  Application,
  BlockedItem,
  BlockedRole,
  InterviewOutcome,
  SyncProgress,
} from '../types/application'

const defaultAIStatus: AIStatus = {
  enabled: false,
  available: false,
  modelInstalled: false,
  model: 'qwen3:4b',
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback
}

export function useSift() {
  const [connected, setConnected] = useState(false)
  const [applications, setApplications] = useState<Application[]>([])
  const [ai, setAI] = useState<AIStatus>(defaultAIStatus)
  const [blockedRoles, setBlockedRoles] = useState<BlockedRole[]>([])
  const [scanProgress, setScanProgress] = useState<SyncProgress | null>(null)
  const [syncing, setSyncing] = useState(false)
  const [aiBusy, setAIBusy] = useState(false)
  const [blockBusy, setBlockBusy] = useState(false)
  const [outcomeBusy, setOutcomeBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    getDashboardData()
      .then(data => {
        setConnected(data.connected)
        setApplications(data.applications)
        if (data.ai) setAI(data.ai)
        if (data.blockedRoles) setBlockedRoles(data.blockedRoles)
      })
      .catch(error => setError(errorMessage(error, 'Start the local Go server to use Job Inbox.')))
  }, [])

  const sync = useCallback(async (mode: 'update' | 'full') => {
    setSyncing(true)
    setError('')
    setScanProgress({
      running: true,
      phase: mode === 'full' ? 'Starting full scan' : 'Checking for updates',
      processed: 0,
      total: 0,
    })
    const progressTimer = window.setInterval(() => {
      getSyncProgress().then(setScanProgress).catch(() => {
        // The main sync request reports actionable errors.
      })
    }, 450)
    try {
      const updatedApplications = await syncApplications(mode)
      setApplications(updatedApplications)
      setScanProgress(current => ({
        running: false,
        phase: mode === 'full' ? 'Full scan complete' : 'Inbox updated',
        processed: current?.total || updatedApplications.length,
        total: current?.total || updatedApplications.length,
      }))
    } catch (error) {
      setError(errorMessage(error, 'Sync failed.'))
      setScanProgress(current => ({
        running: false,
        phase: 'Scan stopped',
        processed: current?.processed || 0,
        total: current?.total || 0,
      }))
    } finally {
      window.clearInterval(progressTimer)
      setSyncing(false)
      window.setTimeout(() => setScanProgress(null), 2200)
    }
  }, [])

  const toggleAI = useCallback(async () => {
    setAIBusy(true)
    setError('')
    try {
      setAI(await setAIEnabled(!ai.enabled))
    } catch (error) {
      setError(errorMessage(error, 'Could not update local AI.'))
    } finally {
      setAIBusy(false)
    }
  }, [ai.enabled])

  const updateBlockedItem = useCallback(async (item: BlockedItem, blocked: boolean) => {
    setBlockBusy(true)
    setError('')
    try {
      setBlockedRoles(await setBlockedItem(item, blocked))
      if (blocked) {
        setApplications(current => current.filter(application => (
          item.messageId
            ? application.id !== item.messageId
            : application.role.trim().toLowerCase() !== item.role?.trim().toLowerCase()
        )))
      }
      return true
    } catch (error) {
      setError(errorMessage(error, 'Could not update blocked roles.'))
      return false
    } finally {
      setBlockBusy(false)
    }
  }, [])

  const closeInterview = useCallback(async (application: Application, status: InterviewOutcome) => {
    setOutcomeBusy(true)
    setError('')
    try {
      const updated = await setInterviewOutcome(application.id, status)
      setApplications(current => current.map(item => item.id === updated.id ? updated : item))
    } catch (error) {
      setError(errorMessage(error, 'Could not close this interview.'))
    } finally {
      setOutcomeBusy(false)
    }
  }, [])

  return {
    connected,
    applications,
    ai,
    blockedRoles,
    scanProgress,
    syncing,
    aiBusy,
    blockBusy,
    outcomeBusy,
    error,
    sync,
    toggleAI,
    updateBlockedItem,
    closeInterview,
  }
}
