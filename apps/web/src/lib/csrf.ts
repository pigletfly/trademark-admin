const CSRF_COOKIE_NAME = 'tm_csrf_token'

/**
 * Read the tm_csrf_token cookie. Backend sets it non-httpOnly specifically so
 * JS can mirror it back via the X-CSRF-Token header (double-submit pattern).
 * Returns empty string when the cookie is not present.
 */
export function readCsrfToken(): string {
  const match = document.cookie
    .split('; ')
    .find((row) => row.startsWith(`${CSRF_COOKIE_NAME}=`))
  if (!match) return ''
  return decodeURIComponent(match.slice(CSRF_COOKIE_NAME.length + 1))
}
