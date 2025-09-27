import React from 'react'
import { motion } from 'framer-motion'
import { X, CheckCircle } from 'lucide-react'

const ProgressModal = ({ progress }) => {
  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
    >
      <motion.div
        initial={{ scale: 0.9, opacity: 0 }}
        animate={{ scale: 1, opacity: 1 }}
        exit={{ scale: 0.9, opacity: 0 }}
        className="bg-white dark:bg-gray-800 rounded-lg p-8 max-w-md w-full mx-4 shadow-2xl"
      >
        <div className="text-center">
          <div className="mb-6">
            {progress.percentage === 100 ? (
              <motion.div
                initial={{ scale: 0 }}
                animate={{ scale: 1 }}
                transition={{ type: "spring", duration: 0.6 }}
              >
                <CheckCircle className="w-16 h-16 text-green-500 mx-auto" />
              </motion.div>
            ) : (
              <motion.div
                animate={{ rotate: 360 }}
                transition={{ duration: 2, repeat: Infinity, ease: "linear" }}
                className="w-16 h-16 border-4 border-blue-500 border-t-transparent rounded-full mx-auto"
              />
            )}
          </div>

          <h3 className="text-xl font-semibold text-gray-900 dark:text-gray-100 mb-2">
            {progress.percentage === 100 ? 'Conversion Complete!' : 'Converting Files'}
          </h3>

          <p className="text-gray-600 dark:text-gray-300 mb-6">
            {progress.stage}: {progress.percentage}%
          </p>

          <div className="progress-bar mb-4">
            <motion.div
              className="progress-fill"
              initial={{ width: 0 }}
              animate={{ width: `${progress.percentage}%` }}
              transition={{ duration: 0.3, ease: "easeOut" }}
            />
          </div>

          {progress.message && (
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
              {progress.message}
            </p>
          )}

          <div className="space-y-2 text-left">
            <div className="flex justify-between items-center text-sm">
              <span className="text-gray-600 dark:text-gray-400">Progress:</span>
              <span className="font-medium text-gray-900 dark:text-gray-100">
                {progress.percentage}%
              </span>
            </div>
            <div className="flex justify-between items-center text-sm">
              <span className="text-gray-600 dark:text-gray-400">Current Stage:</span>
              <span className="font-medium text-gray-900 dark:text-gray-100">
                {progress.stage}
              </span>
            </div>
          </div>
        </div>
      </motion.div>
    </motion.div>
  )
}

export default ProgressModal