import React from 'react'
import { motion } from 'framer-motion'
import { Download, Trash2, Eye, RefreshCw, FileVideo, Calendar, HardDrive } from 'lucide-react'
import { pixelogApi } from '@/services/api'
import type { PixeFile } from '@/types/api'

// ===== COMPONENT TYPES =====

interface FileListProps {
  readonly files: readonly PixeFile[]
  readonly onRefresh: () => void
  readonly onToast: (message: string, type?: 'success' | 'error' | 'warning' | 'info') => void
}

// ===== FILELIST COMPONENT =====

const FileList: React.FC<FileListProps> = ({ files, onRefresh, onToast }) => {
  const handleDownload = async (file: PixeFile): Promise<void> => {
    try {
      const blob = await pixelogApi.downloadPixeFile(file.id)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = file.name
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      
      onToast(`Downloaded ${file.name}`)
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Unknown error'
      onToast(`Failed to download ${file.name}: ${errorMessage}`, 'error')
    }
  }

  const handleDelete = async (file: PixeFile): Promise<void> => {
    if (!confirm(`Are you sure you want to delete ${file.name}?`)) {
      return
    }

    try {
      await pixelogApi.deletePixeFile(file.id)
      onToast(`Deleted ${file.name}`)
      onRefresh()
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Unknown error'
      onToast(`Failed to delete ${file.name}: ${errorMessage}`, 'error')
    }
  }

  const handleExtract = async (file: PixeFile): Promise<void> => {
    try {
      await pixelogApi.extractPixeFile(file.name)
      onToast(`Extraction started for ${file.name}`)
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Unknown error'
      onToast(`Failed to extract ${file.name}: ${errorMessage}`, 'error')
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

  return (
    <div className="card">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
          PixeFiles ({files.length})
        </h2>
        <button
          onClick={onRefresh}
          className="btn-secondary"
          aria-label="Refresh file list"
        >
          <RefreshCw className="w-4 h-4" />
          Refresh
        </button>
      </div>

      {files.length === 0 ? (
        <div className="text-center py-12 text-gray-500 dark:text-gray-400">
          <FileVideo className="w-16 h-16 mx-auto mb-4 opacity-50" />
          <p className="text-lg mb-2">No PixeFiles yet</p>
          <p className="text-sm">Upload some files to get started!</p>
        </div>
      ) : (
        <div className="grid gap-4">
          {files.map((file, index) => (
            <motion.div
              key={file.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3, delay: index * 0.1 }}
              className="p-4 border border-gray-200 dark:border-gray-700 rounded-lg hover:border-gray-300 dark:hover:border-gray-600 transition-colors"
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-3 flex-1 min-w-0">
                  <FileVideo className="w-8 h-8 text-blue-500 flex-shrink-0" />
                  <div className="min-w-0 flex-1">
                    <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 truncate">
                      {file.name}
                    </h3>
                    <div className="flex items-center space-x-4 text-sm text-gray-500 dark:text-gray-400 mt-1">
                      <div className="flex items-center">
                        <HardDrive className="w-4 h-4 mr-1" />
                        {file.size}
                      </div>
                      <div className="flex items-center">
                        <Calendar className="w-4 h-4 mr-1" />
                        {formatDate(file.created_at)}
                      </div>
                    </div>
                  </div>
                </div>

                <div className="flex items-center space-x-2 ml-4">
                  <button
                    onClick={() => void handleExtract(file)}
                    className="p-2 text-gray-500 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded-lg transition-colors"
                    title="Extract contents"
                    aria-label={`Extract contents of ${file.name}`}
                  >
                    <Eye className="w-5 h-5" />
                  </button>
                  
                  <button
                    onClick={() => void handleDownload(file)}
                    className="p-2 text-gray-500 hover:text-green-600 hover:bg-green-50 dark:hover:bg-green-900/20 rounded-lg transition-colors"
                    title="Download file"
                    aria-label={`Download ${file.name}`}
                  >
                    <Download className="w-5 h-5" />
                  </button>
                  
                  <button
                    onClick={() => void handleDelete(file)}
                    className="p-2 text-gray-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
                    title="Delete file"
                    aria-label={`Delete ${file.name}`}
                  >
                    <Trash2 className="w-5 h-5" />
                  </button>
                </div>
              </div>
            </motion.div>
          ))}
        </div>
      )}
    </div>
  )
}

export default FileList
