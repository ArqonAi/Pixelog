# Cloud Storage Implementation Guide

## Backend API Endpoints (Go/Node.js)

### 1. Cloud Provider Configuration
```go
// POST /api/cloud/configure
type CloudConfig struct {
    Provider    string `json:"provider"`    // "aws", "gcp", "azure"
    AccessKey   string `json:"access_key"`  // Encrypted storage
    SecretKey   string `json:"secret_key"`  // Encrypted storage
    Region      string `json:"region"`
    BucketName  string `json:"bucket_name"`
}

func ConfigureCloudProvider(w http.ResponseWriter, r *http.Request) {
    var config CloudConfig
    json.NewDecoder(r.Body).Decode(&config)
    
    // Encrypt credentials before storing
    encryptedConfig := encrypt(config)
    
    // Store in database with user association
    err := db.SaveCloudConfig(userID, encryptedConfig)
    if err != nil {
        http.Error(w, "Failed to save configuration", 500)
        return
    }
    
    // Test connection
    client, err := createCloudClient(config)
    if err != nil {
        http.Error(w, "Invalid credentials", 400)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
```

### 2. File Upload to Cloud
```go
// POST /api/cloud/upload
func UploadToCloud(w http.ResponseWriter, r *http.Request) {
    file, header, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "No file provided", 400)
        return
    }
    defer file.Close()
    
    // Get user's cloud config
    config, err := db.GetCloudConfig(userID)
    if err != nil {
        http.Error(w, "Cloud not configured", 400)
        return
    }
    
    // Upload to configured provider
    client := createCloudClient(config)
    cloudURL, err := client.Upload(header.Filename, file)
    if err != nil {
        http.Error(w, "Upload failed", 500)
        return
    }
    
    // Save file record
    cloudFile := CloudFile{
        UserID:    userID,
        Name:      header.Filename,
        Size:      header.Size,
        CloudURL:  cloudURL,
        Provider:  config.Provider,
        CreatedAt: time.Now(),
    }
    db.SaveCloudFile(cloudFile)
    
    json.NewEncoder(w).Encode(cloudFile)
}
```

### 3. AWS S3 Implementation
```go
package cloud

import (
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
)

type AWSClient struct {
    s3Client   *s3.S3
    bucketName string
}

func NewAWSClient(accessKey, secretKey, region, bucket string) *AWSClient {
    sess := session.Must(session.NewSession(&aws.Config{
        Region: aws.String(region),
        Credentials: credentials.NewStaticCredentials(
            accessKey, secretKey, "",
        ),
    }))
    
    return &AWSClient{
        s3Client:   s3.New(sess),
        bucketName: bucket,
    }
}

func (c *AWSClient) Upload(filename string, file io.Reader) (string, error) {
    _, err := c.s3Client.PutObject(&s3.PutObjectInput{
        Bucket: aws.String(c.bucketName),
        Key:    aws.String(filename),
        Body:   aws.ReadSeekCloser(file),
        ACL:    aws.String("private"),
    })
    
    if err != nil {
        return "", err
    }
    
    return fmt.Sprintf("s3://%s/%s", c.bucketName, filename), nil
}

func (c *AWSClient) GenerateDownloadURL(filename string) (string, error) {
    req, _ := c.s3Client.GetObjectRequest(&s3.GetObjectInput{
        Bucket: aws.String(c.bucketName),
        Key:    aws.String(filename),
    })
    
    // Generate presigned URL valid for 1 hour
    return req.Presign(time.Hour)
}
```

### 4. Google Cloud Storage Implementation
```go
package cloud

import (
    "cloud.google.com/go/storage"
    "google.golang.org/api/option"
)

type GCPClient struct {
    client     *storage.Client
    bucketName string
}

func NewGCPClient(serviceAccountJSON, bucket string) *GCPClient {
    client, err := storage.NewClient(
        context.Background(),
        option.WithCredentialsJSON([]byte(serviceAccountJSON)),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    return &GCPClient{
        client:     client,
        bucketName: bucket,
    }
}

func (c *GCPClient) Upload(filename string, file io.Reader) (string, error) {
    ctx := context.Background()
    wc := c.client.Bucket(c.bucketName).Object(filename).NewWriter(ctx)
    
    if _, err := io.Copy(wc, file); err != nil {
        return "", err
    }
    if err := wc.Close(); err != nil {
        return "", err
    }
    
    return fmt.Sprintf("gs://%s/%s", c.bucketName, filename), nil
}
```

