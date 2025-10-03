import React, { useState } from 'react'
import { Brain, ArrowLeft, Upload, MessageSquare, Settings, Key, Eye, EyeOff, Send, FileText, Bot, User, Sliders, Zap, Trash2, Power, PowerOff, Download, RefreshCw } from 'lucide-react'

interface ProcessedMemory {
  id: string
  filename: string
  chunks: number
  size: number
  status: string
  encrypted: boolean
}

interface ChatMessage {
  id: string
  type: 'user' | 'assistant'
  content: string
  sources?: Array<{
    filename: string
    frame_number: number
    relevance: number
  }>
}

interface PixelogFile {
  id: string
  name: string
  size: string | number
  created_at: string
  path: string
}

interface ProgressUpdate {
  stage?: string
  percentage?: number
  message?: string
  status?: string
  job_id?: string
}

const LLMPage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'upload' | 'chat' | 'create'>('upload')
  const [files, setFiles] = useState<File[]>([])
  const [decryptionKey, setDecryptionKey] = useState<string>('')
  const [showKey, setShowKey] = useState<boolean>(false)
  const [isProcessing, setIsProcessing] = useState<boolean>(false)
  const [processedMemories, setProcessedMemories] = useState<ProcessedMemory[]>(() => {
    const saved = localStorage.getItem('pixelog-llm-memories')
    return saved ? JSON.parse(saved) : []
  })
  const [connectedMemories, setConnectedMemories] = useState<Set<string>>(new Set())
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>(() => {
    const saved = localStorage.getItem('pixelog-llm-chat')
    return saved ? JSON.parse(saved) : []
  })
  const [chatInput, setChatInput] = useState<string>('')
  const [isThinking, setIsThinking] = useState<boolean>(false)
  
  // AI Provider Settings
  const [selectedProvider, setSelectedProvider] = useState<string>(() => {
    return localStorage.getItem('pixelog-llm-provider') || 'openrouter'
  })
  const [selectedModel, setSelectedModel] = useState<string>(() => {
    return localStorage.getItem('pixelog-llm-model') || 'deepseek/deepseek-chat'
  })
  const [apiKey, setApiKey] = useState<string>(() => {
    return localStorage.getItem('pixelog-llm-apikey') || ''
  })
  const [showApiKey, setShowApiKey] = useState<boolean>(false)
  const [showSettings, setShowSettings] = useState<boolean>(false)
  const [showExportModal, setShowExportModal] = useState<boolean>(false)
  const [exportEncryptionKey, setExportEncryptionKey] = useState<string>('')
  const [isExporting, setIsExporting] = useState<boolean>(false)
  const [exportStep, setExportStep] = useState<'format' | 'encryption'>('format')
  const [selectedExportFormat, setSelectedExportFormat] = useState<'pixe' | 'txt'>('pixe')
  
  // Create tab state
  const [pixeFiles, setPixeFiles] = useState<PixelogFile[]>([])
  const [isLoadingFiles, setIsLoadingFiles] = useState<boolean>(false)
  const [isConverting, setIsConverting] = useState<boolean>(false)
  const [conversionProgress, setConversionProgress] = useState<ProgressUpdate | null>(null)

  // AI Provider configurations - REAL VERIFIED WORKING MODELS
  const aiProviders = {
    openai: {
      name: 'OpenAI',
      models: ['gpt-4o', 'gpt-4o-mini', 'gpt-4-turbo', 'gpt-4', 'gpt-3.5-turbo'],
      keyPlaceholder: 'sk-...',
      website: 'https://platform.openai.com/api-keys'
    },
    anthropic: {
      name: 'Anthropic',
      models: [
        'claude-3-5-sonnet-20241022',
        'claude-3-5-haiku-20241022', 
        'claude-3-opus-20240229',
        'claude-3-haiku-20240307'
      ],
      keyPlaceholder: 'sk-ant-...',
      website: 'https://console.anthropic.com/'
    },
    google: {
      name: 'Google Gemini',
      models: [
        'gemini-2.5-flash',
        'gemini-2.0-flash-001',
        'gemini-1.5-pro',
        'gemini-1.5-flash'
      ],
      keyPlaceholder: 'AIza...',
      website: 'https://aistudio.google.com/app/apikey'
    },
    openrouter: {
      name: 'OpenRouter',
      models: [
        // VERIFIED WORKING OPENROUTER MODEL IDS FROM USER
        'deepseek/deepseek-chat',
        'deepseek/deepseek-chat-v3-0324',
        'deepseek/deepseek-chat-v3.1',
        'moonshotai/kimi-k2',
        'anthropic/claude-3-5-sonnet',
        'anthropic/claude-3-opus',
        'anthropic/claude-3.5-haiku',
        'anthropic/claude-3-haiku',
        'anthropic/claude-3.7-sonnet',
        'anthropic/claude-opus-4',
        'anthropic/claude-sonnet-4',
        'openai/gpt-3.5-turbo',
        'openai/gpt-4',
        'openai/gpt-4o',
        'openai/gpt-3.5-turbo-16k',
        'google/gemini-2.0-flash-001',
        'google/gemini-2.0-flash-exp:free',
        'google/gemini-2.0-flash-lite-001',
        'google/gemini-2.5-flash',
        'mistralai/mistral-7b-instruct'
      ],
      keyPlaceholder: 'sk-or-...',
      website: 'https://openrouter.ai/keys'
    },
    grok: {
      name: 'xAI Grok',
      models: [
        'grok-2-1212',
        'grok-2-vision-1212',
        'grok-2-latest',
        'grok-vision-beta',
        'grok-beta'
      ],
      keyPlaceholder: 'xai-...',
      website: 'https://console.x.ai/'
    },
    ollama: {
      name: 'Ollama (Local)',
      models: [
        'llama3.1:8b',
        'llama3.1:70b',
        'llama3.2:3b',
        'llama3.2:1b',
        'mistral:7b',
        'codellama:7b',
        'codellama:13b',
        'phi3:mini',
        'gemma2:9b',
        'qwen2.5:7b'
      ],
      keyPlaceholder: 'No API key needed',
      website: 'https://ollama.com/'
    }
  }

  // Persist memories and chat to localStorage
  React.useEffect(() => {
    localStorage.setItem('pixelog-llm-memories', JSON.stringify(processedMemories))
  }, [processedMemories])

  React.useEffect(() => {
    localStorage.setItem('pixelog-llm-chat', JSON.stringify(chatMessages))
  }, [chatMessages])

  // Persist AI settings
  React.useEffect(() => {
    localStorage.setItem('pixelog-llm-provider', selectedProvider)
  }, [selectedProvider])

  React.useEffect(() => {
    localStorage.setItem('pixelog-llm-model', selectedModel)
  }, [selectedModel])

  React.useEffect(() => {
    localStorage.setItem('pixelog-llm-apikey', apiKey)
  }, [apiKey])

  const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const selectedFiles = event.target.files
    if (selectedFiles) {
      setFiles(Array.from(selectedFiles).filter(file => 
        file.name.endsWith('.pixe') || file.type === 'video/mp4'
      ))
    }
  }

  const toggleMemoryConnection = (memoryId: string) => {
    setConnectedMemories(prev => {
      const newConnected = new Set(prev)
      if (newConnected.has(memoryId)) {
        newConnected.delete(memoryId)
      } else {
        newConnected.add(memoryId)
      }
      return newConnected
    })
  }

  const exportChatAsPixe = () => {
    if (chatMessages.length === 0) {
      alert('No chat messages to export')
      return
    }
    // Reset modal state
    setExportStep('format')
    setSelectedExportFormat('pixe')
    setExportEncryptionKey('')
    setIsExporting(false)
    setShowExportModal(true)
  }

  const createPixeFile = async () => {
    setIsExporting(true)
    
    try {
      // Format chat messages as structured content
      const chatContent = chatMessages.map((msg, index) => {
        const timestamp = new Date(Date.now() - (chatMessages.length - index) * 60000).toISOString()
        const role = msg.type === 'user' ? 'USER' : 'ASSISTANT'
        return `[${timestamp}] ${role}: ${msg.content}`
      }).join('\n\n---\n\n')

      // Add metadata header
      const fullContent = `# Pixelog Chat Session Export\nExported: ${new Date().toISOString()}\nProvider: ${selectedProvider}\nModel: ${selectedModel}\nMemories Connected: ${connectedMemories.size}\n\n---\n\n${chatContent}`

      if (selectedExportFormat === 'txt') {
        // Simple text file download
        const blob = new Blob([fullContent], { type: 'text/plain' })
        const url = URL.createObjectURL(blob)
        
        const a = document.createElement('a')
        a.href = url
        a.download = `pixelog-chat-${new Date().toISOString().split('T')[0]}.txt`
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        URL.revokeObjectURL(url)
        
        alert('Chat exported successfully as text file!')
      } else {
        // Create .pixe file using backend API
        const formData = new FormData()
        
        // Create a text file blob to send to the conversion API
        const textBlob = new Blob([fullContent], { type: 'text/plain' })
        const fileName = `pixelog-chat-${new Date().toISOString().split('T')[0]}.txt`
        const file = new File([textBlob], fileName, { type: 'text/plain' })
        
        formData.append('files', file)
        if (exportEncryptionKey.trim()) {
          formData.append('encryption_key', exportEncryptionKey.trim())
        }

        console.log('Converting chat to .pixe file...')
        const response = await fetch('http://localhost:8080/api/convert', {
          method: 'POST',
          body: formData
        })

        const result = await response.json()
        console.log('Conversion result:', result)
        
        if (response.ok) {
          // Refresh both file lists
          await fetchPixeFiles()
          
          // Add to processed memories immediately
          const newMemory = {
            id: `chat-export-${Date.now()}`,
            filename: fileName.replace('.txt', '.pixe'),
            size: textBlob.size,
            chunks: Math.ceil(fullContent.length / 100), // Estimate chunks
            status: 'ready',
            encrypted: !!exportEncryptionKey.trim()
          }
          
          const updatedMemories = [...processedMemories, newMemory]
          setProcessedMemories(updatedMemories)
          localStorage.setItem('pixelog-llm-memories', JSON.stringify(updatedMemories))
          
          alert('Chat successfully converted to .pixe file and added to your memories!')
        } else {
          throw new Error(`Conversion failed: ${result.error || 'Unknown error'}`)
        }
      }
      
      // Close modal and reset
      setShowExportModal(false)
      setExportEncryptionKey('')
      setExportStep('format')
      setSelectedExportFormat('pixe')
      setIsExporting(false)
      
    } catch (error) {
      console.error('Error creating file:', error)
      const errorMessage = error instanceof Error ? error.message : 'Unknown error occurred'
      alert(`Error creating file: ${errorMessage}`)
      setIsExporting(false)
    }
  }

  // Create tab handlers - simplified versions
  const fetchPixeFiles = async () => {
    try {
      setIsLoadingFiles(true)
      const response = await fetch('http://localhost:8080/api/files')
      if (response.ok) {
        const files = await response.json()
        setPixeFiles(files)
      }
    } catch (error) {
      console.error('Error fetching files:', error)
    } finally {
      setIsLoadingFiles(false)
    }
  }

  const handleFileDrop = async (files: File[]) => {
    if (files.length === 0) return
    
    setIsConverting(true)
    setConversionProgress({ stage: 'Converting...', percentage: 50, message: 'Processing files...' })
    
    try {
      const formData = new FormData()
      files.forEach(file => formData.append('files', file))
      
      const response = await fetch('http://localhost:8080/api/convert', {
        method: 'POST',
        body: formData
      })
      
      if (response.ok) {
        setConversionProgress({ stage: 'Completed', percentage: 100, message: 'Success!' })
        setTimeout(() => setConversionProgress(null), 2000)
        fetchPixeFiles()
      }
    } catch (error) {
      console.error('Error converting:', error)
      setConversionProgress(null)
    } finally {
      setIsConverting(false)
    }
  }

  const handleDownloadFile = async (file: PixelogFile) => {
    try {
      const response = await fetch(`http://localhost:8080/api/files/${file.id}`)
      if (response.ok) {
        const blob = await response.blob()
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = file.name
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        URL.revokeObjectURL(url)
      }
    } catch (error) {
      console.error('Download error:', error)
    }
  }

  const handleDeleteFile = async (fileId: string) => {
    if (confirm('Delete file?')) {
      try {
        await fetch(`http://localhost:8080/api/files/${fileId}`, { method: 'DELETE' })
        fetchPixeFiles()
      } catch (error) {
        console.error('Delete error:', error)
      }
    }
  }

  const handleSendToLLM = (file: PixelogFile) => {
    const newMemory = {
      id: file.id,
      filename: file.name,
      size: typeof file.size === 'string' ? 0 : file.size,
      chunks: 1000,
      status: 'ready',
      encrypted: false
    }
    
    const updated = [...processedMemories, newMemory]
    setProcessedMemories(updated)
    localStorage.setItem('pixelog-llm-memories', JSON.stringify(updated))
    alert('File added to LLM memories!')
  }

  const processFiles = async () => {
    if (files.length === 0) return
    
    setIsProcessing(true)
    
    try {
      const formData = new FormData()
      files.forEach(file => {
        formData.append('files', file)
      })
      if (decryptionKey) {
        formData.append('decryption_key', decryptionKey)
      }

      const response = await fetch('http://localhost:8080/api/llm/memories', {
        method: 'POST',
        body: formData,
      })

      const result = await response.json()
      setProcessedMemories(result.memories || [])
      setFiles([])
      setActiveTab('chat')
    } catch (error) {
      console.error('Error processing files:', error)
    } finally {
      setIsProcessing(false)
    }
  }

  const sendMessage = async () => {
    const connectedMemoryIds = Array.from(connectedMemories)
    // Ollama doesn't need an API key, all others do
    const needsApiKey = selectedProvider !== 'ollama'
    if (!chatInput.trim() || (needsApiKey && !apiKey.trim())) return

    const userMessage: ChatMessage = {
      id: `user-${Date.now()}`,
      type: 'user',
      content: chatInput
    }

    setChatMessages(prev => [...prev, userMessage])
    const currentInput = chatInput
    setChatInput('')
    setIsThinking(true)

    try {
      console.log('Sending chat request:', {
        query: currentInput,
        memory_ids: connectedMemoryIds,
        provider: selectedProvider,
        model: selectedModel,
        api_key: apiKey ? '[REDACTED]' : 'MISSING'
      })
      
      const response = await fetch('http://localhost:8080/api/llm/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: currentInput,
          memory_ids: connectedMemoryIds,
          provider: selectedProvider,
          model: selectedModel,
          api_key: selectedProvider === 'ollama' ? '' : apiKey
        })
      })

      console.log('Response status:', response.status)
      
      if (!response.ok) {
        const errorText = await response.text()
        console.error('API error response:', errorText)
        throw new Error(`API returned ${response.status}: ${errorText}`)
      }

      const result = await response.json()
      console.log('Chat result:', result)
      
      const assistantMessage: ChatMessage = {
        id: `assistant-${Date.now()}`,
        type: 'assistant',
        content: result.content,
        sources: result.sources
      }

      setChatMessages(prev => [...prev, assistantMessage])
    } catch (error) {
      console.error('Error sending message:', error)
      // Add error message to chat
      const errorMessage: ChatMessage = {
        id: `error-${Date.now()}`,
        type: 'assistant',
        content: `Error: ${error instanceof Error ? error.message : 'Failed to send message. Please check your API key and try again.'}`
      }
      setChatMessages(prev => [...prev, errorMessage])
    } finally {
      setIsThinking(false)
    }
  }
  return (
    <div className="min-h-screen cyber-bg-void">
      {/* Pixelog Description */}
      <div className="text-center py-6 border-b border-gray-800/30">
        <div className="flex items-center justify-center gap-8 text-sm">
          <a 
            href="https://github.com/ArqonAi/Pixelog" 
            target="_blank" 
            rel="noopener noreferrer"
            className="cyber-text-secondary hover:cyber-text-primary transition-colors flex items-center gap-2"
          >
            <span>Offline context storage for AI memories</span>
          </a>
          <span className="cyber-text-tertiary">•</span>
          <button 
            onClick={() => setActiveTab('create')}
            className="cyber-text-secondary hover:cyber-text-primary transition-colors flex items-center gap-2 bg-transparent border-none cursor-pointer"
          >
            <span>Convert knowledge to portable .pixe files</span>
          </button>
        </div>
      </div>
      
      <main className="container mx-auto px-4 py-8 max-w-4xl">
        {/* Tab Navigation */}
        <div className="flex items-center space-x-1 mb-8 cyber-bg-panel rounded-lg p-1">
          {[
            { id: 'chat', label: 'LLM Chat', icon: MessageSquare },
            { id: 'upload', label: 'Upload .pixe Files', icon: Upload },
            { id: 'create', label: 'Create .pixe Files', icon: Brain },
          ].map((tab) => {
            const Icon = tab.icon
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as 'upload' | 'chat' | 'create')}
                className={`
                  flex-1 flex items-center justify-center gap-2 py-3 px-4 rounded-md transition-all duration-200
                  ${
                    activeTab === tab.id
                      ? 'cyber-bg-void cyber-text-primary shadow-lg transform scale-105'
                      : 'cyber-text-secondary hover:cyber-text-primary hover:bg-gray-800/50'
                  }
                `}
              >
                <Icon className="w-4 h-4" />
                <span className="cyber-mono text-sm font-medium">{tab.label}</span>
              </button>
            )
          })}</div>

        {/* Upload Tab */}
        {activeTab === 'upload' && (
          <div className="space-y-6 max-w-4xl mx-auto">
            <div className="grid lg:grid-cols-2 gap-6">
              <div className="cyber-terminal">
                <div className="cyber-terminal-header">
                  <h2 className="cyber-h2 text-lg">Upload .pixe Files</h2>
                </div>
                <div className="cyber-terminal-body space-y-6">
                  <div>
                    <label className="block cyber-body cyber-text-primary mb-3">
                      <Upload className="w-4 h-4 inline mr-2" />
                      Select .pixe Memory Files
                    </label>
                    <input
                      type="file"
                      multiple
                      accept=".pixe,.mp4"
                      onChange={handleFileUpload}
                      className="cyber-input w-full"
                    />
                    {files.length > 0 && (
                      <div className="mt-3 space-y-2">
                        {files.map((file, index) => (
                          <div key={index} className="cyber-bg-panel p-3 rounded flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <FileText className="w-4 h-4 cyber-text-blue" />
                              <span className="cyber-mono text-sm">{file.name}</span>
                            </div>
                            <span className="cyber-mono text-xs cyber-text-secondary">
                              {(file.size / (1024 * 1024)).toFixed(1)}MB
                            </span>
                          </div>
                        ))}
                      </div>
                    )}
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
                        Processing memories...
                      </>
                    ) : (
                      <>
                        <Brain className="w-4 h-4" />
                        Create LLM Memory ({files.length} files)
                      </>
                    )}
                  </button>
                </div>
              </div>

              <div className="cyber-terminal">
                <div className="cyber-terminal-header">
                  <h2 className="cyber-h2 text-lg">Processed Memories</h2>
                </div>
                <div className="cyber-terminal-body">
                  {processedMemories.length === 0 ? (
                    <div className="text-center py-8 cyber-text-secondary">
                      <Brain className="w-12 h-12 mx-auto mb-3 cyber-text-tertiary" />
                      <p className="cyber-h3">No memories processed</p>
                      <p className="cyber-body text-sm">Upload .pixe files to create LLM memories</p>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {processedMemories.map((memory) => {
                        const isConnected = connectedMemories.has(memory.id)
                        return (
                        <div key={memory.id} className="cyber-bg-panel p-4 rounded-lg">
                          <div className="flex items-center justify-between mb-2">
                            <h3 className="cyber-body cyber-text-primary font-medium">{memory.filename}</h3>
                            <div className="flex items-center gap-2">
                              {memory.encrypted && (
                                <span className="cyber-mono text-xs bg-yellow-900/30 text-yellow-400 px-2 py-1 rounded">
                                  ENCRYPTED
                                </span>
                              )}
                              <button
                                onClick={() => toggleMemoryConnection(memory.id)}
                                className={`px-3 py-1 rounded-lg text-xs font-medium transition-all flex items-center gap-1 ${
                                  isConnected 
                                    ? 'bg-green-600/20 text-green-400 border border-green-500/30 hover:bg-green-600/30'
                                    : 'bg-gray-600/20 text-gray-400 border border-gray-500/30 hover:bg-gray-600/30'
                                }`}
                                title={isConnected ? 'Disconnect memory' : 'Connect memory'}
                              >
                                {isConnected ? (
                                  <>
                                    <Power className="w-3 h-3" />
                                    Connected
                                  </>
                                ) : (
                                  <>
                                    <PowerOff className="w-3 h-3" />
                                    Connect
                                  </>
                                )}
                              </button>
                            </div>
                          </div>
                          <div className="flex items-center gap-4 cyber-mono text-xs cyber-text-secondary">
                            <span>{memory.chunks.toLocaleString()} chunks</span>
                            <span>{(memory.size / (1024 * 1024)).toFixed(1)}MB</span>
                            <span className={`${
                              isConnected ? 'text-green-400' : 'text-gray-400'
                            }`}>{isConnected ? 'Active' : memory.status}</span>
                          </div>
                        </div>
                      )})
                      }
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Chat Tab */}
        {activeTab === 'chat' && (
          <div className="cyber-terminal h-[700px] flex flex-col mx-auto max-w-4xl">
            <div className="cyber-terminal-header">
              <div className="flex items-center justify-between w-full">
                <div className="flex-1">
                  <h2 className="cyber-h2 text-lg">Chat with Your Memories</h2>
                </div>
                
                {/* Export Chat Button */}
                {chatMessages.length > 0 && (
                  <button
                    onClick={exportChatAsPixe}
                    className="cyber-btn-secondary flex items-center gap-2 px-3 py-2"
                    title="Export chat session as .pixe file"
                  >
                    <Download className="w-4 h-4" />
                    <span className="hidden sm:inline">Export Chat</span>
                  </button>
                )}
                <button
                  onClick={() => setShowSettings(!showSettings)}
                  className="cyber-btn-secondary p-2 flex items-center gap-2"
                >
                  <Settings className="w-4 h-4" />
                  <span className="hidden sm:inline">Settings</span>
                </button>
              </div>
            </div>
            
            {/* Settings Panel */}
            {showSettings && (
              <div className="border-b border-gray-700/50 p-4 bg-gray-900/30">
                <div className="grid md:grid-cols-3 gap-4">
                  {/* Provider Selection */}
                  <div>
                    <label className="block cyber-body cyber-text-primary mb-2">
                      AI Provider
                    </label>
                    <select
                      value={selectedProvider}
                      onChange={(e) => {
                        setSelectedProvider(e.target.value)
                        // Reset to first model when provider changes
                        const newProvider = aiProviders[e.target.value as keyof typeof aiProviders]
                        if (newProvider && newProvider.models.length > 0) {
                          const firstModel = newProvider.models[0]
                          if (firstModel) {
                            setSelectedModel(firstModel)
                          }
                        }
                      }}
                      className="cyber-input w-full"
                    >
                      {Object.entries(aiProviders).map(([key, provider]) => (
                        <option key={key} value={key}>{provider.name}</option>
                      ))}
                    </select>
                  </div>
                  
                  {/* Model Selection */}
                  <div>
                    <label className="block cyber-body cyber-text-primary mb-2">
                      Model
                    </label>
                    <select
                      value={selectedModel}
                      onChange={(e) => setSelectedModel(e.target.value)}
                      className="cyber-input w-full"
                    >
                      {aiProviders[selectedProvider as keyof typeof aiProviders]?.models.map((model) => (
                        <option key={model} value={model}>{model}</option>
                      ))}
                    </select>
                  </div>
                  
                  {/* API Key */}
                  <div>
                    <label className="block cyber-body cyber-text-primary mb-2">
                      API Key
                      <a 
                        href={aiProviders[selectedProvider as keyof typeof aiProviders]?.website}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="ml-2 cyber-text-blue hover:cyber-text-primary text-xs"
                      >
                        Get Key →
                      </a>
                    </label>
                    <div className="relative">
                      <input
                        type={showApiKey ? 'text' : 'password'}
                        value={selectedProvider === 'ollama' ? 'Local - No API Key Required' : apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                        placeholder={aiProviders[selectedProvider as keyof typeof aiProviders]?.keyPlaceholder}
                        className={`cyber-input w-full pr-10 ${selectedProvider === 'ollama' ? 'bg-gray-800/50 cursor-not-allowed' : ''}`}
                        disabled={selectedProvider === 'ollama'}
                        readOnly={selectedProvider === 'ollama'}
                      />
                      {selectedProvider !== 'ollama' && (
                        <button
                          type="button"
                          onClick={() => setShowApiKey(!showApiKey)}
                          className="absolute right-3 top-1/2 -translate-y-1/2 cyber-text-secondary hover:cyber-text-primary"
                        >
                          {showApiKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            )}
            
            <div className="cyber-terminal-body flex-1 flex flex-col min-h-0">
              <div className="flex-1 overflow-y-auto p-4 space-y-6 min-h-0">
                {chatMessages.length === 0 ? (
                  <div className="text-center py-12 cyber-text-secondary">
                    <MessageSquare className="w-16 h-16 mx-auto mb-4 cyber-text-tertiary" />
                    <p className="cyber-h3 text-lg mb-2">Start a conversation</p>
                    <p className="cyber-body text-sm">
                      {connectedMemories.size > 0 
                        ? "Ask questions about your connected memories" 
                        : "Start a conversation or connect memories for context"}
                    </p>
                  </div>
                ) : (
                  <div className="space-y-4 mb-4">
                    {chatMessages.map((msg, index) => (
                      <div key={msg.id || index} className={`flex gap-3 ${
                        msg.type === 'user' ? 'justify-end' : 'justify-start'
                      }`}>
                        {msg.type !== 'user' && (
                          <div className="flex-shrink-0">
                            <Bot className="w-6 h-6 cyber-text-accent" />
                          </div>
                        )}
                        <div className={`max-w-[80%] min-w-0 ${
                          msg.type === 'user' ? 'order-1' : 'order-2'
                        }`}>
                          <div className={`inline-block px-4 py-3 rounded-lg ${
                            msg.type === 'user' 
                              ? 'bg-cyan-500/20 cyber-text-primary border border-cyan-500/30 rounded-br-sm' 
                              : 'cyber-bg-secondary cyber-text-secondary border border-gray-700 rounded-bl-sm'
                          }`}>
                            <div className="whitespace-pre-wrap break-words text-sm leading-relaxed">
                              {msg.content}
                            </div>
                            {msg.sources && msg.sources.length > 0 && (
                              <div className="mt-2 pt-2 border-t border-gray-600">
                                <div className="text-xs cyber-text-muted">Sources: {msg.sources.length}</div>
                              </div>
                            )}
                          </div>
                        </div>
                        {msg.type === 'user' && (
                          <div className="flex-shrink-0 order-2">
                            <User className="w-6 h-6 cyber-text-primary" />
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}
                
                {isThinking && (
                  <div className="flex justify-start">
                    <div className="cyber-bg-panel p-4 rounded-lg">
                      <div className="flex items-center gap-2">
                        <div className="w-4 h-4 border-2 border-cyan-400 border-t-transparent rounded-full animate-spin"></div>
                        <span className="cyber-body cyber-text-secondary">Analyzing memories...</span>
                      </div>
                    </div>
                  </div>
                )}
              </div>
              
              {/* Message Input */}
              <div className="border-t border-gray-700/50 p-4 bg-gray-900/20">
                <div className="flex gap-3">
                  <div className="flex-1 relative">
                    <input
                      type="text"
                      value={chatInput}
                      onChange={(e) => setChatInput(e.target.value)}
                      onKeyPress={(e) => e.key === 'Enter' && !e.shiftKey && sendMessage()}
                      placeholder={
                        !apiKey ? "Enter API key in settings first" :
                        connectedMemories.size === 0 ? "Ask about your memories or start a conversation..." :
                        "Ask about your connected memories..."
                      }
                      className="cyber-input w-full pr-12 py-3 text-base"
                      disabled={!apiKey.trim()}
                    />
                    <button
                      onClick={sendMessage}
                      disabled={!chatInput.trim() || !apiKey.trim() || isThinking}
                      className="absolute right-2 top-1/2 -translate-y-1/2 cyber-btn-secondary p-2 rounded-lg flex items-center justify-center disabled:opacity-50 hover:bg-cyan-500/10 active:bg-cyan-500/20 transition-colors"
                      title="Send message (Enter)"
                    >
                      <Send className="w-4 h-4" />
                    </button>
                  </div>
                </div>
                
                {/* Chat Status */}
                <div className="flex items-center justify-between mt-2 cyber-mono text-xs cyber-text-tertiary">
                  <span>
                    {!apiKey ? "⚠️ Configure API key in settings" :
                     connectedMemories.size === 0 ? "Ready to chat • Connect memories for context" :
                     `${connectedMemories.size} connected • ${processedMemories.filter(m => connectedMemories.has(m.id)).reduce((sum, m) => sum + m.chunks, 0).toLocaleString()} chunks active`
                    }
                  </span>
                  <span className="flex items-center gap-2">
                    {apiKey && (
                      <span className="flex items-center gap-1 text-green-400">
                        <div className="w-2 h-2 bg-green-400 rounded-full animate-pulse"></div>
                        {aiProviders[selectedProvider as keyof typeof aiProviders]?.name}
                      </span>
                    )}
                    <span>Press Enter to send</span>
                  </span>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Create Tab */}
        {activeTab === 'create' && (
          <div className="space-y-6 max-w-4xl mx-auto">
            <div className="grid lg:grid-cols-2 gap-6">
              {/* File Upload Section */}
              <div className="cyber-terminal">
                <div className="cyber-terminal-header">
                  <h2 className="cyber-h2 text-lg">Upload Files to Convert</h2>
                </div>
                <div className="cyber-terminal-body space-y-6">
                  <div>
                    <label className="block cyber-body cyber-text-primary mb-3">
                      <Upload className="w-4 h-4 inline mr-2" />
                      Select Files to Convert
                    </label>
                    <input
                      type="file"
                      multiple
                      onChange={(e) => {
                        const selectedFiles = Array.from(e.target.files || [])
                        handleFileDrop(selectedFiles)
                      }}
                      className="cyber-input w-full"
                      disabled={isConverting}
                    />
                    <p className="text-xs cyber-text-muted mt-2">
                      Supports documents, images, videos, and other file types
                    </p>
                  </div>
                </div>
              </div>

              {/* File List Section */}
              <div className="cyber-terminal">
                <div className="cyber-terminal-header">
                  <div className="flex items-center justify-between w-full">
                    <h2 className="cyber-h2 text-lg">Converted Files</h2>
                    <button
                      onClick={fetchPixeFiles}
                      className="cyber-btn-secondary p-2"
                      disabled={isLoadingFiles}
                    >
                      <RefreshCw className={`w-4 h-4 ${isLoadingFiles ? 'animate-spin' : ''}`} />
                    </button>
                  </div>
                </div>
                <div className="cyber-terminal-body">
                  {isLoadingFiles ? (
                    <div className="flex items-center justify-center py-12">
                      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-cyan-500"></div>
                    </div>
                  ) : pixeFiles.length === 0 ? (
                    <div className="text-center py-12 cyber-text-secondary">
                      <Upload className="w-16 h-16 mx-auto mb-4 cyber-text-tertiary" />
                      <p className="cyber-h3 text-lg mb-2">No .pixe files yet</p>
                      <p className="cyber-body text-sm">Convert some files to get started</p>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {pixeFiles.map((file) => (
                        <div key={file.id} className="cyber-bg-panel p-4 rounded-lg">
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-3">
                              <Upload className="w-5 h-5 cyber-text-blue" />
                              <div>
                                <h3 className="cyber-body cyber-text-primary font-medium">{file.name}</h3>
                                <p className="cyber-mono text-xs cyber-text-secondary">
                                  {typeof file.size === 'string' ? file.size : `${file.size} bytes`} • {new Date(file.created_at).toLocaleDateString()}
                                </p>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <button
                                onClick={() => handleSendToLLM(file)}
                                className="cyber-btn p-2"
                                title="Send to LLM"
                              >
                                <Brain className="w-4 h-4" />
                              </button>
                              <button
                                onClick={() => handleDownloadFile(file)}
                                className="cyber-btn-secondary p-2"
                                title="Download file"
                              >
                                <Download className="w-4 h-4" />
                              </button>
                              <button
                                onClick={() => handleDeleteFile(file.id)}
                                className="cyber-btn-danger p-2"
                                title="Delete file"
                              >
                                <Trash2 className="w-4 h-4" />
                              </button>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>

            {/* Conversion Progress */}
            {conversionProgress && (
              <div className="cyber-terminal">
                <div className="cyber-terminal-header">
                  <h3 className="cyber-h3">Converting Files...</h3>
                </div>
                <div className="cyber-terminal-body">
                  <div className="space-y-2">
                    <div className="flex justify-between cyber-body">
                      <span>{conversionProgress.stage || 'Processing'}</span>
                      <span>{conversionProgress.percentage || 0}%</span>
                    </div>
                    <div className="cyber-bg-void rounded-full h-2 overflow-hidden">
                      <div 
                        className="h-full bg-gradient-to-r from-cyan-500 to-blue-500 transition-all duration-300"
                        style={{ width: `${conversionProgress.percentage || 0}%` }}
                      />
                    </div>
                    {conversionProgress.message && (
                      <p className="cyber-mono text-xs cyber-text-secondary">
                        {conversionProgress.message}
                      </p>
                    )}
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

      </main>

      {/* Export Chat Modal */}
      {showExportModal && (
        <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
          <div className="cyber-bg-panel border border-cyan-500/30 rounded-xl p-8 max-w-md w-full mx-4 shadow-2xl">
            <h3 className="text-xl font-bold cyber-text-primary mb-6 flex items-center gap-2">
              <Download className="w-5 h-5" />
              Export Chat Session
            </h3>
            
            {exportStep === 'format' ? (
              // Step 1: Choose export format
              <div className="space-y-4">
                <div>
                  <label className="block text-sm cyber-text-secondary mb-3">
                    Choose Export Format
                  </label>
                  <div className="space-y-2">
                    <label className="flex items-center gap-3 p-3 rounded-lg border border-gray-600 hover:border-cyan-500/50 cursor-pointer transition-colors">
                      <input
                        type="radio"
                        name="exportFormat"
                        value="pixe"
                        checked={selectedExportFormat === 'pixe'}
                        onChange={(e) => setSelectedExportFormat(e.target.value as 'pixe')}
                        className="cyber-radio"
                      />
                      <div>
                        <div className="cyber-text-primary font-medium">📦 .pixe File</div>
                        <div className="text-xs cyber-text-secondary">Portable encrypted format, can be imported back into LLM</div>
                      </div>
                    </label>
                    <label className="flex items-center gap-3 p-3 rounded-lg border border-gray-600 hover:border-cyan-500/50 cursor-pointer transition-colors">
                      <input
                        type="radio"
                        name="exportFormat"
                        value="txt"
                        checked={selectedExportFormat === 'txt'}
                        onChange={(e) => setSelectedExportFormat(e.target.value as 'txt')}
                        className="cyber-radio"
                      />
                      <div>
                        <div className="cyber-text-primary font-medium">📄 .txt File</div>
                        <div className="text-xs cyber-text-secondary">Simple text format for reading or sharing</div>
                      </div>
                    </label>
                  </div>
                </div>
                
                <div className="text-sm cyber-text-secondary">
                  <p><strong>Chat Summary:</strong></p>
                  <p>• {chatMessages.length} messages</p>
                  <p>• Provider: {selectedProvider}</p>
                  <p>• Model: {selectedModel}</p>
                  <p>• Connected memories: {connectedMemories.size}</p>
                </div>
                
                <div className="flex gap-3 mt-6">
                  <button
                    onClick={() => {
                      setShowExportModal(false)
                      setExportStep('format')
                      setExportEncryptionKey('')
                      setIsExporting(false)
                    }}
                    className="flex-1 px-4 py-2 cyber-btn-secondary"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={() => {
                      if (selectedExportFormat === 'txt') {
                        createPixeFile()
                      } else {
                        setExportStep('encryption')
                      }
                    }}
                    className="flex-1 px-4 py-2 cyber-btn-primary"
                  >
                    Next →
                  </button>
                </div>
              </div>
            ) : (
              // Step 2: Encryption key for .pixe files
              <div className="space-y-4">
                <div>
                  <label className="block text-sm cyber-text-secondary mb-2">
                    Encryption Key (Optional)
                  </label>
                  <input
                    type="password"
                    value={exportEncryptionKey}
                    onChange={(e) => setExportEncryptionKey(e.target.value)}
                    placeholder="Leave empty for no encryption"
                    className="w-full px-3 py-2 cyber-input"
                  />
                  <p className="text-xs cyber-text-muted mt-1">
                    If provided, the .pixe file will be encrypted with this key
                  </p>
                </div>
                
                <div className="text-sm cyber-text-secondary bg-gray-900/50 p-3 rounded-lg">
                  <p><strong>Creating .pixe file with:</strong></p>
                  <p>• {chatMessages.length} messages</p>
                  <p>• {exportEncryptionKey.trim() ? 'Encrypted' : 'No encryption'}</p>
                  <p>• Will be added to your memories automatically</p>
                </div>
                
                <div className="flex gap-3 mt-6">
                  <button
                    onClick={() => setExportStep('format')}
                    disabled={isExporting}
                    className="flex-1 px-4 py-2 cyber-btn-secondary disabled:opacity-50"
                  >
                    ← Back
                  </button>
                  <button
                    onClick={createPixeFile}
                    disabled={isExporting}
                    className="flex-1 px-4 py-2 cyber-btn-primary flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isExporting ? (
                      <>
                        <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                        Creating...
                      </>
                    ) : (
                      <>
                        <Download className="w-4 h-4" />
                        Create .pixe File
                      </>
                    )}
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

export default LLMPage
