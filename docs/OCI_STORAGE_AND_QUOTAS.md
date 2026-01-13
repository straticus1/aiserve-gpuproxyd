# OCI Object Storage & Quota Management

Complete guide to OCI Object Storage integration and storage quota system.

## Overview

The GPU Proxy platform uses Oracle Cloud Infrastructure (OCI) Object Storage for:
- Dataset storage for training jobs
- Model artifact storage
- Training log storage
- User file uploads

A comprehensive quota management system ensures fair resource usage and prevents storage exhaustion.

## OCI Object Storage Integration

### Configuration

Set these environment variables in `.env.local` (NEVER commit to git):

```bash
# OCI Object Storage Configuration
OCI_STORAGE_ENDPOINT=https://objectstorage.us-phoenix-1.oraclecloud.com
OCI_STORAGE_NAMESPACE=your-oci-namespace
OCI_STORAGE_BUCKET=aiserve-storage
OCI_STORAGE_REGION=us-phoenix-1
OCI_ACCESS_KEY_ID=ocid1.credential.oc1...
OCI_SECRET_ACCESS_KEY=your-secret-key
```

### Architecture

```
┌─────────────────┐
│   Application   │
│                 │
│  DarkStorage    │
│     Client      │
└────────┬────────┘
         │
         │ OCI SDK
         ▼
┌─────────────────────────────────────┐
│   OCI Object Storage Service        │
│                                     │
│  ┌──────────────────────────────┐  │
│  │  Namespace: your-namespace   │  │
│  │                              │  │
│  │  Bucket: aiserve-storage     │  │
│  │  ├─ datasets/                │  │
│  │  │  └─ {user_id}/            │  │
│  │  │     └─ {dataset_id}/      │  │
│  │  │        └─ files...        │  │
│  │  ├─ models/                  │  │
│  │  │  └─ {user_id}/            │  │
│  │  │     └─ {model_id}/        │  │
│  │  │        └─ files...        │  │
│  │  └─ logs/                    │  │
│  │     └─ training/             │  │
│  │        └─ {job_id}/          │  │
│  │           └─ output.log      │  │
│  └──────────────────────────────┘  │
└─────────────────────────────────────┘
```

### Path Structure

All files are organized with user-scoped paths:

```
datasets/{user_id}/{dataset_id}/{filename}
models/{user_id}/{model_id}/{filename}
logs/training/{job_id}/output.log
```

### API Operations

#### Upload Dataset
```go
client := storage.NewDarkStorageClient(config)
uri, size, err := client.UploadDataset(
    ctx,
    userID,
    datasetID,
    "data.csv",
    fileReader,
    "text/csv",
)
// Returns: darkstorage://aiserve-storage/datasets/{user_id}/{dataset_id}/data.csv
```

#### Upload Model
```go
uri, size, err := client.UploadModel(
    ctx,
    userID,
    modelID,
    "model.onnx",
    fileReader,
    "application/octet-stream",
)
// Returns: darkstorage://aiserve-storage/models/{user_id}/{model_id}/model.onnx
```

#### Download File
```go
reader, err := client.DownloadFileFromURI(ctx, uri)
defer reader.Close()
```

#### Generate Presigned URL
```go
url, err := client.GeneratePresignedURL(
    ctx,
    "models/user-123/model-456/model.onnx",
    24*time.Hour,
)
// Returns: OCI Pre-Authenticated Request (PAR) URL valid for 24 hours
```

#### Delete Files
```go
// Delete entire dataset
err := client.DeleteDataset(ctx, userID, datasetID)

// Delete entire model
err := client.DeleteModel(ctx, userID, modelID)

// Delete specific file
err := client.DeleteFile(ctx, "path/to/file")
```

## Storage Quota System

### Quota Tiers

#### Default Tier
```go
QuotaLimits{
    MaxStorageBytes:   100 * 1024 * 1024 * 1024, // 100GB
    MaxFileSize:       10 * 1024 * 1024 * 1024,  // 10GB per file
    MaxUploadsPerHour: 50,                       // 50 uploads/hour
    MaxUploadsPerDay:  500,                      // 500 uploads/day
}
```

#### Premium Tier
```go
QuotaLimits{
    MaxStorageBytes:   1024 * 1024 * 1024 * 1024, // 1TB
    MaxFileSize:       100 * 1024 * 1024 * 1024,  // 100GB per file
    MaxUploadsPerHour: 500,                       // 500 uploads/hour
    MaxUploadsPerDay:  5000,                      // 5000 uploads/day
}
```

### Quota Enforcement

The quota system enforces limits at upload time (fail-fast):

1. **File Size Check**: Reject files exceeding `MaxFileSize`
2. **Storage Quota Check**: Reject if upload would exceed `MaxStorageBytes`
3. **Hourly Rate Limit**: Reject if hourly upload limit reached
4. **Daily Rate Limit**: Reject if daily upload limit reached

### API Integration

#### Check Before Upload
```go
quotaManager := storage.GetQuotaManager()

// Check if upload is allowed
err := quotaManager.CheckUploadAllowed(ctx, userID, fileSize)
if err != nil {
    // Upload denied: quota exceeded or rate limited
    return err
}

// Proceed with upload
// ...

// Record successful upload
quotaManager.RecordUpload(userID, fileSize)
```

#### Check Current Usage
```bash
curl -X GET http://localhost:8080/api/v1/quota \
  -H "Authorization: Bearer $JWT_TOKEN"
```

