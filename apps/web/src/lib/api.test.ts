import { describe, it, expect, beforeEach, vi } from 'vitest'
import { AxiosHeaders } from 'axios'
import { api, __resetAuthInterceptorState } from './api'
import { useAuthStore } from '@/stores/auth-store'

describe('api axios instance', () => {
  beforeEach(() => {
    __resetAuthInterceptorState()
    useAuthStore.getState().auth.reset()
    document.cookie = `tm_csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`
  })

  it('base config has withCredentials and /api/v1 baseURL', () => {
    expect(api.defaults.withCredentials).toBe(true)
    expect(api.defaults.baseURL).toBe('/api/v1')
  })

  it('request interceptor injects X-CSRF-Token from cookie', async () => {
    document.cookie = 'tm_csrf_token=token-123; path=/'
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const interceptor = (api.interceptors.request as any).handlers[0]
    const config = {
      headers: new AxiosHeaders(),
      method: 'post',
      url: '/anything',
    }
    const result = await interceptor.fulfilled(config)
    expect(result.headers.get('X-CSRF-Token')).toBe('token-123')
  })

  it('request interceptor does not attach header when cookie absent', async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const interceptor = (api.interceptors.request as any).handlers[0]
    const config = {
      headers: new AxiosHeaders(),
      method: 'post',
      url: '/anything',
    }
    const result = await interceptor.fulfilled(config)
    expect(result.headers.get('X-CSRF-Token')).toBeFalsy()
  })

  it('401 response triggers single refresh attempt and replays original', async () => {
    const replaySpy = vi.fn().mockResolvedValue({ data: { ok: true }, status: 200 })
    const refreshSpy = vi
      .spyOn(api, 'post')
      .mockResolvedValueOnce({ data: {}, status: 200 } as never)

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const rejected = (api.interceptors.response as any).handlers[0].rejected
    const originalConfig = {
      url: '/auth/me',
      method: 'get',
      headers: new AxiosHeaders(),
    }
    const requestFn = vi.spyOn(api, 'request').mockImplementationOnce(replaySpy)

    await rejected({
      config: originalConfig,
      response: { status: 401 },
      isAxiosError: true,
    }).catch(() => undefined)

    expect(refreshSpy).toHaveBeenCalledWith('/auth/refresh')
    expect(requestFn).toHaveBeenCalledTimes(1)
    refreshSpy.mockRestore()
    requestFn.mockRestore()
  })

  it('second 401 within one request chain rejects and marks unauthenticated', async () => {
    vi.spyOn(api, 'post').mockRejectedValueOnce({
      response: { status: 401 },
      isAxiosError: true,
    } as never)
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const rejected = (api.interceptors.response as any).handlers[0].rejected

    const originalConfig = {
      url: '/auth/me',
      method: 'get',
      headers: new AxiosHeaders(),
    }

    await expect(
      rejected({
        config: originalConfig,
        response: { status: 401 },
        isAxiosError: true,
      }),
    ).rejects.toBeDefined()
    expect(useAuthStore.getState().auth.status).toBe('unauthenticated')
  })
})
