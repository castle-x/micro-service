package main

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	assetbiz "github.com/castlexu/micro-service/services/asset/biz"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
	assetgen "github.com/castlexu/micro-service/services/asset/kitex_gen/asset"
	assetbase "github.com/castlexu/micro-service/services/asset/kitex_gen/base"
	assetstorage "github.com/castlexu/micro-service/services/asset/storage"
)

func TestAssetImpl_MediaUploadSessionHandlersBindBaseAndMapDTOs(t *testing.T) {
	ctx := context.Background()
	store := newHandlerMediaStore()
	storage := newHandlerFakeStorage()
	mediaBiz := assetbiz.NewMediaBiz(store, store, storage, handlerMediaConfig())
	impl := &AssetImpl{mediaBiz: mediaBiz}
	userID := primitive.NewObjectID().Hex()
	sha := "declared-sha"

	createResp, err := impl.CreateStorageUploadSession(ctx, &assetgen.CreateStorageUploadSessionReq{
		Base:        &assetbase.BaseReq{UserID: &userID},
		ContentType: "image/png",
		Size:        128,
		Filename:    stringPtr("avatar.png"),
		SHA256:      &sha,
	})
	if err != nil {
		t.Fatalf("CreateStorageUploadSession returned transport error: %v", err)
	}
	if createResp.GetBase().GetCode() != 0 {
		t.Fatalf("create code = %d, want 0: %s", createResp.GetBase().GetCode(), createResp.GetBase().GetMessage())
	}
	if createResp.GetSession().GetWorkspaceID() != userID || createResp.GetSession().GetContentType() != "image/png" || createResp.GetSession().GetSize() != 128 {
		t.Fatalf("session DTO = %#v", createResp.GetSession())
	}
	if createResp.GetUpload().GetMethod() != "PUT" || createResp.GetUpload().GetHeaders()["Content-Type"] != "image/png" {
		t.Fatalf("upload DTO = %#v", createResp.GetUpload())
	}

	storage.headObjects[createResp.GetSession().GetObjectKey()] = &assetstorage.ObjectMeta{
		Bucket:      createResp.GetSession().GetBucket(),
		ObjectKey:   createResp.GetSession().GetObjectKey(),
		ContentType: "image/png",
		Size:        128,
	}
	width, height := int32(32), int32(16)
	finalizeResp, err := impl.FinalizeStorageUploadSession(ctx, &assetgen.FinalizeStorageUploadSessionReq{
		Base:      &assetbase.BaseReq{UserID: &userID},
		SessionID: createResp.GetSession().GetSessionID(),
		Width:     &width,
		Height:    &height,
	})
	if err != nil {
		t.Fatalf("FinalizeStorageUploadSession returned transport error: %v", err)
	}
	if finalizeResp.GetBase().GetCode() != 0 {
		t.Fatalf("finalize code = %d, want 0: %s", finalizeResp.GetBase().GetCode(), finalizeResp.GetBase().GetMessage())
	}
	if finalizeResp.GetSession().GetStatus() != assetgen.UploadSessionStatus_FINALIZED {
		t.Fatalf("session status = %v, want FINALIZED", finalizeResp.GetSession().GetStatus())
	}
	if finalizeResp.GetMedia().GetWidth() != width || finalizeResp.GetMedia().GetHeight() != height {
		t.Fatalf("media dimensions = (%d, %d), want (%d, %d)", finalizeResp.GetMedia().GetWidth(), finalizeResp.GetMedia().GetHeight(), width, height)
	}
}

