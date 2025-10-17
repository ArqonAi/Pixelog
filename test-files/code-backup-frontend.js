// Frontend Application Code Backup
// React Components and Utilities

import React, { useState, useEffect } from 'react'
import axios from 'axios'

// Main App Component
export const App = () => {
  const [data, setData] = useState([])
  const [loading, setLoading] = useState(true)
  
  useEffect(() => {
    fetchData()
  }, [])
  
  const fetchData = async () => {
    try {
      const response = await axios.get('/api/data')
      setData(response.data)
    } catch (error) {
      console.error('Error fetching data:', error)
    } finally {
      setLoading(false)
    }
  }
  
  return (
    <div className="app">
      <h1>My Application</h1>
      {loading ? (
        <div>Loading...</div>
      ) : (
        <DataList items={data} />
      )}
    </div>
  )
}

// Utility functions
export const formatDate = (date) => {
  return new Date(date).toLocaleDateString()
}

export const validateEmail = (email) => {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)
}
