import React from 'react'
import { Routes, Route } from 'react-router-dom'
import Header from './components/Header'
import HomePage from './pages/HomePage'
import CLIPage from './pages/CLIPage'
import LLMPage from './pages/LLMPage'

const App: React.FC = () => {
  return (
    <div className="min-h-screen cyber-bg-void">
      <Header onSearchClick={() => {}} />
      
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/cli" element={<CLIPage />} />
        <Route path="/llm" element={<LLMPage />} />
      </Routes>
    </div>
  )
}

export default App
