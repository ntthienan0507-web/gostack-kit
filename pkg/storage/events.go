package storage

import (
	"strings"
	"time"
)

// S3Event is the top-level event notification from S3/MinIO.
// S3 sends this to SNS/SQS/Kafka when objects are created, deleted, etc.
type S3Event struct {
	Records []S3EventRecord `json:"Records"`
}

// S3EventRecord represents a single event record within an S3 event notification.
type S3EventRecord struct {
	EventName    string       `json:"eventName"`   // "s3:ObjectCreated:Put", "s3:ObjectRemoved:Delete"
	EventTime    time.Time    `json:"eventTime"`
	EventSource  string       `json:"eventSource"` // "aws:s3" or "minio:s3"
	S3           S3Entity     `json:"s3"`
	UserIdentity UserIdentity `json:"userIdentity"`
}

// UserIdentity identifies the user that triggered the event.
type UserIdentity struct {
	PrincipalID string `json:"principalId"`
}

// S3Entity contains the S3-specific details of an event record.
type S3Entity struct {
	Bucket S3Bucket `json:"bucket"`
	Object S3Object `json:"object"`
}

// S3Bucket identifies the bucket involved in the event.
type S3Bucket struct {
	Name string `json:"name"`
}

// S3Object describes the object involved in the event.
type S3Object struct {
	Key         string `json:"key"`
	Size        int64  `json:"size"`
	ETag        string `json:"eTag"`
	ContentType string `json:"contentType"`
}

// IsCreated returns true if the event represents an object creation.
func (r S3EventRecord) IsCreated() bool {
	return strings.Contains(r.EventName, "ObjectCreated")
}

// IsDeleted returns true if the event represents an object deletion.
func (r S3EventRecord) IsDeleted() bool {
	return strings.Contains(r.EventName, "ObjectRemoved")
}

// BucketName returns the name of the bucket involved in the event.
func (r S3EventRecord) BucketName() string {
	return r.S3.Bucket.Name
}

// ObjectKey returns the key of the object involved in the event.
func (r S3EventRecord) ObjectKey() string {
	return r.S3.Object.Key
}
