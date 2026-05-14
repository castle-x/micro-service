package storage

import (
	"context"
	"errors"
	"time"

	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

// ErrObjectNotFound means the object is not visible in object storage yet.
var ErrObjectNotFound = errors.New("asset storage object not found")

type ObjectSpec struct {
	Bucket      string
	ObjectKey   string
	ContentType string
	Size        int64
}

type PresignedRequest struct {
	Method    string
	URL       string
	Headers   map[string]string
	ExpiresAt int64
}

type ObjectMeta struct {
	Bucket      string
	ObjectKey   string
	ContentType string
	Size        int64
	ETag        string
}

type Client interface {
	Provider() assetmodel.StorageProvider
	Bucket() string
	PresignPut(ctx context.Context, spec ObjectSpec, ttl time.Duration) (*PresignedRequest, error)
	PresignGet(ctx context.Context, bucket, objectKey string, ttl time.Duration) (*PresignedRequest, error)
	HeadObject(ctx context.Context, bucket, objectKey string) (*ObjectMeta, error)
}
