package biz

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
	assetstorage "github.com/castlexu/micro-service/services/asset/storage"
)

func TestMediaBiz_CreateUploadSessionValidatesInputAndPresignsPut(t *testing.T) {
	ctx := context.Background()
	store := newAS04MediaStore()
	storage := newAS04FakeStorage()
	biz := NewMediaBiz(store, store, storage, MediaConfig{
		ObjectKeyPrefix:     "assets-test",
		Bucket:              "asset-test-bucket",
		UploadURLTTL:        15 * time.Minute,
		DownloadURLTTL:      15 * time.Minute,
		MaxUploadSizeBytes:  20 << 20,
		AllowedContentTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
		Now:                 func() time.Time { return time.Date(2026, 5, 14, 9, 30, 0, 0, time.UTC) },
	})
	userID := primitive.NewObjectID().Hex()

	session, upload, err := biz.CreateUploadSession(ctx, userID, MediaUploadSessionInput{
		ContentType: "image/png",
		Size:        1024,
		Filename:    "private-name-should-not-leak.png",
		SHA256:      "declared-sha",
	})
	if err != nil {
		t.Fatalf("CreateUploadSession: %v", err)
	}
	if session.WorkspaceID != userID || session.CreatedBy.Hex() != userID {
		t.Fatalf("workspace/creator = (%q, %s), want user id", session.WorkspaceID, session.CreatedBy.Hex())
	}
	if session.ContentType != "image/png" || session.Size != 1024 || session.SHA256 != "declared-sha" {
		t.Fatalf("session upload constraints not persisted: %#v", session)
	}
	if !strings.HasPrefix(session.ObjectKey, "assets-test/"+userID+"/uploads/2026/05/14/"+session.ID.Hex()+"/original.") {
		t.Fatalf("object key = %q, want assets-test/<workspace>/uploads/YYYY/MM/DD/<session>/original.<ext>", session.ObjectKey)
	}
	if !strings.HasSuffix(session.ObjectKey, "/original.png") {
		t.Fatalf("object key = %q, want png extension from content type", session.ObjectKey)
	}
	if strings.Contains(session.ObjectKey, "private-name") {
		t.Fatalf("object key leaked original filename: %q", session.ObjectKey)
	}
	if upload.Method != "PUT" {
		t.Fatalf("upload method = %q, want PUT", upload.Method)
	}
	if got := upload.Headers["Content-Type"]; got != "image/png" {
		t.Fatalf("upload Content-Type header = %q, want image/png", got)
	}
	if storage.presignPutSpec.ObjectKey != session.ObjectKey || storage.presignPutSpec.ContentType != "image/png" || storage.presignPutSpec.Size != 1024 {
		t.Fatalf("presign spec = %#v, want session object constraints", storage.presignPutSpec)
	}

	tests := []struct {
		name  string
		input MediaUploadSessionInput
	}{
		{name: "empty content type", input: MediaUploadSessionInput{Size: 1}},
		{name: "content type not allowed", input: MediaUploadSessionInput{ContentType: "application/pdf", Size: 1}},
		{name: "zero size", input: MediaUploadSessionInput{ContentType: "image/png", Size: 0}},
		{name: "too large", input: MediaUploadSessionInput{ContentType: "image/png", Size: (20 << 20) + 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := biz.CreateUploadSession(ctx, userID, tt.input)
			if !errors.Is(err, errno.ErrInvalidParam) {
				t.Fatalf("CreateUploadSession err = %v, want ErrInvalidParam", err)
			}
		})
	}
}

