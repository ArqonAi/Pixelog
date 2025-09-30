import React, { useState, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Search, X, FileText, Clock, Zap, AlertCircle } from 'lucide-react'
import { pixelogApi } from '@/services/api'
import type { SearchResult, SearchHit } from '@/types/api'

// ===== COMPONENT TYPES =====

interface SearchInterfaceProps {
  readonly isOpen: boolean
  readonly onClose: () => void
}

interface SearchState {
  readonly query: string
  readonly isLoading: boolean
  readonly results: SearchResult | null
  readonly error: string | null
}

// ===== SEARCH INTERFACE COMPONENT =====

const SearchInterface: React.FC<SearchInterfaceProps> = ({ isOpen, onClose }) => {
  const [searchState, setSearchState] = useState<SearchState>({
    query: '',
    isLoading: false,
    results: null,
    error: null
  })

  const handleSearch = useCallback(async (query: string): Promise<void> => {
    if (!query.trim()) {
      setSearchState(prev => ({ ...prev, results: null, error: null }))
      return
    }

    setSearchState(prev => ({ ...prev, isLoading: true, error: null }))

    try {
      const results = await pixelogApi.searchContent({ query: query.trim() })
      setSearchState(prev => ({
        ...prev,
        results,
        isLoading: false
      }))
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Search failed'
      setSearchState(prev => ({
        ...prev,
        error: errorMessage,
        isLoading: false,
        results: null
      }))
    }
  }, [])

  const handleQueryChange = useCallback((e: React.ChangeEvent<HTMLInputElement>): void => {
    const newQuery = e.target.value
    setSearchState(prev => ({ ...prev, query: newQuery }))
    
    // Debounced search
    if (newQuery.trim()) {
      const timeoutId = setTimeout(() => {
        void handleSearch(newQuery)
      }, 500)
      
      clearTimeout(timeoutId)
    }
  }, [handleSearch])

  const handleSubmit = useCallback((e: React.FormEvent): void => {
    e.preventDefault()
    void handleSearch(searchState.query)
  }, [handleSearch, searchState.query])

  const highlightText = (text: string, highlights: readonly string[]): JSX.Element => {
    if (!highlights.length) return <span>{text}</span>
    
    let result = text
    highlights.forEach(highlight => {
      const regex = new RegExp(`(${highlight})`, 'gi')
      result = result.replace(regex, '<mark>$1</mark>')
    })
    
    return <span dangerouslySetInnerHTML={{ __html: result }} />
  }

  const formatQueryTime = (timeMs: number): string => {
    return timeMs < 1000 ? `${Math.round(timeMs)}ms` : `${(timeMs / 1000).toFixed(2)}s`
  }

  return (
    <AnimatePresence>
      {isOpen && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm"
          onClick={onClose}
        >
          <motion.div
            initial={{ opacity: 0, scale: 0.9, y: -50 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.9, y: -50 }}
            className="fixed top-8 left-1/2 transform -translate-x-1/2 w-full max-w-4xl mx-4"
            onClick={e => e.stopPropagation()}
          >
            <div className="bg-white dark:bg-gray-800 rounded-lg shadow-2xl border border-gray-200 dark:border-gray-700">
              {/* Header */}
              <div className="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
                <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100 flex items-center">
                  <Search className="w-6 h-6 mr-2" />
                  Semantic Search
                </h2>
                <button
                  onClick={onClose}
                  className="p-2 text-gray-500 hover:text-gray-700 dark:hover:text-gray-300 transition-colors"
                  aria-label="Close search"
                >
                  <X className="w-6 h-6" />
                </button>
              </div>

              {/* Search Form */}
              <div className="p-6 border-b border-gray-200 dark:border-gray-700">
                <form onSubmit={handleSubmit} className="relative">
                  <Search className="absolute left-4 top-1/2 transform -translate-y-1/2 w-5 h-5 text-gray-400" />
                  <input
                    type="text"
                    value={searchState.query}
                    onChange={handleQueryChange}
                    placeholder="Search through your PixeFiles content..."
                    className="w-full pl-12 pr-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 placeholder-gray-500 focus:border-green-500 focus:ring-2 focus:ring-green-500/20 transition-colors"
                    autoFocus
                  />
                </form>

                {searchState.isLoading && (
                  <div className="mt-4 flex items-center text-green-600 dark:text-green-400">
                    <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-green-600 mr-2" />
                    Searching...
                  </div>
                )}
              </div>

              {/* Results */}
              <div className="max-h-96 overflow-y-auto">
                {searchState.error && (
                  <div className="p-6 text-center">
                    <AlertCircle className="w-12 h-12 text-red-500 mx-auto mb-4" />
                    <p className="text-red-600 dark:text-red-400">{searchState.error}</p>
                  </div>
                )}

                {searchState.results && (
                  <div className="p-6">
                    <div className="flex items-center justify-between mb-4 text-sm text-gray-500 dark:text-gray-400">
                      <span>
                        {searchState.results.total_count} results found
                      </span>
                      <span className="flex items-center">
                        <Zap className="w-4 h-4 mr-1" />
                        {formatQueryTime(searchState.results.query_time)}
                      </span>
                    </div>

                    {searchState.results.results.length === 0 ? (
                      <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                        <FileText className="w-12 h-12 mx-auto mb-4 opacity-50" />
                        <p>No results found for "{searchState.query}"</p>
                      </div>
                    ) : (
                      <div className="space-y-4">
                        {searchState.results.results.map((hit: SearchHit, index: number) => (
                          <motion.div
                            key={`${hit.file_id}-${index}`}
                            initial={{ opacity: 0, y: 10 }}
                            animate={{ opacity: 1, y: 0 }}
                            transition={{ delay: index * 0.05 }}
                            className="p-4 border border-gray-200 dark:border-gray-700 rounded-lg hover:border-gray-300 dark:hover:border-gray-600 transition-colors"
                          >
                            <div className="flex items-start justify-between mb-2">
                              <h3 className="font-medium text-gray-900 dark:text-gray-100">
                                {hit.filename}
                              </h3>
                              <div className="flex items-center text-xs text-gray-500 dark:text-gray-400">
                                <span className="bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-200 px-2 py-1 rounded-full">
                                  {Math.round(hit.relevance_score * 100)}% match
                                </span>
                              </div>
                            </div>
                            
                            <p className="text-gray-600 dark:text-gray-300 text-sm mb-2">
                              {highlightText(hit.content_preview, hit.highlights)}
                            </p>
                            
                            <div className="flex items-center text-xs text-gray-500 dark:text-gray-400 space-x-4">
                              <span className="flex items-center">
                                <FileText className="w-3 h-3 mr-1" />
                                {hit.metadata.file_type}
                              </span>
                              <span className="flex items-center">
                                <Clock className="w-3 h-3 mr-1" />
                                {new Date(hit.metadata.created_at).toLocaleDateString()}
                              </span>
                            </div>
                          </motion.div>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {!searchState.query && !searchState.results && !searchState.error && (
                  <div className="p-12 text-center text-gray-500 dark:text-gray-400">
                    <Search className="w-16 h-16 mx-auto mb-4 opacity-50" />
                    <p className="text-lg mb-2">Search your PixeFiles</p>
                    <p className="text-sm">Enter keywords to find content across all your files</p>
                  </div>
                )}
              </div>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

export default SearchInterface
