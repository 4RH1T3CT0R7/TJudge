import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Оптимизация изображений. Чтобы активировать, установите плагин:
//   npm i -D vite-plugin-image-optimizer
// и раскомментируйте импорт/плагин ниже. PNG-логотипы в /public получат
// 20-30% экономии размера без потери визуального качества.
// import { ViteImageOptimizer } from 'vite-plugin-image-optimizer'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    // ViteImageOptimizer({
    //   png:  { quality: 85 },
    //   jpeg: { quality: 85 },
    //   svg:  { multipass: true },
    // }),
  ],
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: {
          'three': ['three'],
          'vendor-react': ['react', 'react-dom', 'react-router-dom'],
          'vendor-data': ['axios', 'zustand'],
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
