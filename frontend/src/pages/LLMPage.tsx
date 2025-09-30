import React, { useState } from 'react'
import { motion } from 'framer-motion'
import { Brain, Upload, Key, Download, AlertTriangle, Eye, EyeOff, ArrowLeft } from 'lucide-react'

const LLMPage: React.FC = () => {
  const [files, setFiles] = useState<File[]>([])
  const [decryptionKey, setDecryptionKey] = useState<string>('')
  const [showKey, setShowKey] = useState<boolean>(false)
  const [isProcessing, setIsProcessing] = useState<boolean>(false)

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selectedFiles = Array.from(e.target.files || [])
    setFiles(prev => [...prev, ...selectedFiles])
  }

  const processFiles = async () => {
    setIsProcessing(true)
    await new Promise(resolve => setTimeout(resolve, 2000))
    setIsProcessing(false)
  }

  return (
    <div className="min-h-screen cyber-bg-void">
      <div className="cyber-bg-panel border-b border-gray-800/50">
        <div className="container mx-auto px-4 py-6">
          <div className="flex items-center gap-4">
            <a href="/" className="cyber-text-secondary hover:cyber-text-primary transition-colors">
              <ArrowLeft className="w-5 h-5" />
            </a>
            <div className="flex items-center gap-3">
              <Brain className="w-8 h-8 cyber-text-cyber" />
              <div>
                <h1 className="cyber-h1 text-3xl">LLM Integration</h1>
                <p className="cyber-body cyber-text-secondary">Process .pixe files for direct LLM consumption</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <main className="container mx-auto px-4 py-8 max-w-4xl">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          className="cyber-terminal"
        >
          <div className="cyber-terminal-header">
            <h2 className="cyber-h2 text-lg">Process .pixe Files</h2>
          </div>
          
          <div className="cyber-terminal-body space-y-6">
            <div>
              <label className="block cyber-body cyber-text-primary mb-3">
                <Upload className="w-4 h-4 inline mr-2" />
                Upload .pixe Files
              </label>
              <input
                type="file"
                multiple
                accept=".pixe,.mp4"
                onChange={handleFileUpload}
                className="cyber-input w-full"
              />
            </div>

            <div>
              <label className="block cyber-body cyber-text-primary mb-3">
                <Key className="w-4 h-4 inline mr-2" />
                Decryption Key (if encrypted)
              </label>
              <div className="relative">
                <input
                  type={showKey ? 'text' : 'password'}
                  value={decryptionKey}
                  onChange={(e) => setDecryptionKey(e.target.value)}
                  placeholder="Enter decryption password..."
                  className="cyber-input w-full pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowKey(!showKey)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 cyber-text-secondary hover:cyber-text-primary transition-colors"
                >
                  {showKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>

            <button
              onClick={processFiles}
              disabled={files.length === 0 || isProcessing}
              className="cyber-btn w-full flex items-center justify-center gap-2"
            >
              {isProcessing ? (
                <>
                  <div className="w-4 h-4 border-2 border-cyan-400 border-t-transparent rounded-full animate-spin"></div>
                  Processing...
                </>
              ) : (
                <>
                  <Brain className="w-4 h-4" />
                  Process for LLM ({files.length} files)
                </>
              )}
            </button>

            <div className="cyber-bg-panel p-4 rounded-lg border-l-4 border-yellow-500">
              <div className="flex items-start gap-2">
                <AlertTriangle className="w-5 h-5 text-yellow-500 mt-0.5 flex-shrink-0" />
                <div className="text-sm">
                  <p className="cyber-text-primary font-medium mb-1">Security Notice</p>
                  <p className="cyber-text-secondary">
                    Files are processed locally. Decryption keys are not stored.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </motion.div>
      </main>
    </div>
  )
}

export default LLMPage
