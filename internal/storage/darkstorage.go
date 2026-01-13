package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// DarkStorageClient provides OCI Object Storage access for darkstorage.io or OCI
type DarkStorageClient struct {
	client    objectstorage.ObjectStorageClient
	namespace string
	bucket    string
	endpoint  string
}

// Config for DarkStorage
type DarkStorageConfig struct {
	Endpoint        string // OCI endpoint or darkstorage.io endpoint
	Namespace       string // OCI Object Storage namespace
	AccessKeyID     string // OCI access key ID (customer secret key OCID)
	SecretAccessKey string // OCI secret access key
	Bucket          string
	Region          string // OCI region (e.g., "us-phoenix-1")
}

// NewDarkStorageClient creates a new OCI Object Storage client
func NewDarkStorageClient(cfg *DarkStorageConfig) (*DarkStorageClient, error) {
	if cfg.Region == "" {
		cfg.Region = "us-phoenix-1"
	}

	// Create OCI configuration provider using customer secret keys (S3-compatible auth)
	configProvider := common.NewRawConfigurationProvider(
		"", // tenancy OCID (not needed for customer secret keys)
		"", // user OCID (not needed for customer secret keys)
		cfg.Region,
		"", // fingerprint (not needed for customer secret keys)
		"", // private key (not needed for customer secret keys)
		nil,
	)

	// Create Object Storage client
	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI client: %w", err)
	}

	// Set custom endpoint if provided (for darkstorage.io S3-compatible service)
	if cfg.Endpoint != "" {
		client.Host = cfg.Endpoint
	}

	// For S3-compatible authentication with customer secret keys
	// We'll use pre-authenticated requests or direct API calls
	// Note: OCI Object Storage supports both native OCI auth and S3-compatible auth

	return &DarkStorageClient{
		client:    client,
		namespace: cfg.Namespace,
		bucket:    cfg.Bucket,
		endpoint:  cfg.Endpoint,
	}, nil
}

