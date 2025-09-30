import React from 'react'
import { motion } from 'framer-motion'
import { CheckCircle, XCircle, AlertCircle, X } from 'lucide-react'

// ===== COMPONENT TYPES =====

interface ToastProps {
  readonly message: string
  readonly type?: 'success' | 'error' | 'warning' | 'info'
  readonly onClose: () => void
}

// ===== TOAST COMPONENT =====

const Toast: React.FC<ToastProps> = ({ message, type = 'success', onClose }) => {
  const icons = {
    success: CheckCircle,
    error: XCircle,
    warning: AlertCircle,
    info: CheckCircle, // fallback to success icon for info
  } as const

  const colors = {
    success: 'bg-green-50 dark:bg-green-900/20 text-green-800 dark:text-green-200 border-green-200 dark:border-green-800',
    error: 'bg-red-50 dark:bg-red-900/20 text-red-800 dark:text-red-200 border-red-200 dark:border-red-800',
    warning: 'bg-yellow-50 dark:bg-yellow-900/20 text-yellow-800 dark:text-yellow-200 border-yellow-200 dark:border-yellow-800',
    info: 'bg-gray-50 dark:bg-gray-900/20 text-gray-800 dark:text-gray-200 border-gray-200 dark:border-gray-800',
  } as const

  const Icon = icons[type] || CheckCircle

  return (
    <motion.div
      initial={{ opacity: 0, y: 50, scale: 0.9 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: 20, scale: 0.9 }}
      className={`fixed bottom-4 right-4 z-50 flex items-center p-4 rounded-lg border shadow-lg ${colors[type]} min-w-[300px] max-w-md`}
    >
      <Icon className="w-5 h-5 mr-3 flex-shrink-0" />
      <p className="flex-1 text-sm font-medium">{message}</p>
      <button
        onClick={onClose}
        className="ml-3 flex-shrink-0 p-1 hover:bg-black/10 rounded-full transition-colors"
        aria-label="Close notification"
      >
        <X className="w-4 h-4" />
      </button>
    </motion.div>
  )
}

export default Toast