func TestAssetImpl_MediaReadHandlersMapDTOs(t *testing.T) {
	ctx := context.Background()
	store := newHandlerMediaStore()
	storage := newHandlerFakeStorage()
	mediaBiz := assetbiz.NewMediaBiz(store, store, storage, handlerMediaConfig())
	impl := &AssetImpl{mediaBiz: mediaBiz}
	userID := primitive.NewObjectID().Hex()
	media := store.mustCreateMedia(userID, "assets-test/"+userID+"/uploads/2026/05/14/session/original.png")

	getResp, err := impl.GetMediaObject(ctx, &assetgen.GetMediaObjectReq{
		Base:    &assetbase.BaseReq{UserID: &userID},
		MediaID: media.ID.Hex(),
	})
	if err != nil {
		t.Fatalf("GetMediaObject returned transport error: %v", err)
	}
	if getResp.GetBase().GetCode() != 0 || getResp.GetMedia().GetMediaID() != media.ID.Hex() {
		t.Fatalf("get response = %#v", getResp)
	}

	listResp, err := impl.ListMediaObjects(ctx, &assetgen.ListMediaObjectsReq{
		Base:        &assetbase.BaseReq{UserID: &userID},
		Page:        &assetbase.PageReq{PageNum: 0, PageSize: 999},
		Source:      assetgen.AssetSourcePtr(assetgen.AssetSource_UPLOAD),
		ContentType: stringPtr("image/png"),
	})
	if err != nil {
		t.Fatalf("ListMediaObjects returned transport error: %v", err)
	}
	if listResp.GetBase().GetCode() != 0 || len(listResp.GetMedia()) != 1 || listResp.GetPage().GetPageNum() != 1 || listResp.GetPage().GetPageSize() != 20 {
		t.Fatalf("list response = %#v", listResp)
	}

	accessResp, err := impl.GetMediaObjectAccessURL(ctx, &assetgen.GetMediaObjectAccessURLReq{
		Base:             &assetbase.BaseReq{UserID: &userID},
		MediaID:          media.ID.Hex(),
		ExpiresInSeconds: int32Ptr(60),
	})
	if err != nil {
		t.Fatalf("GetMediaObjectAccessURL returned transport error: %v", err)
	}
	if accessResp.GetBase().GetCode() != 0 || accessResp.GetAccess().GetMethod() != "GET" || accessResp.GetMedia().GetMediaID() != media.ID.Hex() {
		t.Fatalf("access response = %#v", accessResp)
	}
}

func TestAssetImpl_MediaHandlersRejectNilRequestAndMissingUser(t *testing.T) {
	impl := &AssetImpl{mediaBiz: assetbiz.NewMediaBiz(newHandlerMediaStore(), newHandlerMediaStore(), newHandlerFakeStorage(), handlerMediaConfig())}

	createResp, err := impl.CreateStorageUploadSession(context.Background(), nil)
	if err != nil {
		t.Fatalf("CreateStorageUploadSession returned transport error: %v", err)
	}
	if createResp.GetBase().GetCode() != errno.ErrInvalidParam.Code {
		t.Fatalf("nil request code = %d, want ErrInvalidParam", createResp.GetBase().GetCode())
	}

	getResp, err := impl.GetMediaObject(context.Background(), &assetgen.GetMediaObjectReq{
		Base:    &assetbase.BaseReq{},
		MediaID: primitive.NewObjectID().Hex(),
	})
	if err != nil {
		t.Fatalf("GetMediaObject returned transport error: %v", err)
	}
	if getResp.GetBase().GetCode() != errno.ErrInvalidParam.Code {
		t.Fatalf("missing user code = %d, want ErrInvalidParam", getResp.GetBase().GetCode())
	}
}

type handlerMediaStore struct {
	media    map[primitive.ObjectID]*assetmodel.MediaObject
	sessions map[primitive.ObjectID]*assetmodel.StorageUploadSession
}

func newHandlerMediaStore() *handlerMediaStore {
	return &handlerMediaStore{
		media:    make(map[primitive.ObjectID]*assetmodel.MediaObject),
		sessions: make(map[primitive.ObjectID]*assetmodel.StorageUploadSession),
	}
}

func handlerMediaConfig() assetbiz.MediaConfig {
	return assetbiz.MediaConfig{
		ObjectKeyPrefix:     "assets-test",
		Bucket:              "asset-test-bucket",
		UploadURLTTL:        15 * time.Minute,
		DownloadURLTTL:      15 * time.Minute,
		MaxUploadSizeBytes:  20 << 20,
		AllowedContentTypes: []string{"image/png"},
		Now:                 func() time.Time { return time.Date(2026, 5, 14, 9, 30, 0, 0, time.UTC) },
	}
}

type handlerFakeStorage struct {
	headObjects map[string]*assetstorage.ObjectMeta
}

func newHandlerFakeStorage() *handlerFakeStorage {
	return &handlerFakeStorage{headObjects: make(map[string]*assetstorage.ObjectMeta)}
}

func (s *handlerFakeStorage) Provider() assetmodel.StorageProvider {
	return assetmodel.StorageProviderAliyunOSS
}

func (s *handlerFakeStorage) Bucket() string {
	return "asset-test-bucket"
}

