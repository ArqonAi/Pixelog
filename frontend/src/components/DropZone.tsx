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

  const processDroppedItems = useCallback(async (items: DataTransferItemList): Promise<File[]> => {
    const files: File[] = []
    
    const processEntry = async (entry: FileSystemEntry): Promise<void> => {
      if (entry.isFile) {
        const fileEntry = entry as FileSystemFileEntry
        return new Promise<void>((resolve) => {
          fileEntry.file((file) => {
            files.push(file)
            resolve()
          })
        })
      } else if (entry.isDirectory) {
        const dirEntry = entry as FileSystemDirectoryEntry
        const reader = dirEntry.createReader()
        return new Promise<void>((resolve) => {
          reader.readEntries(async (entries) => {
            for (const entry of entries) {
              await processEntry(entry)
            }
            resolve()
          })
        })
      }
    }

    const promises: Promise<void>[] = []
    for (let i = 0; i < items.length; i++) {
      const item = items[i]
      if (item && item.webkitGetAsEntry) {
        const entry = item.webkitGetAsEntry()
        if (entry) {
          promises.push(processEntry(entry))
        }
      }
    }
    
    await Promise.all(promises)
    return files
  }, [])

  const handleDrop = useCallback(async (e: React.DragEvent<HTMLDivElement>): Promise<void> => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragOver(false)

    if (isDisabled) return

    // Handle both files and folders
    const files = await processDroppedItems(e.dataTransfer.items)
    if (files.length > 0) {
      const password = encryptionEnabled ? encryptionPassword : undefined
      onFileDrop(files, password)
    }
  }, [onFileDrop, isDisabled, encryptionEnabled, encryptionPassword, processDroppedItems])

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
    <div className="cyber-terminal h-full flex flex-col">
      <div className="cyber-terminal-header">
        <h2 className="cyber-h2 text-lg flex-1">Upload Files</h2>
        <div className="cyber-mono text-xs cyber-text-secondary">
          Max 100MB per file
        </div>
      </div>
      
      <div className="cyber-terminal-body flex-1 flex flex-col">
        <motion.div
          className={`
            relative rounded-lg p-8 transition-all duration-300
            ${isDragOver 
              ? 'bg-gray-700/20' 
              : 'bg-gray-900/20'
            }
            ${isDisabled 
              ? 'opacity-50 cursor-not-allowed' 
              : 'cursor-pointer hover:bg-gray-700/10'
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
            className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
            disabled={isDisabled}
          />
          <input
            type="file"
            // @ts-ignore - webkitdirectory is not in standard types
            webkitdirectory="true"
            directory="true"
            multiple
            onChange={handleFileSelect}
            className="absolute inset-0 w-full h-full opacity-0 cursor-pointer hidden"
            disabled={isDisabled}
            id="folder-input"
          />
          
          <label 
            htmlFor="file-input" 
            className={`block text-center ${isDisabled ? 'cursor-not-allowed' : 'cursor-pointer'}`}
          >
            <motion.div
              animate={isDragOver ? { scale: 1.1 } : { scale: 1 }}
              transition={{ duration: 0.3 }}
            >
              <div className="flex items-center justify-center mb-4">
                <Upload className="w-12 h-12 cyber-text-secondary" />
              </div>
              
              <h3 className="cyber-h3 text-xl mb-3 text-center">
                {isDragOver ? 'Drop your files/folders here!' : 'Drag & drop files or folders'}
              </h3>
              
              <p className="cyber-body cyber-text-secondary text-center mb-4">
                Support for documents, images, videos, and more
              </p>
              
              <div className="flex gap-2 justify-center mb-6">
                <button 
                  type="button" 
                  className="cyber-btn-secondary text-sm px-4 py-2"
                  onClick={() => (document.querySelector('input[type="file"]:not(#folder-input)') as HTMLInputElement)?.click()}
                  disabled={isDisabled}
                >
                  Select Files
                </button>
                <button 
                  type="button" 
                  className="cyber-btn-secondary text-sm px-4 py-2"
                  onClick={() => (document.getElementById('folder-input') as HTMLInputElement)?.click()}
                  disabled={isDisabled}
                >
                  Select Folder
                </button>
              </div>
            
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
            </motion.div>
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
                  ? 'bg-cyan-600' 
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
