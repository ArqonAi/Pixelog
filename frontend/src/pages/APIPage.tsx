import React from 'react'
import { Code, ExternalLink, Book, Terminal, Zap, Lock, Upload, MessageSquare } from 'lucide-react'

const APIPage: React.FC = () => {
  return (
    <div className="min-h-screen cyber-bg-void">
      <div className="container mx-auto px-4 py-8 max-w-6xl">
        {/* Header */}
        <div className="text-center mb-12">
          <h1 className="cyber-h1 text-4xl mb-4">API Documentation</h1>
          <p className="cyber-body cyber-text-secondary text-lg">
            REST API endpoints for Pixelog integration
          </p>
        </div>

        <div className="space-y-12">
          {/* Authentication */}
          <section>
            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <div className="flex items-center gap-2">
                  <Lock className="w-5 h-5 cyber-text-primary" />
                  <h2 className="cyber-h2 text-xl">Authentication</h2>
                </div>
              </div>
              <div className="cyber-terminal-body space-y-4">
                <p className="cyber-body cyber-text-secondary">
                  All API requests require authentication via API key in the Authorization header.
                </p>
                <div className="cyber-bg-panel p-4 rounded-lg">
                  <pre className="cyber-mono text-sm cyber-text-primary overflow-x-auto">
{`curl -H "Authorization: Bearer YOUR_API_KEY" \\
     -H "Content-Type: application/json" \\
     http://localhost:8080/api/endpoint`}
                  </pre>
                </div>
              </div>
            </div>
          </section>

          {/* File Operations */}
          <section>
            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <div className="flex items-center gap-2">
                  <Upload className="w-5 h-5 cyber-text-primary" />
                  <h2 className="cyber-h2 text-xl">File Operations</h2>
                </div>
              </div>
              <div className="cyber-terminal-body space-y-8">
                {/* Convert Files */}
                <div>
                  <h3 className="cyber-h3 text-lg mb-4 cyber-text-primary">POST /api/convert</h3>
                  <p className="cyber-body cyber-text-secondary mb-4">
                    Convert files to .pixe format with optional encryption
                  </p>
                  
                  <div className="space-y-4">
                    <div>
                      <h4 className="cyber-body cyber-text-primary mb-2">Request:</h4>
                      <div className="cyber-bg-panel p-4 rounded-lg">
                        <pre className="cyber-mono text-sm cyber-text-primary overflow-x-auto">
{`curl -X POST http://localhost:8080/api/convert \\
  -H "Content-Type: multipart/form-data" \\
  -F "files=@document.pdf" \\
  -F "files=@video.mp4" \\
  -F "encryption_password=optional-key"`}
                        </pre>
                      </div>
                    </div>
                    
                    <div>
                      <h4 className="cyber-body cyber-text-primary mb-2">Response:</h4>
                      <div className="cyber-bg-panel p-4 rounded-lg">
                        <pre className="cyber-mono text-sm cyber-text-primary overflow-x-auto">
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
                  </div>
                </div>

                {/* List Files */}
                <div>
                  <h3 className="cyber-h3 text-lg mb-4 cyber-text-primary">GET /api/files</h3>
                  <p className="cyber-body cyber-text-secondary mb-4">
                    List all converted .pixe files
                  </p>
                  <div className="cyber-bg-panel p-4 rounded-lg">
                    <pre className="cyber-mono text-sm cyber-text-primary overflow-x-auto">
{`curl -X GET http://localhost:8080/api/files`}
                    </pre>
                  </div>
                </div>

                {/* Delete File */}
                <div>
                  <h3 className="cyber-h3 text-lg mb-4 cyber-text-primary">DELETE /api/files/:id</h3>
                  <p className="cyber-body cyber-text-secondary mb-4">
                    Delete a specific .pixe file
                  </p>
                  <div className="cyber-bg-panel p-4 rounded-lg">
                    <pre className="cyber-mono text-sm cyber-text-primary overflow-x-auto">
{`curl -X DELETE http://localhost:8080/api/files/file_1`}
                    </pre>
                  </div>
                </div>
              </div>
            </div>
          </section>

          {/* LLM Integration */}
          <section>
            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <div className="flex items-center gap-2">
                  <MessageSquare className="w-5 h-5 cyber-text-primary" />
                  <h2 className="cyber-h2 text-xl">LLM Integration</h2>
                </div>
              </div>
              <div className="cyber-terminal-body space-y-8">
                {/* Process Memories */}
                <div>
                  <h3 className="cyber-h3 text-lg mb-4 cyber-text-primary">POST /api/llm/memories</h3>
                  <p className="cyber-body cyber-text-secondary mb-4">
                    Process .pixe files for AI memory integration
                  </p>
                  <div className="cyber-bg-panel p-4 rounded-lg">
                    <pre className="cyber-mono text-sm cyber-text-primary overflow-x-auto">
{`curl -X POST http://localhost:8080/api/llm/memories \\
  -H "Content-Type: multipart/form-data" \\
  -F "files=@documents.pixe" \\
  -F "files=@notes.pixe" \\
  -F "decryption_key=optional-key"`}
                    </pre>
                  </div>
                </div>

                {/* Chat */}
                <div>
                  <h3 className="cyber-h3 text-lg mb-4 cyber-text-primary">POST /api/llm/chat</h3>
                  <p className="cyber-body cyber-text-secondary mb-4">
                    Chat with your processed memories using AI
                  </p>
                  <div className="space-y-4">
                    <div>
                      <h4 className="cyber-body cyber-text-primary mb-2">Request:</h4>
                      <div className="cyber-bg-panel p-4 rounded-lg">
                        <pre className="cyber-mono text-sm cyber-text-primary overflow-x-auto">
{`curl -X POST http://localhost:8080/api/llm/chat \\
  -H "Content-Type: application/json" \\
  -d '{
    "query": "What were the main topics discussed?",
    "memory_ids": ["mem_1", "mem_2"],
    "provider": "openai",
    "model": "gpt-4",
    "api_key": "your-openai-key"
  }'`}
                        </pre>
                      </div>
                    </div>
                    
                    <div>
                      <h4 className="cyber-body cyber-text-primary mb-2">Response:</h4>
                      <div className="cyber-bg-panel p-4 rounded-lg">
                        <pre className="cyber-mono text-sm cyber-text-primary overflow-x-auto">
{`{
  "content": "The main topics discussed were...",
  "sources": [
    {
      "memory_id": "mem_1",
      "filename": "meeting.pixe",
      "chunk_id": "chunk_5",
      "relevance_score": 0.95
    }
  ]
}`}
                        </pre>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Search */}
                <div>
                  <h3 className="cyber-h3 text-lg mb-4 cyber-text-primary">GET /api/llm/search</h3>
                  <p className="cyber-body cyber-text-secondary mb-4">
                    Semantic search across processed memories
                  </p>
                  <div className="cyber-bg-panel p-4 rounded-lg">
                    <pre className="cyber-mono text-sm cyber-text-primary overflow-x-auto">
{`curl -X GET "http://localhost:8080/api/llm/search?q=meeting+notes&limit=10"`}
                    </pre>
                  </div>
                </div>
              </div>
            </div>
          </section>

          {/* Status Codes & Rate Limits */}
          <section>
            <div className="grid md:grid-cols-2 gap-6">
              <div className="cyber-terminal">
                <div className="cyber-terminal-header">
                  <h3 className="cyber-h3 text-lg">Status Codes</h3>
                </div>
                <div className="cyber-terminal-body">
                  <div className="space-y-2">
                    <div className="flex justify-between items-center">
                      <span className="cyber-mono text-sm">200</span>
                      <span className="cyber-body text-sm cyber-text-secondary">Success</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="cyber-mono text-sm text-yellow-400">400</span>
                      <span className="cyber-body text-sm cyber-text-secondary">Bad Request</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="cyber-mono text-sm text-red-400">401</span>
                      <span className="cyber-body text-sm cyber-text-secondary">Unauthorized</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="cyber-mono text-sm text-red-400">429</span>
                      <span className="cyber-body text-sm cyber-text-secondary">Rate Limited</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="cyber-mono text-sm text-red-400">500</span>
                      <span className="cyber-body text-sm cyber-text-secondary">Server Error</span>
                    </div>
                  </div>
                </div>
              </div>
              
              <div className="cyber-terminal">
                <div className="cyber-terminal-header">
                  <h3 className="cyber-h3 text-lg">Rate Limits</h3>
                </div>
                <div className="cyber-terminal-body">
                  <div className="space-y-2">
                    <div className="flex justify-between items-center">
                      <span className="cyber-mono text-sm">File conversion</span>
                      <span className="cyber-body text-sm cyber-text-secondary">10/min</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="cyber-mono text-sm">LLM chat</span>
                      <span className="cyber-body text-sm cyber-text-secondary">60/min</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="cyber-mono text-sm">File operations</span>
                      <span className="cyber-body text-sm cyber-text-secondary">100/min</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="cyber-mono text-sm">Search queries</span>
                      <span className="cyber-body text-sm cyber-text-secondary">120/min</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </section>

          {/* Resources */}
          <section>
            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <div className="flex items-center gap-2">
                  <Book className="w-5 h-5 cyber-text-primary" />
                  <h2 className="cyber-h2 text-xl">Resources</h2>
                </div>
              </div>
              <div className="cyber-terminal-body">
                <div className="grid md:grid-cols-2 gap-4">
                  <a 
                    href="https://github.com/ArqonAi/Pixelog" 
                    target="_blank" 
                    rel="noopener noreferrer"
                    className="cyber-bg-panel p-4 rounded-lg hover:border-cyan-500/30 transition-colors flex items-center gap-3 border border-transparent"
                  >
                    <Terminal className="w-5 h-5 cyber-text-primary" />
                    <div>
                      <h4 className="cyber-body cyber-text-primary font-medium">GitHub Repository</h4>
                      <p className="cyber-mono text-xs cyber-text-secondary">Source code & examples</p>
                    </div>
                    <ExternalLink className="w-4 h-4 cyber-text-secondary ml-auto" />
                  </a>
                  
                  <a 
                    href="/cli" 
                    className="cyber-bg-panel p-4 rounded-lg hover:border-cyan-500/30 transition-colors flex items-center gap-3 border border-transparent"
                  >
                    <Code className="w-5 h-5 cyber-text-primary" />
                    <div>
                      <h4 className="cyber-body cyber-text-primary font-medium">CLI Documentation</h4>
                      <p className="cyber-mono text-xs cyber-text-secondary">Command line usage</p>
                    </div>
                  </a>
                </div>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}

export default APIPage
