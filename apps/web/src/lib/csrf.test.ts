import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { readCsrfToken } from './csrf'

describe('readCsrfToken', () => {
  beforeEach(() => {
    // Clear any existing cookies for a clean slate.
    document.cookie.split(';').forEach((c) => {
      const name = c.split('=')[0].trim()
      document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`
    })
  })

  afterEach(() => {
    document.cookie = `tm_csrf_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`
  })

  it('returns empty string when cookie is absent', () => {
    expect(readCsrfToken()).toBe('')
  })

  it('returns cookie value when present', () => {
    document.cookie = 'tm_csrf_token=abc123; path=/'
    expect(readCsrfToken()).toBe('abc123')
  })

  it('returns decoded value when url-encoded', () => {
    document.cookie = 'tm_csrf_token=abc%2Fdef; path=/'
    expect(readCsrfToken()).toBe('abc/def')
  })

  it('does not confuse with other cookies sharing a prefix', () => {
    document.cookie = 'tm_csrf_token_other=wrong; path=/'
    document.cookie = 'tm_csrf_token=right; path=/'
    expect(readCsrfToken()).toBe('right')
  })
})
