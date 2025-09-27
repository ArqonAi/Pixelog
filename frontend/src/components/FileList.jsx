import React from 'react'
import { motion } from 'framer-motion'
import { Download, Trash2, Eye, RefreshCw, FileVideo } from 'lucide-react'

const FileList = ({ files, onRefresh, onToast }) => {
  const handleDownload = async (file) => {
    try {
      const response = await fetch(`/api/v1/download/${file.id}`)
      if (!response.ok) throw new Error('Download failed')
      
      const blob = await response.blob()
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.style.display = 'none'
      a.href = url
      a.download = file.name
      document.body.appendChild(a)
      a.click()
      window.URL.revokeObjectURL(url)
      document.body.removeChild(a)
      
      onToast('File downloaded successfully!')
    } catch (error) {
      onToast(`Download failed: ${error.message}`, 'error')
    }
  }

  const handleDelete = async (file) => {
    if (!confirm(`Are you sure you want to delete ${file.name}?`)) return
    
    try {
      const response = await fetch(`/api/v1/pixefile/${file.id}`, {
        method: 'DELETE'
      })
      if (!response.ok) throw new Error('Delete failed')
      
      onToast('File deleted successfully!')
      onRefresh()
    } catch (error) {
      onToast(`Delete failed: ${error.message}`, 'error')
    }
  }

  const formatDate = (dateString) => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    })
  }

  return (
    <div className="card">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
          PixeFiles
        </h2>
        <button
          onClick={onRefresh}
          className="btn-secondary"
        >
          <RefreshCw className="w-4 h-4" />
          Refresh
        </button>
      </div>

      {files.length === 0 ? (
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="text-center py-12"
        >
          <FileVideo className="w-16 h-16 text-gray-300 dark:text-gray-600 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">
            No PixeFiles yet
          </h3>
          <p className="text-gray-500 dark:text-gray-400">
            Upload some files to create your first PixeFile!
          </p>
        </motion.div>
      ) : (
        <div className="space-y-3">
          {files.map((file, index) => (
            <motion.div
              key={file.id}
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: index * 0.1 }}
              className="flex items-center justify-between p-4 bg-gray-50 dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-600 hover:border-blue-300 dark:hover:border-blue-600 transition-colors"
            >
              <div className="flex items-center space-x-3">
                <div className="flex-shrink-0">
                  <FileVideo className="w-8 h-8 text-blue-500" />
                </div>
                <div>
                  <h4 className="font-medium text-gray-900 dark:text-gray-100">
                    {file.name}
                  </h4>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    {file.size} • {formatDate(file.created_at)}
                  </p>
                </div>
              </div>

              <div className="flex items-center space-x-2">
                <button
                  onClick={() => handleDownload(file)}
                  className="p-2 text-gray-500 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded-lg transition-colors"
                  title="Download"
                >
                  <Download className="w-4 h-4" />
                </button>
                <button
                  onClick={() => handleDelete(file)}
                  className="p-2 text-gray-500 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
                  title="Delete"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </motion.div>
          ))}
        </div>
      )}

      {files.length > 0 && (
        <div className="mt-6 p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
          <div className="text-sm text-gray-600 dark:text-gray-300">
            <strong>{files.length}</strong> PixeFile{files.length !== 1 ? 's' : ''} • 
            Total size: <strong>
              {files.reduce((acc, file) => {
                const size = parseFloat(file.size.split(' ')[0])
                const unit = file.size.split(' ')[1]
                if (unit === 'KB') return acc + size / 1024
                if (unit === 'MB') return acc + size
                if (unit === 'GB') return acc + size * 1024
                return acc + size / (1024 * 1024)
              }, 0).toFixed(2)} MB
            </strong>
          </div>
        </div>
      )}
    </div>
  )
}

export default FileList