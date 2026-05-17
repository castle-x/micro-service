package biz

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
	assetstorage "github.com/castlexu/micro-service/services/asset/storage"
)

const defaultMediaURLTTL = 15 * time.Minute

type MediaConfig struct {
	ObjectKeyPrefix     string
	Bucket              string
	UploadURLTTL        time.Duration
	DownloadURLTTL      time.Duration
	MaxUploadSizeBytes  int64
	AllowedContentTypes []string
	PublicBaseURL       string
	CDNBaseURL          string
	Now                 func() time.Time
}

type MediaUploadSessionInput struct {
	ContentType string
	Size        int64
	Filename    string
	SHA256      string
}

type MediaFinalizeInput struct {
	SHA256 string
	Width  int32
	Height int32
}

type MediaObjectRepository interface {
	CreateMediaObject(ctx context.Context, doc *assetmodel.MediaObject) (primitive.ObjectID, error)
	FindMediaObjectByID(ctx context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.MediaObject, error)
	FindMediaObjectByObjectKey(ctx context.Context, provider assetmodel.StorageProvider, bucket, objectKey string) (*assetmodel.MediaObject, error)
	ListMediaObjects(ctx context.Context, workspaceID string, pageNum, pageSize int32, source assetmodel.AssetSource, contentType string) ([]*assetmodel.MediaObject, int64, error)
}

type StorageUploadSessionRepository interface {
	CreateStorageUploadSession(ctx context.Context, doc *assetmodel.StorageUploadSession) (primitive.ObjectID, error)
	FindStorageUploadSessionByID(ctx context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.StorageUploadSession, error)
	UpdateStorageUploadSession(ctx context.Context, doc *assetmodel.StorageUploadSession) error
	SetUploadSessionFinalized(ctx context.Context, workspaceID string, sessionID, mediaID primitive.ObjectID, finalizedAt int64) error
	SetUploadSessionExpired(ctx context.Context, workspaceID string, sessionID primitive.ObjectID) error
}

type MediaBiz struct {
	mediaRepo   MediaObjectRepository
	sessionRepo StorageUploadSessionRepository
	storage     assetstorage.Client
	cfg         MediaConfig
	allowed     map[string]struct{}
}

func NewMediaBiz(mediaRepo MediaObjectRepository, sessionRepo StorageUploadSessionRepository, storage assetstorage.Client, cfg MediaConfig) *MediaBiz {
	cfg = normalizeMediaConfig(cfg, storage)
	allowed := make(map[string]struct{}, len(cfg.AllowedContentTypes))
	for _, item := range cfg.AllowedContentTypes {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			allowed[item] = struct{}{}
		}
	}
	return &MediaBiz{mediaRepo: mediaRepo, sessionRepo: sessionRepo, storage: storage, cfg: cfg, allowed: allowed}
}

func (b *MediaBiz) CreateUploadSession(ctx context.Context, userID string, input MediaUploadSessionInput) (*assetmodel.StorageUploadSession, *assetstorage.PresignedRequest, error) {
	if b == nil || b.mediaRepo == nil || b.sessionRepo == nil || b.storage == nil {
		return nil, nil, errno.ErrAssetStorageError.WithMessage("asset: media storage is not configured")
	}
	workspaceID, createdBy, err := workspaceFromUser(userID)
	if err != nil {
		return nil, nil, err
	}
	contentType, err := b.validateUploadInput(input)
	if err != nil {
		return nil, nil, err
	}
	now := b.now()
	sessionID := primitive.NewObjectID()
	objectKey := b.objectKey(workspaceID, sessionID, contentType, now)
	session := &assetmodel.StorageUploadSession{
		WorkspaceID: workspaceID,
		Provider:    b.storage.Provider(),
		Bucket:      b.storage.Bucket(),
		ObjectKey:   objectKey,
		Status:      assetmodel.UploadSessionStatusCreated,
		ExpiresAt:   now.Add(b.cfg.UploadURLTTL).Unix(),
		CreatedBy:   createdBy,
		ContentType: contentType,
		Size:        input.Size,
		SHA256:      strings.TrimSpace(input.SHA256),
	}
	session.ID = sessionID
	id, err := b.sessionRepo.CreateStorageUploadSession(ctx, session)
	if err != nil {
		return nil, nil, mapMediaRepoErr(err)
	}
	if id != sessionID {
		session.ID = id
		session.ObjectKey = b.objectKey(workspaceID, id, contentType, now)
		if err := b.sessionRepo.UpdateStorageUploadSession(ctx, session); err != nil {
			return nil, nil, mapMediaRepoErr(err)
		}
	}
	upload, err := b.storage.PresignPut(ctx, assetstorage.ObjectSpec{
		Bucket:      session.Bucket,
		ObjectKey:   session.ObjectKey,
		ContentType: session.ContentType,
		Size:        session.Size,
	}, b.cfg.UploadURLTTL)
	if err != nil {
		return nil, nil, mapMediaStorageErr(err)
	}
	return session, upload, nil
}