func TestMediaBiz_FinalizeCreatesMediaObjectAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store := newAS04MediaStore()
	storage := newAS04FakeStorage()
	biz := NewMediaBiz(store, store, storage, as04MediaConfig())
	userID := primitive.NewObjectID().Hex()
	session, _, err := biz.CreateUploadSession(ctx, userID, MediaUploadSessionInput{
		ContentType: "image/png",
		Size:        67,
		Filename:    "avatar.png",
	})
	if err != nil {
		t.Fatalf("CreateUploadSession: %v", err)
	}
	storage.headObjects[session.ObjectKey] = &assetstorage.ObjectMeta{
		Bucket:      session.Bucket,
		ObjectKey:   session.ObjectKey,
		ContentType: "image/png",
		Size:        67,
		ETag:        "etag-1",
	}

	finalized, media, err := biz.FinalizeUploadSession(ctx, userID, session.ID.Hex(), MediaFinalizeInput{
		Width:  16,
		Height: 16,
	})
	if err != nil {
		t.Fatalf("FinalizeUploadSession: %v", err)
	}
	if finalized.Status != assetmodel.UploadSessionStatusFinalized || finalized.MediaID != media.ID {
		t.Fatalf("finalized session = %#v, media = %#v", finalized, media)
	}
	if media.WorkspaceID != userID || media.ObjectKey != session.ObjectKey || media.ContentType != "image/png" || media.Size != 67 {
		t.Fatalf("media object = %#v, want uploaded object metadata in workspace", media)
	}
	if media.URLVisibility != assetmodel.URLVisibilitySigned || media.Source != assetmodel.AssetSourceUpload {
		t.Fatalf("media visibility/source = (%v, %v), want signed upload", media.URLVisibility, media.Source)
	}

	againSession, againMedia, err := biz.FinalizeUploadSession(ctx, userID, session.ID.Hex(), MediaFinalizeInput{})
	if err != nil {
		t.Fatalf("FinalizeUploadSession idempotent retry: %v", err)
	}
	if againSession.ID != finalized.ID || againMedia.ID != media.ID {
		t.Fatalf("idempotent retry returned (%s, %s), want (%s, %s)", againSession.ID.Hex(), againMedia.ID.Hex(), finalized.ID.Hex(), media.ID.Hex())
	}
	if store.mediaCreates != 1 {
		t.Fatalf("media create count = %d, want exactly one create across repeated finalize", store.mediaCreates)
	}
}

func TestMediaBiz_FinalizeRejectsExpiredMissingObjectAndMismatchedSize(t *testing.T) {
	ctx := context.Background()
	userID := primitive.NewObjectID().Hex()

	t.Run("expired session", func(t *testing.T) {
		store := newAS04MediaStore()
		storage := newAS04FakeStorage()
		biz := NewMediaBiz(store, store, storage, MediaConfig{
			ObjectKeyPrefix:     "assets-test",
			Bucket:              "asset-test-bucket",
			UploadURLTTL:        -time.Minute,
			DownloadURLTTL:      15 * time.Minute,
			MaxUploadSizeBytes:  20 << 20,
			AllowedContentTypes: []string{"image/png"},
			Now:                 func() time.Time { return time.Date(2026, 5, 14, 9, 30, 0, 0, time.UTC) },
		})
		session, _, err := biz.CreateUploadSession(ctx, userID, MediaUploadSessionInput{ContentType: "image/png", Size: 10})
		if err != nil {
			t.Fatalf("CreateUploadSession: %v", err)
		}
		_, _, err = biz.FinalizeUploadSession(ctx, userID, session.ID.Hex(), MediaFinalizeInput{})
		if !errors.Is(err, errno.ErrAssetConflict) {
			t.Fatalf("FinalizeUploadSession err = %v, want ErrAssetConflict", err)
		}
		got, _ := store.FindStorageUploadSessionByID(ctx, userID, session.ID)
		if got.Status != assetmodel.UploadSessionStatusExpired {
			t.Fatalf("session status = %v, want EXPIRED", got.Status)
		}
	})

	t.Run("missing object", func(t *testing.T) {
		store := newAS04MediaStore()
		storage := newAS04FakeStorage()
		biz := NewMediaBiz(store, store, storage, as04MediaConfig())
		session, _, err := biz.CreateUploadSession(ctx, userID, MediaUploadSessionInput{ContentType: "image/png", Size: 10})
		if err != nil {
			t.Fatalf("CreateUploadSession: %v", err)
		}
		_, _, err = biz.FinalizeUploadSession(ctx, userID, session.ID.Hex(), MediaFinalizeInput{})
		if !errors.Is(err, errno.ErrAssetConflict) {
			t.Fatalf("FinalizeUploadSession err = %v, want ErrAssetConflict", err)
		}
	})

	t.Run("size mismatch", func(t *testing.T) {
		store := newAS04MediaStore()
		storage := newAS04FakeStorage()
		biz := NewMediaBiz(store, store, storage, as04MediaConfig())
		session, _, err := biz.CreateUploadSession(ctx, userID, MediaUploadSessionInput{ContentType: "image/png", Size: 10})
		if err != nil {
			t.Fatalf("CreateUploadSession: %v", err)
		}
		storage.headObjects[session.ObjectKey] = &assetstorage.ObjectMeta{Bucket: session.Bucket, ObjectKey: session.ObjectKey, ContentType: "image/png", Size: 11}
		_, _, err = biz.FinalizeUploadSession(ctx, userID, session.ID.Hex(), MediaFinalizeInput{})
		if !errors.Is(err, errno.ErrAssetConflict) {
			t.Fatalf("FinalizeUploadSession err = %v, want ErrAssetConflict", err)
		}
	})
}

