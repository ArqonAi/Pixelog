import React from 'react'
import { Code, ExternalLink, ArrowLeft, Book, Terminal, Zap } from 'lucide-react'

const APIPage: React.FC = () => {
  return (
    <div className="min-h-screen cyber-bg-void">
      <div className="cyber-bg-panel border-b border-gray-800/50">
        <div className="container mx-auto px-4 py-6">
          <div className="flex items-center gap-4">
            <a href="/" className="cyber-text-secondary hover:cyber-text-primary transition-colors">
              <ArrowLeft className="w-5 h-5" />
            </a>
            <div className="flex items-center gap-3">
              <Code className="w-8 h-8 cyber-text-primary" />
              <div>
                <h1 className="cyber-h1 text-2xl">API Documentation</h1>
                <p className="cyber-body cyber-text-secondary">
                  REST API endpoints and SDK integration
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="container mx-auto px-4 py-8">
        <div className="cyber-terminal">
          <div className="cyber-terminal-header">
            <div className="flex items-center gap-2">
              <Book className="w-5 h-5" />
              <h2 className="cyber-h2 text-xl">API Reference</h2>
            </div>
          </div>
          
          <div className="cyber-terminal-body space-y-8">
            {/* Authentication */}
            <section>
              <div className="flex items-center gap-2 mb-4">
                <Zap className="w-5 h-5 cyber-text-blue" />
                <h3 className="cyber-h3 text-lg">Authentication</h3>
              </div>
              <div className="cyber-bg-panel p-4 rounded-lg mb-4">
                <p className="cyber-body cyber-text-secondary mb-3">
                  All API requests require authentication via API key in the Authorization header.
                </p>
                <pre className="cyber-mono text-sm cyber-text-primary bg-gray-900/50 p-3 rounded overflow-x-auto">
{`curl -H "Authorization: Bearer YOUR_API_KEY" \\
     -H "Content-Type: application/json" \\
     https://api.pixelog.ai/v1/endpoint`}
                </pre>
              </div>
            </section>

            {/* File Conversion */}
            <section>
              <h3 className="cyber-h3 text-lg mb-4">File Conversion</h3>
              
              <div className="space-y-6">
                <div>
                  <h4 className="cyber-body cyber-text-primary mb-2">POST /api/convert</h4>
                  <p className="cyber-mono text-sm cyber-text-secondary mb-3">
                    Convert files to .pixe format with optional encryption
                  </p>
                  <pre className="cyber-mono text-sm text-cyan-400 bg-gray-900/50 p-4 rounded overflow-x-auto">
{`curl -X POST http://localhost:8080/api/convert \\
  -H "Content-Type: multipart/form-data" \\
  -F "files=@document.pdf" \\
  -F "files=@video.mp4" \\
  -F "encryption_password=optional-key"`}
                  </pre>
                  
                  <div className="mt-4">
                    <h5 className="cyber-body cyber-text-primary mb-2">Response:</h5>
                    <pre className="cyber-mono text-sm text-green-400 bg-gray-900/50 p-4 rounded overflow-x-auto">
{`{
  "job_id": "job_1703123456",
  "status": "completed",
  "message": "Successfully processed 2 files",
  "processed_files": [
    {
      "id": "file_1",
      "name": "document.pixe",
      "size": 2048576,
      "created_at": "2025-12-31T12:00:00Z",
      "path": "./output/document.pixe",
      "type": "pixe"
    }
  ]
}`}
                    </pre>
                  </div>
                </div>

                <div>
                  <h4 className="cyber-body cyber-text-primary mb-2">GET /api/files</h4>
                  <p className="cyber-mono text-sm cyber-text-secondary mb-3">
                    List all converted .pixe files
                  </p>
                  <pre className="cyber-mono text-sm text-cyan-400 bg-gray-900/50 p-4 rounded overflow-x-auto">
{`curl -X GET http://localhost:8080/api/files`}
                  </pre>
                </div>

                <div>
                  <h4 className="cyber-body cyber-text-primary mb-2">DELETE /api/files/:id</h4>
                  <p className="cyber-mono text-sm cyber-text-secondary mb-3">
                    Delete a specific .pixe file
                  </p>
                  <pre className="cyber-mono text-sm text-cyan-400 bg-gray-900/50 p-4 rounded overflow-x-auto">
{`curl -X DELETE http://localhost:8080/api/files/file_1`}
                  </pre>
                </div>
              </div>
            </section>

            {/* LLM Integration */}
            <section>
              <h3 className="cyber-h3 text-lg mb-4">LLM Integration</h3>
              
              <div className="space-y-6">
                <div>
                  <h4 className="cyber-body cyber-text-primary mb-2">POST /api/llm/memories</h4>
                  <p className="cyber-mono text-sm cyber-text-secondary mb-3">
                    Process .pixe files for AI memory integration
                  </p>
                  <pre className="cyber-mono text-sm text-cyan-400 bg-gray-900/50 p-4 rounded overflow-x-auto">
{`curl -X POST http://localhost:8080/api/llm/memories \\
  -H "Content-Type: multipart/form-data" \\
  -F "files=@documents.pixe" \\
  -F "files=@notes.pixe" \\
  -F "decryption_key=optional-key"`}
                  </pre>
                </div>

                <div>
                  <h4 className="cyber-body cyber-text-primary mb-2">POST /api/llm/chat</h4>
                  <p className="cyber-mono text-sm cyber-text-secondary mb-3">
                    Chat with your processed memories using AI
                  </p>
                  <pre className="cyber-mono text-sm text-cyan-400 bg-gray-900/50 p-4 rounded overflow-x-auto">
{`curl -X POST http://localhost:8080/api/llm/chat \\
  -H "Content-Type: application/json" \\
  -d '{
    "query": "What were the main topics discussed?",
    "memory_ids": ["mem_1", "mem_2"],
    "provider": "openai",
    "model": "gpt-5",
    "api_key": "your-openai-key"
  }'`}
                  </pre>
                </div>

                <div>
                  <h4 className="cyber-body cyber-text-primary mb-2">GET /api/llm/search</h4>
                  <p className="cyber-mono text-sm cyber-text-secondary mb-3">
                    Semantic search across processed memories
                  </p>
                  <pre className="cyber-mono text-sm text-cyan-400 bg-gray-900/50 p-4 rounded overflow-x-auto">
{`curl -X GET "http://localhost:8080/api/llm/search?q=meeting+notes&limit=10"`}
                  </pre>
                </div>
              </div>
            </section>

            {/* Go SDK */}
            <section>
              <h3 className="cyber-h3 text-lg mb-4">Go SDK Integration</h3>
              
              <div className="cyber-bg-panel p-4 rounded-lg">
                <h4 className="cyber-body cyber-text-primary mb-3">Installation</h4>
                <pre className="cyber-mono text-sm cyber-text-primary bg-gray-900/50 p-3 rounded overflow-x-auto mb-4">
{`go get github.com/ArqonAi/pixelog-sdk-go`}
                </pre>

                <h4 className="cyber-body cyber-text-primary mb-3">Basic Usage</h4>
                <pre className="cyber-mono text-sm text-green-400 bg-gray-900/50 p-4 rounded overflow-x-auto">
{`package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/ArqonAi/pixelog-sdk-go/client"
)

func main() {
    // Initialize client
    pixelog := client.New(client.Config{
        BaseURL: "http://localhost:8080",
        APIKey:  "your-api-key",
    })
    
    // Convert files
    result, err := pixelog.Convert(context.Background(), &client.ConvertRequest{
        Files: []string{"document.pdf", "video.mp4"},
        Options: &client.ConvertOptions{
            Encryption: true,
            Password:   "secure-password",
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Job ID: %s\\n", result.JobID)
    
    // Process for LLM
    memories, err := pixelog.ProcessMemories(context.Background(), &client.MemoryRequest{
        Files: result.ProcessedFiles,
        DecryptionKey: "secure-password",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Chat with memories
    response, err := pixelog.Chat(context.Background(), &client.ChatRequest{
        Query:     "What are the key insights?",
        MemoryIDs: memories.IDs,
        Provider:  "openai",
        Model:     "gpt-5",
        APIKey:    "your-openai-key",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("AI Response: %s\\n", response.Content)
}`}
                </pre>
              </div>
            </section>

            {/* Rate Limits */}
            <section>
              <h3 className="cyber-h3 text-lg mb-4">Rate Limits & Status Codes</h3>
              
              <div className="grid md:grid-cols-2 gap-6">
                <div className="cyber-bg-panel p-4 rounded-lg">
                  <h4 className="cyber-body cyber-text-primary mb-3">Rate Limits</h4>
                  <ul className="cyber-mono text-sm cyber-text-secondary space-y-1">
                    <li>• File conversion: 10 requests/minute</li>
                    <li>• LLM chat: 60 requests/minute</li>
                    <li>• File operations: 100 requests/minute</li>
                    <li>• Search: 120 requests/minute</li>
                  </ul>
                </div>
                
                <div className="cyber-bg-panel p-4 rounded-lg">
                  <h4 className="cyber-body cyber-text-primary mb-3">Status Codes</h4>
                  <ul className="cyber-mono text-sm cyber-text-secondary space-y-1">
                    <li>• <span className="text-green-400">200</span> - Success</li>
                    <li>• <span className="text-yellow-400">400</span> - Bad Request</li>
                    <li>• <span className="text-red-400">401</span> - Unauthorized</li>
                    <li>• <span className="text-red-400">429</span> - Rate Limited</li>
                    <li>• <span className="text-red-400">500</span> - Server Error</li>
                  </ul>
                </div>
              </div>
            </section>

            {/* Support */}
            <section>
              <h3 className="cyber-h3 text-lg mb-4">Support & Resources</h3>
              
              <div className="grid md:grid-cols-2 gap-4">
                <a 
                  href="https://github.com/ArqonAi/Pixelog" 
                  target="_blank" 
                  rel="noopener noreferrer"
                  className="cyber-bg-panel p-4 rounded-lg hover:border-cyan-500/30 transition-colors flex items-center gap-3"
                >
                  <Terminal className="w-5 h-5 cyber-text-blue" />
                  <div>
                    <h4 className="cyber-body cyber-text-primary">GitHub Repository</h4>
                    <p className="cyber-mono text-xs cyber-text-secondary">Source code & examples</p>
                  </div>
                  <ExternalLink className="w-4 h-4 cyber-text-secondary ml-auto" />
                </a>
                
                <a 
                  href="https://docs.pixelog.ai" 
                  target="_blank" 
                  rel="noopener noreferrer"
                  className="cyber-bg-panel p-4 rounded-lg hover:border-cyan-500/30 transition-colors flex items-center gap-3"
                >
                  <Book className="w-5 h-5 cyber-text-blue" />
                  <div>
                    <h4 className="cyber-body cyber-text-primary">Full Documentation</h4>
                    <p className="cyber-mono text-xs cyber-text-secondary">Complete API reference</p>
                  </div>
                  <ExternalLink className="w-4 h-4 cyber-text-secondary ml-auto" />
                </a>
              </div>
            </section>
          </div>
        </div>
      </div>
    </div>
  )
}

export default APIPage