// UploadFile uploads a file to OCI Object Storage
func (c *DarkStorageClient) UploadFile(ctx context.Context, key string, data io.Reader, contentType string, metadata map[string]string) (string, error) {
	request := objectstorage.PutObjectRequest{
		NamespaceName: common.String(c.namespace),
		BucketName:    common.String(c.bucket),
		ObjectName:    common.String(key),
		PutObjectBody: io.NopCloser(data),
		ContentType:   common.String(contentType),
		OpcMeta:       metadata,
	}

	_, err := c.client.PutObject(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Return the darkstorage:// URI
	uri := fmt.Sprintf("darkstorage://%s/%s", c.bucket, key)
	return uri, nil
}

// UploadDataset uploads a dataset with user-scoped path
func (c *DarkStorageClient) UploadDataset(ctx context.Context, userID uuid.UUID, datasetID uuid.UUID, filename string, data io.Reader, contentType string) (string, int64, error) {
	// User-scoped path: datasets/{user_id}/{dataset_id}/{filename}
	key := fmt.Sprintf("datasets/%s/%s/%s", userID.String(), datasetID.String(), filename)

	// Track size by reading data
	size := int64(0)
	sizeTracker := &sizeTrackingReader{reader: data, size: &size}

	uri, err := c.UploadFile(ctx, key, sizeTracker, contentType, map[string]string{
		"user_id":    userID.String(),
		"dataset_id": datasetID.String(),
		"filename":   filename,
	})
	if err != nil {
		return "", 0, err
	}

	return uri, size, nil
}

// UploadModel uploads a trained model
func (c *DarkStorageClient) UploadModel(ctx context.Context, userID uuid.UUID, modelID uuid.UUID, filename string, data io.Reader, contentType string) (string, int64, error) {
	// Model path: models/{user_id}/{model_id}/{filename}
	key := fmt.Sprintf("models/%s/%s/%s", userID.String(), modelID.String(), filename)

	size := int64(0)
	sizeTracker := &sizeTrackingReader{reader: data, size: &size}

	uri, err := c.UploadFile(ctx, key, sizeTracker, contentType, map[string]string{
		"user_id":  userID.String(),
		"model_id": modelID.String(),
		"filename": filename,
	})
	if err != nil {
		return "", 0, err
	}

	return uri, size, nil
}

// UploadTrainingLogs uploads training logs
func (c *DarkStorageClient) UploadTrainingLogs(ctx context.Context, jobID uuid.UUID, logData io.Reader) (string, error) {
	// Logs path: logs/training/{job_id}/output.log
	key := fmt.Sprintf("logs/training/%s/output.log", jobID.String())

	return c.UploadFile(ctx, key, logData, "text/plain", map[string]string{
		"job_id": jobID.String(),
		"type":   "training_log",
	})
}

// DownloadFile downloads a file from OCI Object Storage
func (c *DarkStorageClient) DownloadFile(ctx context.Context, key string) (io.ReadCloser, error) {
	request := objectstorage.GetObjectRequest{
		NamespaceName: common.String(c.namespace),
		BucketName:    common.String(c.bucket),
		ObjectName:    common.String(key),
	}

	response, err := c.client.GetObject(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	return response.Content, nil
}

// DownloadFileFromURI downloads using darkstorage:// URI
func (c *DarkStorageClient) DownloadFileFromURI(ctx context.Context, uri string) (io.ReadCloser, error) {
	key, err := c.parseURI(uri)
	if err != nil {
		return nil, err
	}

	return c.DownloadFile(ctx, key)
}

// DeleteFile deletes a file
func (c *DarkStorageClient) DeleteFile(ctx context.Context, key string) error {
	request := objectstorage.DeleteObjectRequest{
		NamespaceName: common.String(c.namespace),
		BucketName:    common.String(c.bucket),
		ObjectName:    common.String(key),
	}

	_, err := c.client.DeleteObject(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// DeleteDataset deletes all files in a dataset
func (c *DarkStorageClient) DeleteDataset(ctx context.Context, userID uuid.UUID, datasetID uuid.UUID) error {
	prefix := fmt.Sprintf("datasets/%s/%s/", userID.String(), datasetID.String())
	return c.deletePrefix(ctx, prefix)
}

// DeleteModel deletes all files for a model
func (c *DarkStorageClient) DeleteModel(ctx context.Context, userID uuid.UUID, modelID uuid.UUID) error {
	prefix := fmt.Sprintf("models/%s/%s/", userID.String(), modelID.String())
	return c.deletePrefix(ctx, prefix)
}

// ListFiles lists files with a prefix
func (c *DarkStorageClient) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	var files []string
	var nextStartWith *string

	for {
		request := objectstorage.ListObjectsRequest{
			NamespaceName: common.String(c.namespace),
			BucketName:    common.String(c.bucket),
			Prefix:        common.String(prefix),
			Start:         nextStartWith,
		}

		response, err := c.client.ListObjects(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list files: %w", err)
		}

		for _, obj := range response.Objects {
			if obj.Name != nil {
				files = append(files, *obj.Name)
			}
		}

		// Check if there are more results
		if response.NextStartWith == nil || *response.NextStartWith == "" {
			break
		}
		nextStartWith = response.NextStartWith
	}

	return files, nil
}

// GetFileSize gets the size of a file
func (c *DarkStorageClient) GetFileSize(ctx context.Context, key string) (int64, error) {
	request := objectstorage.HeadObjectRequest{
		NamespaceName: common.String(c.namespace),
		BucketName:    common.String(c.bucket),
		ObjectName:    common.String(key),
	}

	response, err := c.client.HeadObject(ctx, request)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	if response.ContentLength == nil {
		return 0, fmt.Errorf("content length not available")
	}

	return *response.ContentLength, nil
}

// GeneratePresignedURL generates a time-limited download URL using Pre-Authenticated Request (PAR)
func (c *DarkStorageClient) GeneratePresignedURL(ctx context.Context, key string, expiresIn time.Duration) (string, error) {
	// Calculate expiration time
	expirationTime := common.SDKTime{Time: time.Now().Add(expiresIn)}

	// Create Pre-Authenticated Request (PAR)
	request := objectstorage.CreatePreauthenticatedRequestRequest{
		NamespaceName: common.String(c.namespace),
		BucketName:    common.String(c.bucket),
		CreatePreauthenticatedRequestDetails: objectstorage.CreatePreauthenticatedRequestDetails{
			Name:       common.String(fmt.Sprintf("par-%s-%d", key, time.Now().Unix())),
			ObjectName: common.String(key),
			AccessType: objectstorage.CreatePreauthenticatedRequestDetailsAccessTypeObjectread,
			TimeExpires: &expirationTime,
		},
	}

	response, err := c.client.CreatePreauthenticatedRequest(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	// Construct full URL
	// Format: https://{namespace}.objectstorage.{region}.oci.customer-oci.com{access_uri}
	// or use custom endpoint if provided
	if response.AccessUri == nil {
		return "", fmt.Errorf("access URI not returned")
	}

	var baseURL string
	if c.endpoint != "" {
		baseURL = c.endpoint
	} else {
		// Use OCI Object Storage URL format
		baseURL = fmt.Sprintf("https://%s.objectstorage.oci.customer-oci.com", c.namespace)
	}

	fullURL := fmt.Sprintf("%s%s", baseURL, *response.AccessUri)
	return fullURL, nil
}

// GetStorageUsage calculates total storage used by a user
func (c *DarkStorageClient) GetStorageUsage(ctx context.Context, userID uuid.UUID) (int64, error) {
	prefix := fmt.Sprintf("datasets/%s/", userID.String())
	datasetSize, err := c.calculatePrefixSize(ctx, prefix)
	if err != nil {
		return 0, err
	}

	prefix = fmt.Sprintf("models/%s/", userID.String())
	modelSize, err := c.calculatePrefixSize(ctx, prefix)
	if err != nil {
		return 0, err
	}

	return datasetSize + modelSize, nil
}

// Helper: parseURI extracts key from darkstorage:// URI
func (c *DarkStorageClient) parseURI(uri string) (string, error) {
	// Expected format: darkstorage://bucket/path/to/file
	if !strings.HasPrefix(uri, "darkstorage://") {
		return "", fmt.Errorf("invalid darkstorage URI: %s", uri)
	}

	// Remove darkstorage:// prefix
	path := strings.TrimPrefix(uri, "darkstorage://")

	// Remove bucket name
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid darkstorage URI format: %s", uri)
	}

	return parts[1], nil
}

// Helper: deletePrefix deletes all objects with a prefix
func (c *DarkStorageClient) deletePrefix(ctx context.Context, prefix string) error {
	files, err := c.ListFiles(ctx, prefix)
	if err != nil {
		return err
	}

	for _, key := range files {
		if err := c.DeleteFile(ctx, key); err != nil {
			return fmt.Errorf("failed to delete %s: %w", key, err)
		}
	}

	return nil
}

// Helper: calculatePrefixSize calculates total size of objects with prefix
func (c *DarkStorageClient) calculatePrefixSize(ctx context.Context, prefix string) (int64, error) {
	var totalSize int64
	var nextStartWith *string

	for {
		request := objectstorage.ListObjectsRequest{
			NamespaceName: common.String(c.namespace),
			BucketName:    common.String(c.bucket),
			Prefix:        common.String(prefix),
			Start:         nextStartWith,
		}

		response, err := c.client.ListObjects(ctx, request)
		if err != nil {
			return 0, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range response.Objects {
			if obj.Size != nil {
				totalSize += *obj.Size
			}
		}

		// Check if there are more results
		if response.NextStartWith == nil || *response.NextStartWith == "" {
			break
		}
		nextStartWith = response.NextStartWith
	}

	return totalSize, nil
}

// sizeTrackingReader tracks bytes read
type sizeTrackingReader struct {
	reader io.Reader
	size   *int64
}

func (r *sizeTrackingReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	*r.size += int64(n)
	return n, err
}
