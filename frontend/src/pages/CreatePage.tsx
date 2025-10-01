import React, { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { ArrowLeft, Upload, RefreshCw, Trash2, Download } from 'lucide-react'
import DropZone from '../components/DropZone'
import Toast from '../components/Toast'
import CloudStorage from '../components/CloudStorage'
import { pixelogApi } from '../services/api'

// Type definitions
interface ProgressUpdate {
  stage?: string
  percentage?: number
  message?: string
  status?: string
  job_id?: string
}

interface PixelogFile {
  id: string
  name: string
  size: string
  created_at: string
  path: string
}

interface ToastState {
  message: string
  type: 'success' | 'error' | 'warning' | 'info'
}

const CreatePage: React.FC = () => {
  // State management
  const [files, setFiles] = useState<PixelogFile[]>([])
  const [isLoading, setIsLoading] = useState<boolean>(true)
  const [isConverting, setIsConverting] = useState<boolean>(false)
  const [conversionProgress, setConversionProgress] = useState<ProgressUpdate | null>(null)
  const [toast, setToast] = useState<ToastState | null>(null)

  // Fetch files on component mount
  useEffect(() => {
    fetchFiles()
  }, [])

  // Auto-hide toast after 5 seconds
  useEffect(() => {
    if (toast) {
      const timer = setTimeout(() => {
        setToast(null)
      }, 5000)
      return () => clearTimeout(timer)
    }
  }, [toast])

  // Fetch files from API
  const fetchFiles = async (): Promise<void> => {
    try {
      setIsLoading(true)
      const response = await fetch('http://localhost:8080/api/files')
      if (!response.ok) throw new Error('Failed to fetch files')
      const fetchedFiles = await response.json()
      setFiles(fetchedFiles)
    } catch (error) {
      console.error('Error fetching files:', error)
      showToast('Failed to fetch files', 'error')
    } finally {
      setIsLoading(false)
    }
  }

  // Refetch files wrapper
  const refetchFiles = (): Promise<void> => fetchFiles()

  // Toast helper
  const showToast = (message: string, type: ToastState['type'] = 'info'): void => {
    setToast({ message, type })
  }

  // Progress handler
  const handleProgress = (progress: ProgressUpdate): void => {
    setConversionProgress(progress)
  }

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
      const formData = new FormData()
      files.forEach(file => formData.append('files', file))
      if (encryptionPassword) {
        formData.append('encryption_password', encryptionPassword)
      }
      
      const response = await fetch('http://localhost:8080/api/convert', {
        method: 'POST',
        body: formData
      })
      
      if (!response.ok) throw new Error('Conversion failed')
      const result = await response.json()
      const jobId = result.job_id

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

  // Delete file handler
  const handleDeleteFile = async (fileId: string): Promise<void> => {
    try {
      await pixelogApi.deleteFile(fileId)
      await fetchFiles() // Refresh the list
      showToast('File deleted successfully', 'success')
    } catch (error) {
      console.error('Failed to delete file:', error)
      const errorMessage = error instanceof Error ? error.message : 'Failed to delete file'
      showToast(errorMessage, 'error')
    }
  }

  // Download file handler
  const handleDownloadFile = async (file: PixelogFile): Promise<void> => {
    try {
      // Create download URL
      const downloadUrl = `http://localhost:8080/api/files/${file.id}/download`
      
      // Create temporary link to trigger download
      const link = document.createElement('a')
      link.href = downloadUrl
      link.download = file.name
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      
      showToast(`Downloaded ${file.name}`, 'success')
    } catch (error) {
      console.error('Failed to download file:', error)
      const errorMessage = error instanceof Error ? error.message : 'Failed to download file'
      showToast(errorMessage, 'error')
    }
  }

  // Toast close handler
  const handleToastClose = (): void => {
    setToast(null)
  }

  return (
    <>
      <div className="min-h-screen cyber-bg-void">

        <div className="container mx-auto px-4 py-8">
          <div className="grid lg:grid-cols-2 gap-8">
            {/* File Upload Section */}
            <motion.div
              initial={{ opacity: 0, x: -20 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ duration: 0.5 }}
              className="h-full"
            >
              <DropZone 
                onFileDrop={handleFileDrop} 
                isDisabled={isConverting} 
              />
            </motion.div>

            {/* File List Section */}
            <motion.div
              initial={{ opacity: 0, x: 20 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ duration: 0.5, delay: 0.1 }}
              className="h-full"
            >
              <div className="cyber-terminal h-full">
                <div className="cyber-terminal-header">
                  <div className="flex items-center justify-between w-full">
                    <h2 className="cyber-h2 text-lg">Converted Files</h2>
                    <button
                      onClick={handleRefresh}
                      className="cyber-btn-secondary p-2"
                      disabled={isLoading}
                    >
                      <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
                    </button>
                  </div>
                </div>
                <div className="cyber-terminal-body">
                  {isLoading ? (
                    <div className="flex items-center justify-center py-12">
                      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-cyan-500"></div>
                    </div>
                  ) : files.length === 0 ? (
                    <div className="text-center py-12 cyber-text-secondary">
                      <Upload className="w-16 h-16 mx-auto mb-4 cyber-text-tertiary" />
                      <p className="cyber-h3 text-lg mb-2">No .pixe files yet</p>
                      <p className="cyber-body text-sm">Convert some files to get started</p>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {files.map((file) => (
                        <div key={file.id} className="cyber-bg-panel p-4 rounded-lg">
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-3">
                              <Upload className="w-5 h-5 cyber-text-blue" />
                              <div>
                                <h3 className="cyber-body cyber-text-primary font-medium">{file.name}</h3>
                                <p className="cyber-mono text-xs cyber-text-secondary">
                                  {typeof file.size === 'string' ? file.size : `${file.size} bytes`} • {new Date(file.created_at).toLocaleDateString()}
                                </p>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <button
                                onClick={() => handleDownloadFile(file)}
                                className="cyber-btn-secondary p-2"
                                title="Download file"
                              >
                                <Download className="w-4 h-4" />
                              </button>
                              <button
                                onClick={() => handleDeleteFile(file.id)}
                                className="cyber-btn-danger p-2"
                                title="Delete file"
                              >
                                <Trash2 className="w-4 h-4" />
                              </button>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </motion.div>
          </div>

          {/* Conversion Progress */}
          {conversionProgress && (
            <motion.div
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3 }}
              className="mt-8"
            >
              <div className="cyber-terminal">
                <div className="cyber-terminal-header">
                  <h3 className="cyber-h3">Converting Files...</h3>
                </div>
                <div className="cyber-terminal-body">
                  <div className="space-y-2">
                    <div className="flex justify-between cyber-body">
                      <span>{conversionProgress.stage || 'Processing'}</span>
                      <span>{conversionProgress.percentage || 0}%</span>
                    </div>
                    <div className="cyber-bg-void rounded-full h-2 overflow-hidden">
                      <div 
                        className="h-full bg-gradient-to-r from-cyan-500 to-blue-500 transition-all duration-300"
                        style={{ width: `${conversionProgress.percentage || 0}%` }}
                      />
                    </div>
                    {conversionProgress.message && (
                      <p className="cyber-mono text-xs cyber-text-secondary">
                        {conversionProgress.message}
                      </p>
                    )}
                  </div>
                </div>
              </div>
            </motion.div>
          )}
          
          {/* Cloud Storage Section */}
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, delay: 0.4 }}
            className="mt-8"
          >
            <CloudStorage />
          </motion.div>
        </div>
      </div>

      {/* Toast Notifications */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={handleToastClose}
        />
      )}
    </>
  )
}

export default CreatePage
