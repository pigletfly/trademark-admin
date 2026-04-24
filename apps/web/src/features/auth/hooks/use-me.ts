import { useQuery } from '@tanstack/react-query'
import { meQueryOptions } from './me-query'

export function useMe() {
  return useQuery(meQueryOptions)
}
