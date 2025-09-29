import React, { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useQuery } from '@tanstack/react-query'
import Header from './components/Header'
import DropZone from './components/DropZone'
import ProgressModal from './components/ProgressModal'
import FileList from './components/FileList'
import Toast from './components/Toast'
import SearchInterface from './components/SearchInterface'
import { pixelogApi, createProgressCallback } from '@/services/api'
import type { PixeFile, ProgressUpdate } from '@/types/api'

// ===== COMPONENT TYPES =====

interface ToastData {
  readonly message: string
  readonly type: 'success' | 'error' | 'warning' | 'info'
}

interface ConversionProgressState {
  readonly stage: string
  readonly percentage: number
  readonly message?: string
  readonly status?: string
}

// ===== MAIN APP COMPONENT =====

function App(): JSX.Element {
  const [isConverting, setIsConverting] = useState<boolean>(false)
  const [conversionProgress, setConversionProgress] = useState<ConversionProgressState | null>(null)
  const [toast, setToast] = useState<ToastData | null>(null)
  const [isSearchOpen, setIsSearchOpen] = useState<boolean>(false)

  // Query for pixe files with proper typing
  const { data: pixeFiles, refetch: refetchFiles } = useQuery({
    queryKey: ['pixeFiles'] as const,
    queryFn: async (): Promise<readonly PixeFile[]> => {
      return await pixelogApi.getPixeFiles()
    },
    refetchInterval: 5000,
    retry: 2,
    staleTime: 1000 * 60, // 1 minute
  })

  // Toast utility function
  const showToast = (message: string, type: ToastData['type'] = 'success'): void => {
    setToast({ message, type })
    setTimeout(() => setToast(null), 5000)
  }

  // Progress callback for file conversion
  const handleProgress = createProgressCallback(
    (progress: number) => {
      setConversionProgress(prev => prev ? { ...prev, percentage: progress } : null)
    },
    (status: string) => {
      setConversionProgress(prev => prev ? { ...prev, status } : null)
    },
    (message: string) => {
      setConversionProgress(prev => prev ? { ...prev, message } : null)
    }
  )

  // File drop handler with proper error handling
  const handleFileDrop = async (files: readonly File[]): Promise<void> => {
    if (files.length === 0) {
      showToast('No files selected', 'warning')
      return
    }

    setIsConverting(true)
    setConversionProgress({ 
      stage: 'Initializing', 
      percentage: 0,
      message: 'Preparing files for conversion...'
    })

    try {
      const jobId = await pixelogApi.convertFiles(files, undefined, handleProgress)

      showToast(`Conversion started! Job ID: ${jobId}`, 'success')
      
      // Refetch files after conversion starts
      setTimeout(() => {
        void refetchFiles()
      }, 2000)

    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Unknown error occurred'
      showToast(`Conversion failed: ${errorMessage}`, 'error')
      console.error('Conversion error:', error)
    } finally {
      // Clear progress after a delay
      setTimeout(() => {
        setIsConverting(false)
        setConversionProgress(null)
      }, 1000)
    }
  }

  // File refresh handler
  const handleRefresh = (): void => {
    void refetchFiles()
    showToast('Files refreshed', 'info')
  }

  // Toast close handler
  const handleToastClose = (): void => {
    setToast(null)
  }

  // Search toggle handlers
  const handleSearchOpen = (): void => {
    setIsSearchOpen(true)
  }

  const handleSearchClose = (): void => {
    setIsSearchOpen(false)
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 via-blue-50 to-indigo-100 dark:from-gray-900 dark:via-gray-800 dark:to-indigo-900">
      <Header onSearchClick={handleSearchOpen} />
      
      <main className="container mx-auto px-4 py-8 max-w-6xl">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6 }}
          className="text-center mb-12"
        >
          <h1 className="text-4xl md:text-6xl font-bold bg-gradient-to-r from-blue-600 via-purple-600 to-indigo-600 bg-clip-text text-transparent mb-4">
            Pixelog
          </h1>
          <p className="text-xl text-gray-600 dark:text-gray-300 mb-2">
            SQLite-meets-YouTube for LLM memories
          </p>
          <p className="text-gray-500 dark:text-gray-400">
            Convert your knowledge into portable, encrypted .pixe files
          </p>
        </motion.div>

        <div className="grid lg:grid-cols-2 gap-8">
          <motion.div
            initial={{ opacity: 0, x: -20 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.6, delay: 0.2 }}
          >
            <DropZone 
              onFileDrop={handleFileDrop} 
              isDisabled={isConverting} 
            />
          </motion.div>

          <motion.div
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.6, delay: 0.4 }}
          >
            <FileList 
              files={pixeFiles ?? []} 
              onRefresh={handleRefresh}
              onToast={showToast}
            />
          </motion.div>
        </div>

        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.6 }}
          className="mt-16"
        >
          <div className="card text-center">
            <h3 className="text-2xl font-semibold text-gray-900 dark:text-gray-100 mb-4">
              ✨ Features
            </h3>
            <div className="grid md:grid-cols-3 gap-6">
              <div className="p-4">
                <div className="text-3xl mb-2">🔒</div>
                <h4 className="font-medium text-gray-900 dark:text-gray-100 mb-1">
                  Encrypted & Compressed
                </h4>
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  Secure QR-encoded video files
                </p>
              </div>
              <div className="p-4">
                <div className="text-3xl mb-2">📱</div>
                <h4 className="font-medium text-gray-900 dark:text-gray-100 mb-1">
                  Portable & Streamable
                </h4>
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  Access anywhere, anytime
                </p>
              </div>
              <div className="p-4">
                <div className="text-3xl mb-2">🔍</div>
                <h4 className="font-medium text-gray-900 dark:text-gray-100 mb-1">
                  Semantic Search
                </h4>
                <p className="text-sm text-gray-600 dark:text-gray-400">
                  AI-powered content discovery
                </p>
              </div>
            </div>
          </div>
        </motion.div>
      </main>

      {/* Progress Modal */}
      <AnimatePresence>
        {isConverting && conversionProgress && (
          <ProgressModal progress={conversionProgress} />
        )}
      </AnimatePresence>

      {/* Toast Notifications */}
      <AnimatePresence>
        {toast && (
          <Toast 
            message={toast.message} 
            type={toast.type} 
            onClose={handleToastClose} 
          />
        )}
      </AnimatePresence>

      {/* Search Interface */}
      <SearchInterface 
        isOpen={isSearchOpen} 
        onClose={handleSearchClose} 
      />
    </div>
  )
}

export default App
