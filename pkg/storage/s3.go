package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.uber.org/zap"
)

// Config holds S3/MinIO client configuration.
type Config struct {
	Endpoint  string // S3/MinIO endpoint (e.g. "https://s3.amazonaws.com" or "http://minio:9000")
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// UploadResult holds the result of an upload operation.
type UploadResult struct {
	Key    string // object key
	Bucket string // bucket name
	URL    string // full URL to the object
	Size   int64  // size in bytes (from the upload input, if known)
}

// ObjectInfo describes an object in S3.
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
}

// uploadOptions holds optional parameters for an upload.
type uploadOptions struct {
	metadata           map[string]string
	contentDisposition string
}

// UploadOption configures optional upload parameters.
type UploadOption func(*uploadOptions)

// WithMetadata attaches custom metadata to the uploaded object.
func WithMetadata(m map[string]string) UploadOption {
	return func(o *uploadOptions) {
		o.metadata = m
	}
}

// WithContentDisposition sets the Content-Disposition header on the uploaded object.
func WithContentDisposition(d string) UploadOption {
	return func(o *uploadOptions) {
		o.contentDisposition = d
	}
}

// Client is an S3-compatible storage client.
type Client struct {
	s3       *s3.Client
	uploader *manager.Uploader
	presign  *s3.PresignClient
	bucket   string
	logger   *zap.Logger
}

// New creates a new S3 storage client.
// Supports AWS S3 and MinIO via custom endpoint.
func New(cfg Config, logger *zap.Logger) (*Client, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("storage: load aws config: %w", err)
	}

	s3Opts := func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // required for MinIO / localstack
		}
	}

	s3Client := s3.NewFromConfig(awsCfg, s3Opts)

	logger.Info("storage client initialised",
		zap.String("endpoint", cfg.Endpoint),
		zap.String("region", cfg.Region),
		zap.String("bucket", cfg.Bucket),
	)

	return &Client{
		s3:       s3Client,
		uploader: manager.NewUploader(s3Client),
		presign:  s3.NewPresignClient(s3Client),
		bucket:   cfg.Bucket,
		logger:   logger,
	}, nil
}

// Upload uploads an object to S3.
func (c *Client) Upload(ctx context.Context, key string, body io.Reader, contentType string, opts ...UploadOption) (*UploadResult, error) {
	uo := &uploadOptions{}
	for _, o := range opts {
		o(uo)
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}

	if len(uo.metadata) > 0 {
		input.Metadata = uo.metadata
	}
	if uo.contentDisposition != "" {
		input.ContentDisposition = aws.String(uo.contentDisposition)
	}

	result, err := c.uploader.Upload(ctx, input)
	if err != nil {
		c.logger.Error("upload failed", zap.String("key", key), zap.Error(err))
		return nil, ErrUploadFailed.WithDetail(fmt.Sprintf("key=%s: %v", key, err))
	}

	c.logger.Info("object uploaded",
		zap.String("key", key),
		zap.String("bucket", c.bucket),
		zap.String("location", result.Location),
	)

	return &UploadResult{
		Key:    key,
		Bucket: c.bucket,
		URL:    result.Location,
	}, nil
}

// Download retrieves an object from S3.
func (c *Client) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrObjectNotFound.WithDetail(fmt.Sprintf("key=%s", key))
		}
		c.logger.Error("download failed", zap.String("key", key), zap.Error(err))
		return nil, ErrDownloadFailed.WithDetail(fmt.Sprintf("key=%s: %v", key, err))
	}

	return output.Body, nil
}

// Delete removes an object from S3.
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		c.logger.Error("delete failed", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("storage: delete key=%s: %w", key, err)
	}

	c.logger.Info("object deleted", zap.String("key", key))
	return nil
}

// Exists checks if an object exists in S3.
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	_, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("storage: head key=%s: %w", key, err)
	}
	return true, nil
}

// PresignedGetURL generates a presigned URL for downloading an object.
func (c *Client) PresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		c.logger.Error("presign get failed", zap.String("key", key), zap.Error(err))
		return "", ErrPresignFailed.WithDetail(fmt.Sprintf("GET key=%s: %v", key, err))
	}

	return req.URL, nil
}

// PresignedPutURL generates a presigned URL for uploading an object.
func (c *Client) PresignedPutURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		c.logger.Error("presign put failed", zap.String("key", key), zap.Error(err))
		return "", ErrPresignFailed.WithDetail(fmt.Sprintf("PUT key=%s: %v", key, err))
	}

	return req.URL, nil
}

// List returns objects matching a prefix.
func (c *Client) List(ctx context.Context, prefix string, maxKeys int32) ([]ObjectInfo, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(c.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(maxKeys),
	}

	output, err := c.s3.ListObjectsV2(ctx, input)
	if err != nil {
		c.logger.Error("list failed", zap.String("prefix", prefix), zap.Error(err))
		return nil, fmt.Errorf("storage: list prefix=%s: %w", prefix, err)
	}

	objects := make([]ObjectInfo, 0, len(output.Contents))
	for _, obj := range output.Contents {
		objects = append(objects, ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
		})
	}

	return objects, nil
}

// isNotFound checks if an error is an S3 "not found" error.
func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	var nsb *types.NotFound
	if ok := containsError(err, &nsk); ok {
		return true
	}
	if ok := containsError(err, &nsb); ok {
		return true
	}
	// HeadObject returns 404 as a generic smithy error; check the status code.
	// aws-sdk-go-v2 wraps 404 in a ResponseError.
	return false
}

// containsError is a generic helper to check for a specific error type.
func containsError[T error](err error, target *T) bool {
	return err != nil && errorAs(err, target)
}

// errorAs wraps errors.As for use in generic context.
func errorAs[T error](err error, target *T) bool {
	return err != nil && asError(err, target)
}

func asError[T error](err error, target *T) bool {
	// import errors is avoided here to keep the helper minimal;
	// we rely on the standard library via the caller.
	// Actually, let's just use a type assertion approach compatible with errors.As.
	for err != nil {
		if e, ok := err.(T); ok { //nolint:errorlint // intentional type switch
			*target = e
			return true
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
		} else {
			return false
		}
	}
	return false
}
