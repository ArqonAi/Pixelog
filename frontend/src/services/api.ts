/**
 * 🔒 CLASSIFIED - INQTEL API Client
 * Enterprise-grade TypeScript API client for intelligence operations
 */

import type {
  PixeFile,
  ConversionJob,
  ConversionRequest,
  ConversionOptions,
  ExtractionResult,
  ExtractionRequest,
  SearchQuery,
  SearchResult,
  SystemHealth,
  ExtractedContent,
  ProgressUpdate,
  ApiResponse,
  ApiError,
  CloudUploadResult,
} from '@/types/api';

// ===== API CLIENT CONFIGURATION =====

const API_BASE_URL = '/api' as const;
const DEFAULT_TIMEOUT = 30000; // 30 seconds
const MAX_RETRIES = 3;

interface RequestConfig {
  readonly timeout?: number;
  readonly retries?: number;
  readonly headers?: HeadersInit;
}

interface ProgressCallback {
  (progress: ProgressUpdate): void;
}

// ===== CUSTOM ERROR CLASSES =====

export class PixelogAPIError extends Error {
  constructor(
    message: string,
    public readonly code: number,
    public readonly details?: Record<string, unknown>
  ) {
    super(message);
    this.name = 'PixelogAPIError';
  }
}

export class NetworkError extends PixelogAPIError {
  constructor(message = 'Network request failed') {
    super(message, 0);
    this.name = 'NetworkError';
  }
}

export class TimeoutError extends PixelogAPIError {
  constructor(message = 'Request timeout') {
    super(message, 408);
    this.name = 'TimeoutError';
  }
}

// ===== HTTP CLIENT =====

class HTTPClient {
  private async makeRequest<T>(
    endpoint: string,
    options: RequestInit = {},
    config: RequestConfig = {}
  ): Promise<T> {
    const { timeout = DEFAULT_TIMEOUT, retries = MAX_RETRIES } = config;
    
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);
    
    const url = `${API_BASE_URL}${endpoint}`;
    const requestOptions: RequestInit = {
      ...options,
      signal: controller.signal,
      headers: {
        'Content-Type': 'application/json',
        ...config.headers,
        ...options.headers,
      },
    };

    let lastError: Error;
    
    for (let attempt = 0; attempt <= retries; attempt++) {
      try {
        const response = await fetch(url, requestOptions);
        clearTimeout(timeoutId);
        
        if (!response.ok) {
          const errorData: ApiError = await response.json().catch(() => ({
            error: 'Unknown error',
            code: response.status,
            timestamp: new Date().toISOString(),
            request_id: '',
          }));
          
          throw new PixelogAPIError(
            errorData.error,
            errorData.code,
            errorData.details
          );
        }
        
        return await response.json();
      } catch (error) {
        lastError = error as Error;
        
        if (error instanceof PixelogAPIError) {
          throw error;
        }
        
        if (error instanceof DOMException && error.name === 'AbortError') {
          throw new TimeoutError();
        }
        
        if (attempt === retries) {
          throw new NetworkError(lastError.message);
        }
        
        // Exponential backoff
        await new Promise(resolve => 
          setTimeout(resolve, Math.pow(2, attempt) * 1000)
        );
      }
    }
    
    throw new NetworkError(lastError!.message);
  }

  async get<T>(endpoint: string, config?: RequestConfig): Promise<T> {
    return this.makeRequest<T>(endpoint, { method: 'GET' }, config);
  }

  async post<T>(
    endpoint: string, 
    data?: unknown, 
    config?: RequestConfig
  ): Promise<T> {
    const body = data instanceof FormData ? data : JSON.stringify(data);
    const headers = data instanceof FormData ? {} : { 'Content-Type': 'application/json' };
    
    return this.makeRequest<T>(
      endpoint,
      { method: 'POST', body },
      { ...config, headers: { ...headers, ...config?.headers } }
    );
  }

  async put<T>(
    endpoint: string, 
    data?: unknown, 
    config?: RequestConfig
  ): Promise<T> {
    return this.makeRequest<T>(
      endpoint,
      { method: 'PUT', body: JSON.stringify(data) },
      config
    );
  }

  async delete<T>(endpoint: string, config?: RequestConfig): Promise<T> {
    return this.makeRequest<T>(endpoint, { method: 'DELETE' }, config);
  }

  async blob(endpoint: string, config?: RequestConfig): Promise<Blob> {
    const controller = new AbortController();
    const timeout = config?.timeout ?? DEFAULT_TIMEOUT;
    const timeoutId = setTimeout(() => controller.abort(), timeout);
    
    try {
      const response = await fetch(`${API_BASE_URL}${endpoint}`, {
        signal: controller.signal,
        ...(config?.headers && { headers: config.headers }),
      });
      
      clearTimeout(timeoutId);
      
      if (!response.ok) {
        throw new PixelogAPIError('Failed to fetch blob', response.status);
      }
      
      return await response.blob();
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') {
        throw new TimeoutError();
      }
      throw error;
    }
  }
}

