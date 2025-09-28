import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  // CRITICAL FIX: Use the relative path './' for the production environment.
  // This tells Vite to generate asset links relative to the index.html file, 
  // which is necessary for GitHub Pages sub-folder deployment (arqonai.github.io/Pixelog/).
  base: process.env.NODE_ENV === 'production' ? './' : '/', 
  
  plugins: [react()],
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
          vendor: ['react', 'react-dom']
        }
      }
    }
  }
})
