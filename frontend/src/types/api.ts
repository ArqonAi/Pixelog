/**
 * 🔒 CLASSIFIED - INQTEL API Type Definitions
 * Intelligence-grade type safety for Pixelog operations
 */

// ===== CORE DATA MODELS =====

export interface PixeFile {
  readonly id: string;
  readonly name: string;
  readonly size: string;
  readonly path: string;
  readonly created_at: string;
  readonly metadata?: PixeMetadata;
  readonly status?: FileStatus;
  readonly classification?: ClassificationLevel;
}

export interface PixeMetadata {
  readonly original_files: readonly string[];
  readonly compression_ratio: number;
  readonly qr_code_count: number;
  readonly video_duration: number;
  readonly frame_rate: number;
  readonly encoding_format: string;
  readonly checksum: string;
}

export interface ExtractedFile {
  readonly name: string;
  readonly type: string;
  readonly size: string;
  readonly hash: string;
  readonly created_at: string;
  readonly content_preview?: string;
}

export interface ExtractedContent {
  readonly contents: readonly ExtractedFile[];
  readonly filename: string;
  readonly extraction_time: string;
  readonly success_count: number;
  readonly total_count: number;
}

// ===== OPERATION TYPES =====

export interface ConversionJob {
  readonly job_id: string;
  readonly status: JobStatus;
  readonly progress: number;
  readonly message: string;
  readonly files?: readonly string[];
  readonly error?: string;
  readonly started_at: string;
  readonly completed_at?: string;
  readonly estimated_completion?: string;
}

export interface ConversionRequest {
  readonly files: readonly File[];
  readonly options?: ConversionOptions;
}

export interface ConversionOptions {
  readonly compression_level?: CompressionLevel;
  readonly qr_code_density?: QRDensity;
  readonly encryption?: boolean;
  readonly classification?: ClassificationLevel;
  readonly metadata_preservation?: boolean;
}

export interface ExtractionRequest {
  readonly filename: string;
  readonly output_path?: string;
  readonly verify_integrity?: boolean;
}

export interface ExtractionResult {
  readonly message: string;
  readonly extracted_files: readonly string[];
  readonly output_dir: string;
  readonly integrity_verified: boolean;
  readonly extraction_duration: number;
}

// ===== SYSTEM STATUS =====

export interface SystemHealth {
  readonly status: 'operational' | 'degraded' | 'offline';
  readonly mode: 'production' | 'development' | 'maintenance';
  readonly services: ServiceStatus;
  readonly capabilities: SystemCapabilities;
  readonly security_level: SecurityLevel;
  readonly uptime: number;
  readonly last_check: string;
}

export interface ServiceStatus {
  readonly search_enabled: boolean;
  readonly cloud_enabled: boolean;
  readonly encryption_enabled: boolean;
  readonly search_status: ServiceState;
  readonly cloud_status: ServiceState;
  readonly encryption_status: ServiceState;
}

export interface SystemCapabilities {
  readonly max_file_size: number;
  readonly supported_formats: readonly string[];
  readonly concurrent_jobs: number;
  readonly storage_quota: StorageInfo;
  readonly processing_power: ProcessingMetrics;
}

export interface StorageInfo {
  readonly used: number;
  readonly total: number;
  readonly available: number;
  readonly usage_percentage: number;
}

export interface ProcessingMetrics {
  readonly cpu_usage: number;
  readonly memory_usage: number;
  readonly active_jobs: number;
  readonly queue_length: number;
}

// ===== SEARCH & INTELLIGENCE =====

export interface SearchQuery {
  readonly query: string;
  readonly filters?: SearchFilters;
  readonly options?: SearchOptions;
}

export interface SearchFilters {
  readonly file_types?: readonly string[];
  readonly date_range?: DateRange;
  readonly size_range?: SizeRange;
  readonly classification?: readonly ClassificationLevel[];
  readonly tags?: readonly string[];
}

export interface SearchOptions {
  readonly limit?: number;
  readonly offset?: number;
  readonly sort_by?: SortField;
  readonly sort_order?: SortOrder;
  readonly highlight?: boolean;
  readonly fuzzy_search?: boolean;
}

