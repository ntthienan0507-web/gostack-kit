package storage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// UploadOption builders
// ---------------------------------------------------------------------------

func TestWithMetadata(t *testing.T) {
	meta := map[string]string{"author": "test", "version": "1"}
	opt := WithMetadata(meta)

	var uo uploadOptions
	opt(&uo)

	assert.Equal(t, meta, uo.metadata)
}

func TestWithContentDisposition(t *testing.T) {
	opt := WithContentDisposition("attachment; filename=\"report.pdf\"")

	var uo uploadOptions
	opt(&uo)

	assert.Equal(t, "attachment; filename=\"report.pdf\"", uo.contentDisposition)
}

func TestUploadOptionsChaining(t *testing.T) {
	opts := []UploadOption{
		WithMetadata(map[string]string{"env": "test"}),
		WithContentDisposition("inline"),
	}

	var uo uploadOptions
	for _, o := range opts {
		o(&uo)
	}

	assert.Equal(t, map[string]string{"env": "test"}, uo.metadata)
	assert.Equal(t, "inline", uo.contentDisposition)
}

// ---------------------------------------------------------------------------
// S3Event JSON parsing
// ---------------------------------------------------------------------------

const sampleS3Event = `{
  "Records": [
    {
      "eventName": "s3:ObjectCreated:Put",
      "eventTime": "2024-06-15T12:34:56.000Z",
      "eventSource": "aws:s3",
      "s3": {
        "bucket": {
          "name": "my-bucket"
        },
        "object": {
          "key": "uploads/photo.jpg",
          "size": 1048576,
          "eTag": "d41d8cd98f00b204e9800998ecf8427e",
          "contentType": "image/jpeg"
        }
      },
      "userIdentity": {
        "principalId": "AIDAJDPLRKLG7UEXAMPLE"
      }
    }
  ]
}`

func TestS3EventParsing(t *testing.T) {
	var event S3Event
	err := json.Unmarshal([]byte(sampleS3Event), &event)
	require.NoError(t, err)

	require.Len(t, event.Records, 1)
	rec := event.Records[0]

	assert.Equal(t, "s3:ObjectCreated:Put", rec.EventName)
	assert.Equal(t, "aws:s3", rec.EventSource)
	assert.Equal(t, "AIDAJDPLRKLG7UEXAMPLE", rec.UserIdentity.PrincipalID)

	expected := time.Date(2024, 6, 15, 12, 34, 56, 0, time.UTC)
	assert.True(t, rec.EventTime.Equal(expected), "expected %v, got %v", expected, rec.EventTime)

	assert.Equal(t, "my-bucket", rec.S3.Bucket.Name)
	assert.Equal(t, "uploads/photo.jpg", rec.S3.Object.Key)
	assert.Equal(t, int64(1048576), rec.S3.Object.Size)
	assert.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", rec.S3.Object.ETag)
	assert.Equal(t, "image/jpeg", rec.S3.Object.ContentType)
}

const sampleS3DeleteEvent = `{
  "Records": [
    {
      "eventName": "s3:ObjectRemoved:Delete",
      "eventTime": "2024-06-15T13:00:00.000Z",
      "eventSource": "minio:s3",
      "s3": {
        "bucket": { "name": "archive" },
        "object": { "key": "old/file.txt", "size": 0, "eTag": "" }
      },
      "userIdentity": { "principalId": "minioadmin" }
    }
  ]
}`

func TestS3EventParsingDeleteEvent(t *testing.T) {
	var event S3Event
	err := json.Unmarshal([]byte(sampleS3DeleteEvent), &event)
	require.NoError(t, err)

	require.Len(t, event.Records, 1)
	rec := event.Records[0]

	assert.Equal(t, "s3:ObjectRemoved:Delete", rec.EventName)
	assert.Equal(t, "minio:s3", rec.EventSource)
	assert.Equal(t, "archive", rec.S3.Bucket.Name)
	assert.Equal(t, "old/file.txt", rec.S3.Object.Key)
}

func TestS3EventMultipleRecords(t *testing.T) {
	raw := `{
		"Records": [
			{"eventName": "s3:ObjectCreated:Put", "s3": {"bucket": {"name": "b1"}, "object": {"key": "k1"}}},
			{"eventName": "s3:ObjectRemoved:Delete", "s3": {"bucket": {"name": "b2"}, "object": {"key": "k2"}}}
		]
	}`

	var event S3Event
	err := json.Unmarshal([]byte(raw), &event)
	require.NoError(t, err)
	require.Len(t, event.Records, 2)

	assert.Equal(t, "k1", event.Records[0].ObjectKey())
	assert.Equal(t, "b2", event.Records[1].BucketName())
}

// ---------------------------------------------------------------------------
// IsCreated / IsDeleted helpers
// ---------------------------------------------------------------------------

func TestIsCreated(t *testing.T) {
	tests := []struct {
		eventName string
		expected  bool
	}{
		{"s3:ObjectCreated:Put", true},
		{"s3:ObjectCreated:Post", true},
		{"s3:ObjectCreated:CompleteMultipartUpload", true},
		{"s3:ObjectRemoved:Delete", false},
		{"s3:ObjectRemoved:DeleteMarkerCreated", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.eventName, func(t *testing.T) {
			rec := S3EventRecord{EventName: tt.eventName}
			assert.Equal(t, tt.expected, rec.IsCreated())
		})
	}
}

func TestIsDeleted(t *testing.T) {
	tests := []struct {
		eventName string
		expected  bool
	}{
		{"s3:ObjectRemoved:Delete", true},
		{"s3:ObjectRemoved:DeleteMarkerCreated", true},
		{"s3:ObjectCreated:Put", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.eventName, func(t *testing.T) {
			rec := S3EventRecord{EventName: tt.eventName}
			assert.Equal(t, tt.expected, rec.IsDeleted())
		})
	}
}

// ---------------------------------------------------------------------------
// BucketName / ObjectKey helpers
// ---------------------------------------------------------------------------

func TestBucketName(t *testing.T) {
	rec := S3EventRecord{
		S3: S3Entity{Bucket: S3Bucket{Name: "my-bucket"}},
	}
	assert.Equal(t, "my-bucket", rec.BucketName())
}

func TestObjectKey(t *testing.T) {
	rec := S3EventRecord{
		S3: S3Entity{Object: S3Object{Key: "path/to/file.txt"}},
	}
	assert.Equal(t, "path/to/file.txt", rec.ObjectKey())
}
