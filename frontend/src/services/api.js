const API_BASE_URL = '/api'

class PixelogAPI {
  async convertFiles(files, onProgress) {
    const formData = new FormData()
    
    files.forEach(file => {
      formData.append('files', file)
    })

    // Add default conversion options
    formData.append('quality', '23')
    formData.append('framerate', '0.5')
    formData.append('chunksize', '2800')

    const response = await fetch(`${API_BASE_URL}/convert`, {
      method: 'POST',
      body: formData
    })

    if (!response.ok) {
      const error = await response.json()
      throw new Error(error.error || 'Conversion failed')
    }

    const result = await response.json()
    
    // Start progress tracking via WebSocket
    if (result.job_id) {
      this.trackProgress(result.job_id, onProgress)
    }

    return result.job_id
  }

  trackProgress(jobId, onProgress) {
    const ws = new WebSocket(`ws://${window.location.host}/api/ws/status/${jobId}`)
    
    ws.onmessage = (event) => {
      const progress = JSON.parse(event.data)
      onProgress(progress)
      
      if (progress.status === 'completed' || progress.status === 'failed') {
        ws.close()
      }
    }

    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
      ws.close()
    }
  }

  async getPixeFiles() {
    const response = await fetch(`${API_BASE_URL}/files`)
    
    if (!response.ok) {
      throw new Error('Failed to fetch PixeFiles')
    }

    return response.json()
  }

  async deletePixeFile(fileId) {
    const response = await fetch(`${API_BASE_URL}/files/${fileId}`, {
      method: 'DELETE'
    })

    if (!response.ok) {
      throw new Error('Failed to delete file')
    }

    return response.json()
  }

  async downloadPixeFile(fileId) {
    const response = await fetch(`${API_BASE_URL}/files/${fileId}`)
    
    if (!response.ok) {
      throw new Error('Failed to download file')
    }

    return response.blob()
  }

  async extractPixeFile(filename) {
    const response = await fetch(`${API_BASE_URL}/extract/${filename}`, {
      method: 'POST',
    })

    if (!response.ok) {
      throw new Error('Failed to extract file')
    }

    return response.json()
  }

  async searchContent(query) {
    const response = await fetch(`${API_BASE_URL}/search/query`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ query })
    })

    if (!response.ok) {
      throw new Error('Search failed')
    }

    return response.json()
  }

  async getHealth() {
    const response = await fetch(`${API_BASE_URL}/health`)
    
    if (!response.ok) {
      throw new Error('Health check failed')
    }

    return response.json()
  }
}

export const pixelogApi = new PixelogAPI()