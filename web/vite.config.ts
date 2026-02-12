import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: {
          'three': ['three'],
          'vendor-react': ['react', 'react-dom', 'react-router-dom'],
          'vendor-data': ['axios', '@tanstack/react-query', 'zustand'],
          'vendor-markdown': ['react-markdown', 'remark-gfm'],
          'vendor-motion': ['motion'],
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true, // Proxy WebSocket connections
      },
    },
  },
})