func (s *handlerFakeStorage) PresignPut(_ context.Context, spec assetstorage.ObjectSpec, ttl time.Duration) (*assetstorage.PresignedRequest, error) {
	return &assetstorage.PresignedRequest{
		Method:    "PUT",
		URL:       "https://oss.example.test/upload",
		Headers:   map[string]string{"Content-Type": spec.ContentType},
		ExpiresAt: time.Date(2026, 5, 14, 9, 30, 0, 0, time.UTC).Add(ttl).Unix(),
	}, nil
}

func (s *handlerFakeStorage) PresignGet(_ context.Context, bucket, objectKey string, ttl time.Duration) (*assetstorage.PresignedRequest, error) {
	return &assetstorage.PresignedRequest{
		Method:    "GET",
		URL:       "https://oss.example.test/access",
		ExpiresAt: time.Date(2026, 5, 14, 9, 30, 0, 0, time.UTC).Add(ttl).Unix(),
	}, nil
}

func (s *handlerFakeStorage) HeadObject(_ context.Context, bucket, objectKey string) (*assetstorage.ObjectMeta, error) {
	meta, ok := s.headObjects[objectKey]
	if !ok {
		return nil, assetstorage.ErrObjectNotFound
	}
	return meta, nil
}

func (s *handlerMediaStore) CreateMediaObject(_ context.Context, doc *assetmodel.MediaObject) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := *doc
	cp.ID = id
	s.media[id] = &cp
	return id, nil
}

func (s *handlerMediaStore) FindMediaObjectByID(_ context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.MediaObject, error) {
	doc, ok := s.media[id]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return nil, errno.ErrMediaObjectNotFound
	}
	cp := *doc
	return &cp, nil
}

func (s *handlerMediaStore) FindMediaObjectByObjectKey(_ context.Context, provider assetmodel.StorageProvider, bucket, objectKey string) (*assetmodel.MediaObject, error) {
	for _, doc := range s.media {
		if doc.DeletedAt == nil && doc.Provider == provider && doc.Bucket == bucket && doc.ObjectKey == objectKey {
			cp := *doc
			return &cp, nil
		}
	}
	return nil, errno.ErrMediaObjectNotFound
}

func (s *handlerMediaStore) ListMediaObjects(_ context.Context, workspaceID string, pageNum, pageSize int32, source assetmodel.AssetSource, contentType string) ([]*assetmodel.MediaObject, int64, error) {
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

func (s *handlerMediaStore) CreateStorageUploadSession(_ context.Context, doc *assetmodel.StorageUploadSession) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := *doc
	cp.ID = id
	s.sessions[id] = &cp
	return id, nil
}

func (s *handlerMediaStore) FindStorageUploadSessionByID(_ context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.StorageUploadSession, error) {
	doc, ok := s.sessions[id]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return nil, errno.ErrAssetUploadSessionNotFound
	}
	cp := *doc
	return &cp, nil
}

func (s *handlerMediaStore) UpdateStorageUploadSession(_ context.Context, doc *assetmodel.StorageUploadSession) error {
	existing, ok := s.sessions[doc.ID]
	if !ok || existing.DeletedAt != nil || existing.WorkspaceID != doc.WorkspaceID {
		return errno.ErrAssetUploadSessionNotFound
	}
	cp := *doc
	s.sessions[doc.ID] = &cp
	return nil
}

func (s *handlerMediaStore) SetUploadSessionFinalized(_ context.Context, workspaceID string, sessionID, mediaID primitive.ObjectID, finalizedAt int64) error {
	doc, ok := s.sessions[sessionID]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return errno.ErrAssetUploadSessionNotFound
	}
	doc.Status = assetmodel.UploadSessionStatusFinalized
	doc.MediaID = mediaID
	doc.FinalizedAt = finalizedAt
	return nil
}

func (s *handlerMediaStore) SetUploadSessionExpired(_ context.Context, workspaceID string, sessionID primitive.ObjectID) error {
	doc, ok := s.sessions[sessionID]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return errno.ErrAssetUploadSessionNotFound
	}
	doc.Status = assetmodel.UploadSessionStatusExpired
	return nil
}

func (s *handlerMediaStore) mustCreateMedia(workspaceID, objectKey string) *assetmodel.MediaObject {
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

func stringPtr(s string) *string {
	return &s
}

func int32Ptr(v int32) *int32 {
	return &v
}