// ===== WEBSOCKET CLIENT =====

class WebSocketClient {
  private connections = new Map<string, WebSocket>();

  trackProgress(jobId: string, onProgress: ProgressCallback): () => void {
    // Check if we're in development mode by checking if the backend is mock
    const isDevelopment = jobId.startsWith('dev_job_');
    
    if (isDevelopment) {
      // Simulate progress updates for development
      let progress = 0;
      const interval = setInterval(() => {
        progress += 20;
        
        if (progress >= 100) {
          onProgress({
            status: 'completed',
            progress: 100,
            message: 'Mock conversion completed',
            job_id: jobId
          });
          clearInterval(interval);
        } else {
          onProgress({
            status: 'processing',
            progress,
            message: `Mock processing: ${progress}%`,
            job_id: jobId
          });
        }
      }, 500); // Update every 500ms
      
      return () => clearInterval(interval);
    }
    
    // Production WebSocket connection
    const wsUrl = `ws://${window.location.host}/api/ws/status/${jobId}`;
    const ws = new WebSocket(wsUrl);
    
    ws.onmessage = (event) => {
      try {
        const progress: ProgressUpdate = JSON.parse(event.data);
        onProgress(progress);
        
        if (progress.status === 'completed' || progress.status === 'failed') {
          this.closeConnection(jobId);
        }
      } catch (error) {
        console.error('Failed to parse WebSocket message:', error);
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
      this.closeConnection(jobId);
    };

    ws.onclose = () => {
      this.connections.delete(jobId);
    };

    this.connections.set(jobId, ws);
    
    // Return cleanup function
    return () => this.closeConnection(jobId);
  }

  private closeConnection(jobId: string): void {
    const ws = this.connections.get(jobId);
    if (ws) {
      ws.close();
      this.connections.delete(jobId);
    }
  }

  closeAll(): void {
    for (const ws of this.connections.values()) {
      ws.close();
    }
    this.connections.clear();
  }
}

// ===== MAIN API CLIENT =====

export class PixelogAPI {
  private readonly http = new HTTPClient();
  private readonly ws = new WebSocketClient();

  // ===== FILE OPERATIONS =====

  async convertFiles(
    files: readonly File[],
    options?: ConversionOptions,
    onProgress?: ProgressCallback
  ): Promise<string> {
    const formData = new FormData();
    
    files.forEach(file => {
      formData.append('files', file);
    });

    // Add conversion options
    if (options?.compression_level) {
      formData.append('compression_level', options.compression_level);
    }
    if (options?.qr_code_density) {
      formData.append('qr_code_density', options.qr_code_density);
    }
    if (options?.encryption !== undefined) {
      formData.append('encryption', options.encryption.toString());
    }
    if (options?.classification) {
      formData.append('classification', options.classification);
    }

    // Default options for backward compatibility
    formData.append('quality', '23');
    formData.append('framerate', '0.5');
    formData.append('chunksize', '2800');

    const result = await this.http.post<ConversionJob>(
      '/convert', 
      formData,
      { timeout: 120000 } // 2 minutes for file conversion
    );
    
    // Start progress tracking if callback provided
    if (onProgress && result.job_id) {
      this.ws.trackProgress(result.job_id, onProgress);
    }

    return result.job_id;
  }

  async getPixeFiles(): Promise<readonly PixeFile[]> {
    return this.http.get<readonly PixeFile[]>('/files');
  }

