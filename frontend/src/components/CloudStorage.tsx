import React, { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { Cloud, Upload, Download, Settings, Check, AlertCircle, RefreshCw } from 'lucide-react'
import { cloudApi, type CloudFile, type CloudStatus } from '@/services/api'
import CloudConfigModal from './CloudConfigModal'

// ===== COMPONENT TYPES =====

interface CloudStorageProps {
  readonly className?: string
}

interface CloudProvider {
  readonly id: string
  readonly name: string
  readonly icon: string
  readonly configured: boolean
}

// CloudFile type imported from API

// ===== CLOUD STORAGE COMPONENT =====

const CloudStorage: React.FC<CloudStorageProps> = ({ className = '' }) => {
  const [selectedProvider, setSelectedProvider] = useState<string>('aws')
  const [isConfigModalOpen, setIsConfigModalOpen] = useState<boolean>(false)
  const [cloudStatus, setCloudStatus] = useState<CloudStatus | null>(null)
  const [cloudFiles, setCloudFiles] = useState<readonly CloudFile[]>([])
  const [isLoading, setIsLoading] = useState<boolean>(true)
  const [isUploading, setIsUploading] = useState<boolean>(false)

  const providers: readonly CloudProvider[] = [
    { id: 'aws', name: 'AWS S3', icon: '🟧', configured: false },
    { id: 'gcp', name: 'Google Cloud', icon: '🟦', configured: false },
    { id: 'azure', name: 'Azure Blob', icon: '🟦', configured: false },
    { id: 'digitalocean', name: 'DigitalOcean', icon: '🟦', configured: false },
  ]

  // Load cloud status and files on mount
  useEffect(() => {
    loadCloudData()
  }, [])

  const loadCloudData = async (): Promise<void> => {
    try {
      setIsLoading(true)
      // Check if cloud endpoints are available by testing the health endpoint first
      const healthResponse = await fetch('/health')
      const health = await healthResponse.json()
      
      if (!health.cloud_enabled) {
        // Cloud storage is disabled on the backend
        setCloudStatus({
          configured: false
        })
        setCloudFiles([])
        setIsLoading(false)
        return
      }
      
      const [status, files] = await Promise.all([
        cloudApi.getStatus(),
        cloudApi.getCloudFiles().catch(() => []) // Don't fail if not configured
      ])
      
      setCloudStatus(status)
      setCloudFiles(files)
      setIsLoading(false)
      
      if (status.provider) {
        setSelectedProvider(status.provider)
      }
    } catch (error) {
      console.error('Failed to load cloud data:', error)
      // Handle missing cloud endpoints gracefully
      setCloudStatus({
        configured: false
      })
      setCloudFiles([])
      setIsLoading(false)
    }
  }

  const handleProviderSelect = (providerId: string): void => {
    setSelectedProvider(providerId)
  }

  const handleConfigure = (): void => {
    setIsConfigModalOpen(true)
  }

  const handleConfigSuccess = (): void => {
    loadCloudData() // Reload data after successful configuration
  }

  const handleFileUpload = async (files: FileList | null): Promise<void> => {
    if (!files || files.length === 0) return

    try {
      setIsUploading(true)
      const fileArray = Array.from(files).filter(f => f.name.endsWith('.pixe'))
      
      if (fileArray.length === 0) {
        alert('Please select .pixe files only')
        return
      }

      const uploadedFiles = await cloudApi.uploadFiles(fileArray)
      setCloudFiles(prev => [...prev, ...uploadedFiles])
    } catch (error) {
      console.error('Upload failed:', error)
      alert('Upload failed: ' + (error instanceof Error ? error.message : 'Unknown error'))
    } finally {
      setIsUploading(false)
    }
  }

  const handleDownload = async (file: CloudFile): Promise<void> => {
    try {
      const { downloadUrl } = await cloudApi.getDownloadUrl(file.id)
      
      // Create temporary link to download
      const link = document.createElement('a')
      link.href = downloadUrl
      link.download = file.filename
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
    } catch (error) {
      console.error('Download failed:', error)
      alert('Download failed: ' + (error instanceof Error ? error.message : 'Unknown error'))
    }
  }

  const formatDate = (dateString: string): string => {
    try {
      return new Date(dateString).toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      })
    } catch {
      return 'Invalid date'
    }
  }

  const formatFileSize = (bytes: number): string => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }

  const selectedProviderData = providers.find(p => p.id === selectedProvider)
  const isConfigured = cloudStatus?.configured && cloudStatus?.provider === selectedProvider

  return (
    <div className={`cyber-terminal ${className}`}>
      <div className="cyber-terminal-header">
        <Cloud className="w-5 h-5 cyber-text-cyber" />
        <h2 className="cyber-h2 text-lg flex-1">Cloud Storage</h2>
        <div className="cyber-mono text-xs cyber-text-secondary">
          Backup & Sync
        </div>
      </div>
      
      <div className="cyber-terminal-body">
        {/* Top Section: Provider Selection */}
        <div className="mb-8">
          <div className="flex items-center justify-between mb-4">
            <h3 className="cyber-h3 text-base cyber-text-primary">Storage Provider</h3>
            {selectedProviderData?.configured && (
              <div className="flex items-center space-x-2 cyber-mono text-xs">
                <Check className="w-3 h-3 text-green-400" />
                <span className="cyber-text-secondary">Connected</span>
              </div>
            )}
          </div>
          
          <div className="flex flex-wrap gap-3 mb-4">
            {providers.map((provider) => (
              <motion.button
                key={provider.id}
                onClick={() => handleProviderSelect(provider.id)}
                className={`
                  flex items-center space-x-2 px-4 py-2 rounded-lg transition-all duration-200
                  ${selectedProvider === provider.id 
                    ? 'bg-gray-600/40 cyber-text-primary border border-gray-500/50' 
                    : 'cyber-bg-panel cyber-text-secondary hover:bg-gray-700/20 border border-transparent'
                  }
                `}
                whileHover={{ scale: 1.02 }}
                whileTap={{ scale: 0.98 }}
              >
                <span className="text-base">{provider.icon}</span>
                <span className="cyber-mono text-sm">{provider.name}</span>
                {provider.configured && <Check className="w-3 h-3 text-green-400" />}
              </motion.button>
            ))}
          </div>

          {/* Configuration Button */}
          {selectedProviderData && !isConfigured && (
            <button
              onClick={handleConfigure}
              className="cyber-btn px-4 py-2 text-sm flex items-center space-x-2"
            >
              <Settings className="w-4 h-4" />
              <span>Setup {selectedProviderData.name}</span>
            </button>
          )}
        </div>

        {/* Loading State */}
        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <RefreshCw className="w-8 h-8 cyber-text-cyber animate-spin" />
            <span className="ml-3 cyber-text-secondary">Loading cloud data...</span>
          </div>
        ) : (
          <>
            {/* Main Content */}
            {isConfigured ? (
          <div className="grid lg:grid-cols-2 gap-8">
            {/* Upload Section */}
            <div>
              <h3 className="cyber-h3 text-base cyber-text-primary mb-4">Upload Files</h3>
              <div className="cyber-bg-panel p-6 rounded-lg text-center">
                <Upload className="w-12 h-12 cyber-text-cyber mx-auto mb-4" />
                <p className="cyber-body cyber-text-secondary mb-4">
                  Select .pixe files to backup to {selectedProviderData?.name || 'cloud storage'}
                </p>
                <button 
                  className="cyber-btn px-6 py-3 text-sm w-full mb-4"
                  onClick={() => document.getElementById('cloud-file-input')?.click()}
                  disabled={isUploading}
                >
                  {isUploading ? 'Uploading...' : 'Choose Files to Upload'}
                </button>
                <input
                  id="cloud-file-input"
                  type="file"
                  multiple
                  accept=".pixe"
                  className="hidden"
                  onChange={(e) => handleFileUpload(e.target.files)}
                />
                
                <label className="flex items-center justify-center space-x-2 cyber-mono text-xs cyber-text-secondary">
                  <input type="checkbox" className="w-3 h-3" />
                  <span>Auto-backup new files</span>
                </label>
              </div>
            </div>

            {/* Cloud Files */}
            <div>
              <h3 className="cyber-h3 text-base cyber-text-primary mb-4">
                Cloud Files ({cloudFiles.length})
              </h3>
              {cloudFiles.length > 0 ? (
                <div className="space-y-3 max-h-80 overflow-y-auto">
                  {cloudFiles.map((file, index) => (
                    <motion.div
                      key={file.id}
                      initial={{ opacity: 0, y: 10 }}
                      animate={{ opacity: 1, y: 0 }}
                      transition={{ duration: 0.3, delay: index * 0.05 }}
                      className="cyber-bg-panel p-4 rounded-lg flex items-center justify-between"
                    >
                      <div className="flex items-center space-x-3 flex-1 min-w-0">
                        <Cloud className="w-5 h-5 cyber-text-blue flex-shrink-0" />
                        <div className="min-w-0 flex-1">
                          <h4 className="cyber-body cyber-text-primary truncate text-sm">
                            {file.filename}
                          </h4>
                          <p className="cyber-mono text-xs cyber-text-secondary">
                            {formatFileSize(file.size)} • {formatDate(file.uploadedAt)}
                          </p>
                        </div>
                      </div>
                      
                      <button
                        className="p-2 cyber-text-secondary hover:cyber-text-primary transition-colors rounded"
                        title="Download from cloud"
                        onClick={() => handleDownload(file)}
                      >
                        <Download className="w-4 h-4" />
                      </button>
                    </motion.div>
                  ))}
                </div>
              ) : (
                <div className="cyber-bg-panel p-8 rounded-lg text-center">
                  <Cloud className="w-12 h-12 cyber-text-secondary mx-auto mb-3" />
                  <p className="cyber-body cyber-text-secondary">No files uploaded yet</p>
                </div>
              )}
            </div>
          </div>
        ) : (
          <div className="cyber-bg-panel p-8 rounded-lg text-center">
            <AlertCircle className="w-12 h-12 text-yellow-400 mx-auto mb-4" />
            <h3 className="cyber-h3 cyber-text-primary mb-2">Setup Required</h3>
            <p className="cyber-body cyber-text-secondary">
              {selectedProviderData 
                ? `Configure ${selectedProviderData.name} to start backing up files`
                : 'Select a cloud storage provider to get started'
              }
            </p>
          </div>
        )}
        </>
        )}
      </div>

      {/* Configuration Modal */}
      <CloudConfigModal
        isOpen={isConfigModalOpen}
        provider={selectedProvider}
        onClose={() => setIsConfigModalOpen(false)}
        onSuccess={handleConfigSuccess}
      />
    </div>
  )
}

export default CloudStorage
