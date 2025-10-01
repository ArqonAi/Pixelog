import React from 'react'
import { Routes, Route } from 'react-router-dom'
import Header from './components/Header'
import LLMPage from './pages/LLMPage'
import CreatePage from './pages/CreatePage'
import CLIPage from './pages/CLIPage'
import APIPage from './pages/APIPage'

const App: React.FC = () => {
  return (
    <div className="min-h-screen cyber-bg-void">
      <Header onSearchClick={() => {}} />
      
      <Routes>
        <Route path="/" element={<LLMPage />} />
        <Route path="/create" element={<CreatePage />} />
        <Route path="/cli" element={<CLIPage />} />
        <Route path="/api" element={<APIPage />} />
        <Route path="/test" element={<div>Test Route Works!</div>} />
      </Routes>
    </div>
  )
}

export default App