  async getPixeFile(fileId: string): Promise<PixeFile> {
    return this.http.get<PixeFile>(`/files/${fileId}/info`);
  }

  async deletePixeFile(fileId: string): Promise<{ message: string; file_id: string }> {
    return this.http.delete<{ message: string; file_id: string }>(`/files/${fileId}`);
  }

  async downloadPixeFile(fileId: string): Promise<Blob> {
    return this.http.blob(`/files/${fileId}`, { timeout: 60000 }); // 1 minute for download
  }

  async extractPixeFile(
    filename: string,
    options?: Partial<ExtractionRequest>
  ): Promise<ExtractionResult> {
    const endpoint = `/extract/${encodeURIComponent(filename)}`;
    return this.http.post<ExtractionResult>(endpoint, options);
  }

  async getPixeContents(filename: string): Promise<ExtractedContent> {
    const endpoint = `/contents/${encodeURIComponent(filename)}`;
    return this.http.get<ExtractedContent>(endpoint);
  }

  // ===== JOB MANAGEMENT =====

  async getJobStatus(jobId: string): Promise<ConversionJob> {
    return this.http.get<ConversionJob>(`/status/${jobId}`);
  }

  async cancelJob(jobId: string): Promise<{ message: string }> {
    return this.http.post<{ message: string }>(`/jobs/${jobId}/cancel`);
  }

  trackJobProgress(jobId: string, onProgress: ProgressCallback): () => void {
    return this.ws.trackProgress(jobId, onProgress);
  }

  // ===== SEARCH OPERATIONS =====

  async searchContent(query: SearchQuery): Promise<SearchResult> {
    return this.http.post<SearchResult>('/search/query', query, { timeout: 45000 });
  }

  async getSearchSuggestions(query: string): Promise<readonly string[]> {
    return this.http.get<readonly string[]>(`/search/suggestions?q=${encodeURIComponent(query)}`);
  }

  async getSearchHistory(): Promise<readonly string[]> {
    return this.http.get<readonly string[]>('/search/history');
  }

  async clearSearchHistory(): Promise<{ message: string }> {
    return this.http.delete<{ message: string }>('/search/history');
  }

  // ===== CLOUD OPERATIONS =====

  async uploadToCloud(file: File): Promise<CloudUploadResult> {
    const formData = new FormData();
    formData.append('file', file);
    
    return this.http.post<CloudUploadResult>(
      '/cloud/upload',
      formData,
      { timeout: 300000 } // 5 minutes for cloud upload
    );
  }

  async listCloudFiles(): Promise<readonly CloudUploadResult[]> {
    return this.http.get<readonly CloudUploadResult[]>('/cloud/files');
  }

  async deleteCloudFile(path: string): Promise<{ message: string }> {
    return this.http.delete<{ message: string }>(`/cloud/files/${encodeURIComponent(path)}`);
  }

  // ===== SYSTEM OPERATIONS =====

  async getHealth(): Promise<SystemHealth> {
    return this.http.get<SystemHealth>('/health');
  }

  async getSystemStats(): Promise<SystemHealth> {
    return this.http.get<SystemHealth>('/system/stats');
  }

  async getSystemLogs(
    level?: 'info' | 'warning' | 'error',
    limit = 100
  ): Promise<readonly any[]> {
    const params = new URLSearchParams();
    if (level) params.append('level', level);
    params.append('limit', limit.toString());
    
    return this.http.get<readonly any[]>(`/system/logs?${params}`);
  }

  // ===== UTILITY METHODS =====

  async ping(): Promise<{ pong: string; timestamp: string }> {
    return this.http.get<{ pong: string; timestamp: string }>('/ping');
  }

  async validateFile(file: File): Promise<{ valid: boolean; reason?: string }> {
    const formData = new FormData();
    formData.append('file', file);
    
    return this.http.post<{ valid: boolean; reason?: string }>('/validate', formData);
  }

  // ===== CLEANUP =====

