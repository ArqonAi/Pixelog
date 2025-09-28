import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.jsx'
import './index.css'
// 1. Import the necessary components from React Query
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// 2. Initialize a new query client instance
const queryClient = new QueryClient({
  // Optional: Set default options for retries, caching, etc.
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60 * 5, // Data is considered fresh for 5 minutes
    },
  },
});

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    {/* 3. Wrap the App with the QueryClientProvider */}
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </React.StrictMode>,
)
