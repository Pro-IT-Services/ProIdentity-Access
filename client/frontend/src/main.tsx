import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { initLanguage, useLanguage } from './i18n'
import './index.css'

// Remount on a language change so every t() call re-evaluates (the language
// is normally picked once, before the first render).
function Root() {
  const lang = useLanguage(s => s.lang)
  return <App key={lang} />
}

initLanguage().finally(() => {
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <Root />
    </React.StrictMode>
  )
})
