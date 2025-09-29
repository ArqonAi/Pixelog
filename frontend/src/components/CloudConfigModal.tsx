import React, { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X, Check, AlertCircle, Settings, Eye, EyeOff } from 'lucide-react'
import { cloudApi, type CloudProvider } from '@/services/api'

// ===== COMPONENT TYPES =====

interface CloudConfigModalProps {
  readonly isOpen: boolean
  readonly provider: string
  readonly onClose: () => void
  readonly onSuccess: () => void
}

interface FormState {
  readonly accessKey: string
  readonly secretKey: string
  readonly region: string
  readonly bucketName: string
  readonly serviceAccountJSON: string
}

// ===== CLOUD CONFIG MODAL =====

const CloudConfigModal: React.FC<CloudConfigModalProps> = ({ 
  isOpen, 
  provider, 
  onClose, 
  onSuccess 
}) => {
  const [formState, setFormState] = useState<FormState>({
    accessKey: '',
    secretKey: '',
    region: 'us-west-2',
    bucketName: '',
    serviceAccountJSON: ''
  })
  
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false)
  const [isTesting, setIsTesting] = useState<boolean>(false)
  const [showSecrets, setShowSecrets] = useState<boolean>(false)
  const [error, setError] = useState<string>('')
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)

  const handleInputChange = (field: keyof FormState) => (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
  ): void => {
    setFormState(prev => ({ ...prev, [field]: e.target.value }))
    setError('')
    setTestResult(null)
  }

  const getProviderConfig = (): CloudProvider => {
    const base = {
      provider: provider as CloudProvider['provider'],
      bucketName: formState.bucketName
    }

    switch (provider) {
      case 'aws':
        return {
          ...base,
          accessKey: formState.accessKey,
          secretKey: formState.secretKey,
          region: formState.region
        }
      case 'gcp':
        return {
          ...base,
          serviceAccountJSON: formState.serviceAccountJSON
        }
      case 'azure':
        return {
          ...base,
          accessKey: formState.accessKey, // Account name
          secretKey: formState.secretKey  // Account key
        }
      case 'digitalocean':
        return {
          ...base,
          accessKey: formState.accessKey,
          secretKey: formState.secretKey,
          region: formState.region
        }
      default:
        return base
    }
  }

  const handleTest = async (): Promise<void> => {
    setIsTesting(true)
    setError('')
    setTestResult(null)

    try {
      const config = getProviderConfig()
      const result = await cloudApi.testConnection(config)
      setTestResult(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Connection test failed')
    } finally {
      setIsTesting(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent): Promise<void> => {
    e.preventDefault()
    setIsSubmitting(true)
    setError('')

    try {
      const config = getProviderConfig()
      const result = await cloudApi.configureProvider(config)
      
      if (result.success) {
        onSuccess()
        onClose()
      } else {
        setError(result.message || 'Configuration failed')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Configuration failed')
    } finally {
      setIsSubmitting(false)
    }
  }

  const renderProviderFields = (): JSX.Element => {
    switch (provider) {
      case 'aws':
      case 'digitalocean':
        return (
          <>
            <div>
              <label className="block cyber-mono text-sm cyber-text-primary mb-2">
                Access Key ID
              </label>
              <input
                type={showSecrets ? 'text' : 'password'}
                value={formState.accessKey}
                onChange={handleInputChange('accessKey')}
                className="cyber-input w-full"
                placeholder="AKIA..."
                required
              />
            </div>
            
            <div>
              <label className="block cyber-mono text-sm cyber-text-primary mb-2">
                Secret Access Key
              </label>
              <div className="relative">
                <input
                  type={showSecrets ? 'text' : 'password'}
                  value={formState.secretKey}
                  onChange={handleInputChange('secretKey')}
                  className="cyber-input w-full pr-10"
                  placeholder="Secret key..."
                  required
                />
                <button
                  type="button"
                  onClick={() => setShowSecrets(!showSecrets)}
                  className="absolute right-3 top-1/2 transform -translate-y-1/2 cyber-text-secondary hover:cyber-text-primary"
                >
                  {showSecrets ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>
            
            <div>
              <label className="block cyber-mono text-sm cyber-text-primary mb-2">
                Region
              </label>
              <input
                type="text"
                value={formState.region}
                onChange={handleInputChange('region')}
                className="cyber-input w-full"
                placeholder="us-west-2"
                required
              />
            </div>
          </>
        )

      case 'gcp':
        return (
          <div>
            <label className="block cyber-mono text-sm cyber-text-primary mb-2">
              Service Account JSON
            </label>
            <textarea
              value={formState.serviceAccountJSON}
              onChange={handleInputChange('serviceAccountJSON')}
              className="cyber-input w-full h-32 resize-none"
              placeholder="Paste your service account JSON here..."
              required
            />
            <p className="cyber-mono text-xs cyber-text-secondary mt-1">
              Download from Google Cloud Console → IAM & Admin → Service Accounts
            </p>
          </div>
        )

      case 'azure':
        return (
          <>
            <div>
              <label className="block cyber-mono text-sm cyber-text-primary mb-2">
                Account Name
              </label>
              <input
                type="text"
                value={formState.accessKey}
                onChange={handleInputChange('accessKey')}
                className="cyber-input w-full"
                placeholder="mystorageaccount"
                required
              />
            </div>
            
            <div>
              <label className="block cyber-mono text-sm cyber-text-primary mb-2">
                Account Key
              </label>
              <input
                type={showSecrets ? 'text' : 'password'}
                value={formState.secretKey}
                onChange={handleInputChange('secretKey')}
                className="cyber-input w-full"
                placeholder="Account key..."
                required
              />
            </div>
          </>
        )

      default:
        return <></>
    }
  }

  const getProviderName = (id: string): string => {
    const names: Record<string, string> = {
      aws: 'AWS S3',
      gcp: 'Google Cloud Storage',
      azure: 'Azure Blob Storage',
      digitalocean: 'DigitalOcean Spaces'
    }
    return names[id] || id
  }

  return (
    <AnimatePresence>
      {isOpen && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4"
          onClick={onClose}
        >
          <motion.div
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9 }}
            className="cyber-terminal max-w-md w-full"
            onClick={e => e.stopPropagation()}
          >
            <div className="cyber-terminal-header">
              <Settings className="w-5 h-5 cyber-text-cyber" />
              <h2 className="cyber-h2 text-lg flex-1">Setup {getProviderName(provider)}</h2>
              <button
                onClick={onClose}
                className="p-2 cyber-text-secondary hover:cyber-text-primary transition-colors"
                aria-label="Close modal"
              >
                <X className="w-5 h-5" />
              </button>
            </div>
            
            <div className="cyber-terminal-body">
              <form onSubmit={handleSubmit} className="space-y-4">
                {renderProviderFields()}
                
                <div>
                  <label className="block cyber-mono text-sm cyber-text-primary mb-2">
                    Bucket Name
                  </label>
                  <input
                    type="text"
                    value={formState.bucketName}
                    onChange={handleInputChange('bucketName')}
                    className="cyber-input w-full"
                    placeholder="my-pixelog-bucket"
                    required
                  />
                  <p className="cyber-mono text-xs cyber-text-secondary mt-1">
                    Bucket must already exist in your {getProviderName(provider)} account
                  </p>
                </div>

                {/* Test Connection */}
                <div className="flex gap-3">
                  <button
                    type="button"
                    onClick={handleTest}
                    disabled={isTesting}
                    className="cyber-btn cyber-btn-secondary px-4 py-2 text-sm flex items-center space-x-2 flex-1"
                  >
                    <Settings className="w-4 h-4" />
                    <span>{isTesting ? 'Testing...' : 'Test Connection'}</span>
                  </button>
                  
                  <button
                    type="submit"
                    disabled={isSubmitting}
                    className="cyber-btn px-4 py-2 text-sm flex items-center space-x-2 flex-1"
                  >
                    <Check className="w-4 h-4" />
                    <span>{isSubmitting ? 'Saving...' : 'Save Config'}</span>
                  </button>
                </div>

                {/* Status Messages */}
                {testResult && (
                  <div className={`p-3 rounded-lg flex items-center space-x-2 ${
                    testResult.success 
                      ? 'bg-green-400/10 border border-green-400/30' 
                      : 'bg-red-400/10 border border-red-400/30'
                  }`}>
                    {testResult.success ? (
                      <Check className="w-4 h-4 text-green-400" />
                    ) : (
                      <AlertCircle className="w-4 h-4 text-red-400" />
                    )}
                    <span className="cyber-mono text-xs cyber-text-primary">
                      {testResult.message}
                    </span>
                  </div>
                )}

                {error && (
                  <div className="p-3 rounded-lg bg-red-400/10 border border-red-400/30 flex items-center space-x-2">
                    <AlertCircle className="w-4 h-4 text-red-400" />
                    <span className="cyber-mono text-xs cyber-text-primary">{error}</span>
                  </div>
                )}
              </form>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

export default CloudConfigModal