func TestAssetBiz_RejectsNonexistentAndCrossWorkspaceCoverMedia(t *testing.T) {
	ctx := context.Background()
	store := newAS04MediaStore()
	typeBiz := NewAssetTypeBiz(store, store)
	assetBiz := NewAssetBiz(store, store, store, store)
	userA := primitive.NewObjectID().Hex()
	userB := primitive.NewObjectID().Hex()
	assetType, err := typeBiz.Create(ctx, userA, AssetTypeInput{Name: "Character", Code: "character"})
	if err != nil {
		t.Fatalf("Create asset type: %v", err)
	}
	otherMedia := store.mustCreateMedia(userB, "assets-test/"+userB+"/uploads/2026/05/14/other/original.png")

	_, err = assetBiz.Create(ctx, userA, AssetInput{
		TypeID:       assetType.ID.Hex(),
		Name:         "Hero",
		CoverMediaID: primitive.NewObjectID().Hex(),
	})
	if !errors.Is(err, errno.ErrMediaObjectNotFound) {
		t.Fatalf("nonexistent cover media err = %v, want ErrMediaObjectNotFound", err)
	}

	_, err = assetBiz.Create(ctx, userA, AssetInput{
		TypeID:       assetType.ID.Hex(),
		Name:         "Hero",
		CoverMediaID: otherMedia.ID.Hex(),
	})
	if !errors.Is(err, errno.ErrMediaObjectNotFound) {
		t.Fatalf("cross-workspace cover media err = %v, want ErrMediaObjectNotFound", err)
	}
}

func TestAssetVersionBiz_RejectsNonexistentAndCrossWorkspaceMediaParts(t *testing.T) {
	ctx := context.Background()
	store := newAS04MediaStore()
	typeBiz := NewAssetTypeBiz(store, store)
	assetBiz := NewAssetBiz(store, store, store, store)
	versionBiz := NewAssetVersionBiz(store, store, store, store)
	userA := primitive.NewObjectID().Hex()
	userB := primitive.NewObjectID().Hex()
	assetType, err := typeBiz.Create(ctx, userA, AssetTypeInput{
		Name: "Character",
		Code: "character",
		PartSchemas: []assetmodel.AssetPartSchema{{
			Key:               "reference_images",
			Name:              "Reference Images",
			AllowedValueKinds: []assetmodel.AssetValueKind{assetmodel.AssetValueKindMedia},
			Required:          true,
		}},
	})
	if err != nil {
		t.Fatalf("Create asset type: %v", err)
	}
	asset, err := assetBiz.Create(ctx, userA, AssetInput{TypeID: assetType.ID.Hex(), Name: "Hero"})
	if err != nil {
		t.Fatalf("Create asset: %v", err)
	}
	otherMedia := store.mustCreateMedia(userB, "assets-test/"+userB+"/uploads/2026/05/14/other/original.png")

	for _, tt := range []struct {
		name    string
		mediaID primitive.ObjectID
	}{
		{name: "nonexistent", mediaID: primitive.NewObjectID()},
		{name: "cross workspace", mediaID: otherMedia.ID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := versionBiz.Create(ctx, userA, asset.ID.Hex(), AssetVersionInput{
				Parts: map[string]assetmodel.AssetPartValue{
					"reference_images": {ValueKind: assetmodel.AssetValueKindMedia, MediaIDs: []primitive.ObjectID{tt.mediaID}},
				},
			})
			if !errors.Is(err, errno.ErrAssetInvalidPart) {
				t.Fatalf("Create version err = %v, want ErrAssetInvalidPart", err)
			}
		})
	}
}

