import React, { useState, useCallback, useRef, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Search, X, FileText, Clock, Zap, AlertCircle, Loader } from 'lucide-react'
import { pixelogApi } from '@/services/api'
import type { SearchResult, SearchHit } from '@/types/api'

// ===== COMPONENT TYPES =====

interface IntegratedSearchProps {
  readonly className?: string
}

interface SearchState {
  readonly query: string
  readonly isLoading: boolean
  readonly results: SearchResult | null
  readonly error: string | null
  readonly isExpanded: boolean
}

// ===== INTEGRATED SEARCH COMPONENT =====

const IntegratedSearch: React.FC<IntegratedSearchProps> = ({ className = '' }) => {
  const [searchState, setSearchState] = useState<SearchState>({
    query: '',
    isLoading: false,
    results: null,
    error: null,
    isExpanded: false
  })
  
  const inputRef = useRef<HTMLInputElement>(null)
  const searchTimeoutRef = useRef<NodeJS.Timeout | null>(null)

  // Auto-focus when expanded
  useEffect(() => {
    if (searchState.isExpanded && inputRef.current) {
      inputRef.current.focus()
    }
  }, [searchState.isExpanded])

  const handleSearch = useCallback(async (query: string): Promise<void> => {
    if (!query.trim()) {
      setSearchState(prev => ({ ...prev, results: null, error: null, isLoading: false }))
      return
    }

    setSearchState(prev => ({ ...prev, isLoading: true, error: null }))

    try {
      // Try the real API first
      const results = await pixelogApi.searchContent({ query: query.trim() })
      setSearchState(prev => ({
        ...prev,
        results,
        isLoading: false
      }))
    } catch (error) {
      // Fallback to local search in development
      console.log('API search failed, falling back to local search')
      
      try {
        const localResults = await performLocalSearch(query.trim())
        setSearchState(prev => ({
          ...prev,
          results: localResults,
          isLoading: false
        }))
      } catch (localError) {
        const errorMessage = localError instanceof Error ? localError.message : 'Search failed'
        setSearchState(prev => ({
          ...prev,
          error: errorMessage,
          isLoading: false,
          results: null
        }))
      }
    }
  }, [])

  // Local search fallback for development
  const performLocalSearch = useCallback(async (query: string): Promise<SearchResult> => {
    // Get files from the existing query
    const files = await pixelogApi.getPixeFiles()
    const searchQuery = query.toLowerCase()
    const startTime = Date.now()
    
    // Simple semantic matching
    const searchResults: SearchHit[] = []
    
    files.forEach(file => {
      let score = 0
      const highlights: string[] = []
      let contentPreview = ''
      
      // Filename matching
      if (file.name.toLowerCase().includes(searchQuery)) {
        score += 0.9
        highlights.push(searchQuery)
        contentPreview = `File: ${file.name}`
      }
      
      // Path matching  
      if (file.path.toLowerCase().includes(searchQuery)) {
        score += 0.7
        if (!highlights.includes(searchQuery)) highlights.push(searchQuery)
        contentPreview = contentPreview || `Path: ${file.path}`
      }
      
      // Semantic matching for common terms
      const semanticMatches = {
        'photo': ['jpg', 'jpeg', 'png', 'gif', 'image', 'pic'],
        'document': ['pdf', 'doc', 'docx', 'txt', 'text'],
        'video': ['mp4', 'mov', 'avi', 'mkv', 'video'],
        'audio': ['mp3', 'wav', 'flac', 'music', 'sound'],
        'archive': ['zip', 'rar', '7z', 'tar', 'archive'],
        'spreadsheet': ['xls', 'xlsx', 'csv', 'sheet'],
        'presentation': ['ppt', 'pptx', 'slide'],
        'code': ['js', 'ts', 'py', 'java', 'cpp', 'code', 'src'],
        'tax': ['tax', 'w2', '1099', 'receipt', 'irs'],
        'work': ['work', 'office', 'business', 'meeting', 'report'],
        'personal': ['personal', 'family', 'vacation', 'trip', 'home']
      }
      
      Object.entries(semanticMatches).forEach(([concept, keywords]) => {
        if (searchQuery.includes(concept)) {
          keywords.forEach(keyword => {
            if (file.name.toLowerCase().includes(keyword) || file.path.toLowerCase().includes(keyword)) {
              score += 0.6
              highlights.push(keyword)
              contentPreview = contentPreview || `${concept} file containing ${keyword}`
            }
          })
        }
      })
      
      // Date-based search
      if (searchQuery.includes('recent') || searchQuery.includes('new') || searchQuery.includes('latest')) {
        const fileDate = new Date(file.created_at)
        const daysSinceCreated = (Date.now() - fileDate.getTime()) / (1000 * 60 * 60 * 24)
        if (daysSinceCreated < 7) {
          score += 0.5
          contentPreview = contentPreview || `Recent file from ${fileDate.toLocaleDateString()}`
        }
      }
      
      if (score > 0) {
        searchResults.push({
          file_id: file.id,
          filename: file.name,
          content_preview: contentPreview || `File: ${file.name}`,
          relevance_score: Math.min(score, 1),
          highlights: highlights as readonly string[],
          metadata: {
            file_type: file.name.split('.').pop()?.toUpperCase() || 'FILE',
            size: parseInt(file.size.replace(/[^\d]/g, '')) * 1024 * 1024 || 0,
            created_at: file.created_at,
            classification: 'unclassified',
            tags: [] as readonly string[]
          }
        })
      }
    })
    
    const results = searchResults
      .sort((a, b) => b.relevance_score - a.relevance_score)
      .slice(0, 10)
    
    const queryTime = Date.now() - startTime
    
    return {
      results,
      total_count: results.length,
      query_time: queryTime,
      suggestions: results.length === 0 ? ['Try "photos", "documents", or "recent files"'] as readonly string[] : [] as readonly string[]
    }
  }, [])

  const handleInputClick = (): void => {
    setSearchState(prev => ({ ...prev, isExpanded: true }))
  }

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>): void => {
    const newQuery = e.target.value
    setSearchState(prev => ({ ...prev, query: newQuery }))
    
    // Clear previous timeout
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current)
    }
    
    // Debounced search
    if (newQuery.trim()) {
      searchTimeoutRef.current = setTimeout(() => {
        void handleSearch(newQuery)
      }, 400)
    } else {
      setSearchState(prev => ({ ...prev, results: null, error: null }))
    }
  }

  const handleClear = (): void => {
    setSearchState({
      query: '',
      isLoading: false,
      results: null,
      error: null,
      isExpanded: false
    })
    
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current)
    }
  }

  const handleResultClick = (hit: SearchHit): void => {
    // Handle result click - could navigate or download
    console.log('Selected search result:', hit)
  }

  const highlightText = (text: string, highlights: readonly string[]): JSX.Element => {
    if (!highlights.length) return <span>{text}</span>
    
    let result = text
    highlights.forEach(highlight => {
      const regex = new RegExp(`(${highlight})`, 'gi')
      result = result.replace(regex, '<mark class="bg-cyan-400/30 text-cyan-100 rounded px-1">$1</mark>')
    })
    
    return <span dangerouslySetInnerHTML={{ __html: result }} />
  }

  const formatQueryTime = (timeMs: number): string => {
    return timeMs < 1000 ? `${Math.round(timeMs)}ms` : `${(timeMs / 1000).toFixed(2)}s`
  }

  const hasResults = searchState.results && searchState.results.results.length > 0
  const showResults = searchState.isExpanded && (hasResults || searchState.error || searchState.query)

  return (
    <div className={className}>
      {/* Search Input */}
      <motion.div
        layout
        className="relative"
      >
        <Search className="absolute left-4 top-1/2 transform -translate-y-1/2 w-5 h-5 cyber-text-secondary z-10" />
        <input
          ref={inputRef}
          type="text"
          value={searchState.query}
          onChange={handleInputChange}
          onClick={handleInputClick}
          placeholder={searchState.isExpanded ? "Search with AI... Try 'vacation photos' or 'tax documents'" : "Search through your converted files..."}
          className="
            w-full pl-12 pr-12 py-4
            bg-gray-900/30 backdrop-blur-sm 
            border border-cyan-500/20 rounded-lg
            cyber-text-primary placeholder-gray-400
            focus:outline-none focus:border-cyan-400/40 focus:bg-gray-900/50
            transition-all duration-300
            cyber-body text-base
            hover:border-cyan-500/30
          "
        />
        
        {/* Loading/Clear Button */}
        <div className="absolute right-4 top-1/2 transform -translate-y-1/2 z-10">
          {searchState.isLoading ? (
            <Loader className="w-5 h-5 cyber-text-cyber animate-spin" />
          ) : searchState.isExpanded && (searchState.query || searchState.results) ? (
            <button
              onClick={handleClear}
              className="cyber-text-secondary hover:cyber-text-primary transition-colors p-1"
              aria-label="Clear search"
            >
              <X className="w-5 h-5" />
            </button>
          ) : null}
        </div>
      </motion.div>

      {/* Search Results */}
      <AnimatePresence mode="wait">
        {showResults && (
          <motion.div
            initial={{ opacity: 0, height: 0, y: -10 }}
            animate={{ opacity: 1, height: 'auto', y: 0 }}
            exit={{ opacity: 0, height: 0, y: -10 }}
            transition={{ duration: 0.3, ease: 'easeInOut' }}
            className="mt-4 overflow-hidden"
          >
            <div className="
              bg-gray-900/50 backdrop-blur-md 
              border border-cyan-500/20 rounded-lg
              shadow-2xl shadow-cyan-500/5
            ">
              {/* Results Header */}
              {searchState.results && (
                <div className="px-6 py-4 border-b border-cyan-500/10">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-4">
                      <h3 className="cyber-h3 text-lg cyber-text-primary">Search Results</h3>
                      <span className="cyber-mono text-sm cyber-text-secondary">
                        {searchState.results.total_count} results
                      </span>
                    </div>
                    <div className="flex items-center space-x-1 cyber-mono text-xs cyber-text-secondary">
                      <Zap className="w-4 h-4" />
                      <span>{formatQueryTime(searchState.results.query_time)}</span>
                    </div>
                  </div>
                </div>
              )}

              {/* Results Content */}
              <div className="p-6">
                {/* Error State */}
                {searchState.error && (
                  <div className="text-center py-8">
                    <AlertCircle className="w-12 h-12 text-red-400 mx-auto mb-4" />
                    <p className="cyber-body cyber-text-primary mb-2">Search Error</p>
                    <p className="cyber-mono text-sm cyber-text-secondary">{searchState.error}</p>
                  </div>
                )}

                {/* No Results */}
                {searchState.results && searchState.results.results.length === 0 && !searchState.error && (
                  <div className="text-center py-8">
                    <FileText className="w-12 h-12 cyber-text-secondary mx-auto mb-4 opacity-50" />
                    <p className="cyber-body cyber-text-primary mb-2">No results found</p>
                    <p className="cyber-mono text-sm cyber-text-secondary">
                      Try different keywords or upload more files
                    </p>
                  </div>
                )}

                {/* Results List */}
                {hasResults && (
                  <div className="space-y-4">
                    {searchState.results.results.map((hit: SearchHit, index: number) => (
                      <motion.button
                        key={`${hit.file_id}-${index}`}
                        initial={{ opacity: 0, x: -20 }}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{ duration: 0.2, delay: index * 0.05 }}
                        onClick={() => handleResultClick(hit)}
                        className="
                          w-full text-left p-4 rounded-lg
                          cyber-bg-panel hover:bg-cyan-400/10 
                          border border-transparent hover:border-cyan-400/20
                          transition-all duration-200
                          group
                        "
                      >
                        <div className="flex items-start justify-between mb-3">
                          <h4 className="cyber-body cyber-text-primary font-medium text-lg group-hover:cyber-text-cyber transition-colors">
                            {hit.filename}
                          </h4>
                          <div className="flex items-center space-x-2">
                            <span className="
                              bg-cyan-400/20 cyber-text-cyber px-3 py-1 rounded-full 
                              cyber-mono text-xs font-medium
                            ">
                              {Math.round(hit.relevance_score * 100)}%
                            </span>
                          </div>
                        </div>
                        
                        <p className="cyber-body cyber-text-secondary text-sm mb-3 leading-relaxed">
                          {highlightText(hit.content_preview, hit.highlights)}
                        </p>
                        
                        <div className="flex items-center space-x-6 cyber-mono text-xs cyber-text-tertiary">
                          <span className="flex items-center space-x-1">
                            <FileText className="w-3 h-3" />
                            <span>{hit.metadata.file_type}</span>
                          </span>
                          <span className="flex items-center space-x-1">
                            <Clock className="w-3 h-3" />
                            <span>{new Date(hit.metadata.created_at).toLocaleDateString()}</span>
                          </span>
                          {hit.metadata.size && (
                            <span>{(hit.metadata.size / 1024 / 1024).toFixed(1)} MB</span>
                          )}
                        </div>
                      </motion.button>
                    ))}
                  </div>
                )}

                {/* Empty Search State */}
                {!searchState.query && !searchState.results && !searchState.error && (
                  <div className="text-center py-12">
                    <Search className="w-16 h-16 cyber-text-secondary mx-auto mb-6 opacity-50" />
                    <h3 className="cyber-h3 text-xl cyber-text-primary mb-3">AI-Powered Semantic Search</h3>
                    <p className="cyber-body cyber-text-secondary mb-4">
                      Search naturally using plain language
                    </p>
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4 max-w-2xl mx-auto">
                      <div className="cyber-bg-panel p-4 rounded-lg">
                        <p className="cyber-mono text-xs cyber-text-cyber mb-1">Try asking:</p>
                        <p className="cyber-body text-sm cyber-text-secondary">"vacation photos"</p>
                      </div>
                      <div className="cyber-bg-panel p-4 rounded-lg">
                        <p className="cyber-mono text-xs cyber-text-cyber mb-1">Or search for:</p>
                        <p className="cyber-body text-sm cyber-text-secondary">"tax documents"</p>
                      </div>
                      <div className="cyber-bg-panel p-4 rounded-lg">
                        <p className="cyber-mono text-xs cyber-text-cyber mb-1">Even try:</p>
                        <p className="cyber-body text-sm cyber-text-secondary">"meeting notes"</p>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

export default IntegratedSearch