func (b *MediaBiz) FinalizeUploadSession(ctx context.Context, userID, sessionID string, input MediaFinalizeInput) (*assetmodel.StorageUploadSession, *assetmodel.MediaObject, error) {
	if b == nil || b.mediaRepo == nil || b.sessionRepo == nil || b.storage == nil {
		return nil, nil, errno.ErrAssetStorageError.WithMessage("asset: media storage is not configured")
	}
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, nil, err
	}
	id, err := parseObjectID(sessionID, "session_id")
	if err != nil {
		return nil, nil, err
	}
	session, err := b.sessionRepo.FindStorageUploadSessionByID(ctx, workspaceID, id)
	if err != nil {
		return nil, nil, mapMediaRepoErr(err)
	}
	if session.Status == assetmodel.UploadSessionStatusFinalized && !session.MediaID.IsZero() {
		media, err := b.mediaRepo.FindMediaObjectByID(ctx, workspaceID, session.MediaID)
		if err != nil {
			return nil, nil, mapMediaRepoErr(err)
		}
		return session, media, nil
	}
	if session.Status != assetmodel.UploadSessionStatusCreated {
		return nil, nil, errno.ErrAssetConflict.WithMessage("asset: upload session is not finalizable")
	}
	if b.now().Unix() > session.ExpiresAt {
		_ = b.sessionRepo.SetUploadSessionExpired(ctx, workspaceID, session.ID)
		session.Status = assetmodel.UploadSessionStatusExpired
		return nil, nil, errno.ErrAssetConflict.WithMessage("asset: upload session expired")
	}

	meta, err := b.storage.HeadObject(ctx, session.Bucket, session.ObjectKey)
	if err != nil {
		if errors.Is(err, assetstorage.ErrObjectNotFound) {
			return nil, nil, errno.ErrAssetConflict.WithMessage("asset: upload object is not available")
		}
		return nil, nil, mapMediaStorageErr(err)
	}
	if meta.Size != session.Size {
		return nil, nil, errno.ErrAssetConflict.WithMessage("asset: uploaded object size mismatch")
	}
	if meta.ContentType != "" && !strings.EqualFold(meta.ContentType, session.ContentType) {
		return nil, nil, errno.ErrAssetConflict.WithMessage("asset: uploaded object content type mismatch")
	}

	media, err := b.createMediaObject(ctx, session, meta, input)
	if err != nil {
		return nil, nil, err
	}
	finalizedAt := b.now().Unix()
	if err := b.sessionRepo.SetUploadSessionFinalized(ctx, workspaceID, session.ID, media.ID, finalizedAt); err != nil {
		return nil, nil, mapMediaRepoErr(err)
	}
	session.Status = assetmodel.UploadSessionStatusFinalized
	session.MediaID = media.ID
	session.FinalizedAt = finalizedAt
	return session, media, nil
}

func (b *MediaBiz) Get(ctx context.Context, userID, mediaID string) (*assetmodel.MediaObject, error) {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, err
	}
	id, err := parseObjectID(mediaID, "media_id")
	if err != nil {
		return nil, err
	}
	doc, err := b.mediaRepo.FindMediaObjectByID(ctx, workspaceID, id)
	if err != nil {
		return nil, mapMediaRepoErr(err)
	}
	return doc, nil
}

func (b *MediaBiz) List(ctx context.Context, userID string, page PageInput, source assetmodel.AssetSource, contentType string) ([]*assetmodel.MediaObject, int64, error) {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, 0, err
	}
	page = normalizePage(page)
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	docs, total, err := b.mediaRepo.ListMediaObjects(ctx, workspaceID, page.PageNum, page.PageSize, source, contentType)
	if err != nil {
		return nil, 0, mapMediaRepoErr(err)
	}
	return docs, total, nil
}

func (b *MediaBiz) GetAccessURL(ctx context.Context, userID, mediaID string, expiresInSeconds int32) (*assetmodel.MediaObject, *assetstorage.PresignedRequest, error) {
	media, err := b.Get(ctx, userID, mediaID)
	if err != nil {
		return nil, nil, err
	}
	ttl := b.accessTTL(expiresInSeconds)
	access, err := b.storage.PresignGet(ctx, media.Bucket, media.ObjectKey, ttl)
	if err != nil {
		return nil, nil, mapMediaStorageErr(err)
	}
	return media, access, nil
}

func (b *MediaBiz) validateUploadInput(input MediaUploadSessionInput) (string, error) {
	contentType := strings.ToLower(strings.TrimSpace(input.ContentType))
	if contentType == "" {
		return "", errno.ErrInvalidParam.WithMessage("asset: content_type is required")
	}
	if _, ok := b.allowed[contentType]; !ok {
		return "", errno.ErrInvalidParam.WithMessage("asset: content_type is not allowed")
	}
	if input.Size <= 0 {
		return "", errno.ErrInvalidParam.WithMessage("asset: size must be positive")
	}
	if b.cfg.MaxUploadSizeBytes > 0 && input.Size > b.cfg.MaxUploadSizeBytes {
		return "", errno.ErrInvalidParam.WithMessage("asset: size exceeds max upload size")
	}
	return contentType, nil
}