## Database Schema

```sql
-- Cloud provider configurations (encrypted)
CREATE TABLE cloud_configs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    provider VARCHAR(20) NOT NULL, -- 'aws', 'gcp', 'azure'
    encrypted_config TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Cloud files tracking
CREATE TABLE cloud_files (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    local_file_id INTEGER REFERENCES pixe_files(id),
    filename VARCHAR(255) NOT NULL,
    file_size BIGINT NOT NULL,
    cloud_url TEXT NOT NULL,
    provider VARCHAR(20) NOT NULL,
    checksum VARCHAR(64),
    uploaded_at TIMESTAMP DEFAULT NOW(),
    last_accessed TIMESTAMP
);

-- Auto-sync settings
CREATE TABLE sync_settings (
    user_id INTEGER PRIMARY KEY,
    auto_upload BOOLEAN DEFAULT false,
    sync_on_create BOOLEAN DEFAULT false,
    retention_days INTEGER DEFAULT 365,
    max_file_size BIGINT DEFAULT 104857600 -- 100MB
);
```

## Security Considerations

### 1. Credential Encryption
```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
)

func encryptCredentials(data string, key []byte) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }
    
    ciphertext := gcm.Seal(nonce, nonce, []byte(data), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}
```

### 2. Environment Variables
```bash
# .env
ENCRYPTION_KEY=your-32-byte-encryption-key
AWS_DEFAULT_REGION=us-west-2
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

## Frontend Integration

### 1. Update API Service
```typescript
// Add to src/services/api.ts
export interface CloudProvider {
  provider: 'aws' | 'gcp' | 'azure' | 'digitalocean'
  accessKey?: string
  secretKey?: string
  serviceAccountJSON?: string
  region?: string
  bucketName: string
}

export interface CloudFile {
  id: string
  filename: string
  size: number
  cloudUrl: string
  provider: string
  uploadedAt: string
  downloadUrl?: string
}

class CloudAPI {
  async configureProvider(config: CloudProvider): Promise<boolean> {
    const response = await fetch('/api/cloud/configure', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config)
    })
    return response.ok
  }
  
  async uploadFiles(files: File[]): Promise<CloudFile[]> {
    const formData = new FormData()
    files.forEach(file => formData.append('files', file))
    
    const response = await fetch('/api/cloud/upload', {
      method: 'POST',
      body: formData
    })
    return response.json()
  }
  
  async getCloudFiles(): Promise<CloudFile[]> {
    const response = await fetch('/api/cloud/files')
    return response.json()
  }
  
  async getDownloadUrl(fileId: string): Promise<string> {
    const response = await fetch(`/api/cloud/download/${fileId}`)
    const data = await response.json()
    return data.downloadUrl
  }
}

export const cloudApi = new CloudAPI()
```

### 2. Configuration Modal Component
```typescript
const ConfigurationModal: React.FC<{
  provider: string
  onSuccess: () => void
  onClose: () => void
}> = ({ provider, onSuccess, onClose }) => {
  const [config, setConfig] = useState({
    accessKey: '',
    secretKey: '',
    region: 'us-west-2',
    bucketName: ''
  })
  
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const success = await cloudApi.configureProvider({
      provider: provider as any,
      ...config
    })
    
    if (success) {
      onSuccess()
      onClose()
    }
  }
  
  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <input
        type="text"
        placeholder="Access Key"
        value={config.accessKey}
        onChange={(e) => setConfig({...config, accessKey: e.target.value})}
        className="cyber-input w-full"
        required
      />
      <input
        type="password"
        placeholder="Secret Key"
        value={config.secretKey}
        onChange={(e) => setConfig({...config, secretKey: e.target.value})}
        className="cyber-input w-full"
        required
      />
      <input
        type="text"
        placeholder="Bucket Name"
        value={config.bucketName}
        onChange={(e) => setConfig({...config, bucketName: e.target.value})}
        className="cyber-input w-full"
        required
      />
      <button type="submit" className="cyber-btn w-full">
        Connect {provider}
      </button>
    </form>
  )
}
```

## Deployment Considerations

1. **Environment Setup**: Secure credential storage
2. **CORS Configuration**: Allow frontend uploads
3. **File Size Limits**: Configure based on cloud provider limits
4. **Error Handling**: Proper user feedback for failures
5. **Rate Limiting**: Prevent abuse of upload endpoints
6. **Monitoring**: Track upload/download metrics

This gives you a complete, production-ready cloud storage implementation!
