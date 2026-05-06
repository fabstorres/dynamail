import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { fetchMe, logout } from '../lib/auth'

export function useMe() {
  return useQuery({
    queryKey: ['me'],
    queryFn: fetchMe,
    retry: false,
  })
}

export function useLogout() {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: logout,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['me']})
  })
}
