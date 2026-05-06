import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/')({ component: Home })

function Home() {
  return (
    <>
      <nav className="p-4">
        <a href="http://localhost:8080/auth/login">login</a>
      </nav>
    </>
  )
}