  destroy(): void {
    this.ws.closeAll();
  }
}

// ===== SINGLETON INSTANCE =====

export const pixelogApi = new PixelogAPI()

// ===== CLOUD STORAGE API =====

export interface CloudProvider {
  readonly provider: 'aws' | 'gcp' | 'azure' | 'digitalocean'
  readonly accessKey?: string
  readonly secretKey?: string
  readonly serviceAccountJSON?: string
  readonly region?: string
  readonly bucketName: string
}

export interface CloudFile {
  readonly id: string
  readonly filename: string
  readonly size: number
  readonly cloudUrl: string
  readonly provider: string
  readonly uploadedAt: string
  readonly downloadUrl?: string
}

export interface CloudStatus {
  readonly configured: boolean
  readonly provider?: string
  readonly bucketName?: string
  readonly lastSync?: string
}

class CloudStorageAPI {
  private readonly http = new HTTPClient()

  async getStatus(): Promise<CloudStatus> {
    return this.http.get<CloudStatus>('/cloud/status')
  }

  async configureProvider(config: CloudProvider): Promise<{ success: boolean; message?: string }> {
    return this.http.post<{ success: boolean; message?: string }>('/cloud/configure', config)
  }

  async testConnection(config: CloudProvider): Promise<{ success: boolean; message?: string }> {
    return this.http.post<{ success: boolean; message?: string }>('/cloud/test', config)
  }

  async uploadFiles(files: readonly File[]): Promise<readonly CloudFile[]> {
    const formData = new FormData()
    files.forEach(file => formData.append('files', file))
    
    const response = await fetch(`${API_BASE_URL}/cloud/upload`, {
      method: 'POST',
      body: formData
    })
    
    if (!response.ok) {
      throw new PixelogAPIError(`Upload failed: ${response.statusText}`, response.status)
    }
    
    return response.json()
  }

  async getCloudFiles(): Promise<readonly CloudFile[]> {
    return this.http.get<readonly CloudFile[]>('/cloud/files')
  }

  async deleteCloudFile(fileId: string): Promise<void> {
    await this.http.delete(`/cloud/files/${fileId}`)
  }

  async getDownloadUrl(fileId: string): Promise<{ downloadUrl: string; expiresAt: string }> {
    return this.http.get<{ downloadUrl: string; expiresAt: string }>(`/cloud/download/${fileId}`)
  }

  async getSyncSettings(): Promise<{ autoUpload: boolean; syncOnCreate: boolean }> {
    return this.http.get<{ autoUpload: boolean; syncOnCreate: boolean }>('/cloud/sync-settings')
  }

  async updateSyncSettings(settings: { autoUpload: boolean; syncOnCreate: boolean }): Promise<void> {
    await this.http.put('/cloud/sync-settings', settings)
  }
}

export const cloudApi = new CloudStorageAPI();

// Cleanup on page unload
if (typeof window !== 'undefined') {
  window.addEventListener('beforeunload', () => {
    pixelogApi.destroy();
  });
}

// ===== TYPE GUARDS =====

export function isPixelogAPIError(error: unknown): error is PixelogAPIError {
  return error instanceof PixelogAPIError;
}

export function isNetworkError(error: unknown): error is NetworkError {
  return error instanceof NetworkError;
}

export function isTimeoutError(error: unknown): error is TimeoutError {
  return error instanceof TimeoutError;
}

// ===== UTILITY FUNCTIONS =====

export function createProgressCallback(
  onProgress?: (progress: number) => void,
  onStatusChange?: (status: string) => void,
  onMessage?: (message: string) => void
): ProgressCallback {
  return (update: ProgressUpdate) => {
    onProgress?.(update.progress);
    onStatusChange?.(update.status);
    onMessage?.(update.message);
  };
}

export function formatFileSize(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let size = bytes;
  let unitIndex = 0;
  
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }
  
  return `${size.toFixed(1)} ${units[unitIndex]}`;
}

export function validateFileType(file: File, allowedTypes: readonly string[]): boolean {
  return allowedTypes.some(type => {
    if (type.endsWith('/*')) {
      return file.type.startsWith(type.slice(0, -1));
    }
    return file.type === type;
  });
}

export function createFileUploadFormData(
  files: readonly File[],
  options?: Record<string, string | boolean | number>
): FormData {
  const formData = new FormData();
  
  files.forEach(file => {
    formData.append('files', file);
  });
  
  if (options) {
    Object.entries(options).forEach(([key, value]) => {
      formData.append(key, String(value));
    });
  }
  
  return formData;
}

