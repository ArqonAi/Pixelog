import React, { useState, useCallback } from 'react'
import { motion } from 'framer-motion'
import { Upload, File, Image, Music, Video } from 'lucide-react'

const DropZone = ({ onFileDrop, isDisabled }) => {
  const [isDragOver, setIsDragOver] = useState(false)

  const handleDragOver = useCallback((e) => {
    e.preventDefault()
    e.stopPropagation()
    if (!isDisabled) {
      setIsDragOver(true)
    }
  }, [isDisabled])

  const handleDragLeave = useCallback((e) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragOver(false)
  }, [])

  const handleDrop = useCallback((e) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragOver(false)

    if (isDisabled) return

    const files = Array.from(e.dataTransfer.files)
    if (files.length > 0) {
      onFileDrop(files)
    }
  }, [onFileDrop, isDisabled])

  const handleFileSelect = useCallback((e) => {
    const files = Array.from(e.target.files)
    if (files.length > 0) {
      onFileDrop(files)
    }
  }, [onFileDrop])

  const getFileIcon = (file) => {
    const type = file.type.split('/')[0]
    switch (type) {
      case 'image': return <Image className="w-6 h-6" />
      case 'audio': return <Music className="w-6 h-6" />
      case 'video': return <Video className="w-6 h-6" />
      default: return <File className="w-6 h-6" />
    }
  }

  return (
    <div className="card">
      <h2 className="text-2xl font-semibold text-gray-900 dark:text-gray-100 mb-6">
        Create PixeFile
      </h2>
      
      <motion.div
        className={`drag-zone ${isDragOver ? 'drag-over' : ''} ${isDisabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}`}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        whileHover={!isDisabled ? { scale: 1.02 } : {}}
        whileTap={!isDisabled ? { scale: 0.98 } : {}}
      >
        <input
          type="file"
          multiple
          onChange={handleFileSelect}
          className="hidden"
          id="file-input"
          disabled={isDisabled}
        />
        
        <label htmlFor="file-input" className={`block text-center ${isDisabled ? 'cursor-not-allowed' : 'cursor-pointer'}`}>
          <motion.div
            animate={isDragOver ? { scale: 1.1 } : { scale: 1 }}
            className="mb-4"
          >
            <Upload className="w-16 h-16 text-gray-400 mx-auto mb-4" />
          </motion.div>
          
          <h3 className="text-xl font-medium text-gray-900 dark:text-gray-100 mb-2">
            {isDragOver ? 'Drop files here!' : 'Drag & drop files'}
          </h3>
          
          <p className="text-gray-500 dark:text-gray-400 mb-4">
            or click to select files
          </p>
          
          <div className="flex justify-center space-x-4 mb-4">
            <div className="flex items-center text-gray-400">
              <File className="w-5 h-5 mr-1" />
              <span className="text-sm">Text</span>
            </div>
            <div className="flex items-center text-gray-400">
              <Image className="w-5 h-5 mr-1" />
              <span className="text-sm">Images</span>
            </div>
            <div className="flex items-center text-gray-400">
              <Music className="w-5 h-5 mr-1" />
              <span className="text-sm">Audio</span>
            </div>
            <div className="flex items-center text-gray-400">
              <Video className="w-5 h-5 mr-1" />
              <span className="text-sm">Video</span>
            </div>
          </div>
          
          <div className="text-xs text-gray-400">
            Maximum file size: 100MB per file
          </div>
        </label>
      </motion.div>

      <div className="mt-6 p-4 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
        <h4 className="font-medium text-blue-900 dark:text-blue-100 mb-2">
          💡 How it works:
        </h4>
        <ul className="text-sm text-blue-700 dark:text-blue-300 space-y-1">
          <li>• Files are converted to QR code sequences</li>
          <li>• Assembled into low-framerate MP4 videos</li>
          <li>• Compressed, encrypted, and portable</li>
          <li>• Fully searchable and streamable</li>
        </ul>
      </div>
    </div>
  )
}

export default DropZone