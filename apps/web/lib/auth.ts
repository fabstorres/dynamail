// TODO: replace hardcoded URLs with environment variables


export type User = {
  name: string;
  email: string;
};

export async function fetchMe() {
  const res = await fetch('http://localhost:8080/user/me', { credentials: 'include' })

  if (!res.ok) {
    throw new Error('Failed to fetch user')
  }

  const data = await res.json()
  return data
}

export async function logout() {
  const res = await fetch('http://localhost:8080/auth/logout', { method: 'POST', credentials: 'include' })
  return res.ok
}
