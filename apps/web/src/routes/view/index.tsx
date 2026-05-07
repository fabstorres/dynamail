import { useEffect, useState } from 'react'
import { createFileRoute } from '@tanstack/react-router'
import { useAuth } from '../../../components/providers/auth-provider'

export const Route = createFileRoute('/view/')({
  component: View,
})

function View() {
  const { isLoading, isAuthenticated } = useAuth()
  const [threads, setThreads] = useState<any[]>([])
  const [fetching, setFetching] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!isAuthenticated) return

    setFetching(true)
    fetch('http://localhost:8080/threads', { credentials: 'include' })
      .then(async (res) => {
        if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`)
        const data = await res.json()
        setThreads(Array.isArray(data) ? data : [])
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Unknown error'))
      .finally(() => setFetching(false))
  }, [isAuthenticated])

  if (isLoading) return <div>Loading auth...</div>
  if (!isAuthenticated) {
    return (
      <div className="p-4">
        <p>Please log in to view threads.</p>
        <a href="http://localhost:8080/auth/login" className="text-blue-600 underline">Login</a>
      </div>
    )
  }

  return (
    <div className="p-4">
      <h1 className="text-xl font-bold mb-4">Threads</h1>
      {fetching && <div>Loading threads...</div>}
      {error && <div className="text-red-600">Error: {error}</div>}
      {!fetching && threads.length === 0 && <div>No threads found.</div>}
      <ul className="space-y-2">
        {threads.map((t) => (
          <li key={t.id} className="border p-2 rounded">
            <div className="font-medium">{t.snippet ?? t.id}</div>
          </li>
        ))}
      </ul>
    </div>
  )
}
