import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { shouldReloadOnChunkError } from './utils/chunkReload'

// Чанк прошлого релиза не загрузился: перезагрузка за новой сборкой.
window.addEventListener('vite:preloadError', () => {
  if (shouldReloadOnChunkError()) window.location.reload()
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
