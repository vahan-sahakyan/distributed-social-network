import Keycloak from 'keycloak-js'

// Keycloak is served under /auth next to the app; `make ui` points at the compose one
export const keycloak = new Keycloak({
  url: import.meta.env.VITE_KEYCLOAK_URL || `${location.origin}/auth`,
  realm: 'dsn',
  clientId: 'dsn-ui',
})

export function initAuth() {
  return keycloak.init({
    onLoad: 'check-sso',
    pkceMethod: 'S256',
    // the session-status iframe can fail to load, and updateToken then waits on it
    // forever; an ended session still shows up as a failed refresh
    checkLoginIframe: false,
    silentCheckSsoRedirectUri: `${location.origin}/silent-check-sso.html`,
  })
}

// bearer returns a token valid for at least 30s, or null when logged out
export async function bearer() {
  if (!keycloak.authenticated) return null
  try {
    await keycloak.updateToken(30)
  } catch {
    keycloak.login()
    return null
  }
  return keycloak.token
}

export const login = () => keycloak.login({ redirectUri: `${location.origin}/home` })
export const register = () => keycloak.register({ redirectUri: `${location.origin}/home` })
export const logout = () => keycloak.logout({ redirectUri: `${location.origin}/auth` })
