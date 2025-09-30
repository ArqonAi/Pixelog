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
              <h2 className="cyber-h2 text-lg">Go API Integration</h2>
            </div>
            <div className="cyber-terminal-body space-y-6">
              <div>
                <h3 className="cyber-h3 mb-3">Pixelog Go SDK</h3>
                <div className="cyber-bg-panel p-4 rounded-lg">
                  <pre className="cyber-mono text-sm text-cyan-400 whitespace-pre-wrap">
{`// Initialize Pixelog LLM client
package main

import (
    "github.com/ArqonAi/Pixelog/pkg/llm"
    "github.com/ArqonAi/Pixelog/pkg/memory"
)

func main() {
    // Create memory client for .pixe files
    memClient := memory.NewClient()
    
    // Load .pixe video memories
    memories, err := memClient.LoadMemories([]string{
        "documents.pixe",
        "notes.pixe",
    }, memory.Options{
        DecryptionKey: "your-key-here",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Initialize LLM chat with memories
    chat := llm.NewChat(memories, llm.Config{
        Provider: "openai",
        Model:    "gpt-4",
        APIKey:   os.Getenv("OPENAI_API_KEY"),
    })
    
    // Frame-level semantic search
    query := "machine learning algorithms"
    response, err := chat.Ask(query)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Answer: %s\\n", response.Content)
    
    // Show source frames with relevance scores
    for _, source := range response.Sources {
        fmt.Printf("Source: %s (frame %d, relevance: %.2f)\\n", 
            source.Filename, source.FrameNumber, source.Relevance)
    }
}`}
                  </pre>
                </div>
              </div>

              <div>
                <h3 className="cyber-h3 mb-3">REST API Endpoints</h3>
                <div className="space-y-4">
                  <div className="cyber-bg-panel p-4 rounded-lg">
                    <h4 className="cyber-body cyber-text-primary mb-2">POST /api/llm/memories</h4>
                    <pre className="cyber-mono text-sm text-cyan-400">
{`curl -X POST http://localhost:8080/api/llm/memories \\
  -H "Content-Type: multipart/form-data" \\
  -F "files=@documents.pixe" \\
  -F "files=@notes.pixe" \\
  -F "decryption_key=optional-key"`}
                    </pre>
                  </div>

                  <div className="cyber-bg-panel p-4 rounded-lg">
                    <h4 className="cyber-body cyber-text-primary mb-2">POST /api/llm/chat</h4>
                    <pre className="cyber-mono text-sm text-cyan-400">
{`curl -X POST http://localhost:8080/api/llm/chat \\
  -H "Content-Type: application/json" \\
  -d '{
    "query": "What are the key insights?",
    "memory_ids": ["mem_123", "mem_456"],
    "provider": "openai",
    "model": "gpt-4"
  }'`}
                    </pre>
                  </div>

                  <div className="cyber-bg-panel p-4 rounded-lg">
                    <h4 className="cyber-body cyber-text-primary mb-2">GET /api/llm/search</h4>
                    <pre className="cyber-mono text-sm text-cyan-400">
{`curl "http://localhost:8080/api/llm/search?q=machine+learning&limit=5&memory_id=mem_123"`}
                    </pre>
                  </div>
                </div>
              </div>

              <div>
                <h3 className="cyber-h3 mb-3">Memory Processing</h3>
                <div className="cyber-bg-panel p-4 rounded-lg">
                  <pre className="cyber-mono text-sm text-cyan-400">
{`// Process .pixe files for LLM consumption
type MemoryProcessor struct {
    decoder    *qr.Decoder
    embedder   *embedding.Client
    indexer    *search.Index
}

func (p *MemoryProcessor) ProcessPixeFile(filepath string, password string) (*Memory, error) {
    // 1. Open .pixe video file
    video, err := p.decoder.OpenVideo(filepath)
    if err != nil {
        return nil, err
    }
    defer video.Close()
    
    // 2. Extract text chunks from QR frames
    chunks := make([]TextChunk, 0)
    for frameNum := 0; frameNum < video.FrameCount(); frameNum++ {
        frame, err := video.GetFrame(frameNum)
        if err != nil {
            continue
        }
        
        text, err := p.decoder.DecodeQR(frame, password)
        if err != nil {
            continue // Skip corrupted frames
        }
        
        chunks = append(chunks, TextChunk{
            Content:     text,
            FrameNumber: frameNum,
            Timestamp:   frame.Timestamp,
        })
    }
    
    // 3. Generate embeddings for semantic search
    embeddings, err := p.embedder.GenerateEmbeddings(chunks)
    if err != nil {
        return nil, err
    }
    
    // 4. Build search index
    index, err := p.indexer.BuildIndex(chunks, embeddings)
    if err != nil {
        return nil, err
    }
    
    return &Memory{
        ID:         generateID(),
        Filename:   filepath,
        Chunks:     chunks,
        Index:      index,
        Encrypted:  password != "",
    }, nil
}`}
                  </pre>
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
