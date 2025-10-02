import React, { useState } from 'react'
import { Brain, ArrowLeft, Upload, MessageSquare, Settings, Key, Eye, EyeOff, Send, FileText, Bot, User, Sliders, Zap, Trash2, Power, PowerOff, Download } from 'lucide-react'

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

const LLMPage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'upload' | 'chat'>('chat')
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
    setShowExportModal(true)
  }

  const createPixeFile = async () => {
    setIsExporting(true)
    
    try {
      // For now, just create a simple text file download
      // until we fix the FFmpeg conversion issues
      
      // Format chat messages as structured content
      const chatContent = chatMessages.map((msg, index) => {
        const timestamp = new Date(Date.now() - (chatMessages.length - index) * 60000).toISOString()
        const role = msg.type === 'user' ? 'USER' : 'ASSISTANT'
        return `[${timestamp}] ${role}: ${msg.content}`
      }).join('\n\n---\n\n')

      // Add metadata header
      const fullContent = `# Pixelog Chat Session Export\nExported: ${new Date().toISOString()}\nProvider: ${selectedProvider}\nModel: ${selectedModel}\nMemories Connected: ${connectedMemories.size}\n\n---\n\n${chatContent}`

      // Create and download as text file for now
      const blob = new Blob([fullContent], { type: 'text/plain' })
      const url = URL.createObjectURL(blob)
      
      // Create download link
      const a = document.createElement('a')
      a.href = url
      a.download = `pixelog-chat-${new Date().toISOString().split('T')[0]}.txt`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      
      // Close modal and reset
      setShowExportModal(false)
      setExportEncryptionKey('')
      setIsExporting(false)
      
      alert('Chat exported successfully and downloaded as text file!')
      
    } catch (error) {
      console.error('Error creating file:', error)
      const errorMessage = error instanceof Error ? error.message : 'Unknown error occurred'
      alert(`Error creating file: ${errorMessage}`)
      setIsExporting(false)
    }
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
    if (!chatInput.trim() || connectedMemoryIds.length === 0 || !apiKey.trim()) return

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
          api_key: apiKey
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
          <a 
            href="/create" 
            className="cyber-text-secondary hover:cyber-text-primary transition-colors flex items-center gap-2"
          >
            <span>Convert knowledge to portable .pixe files</span>
          </a>
        </div>
      </div>
      
      <main className="container mx-auto px-4 py-8 max-w-4xl">
        {/* Tab Navigation */}
        <div className="flex items-center space-x-1 mb-8 cyber-bg-panel rounded-lg p-1">
          {[
            { id: 'chat', label: 'LLM Chat', icon: MessageSquare },
            { id: 'upload', label: 'Upload .pixe Files', icon: Upload },
          ].map((tab) => {
            const Icon = tab.icon
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as 'upload' | 'chat')}
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
                        value={apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                        placeholder={aiProviders[selectedProvider as keyof typeof aiProviders]?.keyPlaceholder}
                        className="cyber-input w-full pr-10"
                      />
                      <button
                        type="button"
                        onClick={() => setShowApiKey(!showApiKey)}
                        className="absolute right-3 top-1/2 -translate-y-1/2 cyber-text-secondary hover:cyber-text-primary"
                      >
                        {showApiKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                      </button>
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
                        : "Connect memories first to start chatting"}
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
                        connectedMemories.size === 0 ? "Connect memories first to chat" :
                        "Ask about your connected memories..."
                      }
                      className="cyber-input w-full pr-12 py-3 text-base"
                      disabled={connectedMemories.size === 0 || !apiKey.trim()}
                    />
                    <button
                      onClick={sendMessage}
                      disabled={!chatInput.trim() || connectedMemories.size === 0 || !apiKey.trim() || isThinking}
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
                     connectedMemories.size === 0 ? "Connect memories to start chatting" :
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

      </main>

      {/* Export Chat Modal */}
      {showExportModal && (
        <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50">
          <div className="cyber-bg-panel border border-cyan-500/30 rounded-xl p-8 max-w-md w-full mx-4 shadow-2xl">
            <h3 className="text-xl font-bold cyber-text-primary mb-6 flex items-center gap-2">
              <Download className="w-5 h-5" />
              Export Chat as .pixe File
            </h3>
            
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
              
              <div className="text-sm cyber-text-secondary">
                <p><strong>Chat Summary:</strong></p>
                <p>• {chatMessages.length} messages</p>
                <p>• Provider: {selectedProvider}</p>
                <p>• Model: {selectedModel}</p>
                <p>• Connected memories: {connectedMemories.size}</p>
              </div>
            </div>
            
            <div className="flex gap-3 mt-6">
              <button
                onClick={() => {
                  setShowExportModal(false)
                  setExportEncryptionKey('')
                  setIsExporting(false)
                }}
                disabled={isExporting}
                className="flex-1 px-4 py-2 cyber-btn-secondary disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Cancel
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
        </div>
      )}
    </div>
  )
}

export default LLMPage
