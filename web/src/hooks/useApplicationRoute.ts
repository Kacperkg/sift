import { useCallback, useEffect, useState } from 'react'

function applicationIDFromHash() {
  const match = window.location.hash.match(/^#\/application\/(.+)$/)
  if (!match) return ''
  try {
    return decodeURIComponent(match[1])
  } catch {
    return ''
  }
}

export function useApplicationRoute() {
  const [selectedID, setSelectedID] = useState(applicationIDFromHash)

  useEffect(() => {
    const handleRoute = () => setSelectedID(applicationIDFromHash())
    window.addEventListener('hashchange', handleRoute)
    return () => window.removeEventListener('hashchange', handleRoute)
  }, [])

  const openApplication = useCallback((id: string) => {
    window.location.hash = `/application/${encodeURIComponent(id)}`
  }, [])

  const closeApplication = useCallback(() => {
    window.history.pushState(null, '', `${window.location.pathname}${window.location.search}`)
    setSelectedID('')
  }, [])

  return { selectedID, openApplication, closeApplication }
}
