import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores/auth-store'
import { readCsrfToken } from './csrf'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

export const api = axios.create({
  baseURL: BASE_URL,
  withCredentials: true,
  headers: { 'Content-Type': 'application/json' },
})

type RetryableConfig = InternalAxiosRequestConfig & { __retried?: boolean }

let refreshInFlight: Promise<void> | null = null

/**
 * Reset in-memory state shared across test runs. Do NOT call in prod.
 */
export function __resetAuthInterceptorState(): void {
  refreshInFlight = null
}

api.interceptors.request.use((config) => {
  if (config.method && config.method.toLowerCase() !== 'get') {
    const token = readCsrfToken()
    if (token) {
      config.headers.set('X-CSRF-Token', token)
    }
  }
  return config
})

api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const original = error.config as RetryableConfig | undefined
    const status = error.response?.status

    // Only intercept 401, only retry once, and never re-enter /auth/refresh or /auth/login.
    // (A 401 on /auth/login means wrong credentials, not an expired session.)
    if (
      status === 401 &&
      original &&
      !original.__retried &&
      !original.url?.endsWith('/auth/refresh') &&
      !original.url?.endsWith('/auth/login')
    ) {
      original.__retried = true

      try {
        if (!refreshInFlight) {
          refreshInFlight = api
            .post('/auth/refresh')
            .then(() => undefined)
            .finally(() => {
              refreshInFlight = null
            })
        }
        await refreshInFlight
        return api.request(original)
      } catch (refreshErr) {
        useAuthStore.getState().auth.markUnauthenticated()
        throw refreshErr
      }
    }

    if (status === 401) {
      useAuthStore.getState().auth.markUnauthenticated()
    }
    throw error
  },
)
