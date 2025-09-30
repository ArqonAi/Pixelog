import React from 'react'
import { motion } from 'framer-motion'
import { Github, Terminal, Brain } from 'lucide-react'

// ===== COMPONENT TYPES =====

interface HeaderProps {
  readonly onSearchClick: () => void
}

// ===== HEADER COMPONENT =====

const Header: React.FC<HeaderProps> = ({ onSearchClick }) => {
  return (
    <header className="cyber-bg-panel sticky top-0 z-50 backdrop-blur-md">
      <div className="container mx-auto px-4 py-3">
        <div className="flex items-center justify-between">
          <motion.div
            initial={{ opacity: 0, x: -20 }}
            animate={{ opacity: 1, x: 0 }}
            className="flex items-center space-x-4"
          >
            {/* Logo & Brand - Link to Home */}
            <a href="/" className="flex items-center space-x-4 hover:opacity-80 transition-opacity">
              <div className="w-10 h-10 cyber-bg-void rounded flex items-center justify-center">
                <span className="cyber-text-cyber font-bold text-lg font-mono">Π</span>
              </div>
              
              <div>
                <h1 className="cyber-h2 text-xl cyber-text-primary font-display">
                  Pixelog
                </h1>
              </div>
            </a>
          </motion.div>

          {/* Controls */}
          <motion.div
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            className="flex items-center space-x-1"
          >
            {/* CLI */}
            <a
              href="/cli"
              className="px-3 py-2 text-sm flex items-center space-x-2 cyber-text-secondary hover:cyber-text-primary transition-colors"
              aria-label="CLI Documentation"
            >
              <Terminal className="w-4 h-4" />
              <span>CLI</span>
            </a>
            
            {/* LLM */}
            <a
              href="/llm"
              className="px-3 py-2 text-sm flex items-center space-x-2 cyber-text-secondary hover:cyber-text-primary transition-colors"
              aria-label="LLM Integration"
            >
              <Brain className="w-4 h-4" />
              <span>LLM</span>
            </a>
            
            {/* Source */}
            <a
              href="https://github.com/ArqonAi/Pixelog"
              target="_blank"
              rel="noopener noreferrer"
              className="px-3 py-2 text-sm flex items-center space-x-2 cyber-text-secondary hover:cyber-text-primary transition-colors"
              aria-label="View source code"
            >
              <Github className="w-4 h-4" />
              <span>Source</span>
            </a>
          </motion.div>
        </div>
      </div>
    </header>
  )
}

export default Header