Response:
```json
{
  "user_id": "123e4567-e89b-12d3-a456-426614174000",
  "storage": {
    "used_bytes": 52428800000,
    "limit_bytes": 107374182400,
    "used_pct": 48.8
  },
  "file_size": {
    "max_bytes": 10737418240
  },
  "rate_limits": {
    "uploads_last_hour": 12,
    "hourly_limit": 50,
    "uploads_last_day": 87,
    "daily_limit": 500
  }
}
```

#### Set Custom Limits (Admin)
```go
quotaManager.SetUserLimits(userID, storage.PremiumQuotaLimits())
```

### Upload Flow

```
┌──────────────┐
│ User Upload  │
└──────┬───────┘
       │
       ▼
┌─────────────────────────────────┐
│ Check Quota & Rate Limits       │
│ - File size <= MaxFileSize?     │
│ - Storage + upload <= MaxStorage│
│ - Uploads this hour < limit?    │
│ - Uploads today < limit?        │
└──────┬──────────────────────────┘
       │
       ├─── FAIL ──► HTTP 429 (Too Many Requests)
       │
       ▼ PASS
┌─────────────────────────────────┐
│ Upload to OCI Object Storage    │
└──────┬──────────────────────────┘
       │
       ▼
┌─────────────────────────────────┐
│ Record Upload                   │
│ - Increment storage used        │
│ - Add timestamp to hourly list  │
│ - Add timestamp to daily list   │
└─────────────────────────────────┘
```

### Quota Cleanup

The quota manager automatically cleans up old rate limit timestamps every 15 minutes:

```go
func (qm *QuotaManager) cleanupLoop() {
    ticker := time.NewTicker(15 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        // Remove timestamps older than 1 hour
        // Remove timestamps older than 24 hours
        // ...
    }
}
```

## Error Handling

### Upload Denied Errors

```json
{
  "error": "Upload denied: file size 15000000000 bytes exceeds maximum allowed 10737418240 bytes",
  "quota": "Check /api/v1/quota for current limits"
}
```

```json
{
  "error": "Upload denied: storage quota exceeded: current 95000000000 + upload 15000000000 > limit 107374182400 bytes",
  "quota": "Check /api/v1/quota for current limits"
}
```

```json
{
  "error": "Upload denied: hourly upload limit reached: 50/50 uploads",
  "quota": "Check /api/v1/quota for current limits"
}
```

## Monitoring & Analytics

### Track Storage Usage
```go
totalStorage, err := client.GetStorageUsage(ctx, userID)
// Returns total bytes across datasets and models
```

### Quota Information
```go
quotaInfo := quotaManager.GetQuotaInfo(userID)
// Returns complete quota status (see JSON example above)
```

## Security Considerations

### Credential Management
- **NEVER** commit OCI credentials to git
- Store credentials in `.env.local` (excluded in `.gitignore`)
- Use OCI Vault or secrets manager in production
- Rotate access keys regularly

### Access Control
- All paths are user-scoped (`{user_id}/...`)
- Application verifies ownership before operations
- Pre-Authenticated Requests (PAR) have time limits
- Use principle of least privilege for OCI IAM policies

### Data Isolation
- Each user's files are isolated by path prefix
- No cross-user access possible
- Deletion only allowed by owner
- Storage quotas prevent resource exhaustion

## Production Deployment

### OCI IAM Policy

Create a policy for the application user:

```
Allow group gpu-proxy-app to manage objects in compartment aiserve where target.bucket.name='aiserve-storage'
Allow group gpu-proxy-app to manage preauthenticated-requests in compartment aiserve where target.bucket.name='aiserve-storage'
```

### Bucket Configuration

1. Create bucket with **private** access (no public access)
2. Enable versioning for data protection
3. Set lifecycle policy to delete old PAR URLs
4. Enable logging for audit trail

### Monitoring

- Set CloudWatch/OCI Monitoring alerts for storage usage
- Monitor API request rates
- Track quota violation patterns
- Alert on unusual upload patterns

## Migration from AWS S3

The OCI client maintains an S3-compatible interface:

```go
// Old AWS S3 code
uri, err := client.UploadFile(ctx, key, data, contentType, metadata)

// New OCI code (same interface!)
uri, err := client.UploadFile(ctx, key, data, contentType, metadata)
```

URI format remains the same: `darkstorage://bucket/path`

Only configuration changes:
- AWS credentials → OCI credentials
- S3 endpoint → OCI endpoint
- No code changes required

## Troubleshooting

### Connection Issues
```bash
# Test OCI connectivity
curl https://<namespace>.objectstorage.<region>.oraclecloud.com
```

### Authentication Errors
```
Error: failed to create OCI client: invalid credentials
Solution: Verify OCI_ACCESS_KEY_ID and OCI_SECRET_ACCESS_KEY
```

### Quota Errors
```
Error: storage quota exceeded
Solution: Delete old files or upgrade to premium tier
```

## References

- OCI Object Storage Documentation: https://docs.oracle.com/en-us/iaas/Content/Object/home.htm
- OCI Go SDK: https://github.com/oracle/oci-go-sdk
- Pre-Authenticated Requests: https://docs.oracle.com/en-us/iaas/Content/Object/Tasks/usingpreauthenticatedrequests.htm

---

Last Updated: 2026-01-13