func (b *MediaBiz) createMediaObject(ctx context.Context, session *assetmodel.StorageUploadSession, meta *assetstorage.ObjectMeta, input MediaFinalizeInput) (*assetmodel.MediaObject, error) {
	contentType := meta.ContentType
	if contentType == "" {
		contentType = session.ContentType
	}
	sha := strings.TrimSpace(input.SHA256)
	if sha == "" {
		sha = session.SHA256
	}
	media := &assetmodel.MediaObject{
		WorkspaceID:   session.WorkspaceID,
		Provider:      session.Provider,
		Bucket:        session.Bucket,
		ObjectKey:     session.ObjectKey,
		CDNURL:        joinBaseURL(b.cfg.CDNBaseURL, session.ObjectKey),
		URLVisibility: assetmodel.URLVisibilitySigned,
		ContentType:   contentType,
		Size:          meta.Size,
		Width:         input.Width,
		Height:        input.Height,
		SHA256:        sha,
		Source:        assetmodel.AssetSourceUpload,
		CreatedBy:     session.CreatedBy,
	}
	id, err := b.mediaRepo.CreateMediaObject(ctx, media)
	if err != nil {
		if errors.Is(err, errno.ErrDuplicateKey) {
			existing, findErr := b.mediaRepo.FindMediaObjectByObjectKey(ctx, session.Provider, session.Bucket, session.ObjectKey)
			if findErr != nil {
				return nil, mapMediaRepoErr(findErr)
			}
			return existing, nil
		}
		return nil, mapMediaRepoErr(err)
	}
	media.ID = id
	return media, nil
}

func (b *MediaBiz) objectKey(workspaceID string, sessionID primitive.ObjectID, contentType string, now time.Time) string {
	prefix := strings.Trim(strings.TrimSpace(b.cfg.ObjectKeyPrefix), "/")
	ext := extensionForContentType(contentType)
	parts := []string{
		workspaceID,
		"uploads",
		now.UTC().Format("2006"),
		now.UTC().Format("01"),
		now.UTC().Format("02"),
		sessionID.Hex(),
		"original." + ext,
	}
	if prefix != "" {
		parts = append([]string{prefix}, parts...)
	}
	return path.Join(parts...)
}

func (b *MediaBiz) accessTTL(expiresInSeconds int32) time.Duration {
	maxTTL := b.cfg.DownloadURLTTL
	if maxTTL <= 0 {
		maxTTL = defaultMediaURLTTL
	}
	if expiresInSeconds <= 0 {
		return maxTTL
	}
	ttl := time.Duration(expiresInSeconds) * time.Second
	if ttl > maxTTL {
		return maxTTL
	}
	return ttl
}

func (b *MediaBiz) now() time.Time {
	if b.cfg.Now != nil {
		return b.cfg.Now().UTC()
	}
	return time.Now().UTC()
}

func normalizeMediaConfig(cfg MediaConfig, storage assetstorage.Client) MediaConfig {
	cfg.ObjectKeyPrefix = strings.Trim(strings.TrimSpace(cfg.ObjectKeyPrefix), "/")
	if cfg.ObjectKeyPrefix == "" {
		cfg.ObjectKeyPrefix = "assets"
	}
	if cfg.UploadURLTTL == 0 {
		cfg.UploadURLTTL = defaultMediaURLTTL
	}
	if cfg.DownloadURLTTL == 0 {
		cfg.DownloadURLTTL = defaultMediaURLTTL
	}
	if cfg.MaxUploadSizeBytes == 0 {
		cfg.MaxUploadSizeBytes = 20 << 20
	}
	if len(cfg.AllowedContentTypes) == 0 {
		cfg.AllowedContentTypes = []string{"image/jpeg", "image/png", "image/webp", "image/gif"}
	}
	if cfg.Bucket == "" && storage != nil {
		cfg.Bucket = storage.Bucket()
	}
	return cfg
}

func extensionForContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	default:
		return "bin"
	}
}

func joinBaseURL(baseURL, objectKey string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", baseURL, strings.TrimLeft(objectKey, "/"))
}

func mapMediaStorageErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, assetstorage.ErrObjectNotFound) {
		return errno.ErrAssetConflict.WithMessage("asset: upload object is not available")
	}
	if errors.Is(err, errno.ErrAssetStorageError) {
		return err
	}
	return errno.ErrAssetStorageError.WithMessage("asset: storage operation failed")
}

func mapMediaRepoErr(err error) error {
	if err == nil {
		return nil
	}
	for _, target := range []error{
		errno.ErrMediaObjectNotFound,
		errno.ErrAssetUploadSessionNotFound,
		errno.ErrAssetConflict,
		errno.ErrDuplicateKey,
		errno.ErrInvalidParam,
		errno.ErrAssetStorageError,
	} {
		if errors.Is(err, target) {
			return err
		}
	}
	return errno.ErrInternal.WithMessagef("asset: media repository error: %v", err)
}
