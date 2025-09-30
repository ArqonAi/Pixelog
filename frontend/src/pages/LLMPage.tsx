import React, { useState } from 'react'
import { Brain, ArrowLeft, Upload, MessageSquare, Settings } from 'lucide-react'

const LLMPage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<'upload' | 'chat' | 'api'>('upload')
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
                <h1 className="cyber-h1 text-3xl">LLM Memory Integration</h1>
                <p className="cyber-body cyber-text-secondary">Chat with your .pixe video memories using AI</p>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <main className="container mx-auto px-4 py-8 max-w-6xl">
        {/* Tab Navigation */}
        <div className="flex items-center space-x-1 mb-8 cyber-bg-panel rounded-lg p-1">
          {[
            { id: 'upload', label: 'Upload Memories', icon: Upload },
            { id: 'chat', label: 'Chat Interface', icon: MessageSquare },
            { id: 'api', label: 'API Integration', icon: Settings }
          ].map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              onClick={() => setActiveTab(id as any)}
              className={`flex items-center gap-2 px-4 py-2 rounded-md transition-all ${
                activeTab === id ? 'cyber-bg-void cyber-text-primary' : 'cyber-text-secondary hover:cyber-text-primary'
              }`}
            >
              <Icon className="w-4 h-4" />
              <span className="cyber-mono text-sm">{label}</span>
            </button>
          ))}
        </div>

        {/* Upload Tab */}
        {activeTab === 'upload' && (
          <div className="cyber-terminal">
            <div className="cyber-terminal-header">
              <h2 className="cyber-h2 text-lg">Process .pixe Memories</h2>
            </div>
            <div className="cyber-terminal-body space-y-4">
              <p className="cyber-body cyber-text-secondary">
                Upload .pixe files to create searchable AI memories (memvid-style approach)
              </p>
            </div>
          </div>
        )}

        {/* Chat Tab */}
        {activeTab === 'chat' && (
          <div className="cyber-terminal">
            <div className="cyber-terminal-header">
              <h2 className="cyber-h2 text-lg">Chat Interface</h2>
            </div>
            <div className="cyber-terminal-body">
              <p className="cyber-body cyber-text-secondary">
                Frame-level semantic search and chat functionality
              </p>
            </div>
          </div>
        )}

        {/* API Tab */}
        {activeTab === 'api' && (
          <div className="cyber-terminal">
            <div className="cyber-terminal-header">
              <h2 className="cyber-h2 text-lg">API Integration</h2>
            </div>
            <div className="cyber-terminal-body">
              <p className="cyber-body cyber-text-secondary">
                Python API code examples for LLM integration
              </p>
            </div>
          </div>
        )}
      </main>
    </div>
  )
}

export default LLMPage
