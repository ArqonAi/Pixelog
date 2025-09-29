import React, { useState, useCallback } from 'react'
import { motion } from 'framer-motion'
import { Upload, File, Image, Music, Video } from 'lucide-react'

// ===== COMPONENT TYPES =====

interface DropZoneProps {
  readonly onFileDrop: (files: readonly File[]) => void
  readonly isDisabled?: boolean
}

// ===== DROPZONE COMPONENT =====

const DropZone: React.FC<DropZoneProps> = ({ onFileDrop, isDisabled = false }) => {
  const [isDragOver, setIsDragOver] = useState<boolean>(false)

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
      onFileDrop(files)
    }
  }, [onFileDrop, isDisabled])

  const handleFileSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>): void => {
    const files = Array.from(e.target.files ?? [])
    if (files.length > 0) {
      onFileDrop(files)
    }
  }, [onFileDrop])

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
            
            <div className="cyber-mono text-xs cyber-text-tertiary">
              All file types supported • 100MB maximum per file
            </div>
          </label>
        </motion.div>
      </div>
    </div>
  )
}

export default DropZone
