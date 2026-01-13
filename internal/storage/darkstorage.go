package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// DarkStorageClient provides S3-compatible access to darkstorage.io
type DarkStorageClient struct {
	s3Client *s3.Client
	bucket   string
	endpoint string
	region   string
}

// Config for DarkStorage
type DarkStorageConfig struct {
	Endpoint        string // darkstorage.io endpoint
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	Region          string // default: "us-east-1"
}

// NewDarkStorageClient creates a new S3-compatible client for darkstorage.io
func NewDarkStorageClient(cfg *DarkStorageConfig) (*DarkStorageClient, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	// Create custom endpoint resolver
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               cfg.Endpoint,
			SigningRegion:     cfg.Region,
			HostnameImmutable: true,
		}, nil
	})

	// Load AWS SDK config with custom endpoint
	awsConfig, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.UsePathStyle = true // Required for S3-compatible services
	})

	return &DarkStorageClient{
		s3Client: s3Client,
		bucket:   cfg.Bucket,
		endpoint: cfg.Endpoint,
		region:   cfg.Region,
	}, nil
}

// UploadFile uploads a file to darkstorage.io
func (c *DarkStorageClient) UploadFile(ctx context.Context, key string, data io.Reader, contentType string, metadata map[string]string) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        data,
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	}

	_, err := c.s3Client.PutObject(ctx, input)
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

// DownloadFile downloads a file from darkstorage.io
func (c *DarkStorageClient) DownloadFile(ctx context.Context, key string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	result, err := c.s3Client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	return result.Body, nil
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
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	_, err := c.s3Client.DeleteObject(ctx, input)
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
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	}

	var files []string
	paginator := s3.NewListObjectsV2Paginator(c.s3Client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list files: %w", err)
		}

		for _, obj := range page.Contents {
			files = append(files, *obj.Key)
		}
	}

	return files, nil
}

// GetFileSize gets the size of a file
func (c *DarkStorageClient) GetFileSize(ctx context.Context, key string) (int64, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	result, err := c.s3Client.HeadObject(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	return *result.ContentLength, nil
}

// GeneratePresignedURL generates a time-limited download URL
func (c *DarkStorageClient) GeneratePresignedURL(ctx context.Context, key string, expiresIn time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(c.s3Client)

	input := &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}

	result, err := presignClient.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return result.URL, nil
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
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	}

	var totalSize int64
	paginator := s3.NewListObjectsV2Paginator(c.s3Client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			totalSize += *obj.Size
		}
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
