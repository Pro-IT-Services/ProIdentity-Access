import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import ErrorBoundary from './components/ErrorBoundary'
import { initLanguage, useLanguage } from './i18n'
import './index.css'

// Remount on a language change so every t() call re-evaluates (the language
// is normally picked once, before the first render). The root boundary keeps
// a render error from blanking the whole window.
function Root() {
  const lang = useLanguage(s => s.lang)
  return (
    <ErrorBoundary key={lang}>
      <App />
    </ErrorBoundary>
  )
}

initLanguage().finally(() => {
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <Root />
    </React.StrictMode>
  )
})
