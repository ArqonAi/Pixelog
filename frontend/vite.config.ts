import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@/components': path.resolve(__dirname, './src/components'),
      '@/services': path.resolve(__dirname, './src/services'),
      '@/types': path.resolve(__dirname, './src/types'),
      '@/utils': path.resolve(__dirname, './src/utils'),
      '@/hooks': path.resolve(__dirname, './src/hooks'),
      '@/styles': path.resolve(__dirname, './src/styles')
    }
  },
  server: {
    proxy: {
      '^/api/(files|convert|extract|contents|search|llm|health)': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    },
    // Force cache clear for development
    headers: {
      'Cache-Control': 'no-cache, no-store, must-revalidate',
      'Pragma': 'no-cache',
      'Expires': '0'
    }
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['react', 'react-dom'],
          api: ['@tanstack/react-query'],
          ui: ['framer-motion', 'lucide-react']
        }
      }
    }
  },
  esbuild: {
    // Enable JSX in .ts files
    loader: 'tsx',
    include: /src\/.*\.[tj]sx?$/,
    exclude: []
  }
})