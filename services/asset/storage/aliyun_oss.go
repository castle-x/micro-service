package storage

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type AliyunOSSConfig struct {
	Region          string `mapstructure:"region"`
	Endpoint        string `mapstructure:"endpoint"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	SecurityToken   string `mapstructure:"security_token"`
	PublicBaseURL   string `mapstructure:"public_base_url"`
	CDNBaseURL      string `mapstructure:"cdn_base_url"`
}

type AliyunOSSClient struct {
	client *oss.Client
	bucket string
}

func NewAliyunOSSClient(cfg AliyunOSSConfig) (*AliyunOSSClient, error) {
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.AccessKeyID = strings.TrimSpace(cfg.AccessKeyID)
	cfg.AccessKeySecret = strings.TrimSpace(cfg.AccessKeySecret)
	cfg.SecurityToken = strings.TrimSpace(cfg.SecurityToken)
	if cfg.Region == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return nil, errno.ErrAssetStorageError.WithMessage("asset: aliyun oss config is incomplete")
	}
	provider := credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.AccessKeySecret, cfg.SecurityToken)
	ossCfg := oss.LoadDefaultConfig().
		WithCredentialsProvider(provider).
		WithRegion(cfg.Region)
	if cfg.Endpoint != "" {
		ossCfg = ossCfg.WithEndpoint(cfg.Endpoint)
	}
	return &AliyunOSSClient{
		client: oss.NewClient(ossCfg),
		bucket: cfg.Bucket,
	}, nil
}

func (c *AliyunOSSClient) Provider() assetmodel.StorageProvider {
	return assetmodel.StorageProviderAliyunOSS
}

func (c *AliyunOSSClient) Bucket() string {
	return c.bucket
}

func (c *AliyunOSSClient) PresignPut(ctx context.Context, spec ObjectSpec, ttl time.Duration) (*PresignedRequest, error) {
	ctx, span := startOSSSpan(ctx, "OSS presign_put", "presign_put", c.bucket)
	var opErr error
	defer func() {
		endOSSSpan(span, opErr)
	}()

	result, err := c.client.Presign(ctx, &oss.PutObjectRequest{
		Bucket:      oss.Ptr(c.bucket),
		Key:         oss.Ptr(spec.ObjectKey),
		ContentType: oss.Ptr(spec.ContentType),
	}, oss.PresignExpires(ttl))
	if err != nil {
		opErr = err
		return nil, errno.ErrAssetStorageError.WithMessage("asset: oss presign put failed")
	}
	return presignedResult(result), nil
}

func (c *AliyunOSSClient) PresignGet(ctx context.Context, bucket, objectKey string, ttl time.Duration) (*PresignedRequest, error) {
	if strings.TrimSpace(bucket) == "" {
		bucket = c.bucket
	}
	ctx, span := startOSSSpan(ctx, "OSS presign_get", "presign_get", bucket)
	var opErr error
	defer func() {
		endOSSSpan(span, opErr)
	}()

	result, err := c.client.Presign(ctx, &oss.GetObjectRequest{
		Bucket: oss.Ptr(bucket),
		Key:    oss.Ptr(objectKey),
	}, oss.PresignExpires(ttl))
	if err != nil {
		opErr = err
		return nil, errno.ErrAssetStorageError.WithMessage("asset: oss presign get failed")
	}
	return presignedResult(result), nil
}

func (c *AliyunOSSClient) HeadObject(ctx context.Context, bucket, objectKey string) (*ObjectMeta, error) {
	if strings.TrimSpace(bucket) == "" {
		bucket = c.bucket
	}
	ctx, span := startOSSSpan(ctx, "OSS head_object", "head_object", bucket)
	var opErr error
	defer func() {
		endOSSSpan(span, opErr)
	}()

	result, err := c.client.HeadObject(ctx, &oss.HeadObjectRequest{
		Bucket: oss.Ptr(bucket),
		Key:    oss.Ptr(objectKey),
	})
	if err != nil {
		opErr = err
		return nil, mapOSSError("head_object", err)
	}
	return &ObjectMeta{
		Bucket:      bucket,
		ObjectKey:   objectKey,
		ContentType: oss.ToString(result.ContentType),
		Size:        result.ContentLength,
		ETag:        oss.ToString(result.ETag),
	}, nil
}

func presignedResult(result *oss.PresignResult) *PresignedRequest {
	if result == nil {
		return nil
	}
	headers := make(map[string]string, len(result.SignedHeaders))
	for k, v := range result.SignedHeaders {
		headers[k] = v
	}
	return &PresignedRequest{
		Method:    result.Method,
		URL:       result.URL,
		Headers:   headers,
		ExpiresAt: result.Expiration.Unix(),
	}
}

func mapOSSError(operation string, err error) error {
	var serviceErr *oss.ServiceError
	if errors.As(err, &serviceErr) {
		if serviceErr.StatusCode == http.StatusNotFound || serviceErr.Code == "NoSuchKey" {
			return ErrObjectNotFound
		}
		return errno.ErrAssetStorageError.WithMessagef("asset: oss %s failed: status=%d code=%s", operation, serviceErr.StatusCode, serviceErr.Code)
	}
	return errno.ErrAssetStorageError.WithMessagef("asset: oss %s failed", operation)
}

func startOSSSpan(ctx context.Context, spanName, operation, bucket string) (context.Context, trace.Span) {
	return otel.Tracer("github.com/castlexu/micro-service/services/asset/storage").Start(
		ctx,
		spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("storage.provider", "aliyun_oss"),
			attribute.String("storage.bucket", bucket),
			attribute.String("storage.operation", operation),
		),
	)
}

func endOSSSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