type as04MediaStore struct {
	*memoryAssetStore
	media    map[primitive.ObjectID]*assetmodel.MediaObject
	sessions map[primitive.ObjectID]*assetmodel.StorageUploadSession

	mediaCreates int
}

func newAS04MediaStore() *as04MediaStore {
	return &as04MediaStore{
		memoryAssetStore: newMemoryAssetStore(),
		media:            make(map[primitive.ObjectID]*assetmodel.MediaObject),
		sessions:         make(map[primitive.ObjectID]*assetmodel.StorageUploadSession),
	}
}

func as04MediaConfig() MediaConfig {
	return MediaConfig{
		ObjectKeyPrefix:     "assets-test",
		Bucket:              "asset-test-bucket",
		UploadURLTTL:        15 * time.Minute,
		DownloadURLTTL:      15 * time.Minute,
		MaxUploadSizeBytes:  20 << 20,
		AllowedContentTypes: []string{"image/png"},
		Now:                 func() time.Time { return time.Date(2026, 5, 14, 9, 30, 0, 0, time.UTC) },
	}
}

func (s *as04MediaStore) CreateMediaObject(_ context.Context, doc *assetmodel.MediaObject) (primitive.ObjectID, error) {
	for _, existing := range s.media {
		if existing.Provider == doc.Provider && existing.Bucket == doc.Bucket && existing.ObjectKey == doc.ObjectKey {
			return primitive.NilObjectID, errno.ErrDuplicateKey
		}
	}
	id := primitive.NewObjectID()
	cp := *doc
	cp.ID = id
	s.media[id] = &cp
	s.mediaCreates++
	return id, nil
}

func (s *as04MediaStore) FindMediaObjectByID(_ context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.MediaObject, error) {
	doc, ok := s.media[id]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return nil, errno.ErrMediaObjectNotFound
	}
	cp := *doc
	return &cp, nil
}

func (s *as04MediaStore) FindMediaObjectByObjectKey(_ context.Context, provider assetmodel.StorageProvider, bucket, objectKey string) (*assetmodel.MediaObject, error) {
	for _, doc := range s.media {
		if doc.DeletedAt == nil && doc.Provider == provider && doc.Bucket == bucket && doc.ObjectKey == objectKey {
			cp := *doc
			return &cp, nil
		}
	}
	return nil, errno.ErrMediaObjectNotFound
}

func (s *as04MediaStore) ListMediaObjects(_ context.Context, workspaceID string, pageNum, pageSize int32, source assetmodel.AssetSource, contentType string) ([]*assetmodel.MediaObject, int64, error) {
	out := make([]*assetmodel.MediaObject, 0)
	for _, doc := range s.media {
		if doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
			continue
		}
		if source != assetmodel.AssetSourceUnknown && doc.Source != source {
			continue
		}
		if contentType != "" && doc.ContentType != contentType {
			continue
		}
		cp := *doc
		out = append(out, &cp)
	}
	return out, int64(len(out)), nil
}

func (s *as04MediaStore) CreateStorageUploadSession(_ context.Context, doc *assetmodel.StorageUploadSession) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := *doc
	cp.ID = id
	s.sessions[id] = &cp
	return id, nil
}

