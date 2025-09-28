import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'

// The repository name must be used as the base path for GitHub Pages
const repoName = 'Pixelog'; 

export default defineConfig({
  // CRITICAL FIX: Base path set to /repository-name/
  base: `./`, 

  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'Pixelog - Knowledge Storage',
        short_name: 'Pixelog',
        description: 'SQLite-meets-YouTube for LLM memories - Store knowledge as portable .pixe files',
        theme_color: '#6366f1',
        background_color: '#1f2937',
        display: 'standalone',
        icons: [
          {
            src: 'pwa-192x192.png',
            sizes: '192x192',
            type: 'image/png'
          },
          {
            src: 'pwa-512x512.png',
            sizes: '512x512',
            type: 'image/png'
          }
        ]
      }
    })
  ],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['react', 'react-dom'],
          motion: ['framer-motion'],
          query: ['@tanstack/react-query']
        }
      }
    }
  }
})
