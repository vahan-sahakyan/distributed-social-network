import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import './index.css'
import App from './App.jsx'
import { initAuth } from './auth'

const root = createRoot(document.getElementById('root'))

// render either way: a down Keycloak should not blank the read-only pages
initAuth()
  .catch(err => console.error('keycloak init failed', err))
  .finally(() =>
    root.render(
      <StrictMode>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </StrictMode>,
    ),
  )
