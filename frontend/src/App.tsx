import React, { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { useQuery } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import Header from './components/Header'
import DropZone from './components/DropZone'
import ProgressModal from './components/ProgressModal'
import FileList from './components/FileList'
import Toast from './components/Toast'
import IntegratedSearch from './components/IntegratedSearch'
import CloudStorage from './components/CloudStorage'
import Footer from './components/Footer'
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


  return (
    <div className="min-h-screen cyber-bg-void">
      <Header onSearchClick={() => {}} />
      
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
        <div className="grid lg:grid-cols-2 gap-6 mb-8">
          {/* Upload Section */}
          <motion.div
            initial={{ opacity: 0, x: -30 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.8, delay: 0.2 }}
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
          transition={{ duration: 0.8, delay: 0.6 }}
          className="mt-16"
        >
          <div className="cyber-terminal">
            <div className="cyber-terminal-header">
              <h2 className="cyber-h2 text-xl flex-1">How Pixelog Works</h2>
            </div>
            <div className="cyber-terminal-body">
              <div className="grid md:grid-cols-2 gap-8">
                {/* Process Steps */}
                <div>
                  <h3 className="cyber-h3 text-lg mb-4 cyber-text-primary">Conversion Process</h3>
                  <div className="space-y-4">
                    <div className="flex items-start space-x-3">
                      <div className="w-6 h-6 rounded-full bg-cyan-500 flex items-center justify-center cyber-mono text-xs font-bold text-black">1</div>
                      <div>
                        <h4 className="cyber-body font-semibold cyber-text-primary">File Encoding</h4>
                        <p className="cyber-body text-sm cyber-text-secondary">Your files are converted into QR code sequences, preserving all data integrity</p>
                      </div>
                    </div>
                    <div className="flex items-start space-x-3">
                      <div className="w-6 h-6 rounded-full bg-cyan-500 flex items-center justify-center cyber-mono text-xs font-bold text-black">2</div>
                      <div>
                        <h4 className="cyber-body font-semibold cyber-text-primary">Video Assembly</h4>
                        <p className="cyber-body text-sm cyber-text-secondary">QR frames are compiled into standard MP4 video files for portability</p>
                      </div>
                    </div>
                    <div className="flex items-start space-x-3">
                      <div className="w-6 h-6 rounded-full bg-cyan-500 flex items-center justify-center cyber-mono text-xs font-bold text-black">3</div>
                      <div>
                        <h4 className="cyber-body font-semibold cyber-text-primary">Indexing</h4>
                        <p className="cyber-body text-sm cyber-text-secondary">Content is analyzed for semantic search capabilities</p>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Features */}
                <div>
                  <h3 className="cyber-h3 text-lg mb-4 cyber-text-primary">Key Features</h3>
                  <div className="space-y-3">
                    <div className="flex items-center space-x-3">
                      <div className="w-2 h-2 bg-cyan-400 rounded-full"></div>
                      <span className="cyber-body cyber-text-secondary">100% lossless encoding</span>
                    </div>
                    <div className="flex items-center space-x-3">
                      <div className="w-2 h-2 bg-cyan-400 rounded-full"></div>
                      <span className="cyber-body cyber-text-secondary">Standard MP4 format</span>
                    </div>
                    <div className="flex items-center space-x-3">
                      <div className="w-2 h-2 bg-cyan-400 rounded-full"></div>
                      <span className="cyber-body cyber-text-secondary">Works offline</span>
                    </div>
                    <div className="flex items-center space-x-3">
                      <div className="w-2 h-2 bg-cyan-400 rounded-full"></div>
                      <span className="cyber-body cyber-text-secondary">AI-powered search</span>
                    </div>
                    <div className="flex items-center space-x-3">
                      <div className="w-2 h-2 bg-cyan-400 rounded-full"></div>
                      <span className="cyber-body cyber-text-secondary">Cross-platform compatible</span>
                    </div>
                    <div className="flex items-center space-x-3">
                      <div className="w-2 h-2 bg-cyan-400 rounded-full"></div>
                      <span className="cyber-body cyber-text-secondary">Supports all file types</span>
                    </div>
                  </div>
                </div>
              </div>

            </div>
          </div>
        </motion.div>
      </main>

      {/* Footer */}
      <Footer />

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

    </div>
  )
}

export default App
