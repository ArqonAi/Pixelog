import React, { useState, useEffect, useRef } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Search, X, File, Calendar, Loader, Download } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { pixelogApi } from '@/services/api'
import type { PixeFile } from '@/types/api'

// ===== COMPONENT TYPES =====

interface SearchBarProps {
  readonly className?: string
}

interface SearchResult {
  readonly file: PixeFile
  readonly matchedContent?: string
  readonly score: number
}

// ===== SEARCH BAR COMPONENT =====

const SearchBar: React.FC<SearchBarProps> = ({ className = '' }) => {
  const [query, setQuery] = useState<string>('')
  const [isOpen, setIsOpen] = useState<boolean>(false)
  const [results, setResults] = useState<readonly SearchResult[]>([])
  const [isSearching, setIsSearching] = useState<boolean>(false)
  const searchRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Get all files for search
  const { data: files = [] } = useQuery({
    queryKey: ['pixe-files'],
    queryFn: () => pixelogApi.getPixeFiles(),
    staleTime: 30000
  })

  // Search function with debouncing
  useEffect(() => {
    if (query.length < 2) {
      setResults([])
      setIsOpen(false)
      return
    }

    const timeoutId = setTimeout(() => {
      performSearch(query)
    }, 300)

    return () => clearTimeout(timeoutId)
  }, [query, files])

  // Close search on outside click
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent): void => {
      if (searchRef.current && !searchRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const performSearch = async (searchQuery: string): Promise<void> => {
    setIsSearching(true)
    try {
      const searchResults: SearchResult[] = files
        .map(file => {
          const score = calculateSearchScore(file, searchQuery)
          return {
            file,
            score,
            matchedContent: extractMatchedContent(file, searchQuery)
          }
        })
        .filter(result => result.score > 0)
        .sort((a, b) => b.score - a.score)
        .slice(0, 8)

      setResults(searchResults)
      setIsOpen(true)
    } catch (error) {
      console.error('Search failed:', error)
    } finally {
      setIsSearching(false)
    }
  }

  const calculateSearchScore = (file: PixeFile, searchQuery: string): number => {
    const query = searchQuery.toLowerCase()
    let score = 0

    // Filename match (highest priority)
    if (file.name.toLowerCase().includes(query)) {
      score += 100
    }

    // Date-based search
    const createdDate = new Date(file.created_at).toLocaleDateString()
    if (createdDate.includes(query)) {
      score += 50
    }

    // Path match
    if (file.path.toLowerCase().includes(query)) {
      score += 30
    }

    return score
  }

  const extractMatchedContent = (file: PixeFile, searchQuery: string): string => {
    const query = searchQuery.toLowerCase()
    
    if (file.name.toLowerCase().includes(query)) {
      return file.name
    }
    
    if (file.path.toLowerCase().includes(query)) {
      return file.path
    }
    
    return ''
  }

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>): void => {
    const value = e.target.value
    setQuery(value)
  }

  const handleClear = (): void => {
    setQuery('')
    setResults([])
    setIsOpen(false)
    inputRef.current?.focus()
  }

  const handleResultClick = (file: PixeFile): void => {
    console.log('Selected file:', file)
    setIsOpen(false)
    // Could trigger download or show file details
  }

  const formatFileSize = (size: string): string => {
    // The size is already formatted as a string like "2.4 MB"
    return size
  }

  const formatDate = (dateString: string): string => {
    try {
      return new Date(dateString).toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric'
      })
    } catch {
      return 'Unknown'
    }
  }

  return (
    <div ref={searchRef} className={`relative ${className}`}>
      {/* Search Input */}
      <div className="relative">
        <Search className="absolute left-4 top-1/2 transform -translate-y-1/2 w-5 h-5 cyber-text-secondary" />
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={handleInputChange}
          placeholder="Search through your uploaded files..."
          className="
            w-full pl-12 pr-12 py-4
            bg-gray-900/30 backdrop-blur-sm 
            border border-gray-600/40 rounded-lg
            cyber-text-primary placeholder-gray-400
            focus:outline-none focus:border-gray-500/60 focus:bg-gray-900/50
            transition-all duration-300
            cyber-body text-base
            hover:border-gray-600/50
          "
        />
        
        {/* Clear/Loading Button */}
        <div className="absolute right-4 top-1/2 transform -translate-y-1/2">
          {isSearching ? (
            <Loader className="w-5 h-5 cyber-text-cyber animate-spin" />
          ) : query ? (
            <button
              onClick={handleClear}
              className="cyber-text-secondary hover:cyber-text-primary transition-colors"
              aria-label="Clear search"
            >
              <X className="w-5 h-5" />
            </button>
          ) : null}
        </div>
      </div>

      {/* Search Results */}
      <AnimatePresence>
        {isOpen && results.length > 0 && (
          <motion.div
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.2 }}
            className="
              absolute top-full left-0 right-0 mt-2 z-50
              bg-gray-900/95 backdrop-blur-md 
              border border-gray-600/40 rounded-lg
              max-h-80 overflow-y-auto
              shadow-2xl shadow-gray-900/30
            "
          >
            <div className="p-2">
              <div className="cyber-mono text-xs cyber-text-secondary px-3 py-2 border-b border-gray-600/20">
                {results.length} result{results.length !== 1 ? 's' : ''} found
              </div>
              
              {results.map((result, index) => (
                <motion.button
                  key={result.file.id}
                  initial={{ opacity: 0, x: -10 }}
                  animate={{ opacity: 1, x: 0 }}
                  transition={{ duration: 0.2, delay: index * 0.05 }}
                  onClick={() => handleResultClick(result.file)}
                  className="
                    w-full text-left p-3 rounded-lg
                    hover:bg-gray-700/20 transition-colors
                    focus:outline-none focus:bg-gray-700/20
                    group
                  "
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-3 flex-1 min-w-0">
                      <File className="w-5 h-5 cyber-text-blue flex-shrink-0" />
                      <div className="min-w-0 flex-1">
                        <h4 className="cyber-body cyber-text-primary truncate text-sm font-medium">
                          {result.file.name}
                        </h4>
                        <div className="flex items-center space-x-4 cyber-mono text-xs cyber-text-secondary mt-1">
                          <span>{formatFileSize(result.file.size)}</span>
                          <span className="flex items-center space-x-1">
                            <Calendar className="w-3 h-3" />
                            <span>{formatDate(result.file.created_at)}</span>
                          </span>
                        </div>
                      </div>
                    </div>
                    
                    <Download className="w-4 h-4 cyber-text-secondary group-hover:cyber-text-primary transition-colors flex-shrink-0" />
                  </div>
                </motion.button>
              ))}
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* No Results */}
      <AnimatePresence>
        {isOpen && query.length >= 2 && results.length === 0 && !isSearching && (
          <motion.div
            initial={{ opacity: 0, y: -10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.2 }}
            className="
              absolute top-full left-0 right-0 mt-2 z-50
              bg-gray-900/95 backdrop-blur-md 
              border border-gray-600/40 rounded-lg
              p-6 text-center
              shadow-2xl shadow-gray-900/30
            "
          >
            <Search className="w-12 h-12 cyber-text-secondary mx-auto mb-3" />
            <p className="cyber-body cyber-text-secondary">No files found for "{query}"</p>
            <p className="cyber-mono text-xs cyber-text-tertiary mt-1">
              Try searching for filename, type, or date
            </p>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

export default SearchBar
