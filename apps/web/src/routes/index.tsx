import { createFileRoute } from '@tanstack/react-router'
import { useAuth } from '../../components/providers/auth-provider'

export const Route = createFileRoute('/')({ component: Home })

function Home() {
  const { isLoading, isAuthenticated, logout } = useAuth()
  return (
    <>
      <nav className="p-4">
        {isLoading || !isAuthenticated ?
          <a href="http://localhost:8080/auth/login">login</a> :
          <button className='hover:cursor-pointer' onClick={logout}>logout</button>
        }
      </nav>
    </>
  )
}
