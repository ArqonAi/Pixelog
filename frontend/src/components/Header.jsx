import React from 'react'
import { motion } from 'framer-motion'
import { Github, Heart, Star, Search } from 'lucide-react'

const Header = ({ onSearchClick }) => {
  return (
    <header className="bg-white/80 dark:bg-gray-800/80 backdrop-blur-md border-b border-gray-200 dark:border-gray-700 sticky top-0 z-40">
      <div className="container mx-auto px-4 py-4">
        <div className="flex items-center justify-between">
          <motion.div
            initial={{ opacity: 0, x: -20 }}
            animate={{ opacity: 1, x: 0 }}
            className="flex items-center space-x-3"
          >
            <div className="w-10 h-10 bg-gradient-to-br from-blue-500 to-purple-600 rounded-lg flex items-center justify-center">
              <span className="text-white font-bold text-lg">P</span>
            </div>
            <div>
              <h1 className="text-xl font-bold text-gray-900 dark:text-gray-100">
                Pixelog
              </h1>
              <p className="text-xs text-gray-500 dark:text-gray-400">
                v1.0.0
              </p>
            </div>
          </motion.div>

          <motion.div
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            className="flex items-center space-x-4"
          >
            <div className="hidden sm:flex items-center space-x-2 text-sm text-gray-600 dark:text-gray-300">
              <Star className="w-4 h-4 text-yellow-500" />
              <span>Star on GitHub</span>
            </div>
            
            <button
              onClick={onSearchClick}
              className="p-2 text-gray-600 dark:text-gray-300 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
              title="Semantic Search"
            >
              <Search className="w-5 h-5" />
            </button>
            
            <a
              href="https://github.com/ArqonAi/Pixelog"
              target="_blank"
              rel="noopener noreferrer"
              className="p-2 text-gray-600 dark:text-gray-300 hover:text-blue-600 dark:hover:text-blue-400 transition-colors"
            >
              <Github className="w-5 h-5" />
            </a>
          </motion.div>
        </div>
      </div>
    </header>
  )
}

export default Header