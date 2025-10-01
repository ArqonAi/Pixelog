import React, { useState } from 'react'
import { Brain, ArrowLeft, Upload, MessageSquare, Settings, Key, Eye, EyeOff, Send, FileText, Bot, Sliders } from 'lucide-react'

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
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>(() => {
    const saved = localStorage.getItem('pixelog-llm-chat')
    return saved ? JSON.parse(saved) : []
  })
  const [chatInput, setChatInput] = useState<string>('')
  const [isThinking, setIsThinking] = useState<boolean>(false)
  
  // AI Provider Settings
  const [selectedProvider, setSelectedProvider] = useState<string>(() => {
    return localStorage.getItem('pixelog-llm-provider') || 'openai'
  })
  const [selectedModel, setSelectedModel] = useState<string>(() => {
    return localStorage.getItem('pixelog-llm-model') || 'gpt-5'
  })
  const [apiKey, setApiKey] = useState<string>(() => {
    return localStorage.getItem('pixelog-llm-apikey') || ''
  })
  const [showApiKey, setShowApiKey] = useState<boolean>(false)
  const [showSettings, setShowSettings] = useState<boolean>(false)

  // AI Provider configurations
  const aiProviders = {
    openai: {
      name: 'OpenAI',
      models: ['gpt-5', 'gpt-4o', 'gpt-4-turbo', 'gpt-4'],
      keyPlaceholder: 'sk-...',
      website: 'https://platform.openai.com/api-keys'
    },
    anthropic: {
      name: 'Anthropic',
      models: ['claude-4-opus', 'claude-4-sonnet', 'claude-4-haiku', 'claude-3.5-sonnet'],
      keyPlaceholder: 'sk-ant-...',
      website: 'https://console.anthropic.com/'
    },
    google: {
      name: 'Google Gemini',
      models: ['gemini-2.5-pro', 'gemini-2.0-flash', 'gemini-1.5-pro', 'gemini-1.5-flash'],
      keyPlaceholder: 'AIza...',
      website: 'https://aistudio.google.com/app/apikey'
    },
    grok: {
      name: 'Grok (xAI)',
      models: ['grok-3', 'grok-2', 'grok-1.5'],
      keyPlaceholder: 'xai-...',
      website: 'https://x.ai/'
    },
    openrouter: {
      name: 'OpenRouter',
      models: [
        // OpenAI Models
        'openai/gpt-5',
        'openai/gpt-4o',
        'openai/gpt-4-turbo',
        'openai/gpt-4',
        // Anthropic Models
        'anthropic/claude-4-opus',
        'anthropic/claude-4-sonnet', 
        'anthropic/claude-3.5-sonnet',
        'anthropic/claude-3-opus',
        // Google Models
        'google/gemini-2.5-pro',
        'google/gemini-2.0-flash',
        'google/gemini-1.5-pro',
        // Meta Models
        'meta-llama/llama-4-405b-instruct',
        'meta-llama/llama-3.3-70b-instruct', 
        'meta-llama/llama-3.1-405b-instruct',
        'meta-llama/llama-3.1-70b-instruct',
        // DeepSeek Models
        'deepseek/deepseek-v3-chat',
        'deepseek/deepseek-v3-coder',
        'deepseek/deepseek-r1',
        // Moonshot (Kimi) Models
        'moonshot/moonshot-v2-128k',
        'moonshot/moonshot-v1-128k',
        'moonshot/moonshot-v1-32k',
        // Mistral Models
        'mistralai/mistral-large-2',
        'mistralai/mixtral-8x22b-instruct',
        'mistralai/mixtral-8x7b-instruct',
        // Cohere Models
        'cohere/command-r',
        'cohere/command-r-plus',
        // Qwen Models
        'qwen/qwen-2-72b-instruct',
        'qwen/qwen-2.5-72b-instruct',
        // Perplexity Models
        'perplexity/llama-3.1-sonar-large-128k-online',
        'perplexity/llama-3.1-sonar-small-128k-online',
        // Other Popular Models
        'nvidia/nemotron-4-340b-instruct',
        'liquid/lfm-40b',
        'inflection/inflection-3-pi'
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

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selectedFiles = Array.from(e.target.files || [])
    setFiles(selectedFiles)
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
    if (!chatInput.trim() || processedMemories.length === 0 || !apiKey.trim()) return

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
      const response = await fetch('http://localhost:8080/api/llm/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: currentInput,
          memory_ids: processedMemories.map(m => m.id),
          provider: selectedProvider,
          model: selectedModel,
          api_key: apiKey
        })
      })

      const result = await response.json()
      
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
      
      <main className="container mx-auto px-4 py-8 max-w-6xl">
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
          <div className="space-y-6">
            <div className="grid lg:grid-cols-2 gap-6">
              <div className="cyber-terminal">
                <div className="cyber-terminal-header">
                  <h2 className="cyber-h2 text-lg">Process .pixe Memories</h2>
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
                      {processedMemories.map((memory) => (
                        <div key={memory.id} className="cyber-bg-panel p-4 rounded-lg">
                          <div className="flex items-center justify-between mb-2">
                            <h3 className="cyber-body cyber-text-primary font-medium">{memory.filename}</h3>
                            {memory.encrypted && (
                              <span className="cyber-mono text-xs bg-yellow-900/30 text-yellow-400 px-2 py-1 rounded">
                                ENCRYPTED
                              </span>
                            )}
                          </div>
                          <div className="flex items-center gap-4 cyber-mono text-xs cyber-text-secondary">
                            <span>{memory.chunks.toLocaleString()} chunks</span>
                            <span>{(memory.size / (1024 * 1024)).toFixed(1)}MB</span>
                            <span className="text-green-400">{memory.status}</span>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Chat Tab */}
        {activeTab === 'chat' && (
          <div className="cyber-terminal h-[600px] flex flex-col">
            <div className="cyber-terminal-header">
              <div className="flex items-center justify-between w-full">
                <div>
                  <h2 className="cyber-h2 text-lg">Chat with Your Memories</h2>
                  <div className="flex items-center gap-4 cyber-mono text-xs cyber-text-secondary">
                    <span>{processedMemories.length} memories loaded</span>
                    <span className="flex items-center gap-1">
                      <Bot className="w-3 h-3" />
                      {aiProviders[selectedProvider as keyof typeof aiProviders]?.name} • {selectedModel}
                    </span>
                    <span className={`px-2 py-1 rounded text-xs ${
                      apiKey ? 'bg-green-900/30 text-green-400' : 'bg-red-900/30 text-red-400'
                    }`}>
                      {apiKey ? 'API Connected' : 'API Key Required'}
                    </span>
                  </div>
                </div>
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
                      {processedMemories.length > 0 
                        ? "Ask questions about your uploaded memories" 
                        : "Upload memories first to start chatting"}
                    </p>
                  </div>
                ) : (
                  chatMessages.map((message) => (
                    <div key={message.id} className={`flex ${message.type === 'user' ? 'justify-end' : 'justify-start'} mb-6`}>
                      <div className={`
                        max-w-[85%] min-w-[200px] rounded-xl p-5 shadow-lg
                        ${message.type === 'user' 
                          ? 'bg-gradient-to-br from-cyan-600/20 to-cyan-500/30 border border-cyan-500/30 cyber-text-primary' 
                          : 'cyber-bg-panel border border-gray-600/40 cyber-text-primary'
                        }
                      `}>
                        <p className="cyber-body">{message.content}</p>
                        {message.sources && (
                          <div className="mt-3 pt-3 border-t border-gray-700/50">
                            <p className="cyber-mono text-xs cyber-text-secondary mb-2">Sources:</p>
                            {message.sources.map((source, idx) => (
                              <div key={idx} className="flex items-center justify-between cyber-mono text-xs mb-1">
                                <span className="cyber-text-primary">{source.filename}</span>
                                <span className="cyber-text-secondary">
                                  Frame {source.frame_number} • {(source.relevance * 100).toFixed(0)}% match
                                </span>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  ))
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
                        processedMemories.length === 0 ? "Upload memories first to chat" :
                        "Ask about your memories..."
                      }
                      className="cyber-input w-full pr-12 py-3 text-base"
                      disabled={processedMemories.length === 0 || !apiKey.trim()}
                    />
                    <button
                      onClick={sendMessage}
                      disabled={!chatInput.trim() || processedMemories.length === 0 || !apiKey.trim() || isThinking}
                      className="absolute right-2 top-1/2 -translate-y-1/2 cyber-btn-secondary p-2 rounded-lg flex items-center justify-center disabled:opacity-50"
                    >
                      <Send className="w-4 h-4" />
                    </button>
                  </div>
                </div>
                
                {/* Chat Status */}
                <div className="flex items-center justify-between mt-2 cyber-mono text-xs cyber-text-tertiary">
                  <span>
                    {!apiKey ? "⚠️ Configure API key in settings" :
                     processedMemories.length === 0 ? "Upload .pixe files to start chatting" :
                     `${processedMemories.length} memories • ${processedMemories.reduce((sum, m) => sum + m.chunks, 0).toLocaleString()} chunks ready`
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
    </div>
  )
}

export default LLMPage
