import React, { useState, useCallback } from 'react'
import { motion } from 'framer-motion'
import { Upload, File, Image, Music, Video, Lock, LockOpen, Eye, EyeOff } from 'lucide-react'

// ===== COMPONENT TYPES =====

interface DropZoneProps {
  readonly onFileDrop: (files: readonly File[], encryptionPassword?: string) => void
  readonly isDisabled?: boolean
}

// ===== DROPZONE COMPONENT =====

const DropZone: React.FC<DropZoneProps> = ({ onFileDrop, isDisabled = false }) => {
  const [isDragOver, setIsDragOver] = useState<boolean>(false)
  const [encryptionEnabled, setEncryptionEnabled] = useState<boolean>(false)
  const [encryptionPassword, setEncryptionPassword] = useState<string>('')
  const [showPassword, setShowPassword] = useState<boolean>(false)

  const handleDragOver = useCallback((e: React.DragEvent<HTMLDivElement>): void => {
    e.preventDefault()
    e.stopPropagation()
    if (!isDisabled) {
      setIsDragOver(true)
    }
  }, [isDisabled])

  const handleDragLeave = useCallback((e: React.DragEvent<HTMLDivElement>): void => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragOver(false)
  }, [])

  const handleDrop = useCallback((e: React.DragEvent<HTMLDivElement>): void => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragOver(false)

    if (isDisabled) return

    const files = Array.from(e.dataTransfer.files)
    if (files.length > 0) {
      const password = encryptionEnabled ? encryptionPassword : undefined
      onFileDrop(files, password)
    }
  }, [onFileDrop, isDisabled, encryptionEnabled, encryptionPassword])

  const handleFileSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>): void => {
    const files = Array.from(e.target.files ?? [])
    if (files.length > 0) {
      const password = encryptionEnabled ? encryptionPassword : undefined
      onFileDrop(files, password)
    }
  }, [onFileDrop, encryptionEnabled, encryptionPassword])

  const getFileIcon = (fileType: string): JSX.Element => {
    const type = fileType.split('/')[0]
    switch (type) {
      case 'image': 
        return <Image className="w-6 h-6" />
      case 'audio': 
        return <Music className="w-6 h-6" />
      case 'video': 
        return <Video className="w-6 h-6" />
      default: 
        return <File className="w-6 h-6" />
    }
  }

  return (
    <div className="cyber-terminal">
      <div className="cyber-terminal-header">
        <h2 className="cyber-h2 text-lg flex-1">Upload Files</h2>
        <div className="cyber-mono text-xs cyber-text-secondary">
          Max 100MB per file
        </div>
      </div>
      
      <div className="cyber-terminal-body">
        <motion.div
          className={`
            relative rounded-lg p-8 transition-all duration-300
            ${isDragOver 
              ? 'bg-cyan-400/10' 
              : 'bg-gray-900/20'
            }
            ${isDisabled 
              ? 'opacity-50 cursor-not-allowed' 
              : 'cursor-pointer hover:bg-cyan-400/5'
            }
          `}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
          whileHover={!isDisabled ? { scale: 1.01 } : {}}
          whileTap={!isDisabled ? { scale: 0.99 } : {}}
        >
          <input
            type="file"
            multiple
            onChange={handleFileSelect}
            className="hidden"
            id="file-input"
            disabled={isDisabled}
            accept="*/*"
          />
          
          <label 
            htmlFor="file-input" 
            className={`block text-center ${isDisabled ? 'cursor-not-allowed' : 'cursor-pointer'}`}
          >
            <motion.div
              animate={isDragOver ? { scale: 1.1 } : { scale: 1 }}
              transition={{ duration: 0.3 }}
              className="mb-6"
            >
              <Upload className="w-16 h-16 cyber-text-cyber mx-auto mb-4" />
            </motion.div>
            
            <h3 className="cyber-h3 text-xl cyber-text-primary mb-3">
              {isDragOver ? 'Drop files here' : 'Upload your files'}
            </h3>
            
            <p className="cyber-body cyber-text-secondary mb-6">
              Drag and drop files or click to browse
            </p>
            
            {/* File Type Support */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
              <div className="cyber-bg-panel p-3 rounded text-center">
                <File className="w-5 h-5 cyber-text-cyber mx-auto mb-1" />
                <span className="cyber-mono text-xs cyber-text-secondary block">Documents</span>
              </div>
              <div className="cyber-bg-panel p-3 rounded text-center">
                <Image className="w-5 h-5 cyber-text-cyber mx-auto mb-1" />
                <span className="cyber-mono text-xs cyber-text-secondary block">Images</span>
              </div>
              <div className="cyber-bg-panel p-3 rounded text-center">
                <Music className="w-5 h-5 cyber-text-cyber mx-auto mb-1" />
                <span className="cyber-mono text-xs cyber-text-secondary block">Audio</span>
              </div>
              <div className="cyber-bg-panel p-3 rounded text-center">
                <Video className="w-5 h-5 cyber-text-cyber mx-auto mb-1" />
                <span className="cyber-mono text-xs cyber-text-secondary block">Video</span>
              </div>
            </div>
          </label>
        </motion.div>
        
        {/* Encryption Options */}
        <div className="mt-6 space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              {encryptionEnabled ? (
                <Lock className="w-4 h-4 cyber-text-cyber" />
              ) : (
                <LockOpen className="w-4 h-4 cyber-text-secondary" />
              )}
              <span className="cyber-body cyber-text-primary">
                Encryption
              </span>
            </div>
            <button
              type="button"
              onClick={() => setEncryptionEnabled(!encryptionEnabled)}
              className={`
                relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-200
                ${encryptionEnabled 
                  ? 'bg-green-600' 
                  : 'bg-gray-600'
                }
              `}
              disabled={isDisabled}
            >
              <span
                className={`
                  inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-200
                  ${encryptionEnabled ? 'translate-x-6' : 'translate-x-1'}
                `}
              />
            </button>
          </div>
          
          {encryptionEnabled && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.3 }}
              className="space-y-3"
            >
              <div>
                <label className="block cyber-body text-sm cyber-text-secondary mb-2">
                  Encryption Password
                </label>
                <div className="relative">
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={encryptionPassword}
                    onChange={(e) => setEncryptionPassword(e.target.value)}
                    placeholder="Enter secure password..."
                    className="cyber-input w-full pr-10"
                    disabled={isDisabled}
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 cyber-text-secondary hover:cyber-text-primary transition-colors"
                    disabled={isDisabled}
                  >
                    {showPassword ? (
                      <EyeOff className="w-4 h-4" />
                    ) : (
                      <Eye className="w-4 h-4" />
                    )}
                  </button>
                </div>
              </div>
              
              <div className="flex items-start gap-2 cyber-bg-panel p-3 rounded-lg">
                <Lock className="w-4 h-4 cyber-text-amber mt-0.5 flex-shrink-0" />
                <div className="text-xs cyber-text-secondary">
                  <strong className="cyber-text-amber">Secure Encryption:</strong> Files will be encrypted using AES-256-GCM. 
                  Store your password safely - lost passwords cannot be recovered.
                </div>
              </div>
            </motion.div>
          )}
        </div>
      </div>
    </div>
  )
}

export default DropZone
