import React, { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useQuery } from '@tanstack/react-query'
import DropZone from '../components/DropZone'
import ProgressModal from '../components/ProgressModal'
import FileList from '../components/FileList'
import Toast from '../components/Toast'
import IntegratedSearch from '../components/IntegratedSearch'
import CloudStorage from '../components/CloudStorage'
import Footer from '../components/Footer'
import { pixelogApi, createProgressCallback } from '@/services/api'
import type { PixeFile, ProgressUpdate } from '@/types/api'

// ===== COMPONENT TYPES =====

interface ToastData {
  readonly message: string
  readonly type: 'success' | 'error' | 'warning' | 'info'
}

interface ConversionProgress {
  readonly stage: string
  readonly percentage: number
  readonly status?: string
  readonly message?: string
}

// ===== MAIN APP COMPONENT =====

const HomePage: React.FC = () => {
  // State management
  const [isConverting, setIsConverting] = useState<boolean>(false)
  const [conversionProgress, setConversionProgress] = useState<ConversionProgress | null>(null)
  const [toast, setToast] = useState<ToastData | null>(null)

  // Fetch files using React Query
  const { data: pixeFiles, refetch: refetchFiles } = useQuery({
    queryKey: ['pixeFiles'],
    queryFn: () => pixelogApi.getPixeFiles(),
    staleTime: 1000 * 30, // 30 seconds
    refetchInterval: 1000 * 30, // Auto refetch every 30 seconds
  })

  // Toast helper
  const showToast = (message: string, type: ToastData['type'] = 'info'): void => {
    setToast({ message, type })
  }

  // Progress callback
  const handleProgress = createProgressCallback(
    (percentage: number, stage: string) => {
      setConversionProgress(prev => prev ? { ...prev, percentage, stage } : null)
    },
    (status: string) => {
      setConversionProgress(prev => prev ? { ...prev, status } : null)
    },
    (message: string) => {
      setConversionProgress(prev => prev ? { ...prev, message } : null)
    }
  )

  // File drop handler with proper error handling
  const handleFileDrop = async (files: readonly File[], encryptionPassword?: string): Promise<void> => {
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
      const options = encryptionPassword ? { encryption: true } : undefined
      const jobId = await pixelogApi.convertFiles(files, options, handleProgress)

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
      setIsConverting(false)
      setConversionProgress(null)
    }
  }

  // Refresh handler
  const handleRefresh = (): void => {
    void refetchFiles()
    showToast('File list refreshed', 'info')
  }

  // Toast close handler
  const handleToastClose = (): void => {
    setToast(null)
  }

  return (
    <>
      {/* Main Intelligence Interface */}
      <main className="container mx-auto px-4 py-8 max-w-7xl">
        {/* Main Header */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6 }}
          className="text-center mb-12"
        >
          <p className="cyber-body text-xl cyber-text-secondary mb-2">
            Convert files into portable, searchable video archives
          </p>
          <p className="cyber-body cyber-text-tertiary">
            Encode any file as QR codes in MP4 format for secure, offline storage
          </p>
        </motion.div>

        {/* Integrated Search */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.2 }}
          className="mb-8"
        >
          <IntegratedSearch />
        </motion.div>

        {/* Upload and Files */}
        <div className="grid lg:grid-cols-2 gap-6 mb-8 lg:items-stretch">
          {/* Upload Section */}
          <motion.div
            initial={{ opacity: 0, x: -30 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.8, delay: 0.2 }}
            className="h-full"
          >
            <DropZone 
              onFileDrop={handleFileDrop} 
              isDisabled={isConverting} 
            />
          </motion.div>

          {/* Local Files */}
          <motion.div
            initial={{ opacity: 0, x: 30 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.8, delay: 0.4 }}
            className="h-full"
          >
            <FileList 
              files={pixeFiles ?? []} 
              onRefresh={handleRefresh}
              onToast={showToast}
            />
          </motion.div>
        </div>

        {/* Cloud Storage Section */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, delay: 0.6 }}
          className="mb-8"
        >
          <CloudStorage />
        </motion.div>

        {/* How it Works Section */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, delay: 0.8 }}
        >
          <Footer />
        </motion.div>
      </main>

      {/* Progress Modal */}
      <AnimatePresence>
        {isConverting && conversionProgress && (
          <ProgressModal
            progress={conversionProgress}
            onClose={() => {
              setIsConverting(false)
              setConversionProgress(null)
            }}
          />
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
    </>
  )
}

export default HomePage