export interface SearchResult {
  readonly results: readonly SearchHit[];
  readonly total_count: number;
  readonly query_time: number;
  readonly suggestions?: readonly string[];
  readonly facets?: SearchFacets;
}

export interface SearchHit {
  readonly file_id: string;
  readonly filename: string;
  readonly content_preview: string;
  readonly relevance_score: number;
  readonly highlights: readonly string[];
  readonly metadata: SearchMetadata;
}

export interface SearchMetadata {
  readonly file_type: string;
  readonly size: number;
  readonly created_at: string;
  readonly classification: ClassificationLevel;
  readonly tags: readonly string[];
}

export interface SearchFacets {
  readonly file_types: readonly FacetCount[];
  readonly classifications: readonly FacetCount[];
  readonly date_ranges: readonly FacetCount[];
}

export interface FacetCount {
  readonly value: string;
  readonly count: number;
}

// ===== CLOUD OPERATIONS =====

export interface CloudProvider {
  readonly name: string;
  readonly type: 'aws' | 'gcp' | 'azure' | 'custom';
  readonly status: ServiceState;
  readonly regions: readonly string[];
  readonly features: readonly CloudFeature[];
}

export interface CloudUploadResult {
  readonly remote_path: string;
  readonly public_url: string;
  readonly size: number;
  readonly uploaded_at: string;
  readonly provider: string;
  readonly region: string;
}

// ===== ENUMS =====

export type JobStatus = 
  | 'pending' 
  | 'processing' 
  | 'completed' 
  | 'failed' 
  | 'cancelled';

export type FileStatus = 
  | 'active' 
  | 'archived' 
  | 'corrupted' 
  | 'processing';

export type ServiceState = 
  | 'enabled' 
  | 'disabled' 
  | 'error' 
  | 'maintenance';

export type ClassificationLevel = 
  | 'unclassified' 
  | 'confidential' 
  | 'secret' 
  | 'top-secret';

export type SecurityLevel = 
  | 'minimal' 
  | 'standard' 
  | 'enhanced' 
  | 'maximum';

export type CompressionLevel = 
  | 'none' 
  | 'low' 
  | 'medium' 
  | 'high' 
  | 'maximum';

export type QRDensity = 
  | 'sparse' 
  | 'normal' 
  | 'dense' 
  | 'maximum';

export type SortField = 
  | 'name' 
  | 'size' 
  | 'created_at' 
  | 'relevance';

export type SortOrder = 'asc' | 'desc';

export type CloudFeature = 
  | 'encryption' 
  | 'versioning' 
  | 'backup' 
  | 'cdn' 
  | 'analytics';

// ===== UTILITY TYPES =====

export interface DateRange {
  readonly start: string;
  readonly end: string;
}

export interface SizeRange {
  readonly min: number;
  readonly max: number;
}

export interface PaginationInfo {
  readonly page: number;
  readonly per_page: number;
  readonly total_pages: number;
  readonly total_items: number;
}

export interface ApiResponse<T = unknown> {
  readonly data: T;
  readonly success: boolean;
  readonly message?: string;
  readonly timestamp: string;
  readonly request_id: string;
}

export interface ApiError {
  readonly error: string;
  readonly code: number;
  readonly details?: Record<string, unknown>;
  readonly timestamp: string;
  readonly request_id: string;
}

// ===== WEBSOCKET TYPES =====

export interface WebSocketMessage<T = unknown> {
  readonly type: string;
  readonly payload: T;
  readonly timestamp: string;
  readonly session_id: string;
}

export interface ProgressUpdate {
  readonly job_id: string;
  readonly progress: number;
  readonly status: JobStatus;
  readonly message: string;
  readonly eta?: number;
}

export interface SystemAlert {
  readonly level: 'info' | 'warning' | 'error' | 'critical';
  readonly title: string;
  readonly message: string;
  readonly timestamp: string;
  readonly source: string;
  readonly actions?: readonly AlertAction[];
}

export interface AlertAction {
  readonly label: string;
  readonly action: string;
  readonly variant: 'primary' | 'secondary' | 'danger';
}
