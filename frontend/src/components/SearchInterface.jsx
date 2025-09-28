import React, { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Search, FileText, Clock, Zap, X, Filter } from 'lucide-react';

const SearchInterface = ({ isOpen, onClose }) => {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [searchEnabled, setSearchEnabled] = useState(false);
  const [searchProvider, setSearchProvider] = useState('');
  const [filters, setFilters] = useState({
    limit: 10,
    threshold: 0.7
  });

  // Check if search is enabled
  useEffect(() => {
    const checkSearchStatus = async () => {
      try {
        const response = await fetch('/api/health');
        const data = await response.json();
        setSearchEnabled(data.search_enabled && data.search_status === 'available');
        setSearchProvider(data.search_provider || '');
      } catch (err) {
        console.error('Failed to check search status:', err);
      }
    };

    if (isOpen) {
      checkSearchStatus();
    }
  }, [isOpen]);

  const handleSearch = async (e) => {
    e.preventDefault();
    if (!query.trim() || !searchEnabled) return;

    setLoading(true);
    setError('');

    try {
      const response = await fetch('/api/search/query', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          query: query.trim(),
          limit: filters.limit,
          threshold: filters.threshold,
        }),
      });

      if (!response.ok) {
        throw new Error(`Search failed: ${response.statusText}`);
      }

      const data = await response.json();
      setResults(data.results || []);
    } catch (err) {
      setError(err.message);
      setResults([]);
    } finally {
      setLoading(false);
    }
  };

  const formatSimilarity = (similarity) => {
    return `${Math.round(similarity * 100)}%`;
  };

  const truncateContent = (content, maxLength = 200) => {
    if (content.length <= maxLength) return content;
    return content.substring(0, maxLength) + '...';
  };

  if (!isOpen) return null;

  return (
    <AnimatePresence>
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        exit={{ opacity: 0 }}
        className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 flex items-center justify-center p-4"
        onClick={onClose}
      >
        <motion.div
          initial={{ scale: 0.95, opacity: 0 }}
          animate={{ scale: 1, opacity: 1 }}
          exit={{ scale: 0.95, opacity: 0 }}
          className="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl max-w-4xl w-full max-h-[90vh] overflow-hidden"
          onClick={(e) => e.stopPropagation()}
        >
          {/* Header */}
          <div className="border-b border-gray-200 dark:border-gray-700 p-6">
            <div className="flex items-center justify-between">
              <div className="flex items-center space-x-3">
                <div className="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
                  <Search className="w-5 h-5 text-blue-600 dark:text-blue-400" />
                </div>
                <div>
                  <h2 className="text-xl font-semibold text-gray-900 dark:text-white">
                    Semantic Search
                  </h2>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    Find content using natural language queries
                    {searchProvider && <span className="ml-2 text-blue-600 dark:text-blue-400">• {searchProvider}</span>}
                  </p>
                </div>
              </div>
              <button
                onClick={onClose}
                className="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              >
                <X className="w-5 h-5 text-gray-500" />
              </button>
            </div>
          </div>

          {/* Search Form */}
          <div className="p-6 border-b border-gray-200 dark:border-gray-700">
            {!searchEnabled ? (
              <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
                <div className="flex items-center space-x-2">
                  <Zap className="w-5 h-5 text-yellow-600 dark:text-yellow-400" />
                  <p className="text-yellow-800 dark:text-yellow-200">
                    Search functionality requires an AI provider. Configure one of: OpenAI, OpenRouter, Gemini, Grok, or local Ollama.
                  </p>
                </div>
              </div>
            ) : (
              <form onSubmit={handleSearch} className="space-y-4">
                <div className="relative">
                  <Search className="absolute left-3 top-3 w-5 h-5 text-gray-400" />
                  <input
                    type="text"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    placeholder="Search for concepts, keywords, or ask questions..."
                    className="w-full pl-10 pr-4 py-3 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-500 focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                    disabled={loading}
                  />
                </div>

                <div className="flex items-center justify-between">
                  <div className="flex items-center space-x-4">
                    <div className="flex items-center space-x-2">
                      <Filter className="w-4 h-4 text-gray-500" />
                      <select
                        value={filters.limit}
                        onChange={(e) => setFilters(prev => ({ ...prev, limit: parseInt(e.target.value) }))}
                        className="text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                      >
                        <option value={5}>5 results</option>
                        <option value={10}>10 results</option>
                        <option value={20}>20 results</option>
                      </select>
                    </div>
                    <div className="flex items-center space-x-2">
                      <span className="text-sm text-gray-500">Similarity:</span>
                      <select
                        value={filters.threshold}
                        onChange={(e) => setFilters(prev => ({ ...prev, threshold: parseFloat(e.target.value) }))}
                        className="text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                      >
                        <option value={0.5}>50%+</option>
                        <option value={0.7}>70%+</option>
                        <option value={0.8}>80%+</option>
                        <option value={0.9}>90%+</option>
                      </select>
                    </div>
                  </div>

                  <button
                    type="submit"
                    disabled={!query.trim() || loading}
                    className="px-6 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-400 text-white rounded-lg transition-colors flex items-center space-x-2"
                  >
                    {loading ? (
                      <>
                        <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                        <span>Searching...</span>
                      </>
                    ) : (
                      <>
                        <Search className="w-4 h-4" />
                        <span>Search</span>
                      </>
                    )}
                  </button>
                </div>
              </form>
            )}
          </div>

          {/* Results */}
          <div className="flex-1 overflow-y-auto max-h-96">
            {error && (
              <div className="p-6 bg-red-50 dark:bg-red-900/20 border-l-4 border-red-500">
                <p className="text-red-700 dark:text-red-300">{error}</p>
              </div>
            )}

            {results.length === 0 && !loading && !error && query && (
              <div className="p-8 text-center">
                <div className="w-16 h-16 mx-auto mb-4 bg-gray-100 dark:bg-gray-700 rounded-full flex items-center justify-center">
                  <Search className="w-8 h-8 text-gray-400" />
                </div>
                <p className="text-gray-500 dark:text-gray-400">No results found for your query.</p>
                <p className="text-sm text-gray-400 dark:text-gray-500 mt-1">
                  Try different keywords or lower the similarity threshold.
                </p>
              </div>
            )}

            {results.length > 0 && (
              <div className="p-6 space-y-4">
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  Found {results.length} result{results.length !== 1 ? 's' : ''}
                </p>
                
                {results.map((result, index) => (
                  <motion.div
                    key={result.document.id}
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: index * 0.1 }}
                    className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
                  >
                    <div className="flex items-start justify-between mb-2">
                      <div className="flex items-center space-x-2">
                        <FileText className="w-4 h-4 text-blue-600" />
                        <span className="font-medium text-gray-900 dark:text-white">
                          {result.document.metadata?.filename || 'Document'}
                        </span>
                      </div>
                      <div className="flex items-center space-x-2 text-sm text-gray-500">
                        <span className="bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 px-2 py-1 rounded">
                          {formatSimilarity(result.similarity)} match
                        </span>
                      </div>
                    </div>
                    
                    <p className="text-gray-700 dark:text-gray-300 text-sm leading-relaxed mb-2">
                      {truncateContent(result.document.content)}
                    </p>
                    
                    {result.document.metadata && (
                      <div className="flex items-center space-x-4 text-xs text-gray-500">
                        {result.document.created_at && (
                          <div className="flex items-center space-x-1">
                            <Clock className="w-3 h-3" />
                            <span>{new Date(result.document.created_at).toLocaleDateString()}</span>
                          </div>
                        )}
                        {result.document.metadata.content_length && (
                          <span>{result.document.metadata.content_length} characters</span>
                        )}
                      </div>
                    )}
                  </motion.div>
                ))}
              </div>
            )}
          </div>
        </motion.div>
      </motion.div>
    </AnimatePresence>
  );
};

export default SearchInterface;