func (s *as04MediaStore) FindStorageUploadSessionByID(_ context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.StorageUploadSession, error) {
	doc, ok := s.sessions[id]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return nil, errno.ErrAssetUploadSessionNotFound
	}
	cp := *doc
	return &cp, nil
}

func (s *as04MediaStore) UpdateStorageUploadSession(_ context.Context, doc *assetmodel.StorageUploadSession) error {
	existing, ok := s.sessions[doc.ID]
	if !ok || existing.DeletedAt != nil || existing.WorkspaceID != doc.WorkspaceID {
		return errno.ErrAssetUploadSessionNotFound
	}
	cp := *doc
	s.sessions[doc.ID] = &cp
	return nil
}

func (s *as04MediaStore) SetUploadSessionFinalized(_ context.Context, workspaceID string, sessionID, mediaID primitive.ObjectID, finalizedAt int64) error {
	doc, ok := s.sessions[sessionID]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return errno.ErrAssetUploadSessionNotFound
	}
	doc.Status = assetmodel.UploadSessionStatusFinalized
	doc.MediaID = mediaID
	doc.FinalizedAt = finalizedAt
	return nil
}

func (s *as04MediaStore) SetUploadSessionExpired(_ context.Context, workspaceID string, sessionID primitive.ObjectID) error {
	doc, ok := s.sessions[sessionID]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return errno.ErrAssetUploadSessionNotFound
	}
	doc.Status = assetmodel.UploadSessionStatusExpired
	return nil
}

func (s *as04MediaStore) mustCreateMedia(workspaceID, objectKey string) *assetmodel.MediaObject {
	id := primitive.NewObjectID()
	createdBy, _ := primitive.ObjectIDFromHex(workspaceID)
	doc := &assetmodel.MediaObject{
		WorkspaceID:   workspaceID,
		Provider:      assetmodel.StorageProviderAliyunOSS,
		Bucket:        "asset-test-bucket",
		ObjectKey:     objectKey,
		URLVisibility: assetmodel.URLVisibilitySigned,
		ContentType:   "image/png",
		Size:          10,
		Source:        assetmodel.AssetSourceUpload,
		CreatedBy:     createdBy,
	}
	doc.ID = id
	s.media[id] = doc
	return doc
}

type as04FakeStorage struct {
	presignPutSpec assetstorage.ObjectSpec
	headObjects    map[string]*assetstorage.ObjectMeta
}

func newAS04FakeStorage() *as04FakeStorage {
	return &as04FakeStorage{headObjects: make(map[string]*assetstorage.ObjectMeta)}
}

func (s *as04FakeStorage) Provider() assetmodel.StorageProvider {
	return assetmodel.StorageProviderAliyunOSS
}

func (s *as04FakeStorage) Bucket() string {
	return "asset-test-bucket"
}

func (s *as04FakeStorage) PresignPut(_ context.Context, spec assetstorage.ObjectSpec, ttl time.Duration) (*assetstorage.PresignedRequest, error) {
	s.presignPutSpec = spec
	return &assetstorage.PresignedRequest{
		Method:    "PUT",
		URL:       "https://oss.example.test/upload",
		Headers:   map[string]string{"Content-Type": spec.ContentType},
		ExpiresAt: time.Date(2026, 5, 14, 9, 30, 0, 0, time.UTC).Add(ttl).Unix(),
	}, nil
}

func (s *as04FakeStorage) PresignGet(_ context.Context, bucket, objectKey string, ttl time.Duration) (*assetstorage.PresignedRequest, error) {
	return &assetstorage.PresignedRequest{
		Method:    "GET",
		URL:       "https://oss.example.test/access",
		ExpiresAt: time.Date(2026, 5, 14, 9, 30, 0, 0, time.UTC).Add(ttl).Unix(),
	}, nil
}

func (s *as04FakeStorage) HeadObject(_ context.Context, bucket, objectKey string) (*assetstorage.ObjectMeta, error) {
	meta, ok := s.headObjects[objectKey]
	if !ok {
		return nil, assetstorage.ErrObjectNotFound
	}
	return meta, nil
}
