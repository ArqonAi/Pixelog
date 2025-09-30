import React from 'react'
import { motion } from 'framer-motion'
import { Terminal, Download, Copy, CheckCircle, AlertCircle, Code, ArrowLeft } from 'lucide-react'

const CLIPage: React.FC = () => {
  const [copiedCommand, setCopiedCommand] = React.useState<string | null>(null)

  const copyToClipboard = (text: string, commandId: string) => {
    navigator.clipboard.writeText(text)
    setCopiedCommand(commandId)
    setTimeout(() => setCopiedCommand(null), 2000)
  }

  return (
    <div className="min-h-screen cyber-bg-void">
      {/* Header */}
      <div className="cyber-bg-panel border-b border-gray-800/50">
        <div className="container mx-auto px-4 py-6">
          <div className="flex items-center gap-4">
            <a href="/" className="cyber-text-secondary hover:cyber-text-primary transition-colors">
              <ArrowLeft className="w-5 h-5" />
            </a>
            <div className="flex items-center gap-3">
              <Terminal className="w-8 h-8 cyber-text-cyber" />
              <div>
                <h1 className="cyber-h1 text-3xl">Pixelog CLI</h1>
                <p className="cyber-body cyber-text-secondary">Command-line interface for file conversion and management</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <main className="container mx-auto px-4 py-8 max-w-4xl">
        {/* Installation */}
        <motion.section
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6 }}
          className="mb-12"
        >
          <h2 className="cyber-h2 text-2xl mb-6 flex items-center gap-2">
            <Download className="w-6 h-6 cyber-text-cyber" />
            Installation
          </h2>
          
          <div className="space-y-4">
            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <span className="cyber-mono text-sm">Install via Go</span>
                <button
                  onClick={() => copyToClipboard('go install github.com/ArqonAi/Pixelog/cmd/pixelog@latest', 'install-go')}
                  className="cyber-btn-secondary px-2 py-1 text-xs flex items-center gap-1"
                >
                  {copiedCommand === 'install-go' ? <CheckCircle className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                  {copiedCommand === 'install-go' ? 'Copied!' : 'Copy'}
                </button>
              </div>
              <div className="cyber-terminal-body">
                <code className="cyber-mono text-cyan-400">
                  go install github.com/ArqonAi/Pixelog/cmd/pixelog@latest
                </code>
              </div>
            </div>

            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <span className="cyber-mono text-sm">Install via Homebrew (macOS/Linux)</span>
                <button
                  onClick={() => copyToClipboard('brew install ArqonAi/tap/pixelog', 'install-brew')}
                  className="cyber-btn-secondary px-2 py-1 text-xs flex items-center gap-1"
                >
                  {copiedCommand === 'install-brew' ? <CheckCircle className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                  {copiedCommand === 'install-brew' ? 'Copied!' : 'Copy'}
                </button>
              </div>
              <div className="cyber-terminal-body">
                <code className="cyber-mono text-cyan-400">
                  brew install ArqonAi/tap/pixelog
                </code>
              </div>
            </div>

            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <span className="cyber-mono text-sm">Download Binary</span>
              </div>
              <div className="cyber-terminal-body">
                <p className="cyber-body cyber-text-secondary mb-3">
                  Download pre-built binaries from GitHub Releases:
                </p>
                <a 
                  href="https://github.com/ArqonAi/Pixelog/releases/latest"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="cyber-btn inline-flex items-center gap-2"
                >
                  <Download className="w-4 h-4" />
                  Download Latest Release
                </a>
              </div>
            </div>
          </div>
        </motion.section>

        {/* Quick Start */}
        <motion.section
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.2 }}
          className="mb-12"
        >
          <h2 className="cyber-h2 text-2xl mb-6 flex items-center gap-2">
            <Code className="w-6 h-6 cyber-text-cyber" />
            Quick Start
          </h2>

          <div className="space-y-6">
            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <span className="cyber-mono text-sm">Convert a single file</span>
                <button
                  onClick={() => copyToClipboard('pixelog convert document.pdf', 'convert-single')}
                  className="cyber-btn-secondary px-2 py-1 text-xs flex items-center gap-1"
                >
                  {copiedCommand === 'convert-single' ? <CheckCircle className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                  {copiedCommand === 'convert-single' ? 'Copied!' : 'Copy'}
                </button>
              </div>
              <div className="cyber-terminal-body">
                <code className="cyber-mono text-cyan-400">
                  pixelog convert document.pdf
                </code>
              </div>
            </div>

            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <span className="cyber-mono text-sm">Convert multiple files</span>
                <button
                  onClick={() => copyToClipboard('pixelog convert *.pdf *.docx *.txt', 'convert-multiple')}
                  className="cyber-btn-secondary px-2 py-1 text-xs flex items-center gap-1"
                >
                  {copiedCommand === 'convert-multiple' ? <CheckCircle className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                  {copiedCommand === 'convert-multiple' ? 'Copied!' : 'Copy'}
                </button>
              </div>
              <div className="cyber-terminal-body">
                <code className="cyber-mono text-cyan-400">
                  pixelog convert *.pdf *.docx *.txt
                </code>
              </div>
            </div>

            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <span className="cyber-mono text-sm">Convert with encryption</span>
                <button
                  onClick={() => copyToClipboard('pixelog convert --encrypt --password "secure123" document.pdf', 'convert-encrypt')}
                  className="cyber-btn-secondary px-2 py-1 text-xs flex items-center gap-1"
                >
                  {copiedCommand === 'convert-encrypt' ? <CheckCircle className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                  {copiedCommand === 'convert-encrypt' ? 'Copied!' : 'Copy'}
                </button>
              </div>
              <div className="cyber-terminal-body">
                <code className="cyber-mono text-cyan-400">
                  pixelog convert --encrypt --password "secure123" document.pdf
                </code>
              </div>
            </div>
          </div>
        </motion.section>

        {/* Commands */}
        <motion.section
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.4 }}
          className="mb-12"
        >
          <h2 className="cyber-h2 text-2xl mb-6">Commands</h2>
          
          <div className="space-y-6">
            <div className="cyber-bg-panel p-6 rounded-lg">
              <h3 className="cyber-h3 text-lg mb-3 text-cyan-400">convert</h3>
              <p className="cyber-body cyber-text-secondary mb-4">
                Convert files to .pixe format (QR-encoded MP4 videos)
              </p>
              <div className="cyber-mono text-sm space-y-2">
                <div><span className="text-yellow-400">--quality</span> <span className="cyber-text-tertiary">Video quality (18-51, default: 23)</span></div>
                <div><span className="text-yellow-400">--framerate</span> <span className="cyber-text-tertiary">Video framerate (default: 0.5)</span></div>
                <div><span className="text-yellow-400">--encrypt</span> <span className="cyber-text-tertiary">Enable AES-256-GCM encryption</span></div>
                <div><span className="text-yellow-400">--password</span> <span className="cyber-text-tertiary">Encryption password</span></div>
                <div><span className="text-yellow-400">--output</span> <span className="cyber-text-tertiary">Output directory</span></div>
              </div>
            </div>

            <div className="cyber-bg-panel p-6 rounded-lg">
              <h3 className="cyber-h3 text-lg mb-3 text-cyan-400">extract</h3>
              <p className="cyber-body cyber-text-secondary mb-4">
                Extract original files from .pixe archives
              </p>
              <div className="cyber-mono text-sm space-y-2">
                <div><span className="text-yellow-400">--password</span> <span className="cyber-text-tertiary">Decryption password (if encrypted)</span></div>
                <div><span className="text-yellow-400">--output</span> <span className="cyber-text-tertiary">Output directory</span></div>
                <div><span className="text-yellow-400">--verify</span> <span className="cyber-text-tertiary">Verify file integrity</span></div>
              </div>
            </div>

            <div className="cyber-bg-panel p-6 rounded-lg">
              <h3 className="cyber-h3 text-lg mb-3 text-cyan-400">list</h3>
              <p className="cyber-body cyber-text-secondary mb-4">
                List contents of .pixe archives without extracting
              </p>
              <div className="cyber-mono text-sm space-y-2">
                <div><span className="text-yellow-400">--detailed</span> <span className="cyber-text-tertiary">Show detailed file information</span></div>
                <div><span className="text-yellow-400">--json</span> <span className="cyber-text-tertiary">Output in JSON format</span></div>
              </div>
            </div>

            <div className="cyber-bg-panel p-6 rounded-lg">
              <h3 className="cyber-h3 text-lg mb-3 text-cyan-400">search</h3>
              <p className="cyber-body cyber-text-secondary mb-4">
                Search through .pixe archives using AI-powered semantic search
              </p>
              <div className="cyber-mono text-sm space-y-2">
                <div><span className="text-yellow-400">--query</span> <span className="cyber-text-tertiary">Search query</span></div>
                <div><span className="text-yellow-400">--limit</span> <span className="cyber-text-tertiary">Maximum results (default: 10)</span></div>
                <div><span className="text-yellow-400">--provider</span> <span className="cyber-text-tertiary">AI provider (openai, google, ollama)</span></div>
              </div>
            </div>
          </div>
        </motion.section>

        {/* Examples */}
        <motion.section
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.6 }}
          className="mb-12"
        >
          <h2 className="cyber-h2 text-2xl mb-6">Examples</h2>
          
          <div className="space-y-4">
            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <span className="cyber-mono text-sm">Batch convert with high quality</span>
              </div>
              <div className="cyber-terminal-body">
                <code className="cyber-mono text-cyan-400">
                  pixelog convert --quality 18 --output ./converted/ ~/Documents/*.pdf
                </code>
              </div>
            </div>

            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <span className="cyber-mono text-sm">Extract with password</span>
              </div>
              <div className="cyber-terminal-body">
                <code className="cyber-mono text-cyan-400">
                  pixelog extract --password "mypass123" --output ./extracted/ archive.pixe
                </code>
              </div>
            </div>

            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <span className="cyber-mono text-sm">Search across multiple archives</span>
              </div>
              <div className="cyber-terminal-body">
                <code className="cyber-mono text-cyan-400">
                  pixelog search --query "tax documents 2024" --limit 5 *.pixe
                </code>
              </div>
            </div>
          </div>
        </motion.section>

        {/* Configuration */}
        <motion.section
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.8 }}
        >
          <h2 className="cyber-h2 text-2xl mb-6 flex items-center gap-2">
            <AlertCircle className="w-6 h-6 cyber-text-amber" />
            Configuration
          </h2>
          
          <div className="cyber-bg-panel p-6 rounded-lg">
            <p className="cyber-body cyber-text-secondary mb-4">
              Create a configuration file at <code className="cyber-mono bg-gray-800 px-2 py-1 rounded">~/.pixelog/config.yaml</code>:
            </p>
            
            <div className="cyber-terminal">
              <div className="cyber-terminal-header">
                <span className="cyber-mono text-sm">config.yaml</span>
              </div>
              <div className="cyber-terminal-body">
                <pre className="cyber-mono text-sm text-cyan-400">
{`# Video encoding settings
quality: 23
framerate: 0.5
chunk_size: 2800

# Encryption settings
encryption_enabled: true
default_password: ""

# AI Search providers
search:
  provider: "openai"  # openai, google, ollama
  openai_api_key: "your-api-key"
  google_api_key: "your-api-key"
  ollama_base_url: "http://localhost:11434"

# Output settings
output_dir: "./output"
preserve_metadata: true`}
                </pre>
              </div>
            </div>
          </div>
        </motion.section>
      </main>
    </div>
  )
}

export default CLIPage
