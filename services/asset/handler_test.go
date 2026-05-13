package main

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	assetbiz "github.com/castlexu/micro-service/services/asset/biz"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
	assetgen "github.com/castlexu/micro-service/services/asset/kitex_gen/asset"
	assetbase "github.com/castlexu/micro-service/services/asset/kitex_gen/base"
)

func TestAssetImpl_Health(t *testing.T) {
	resp, err := NewAssetImpl(assetbiz.NewHealthBiz(), nil, nil, nil).Health(context.Background(), &assetgen.HealthReq{
		Base: &assetbase.BaseReq{},
	})
	if err != nil {
		t.Fatalf("Health returned transport error: %v", err)
	}
	if resp.GetBase().GetCode() != 0 {
		t.Fatalf("code = %d, want 0", resp.GetBase().GetCode())
	}
	if resp.GetBase().GetMessage() != "ok" {
		t.Fatalf("message = %q, want ok", resp.GetBase().GetMessage())
	}
	if resp.GetService() != assetbiz.ServiceName {
		t.Fatalf("service = %q, want %q", resp.GetService(), assetbiz.ServiceName)
	}
	if resp.GetStatus() != assetbiz.HealthStatusOK {
		t.Fatalf("status = %q, want %q", resp.GetStatus(), assetbiz.HealthStatusOK)
	}
}

func TestAssetImpl_HealthNilRequest(t *testing.T) {
	resp, err := NewAssetImpl(assetbiz.NewHealthBiz(), nil, nil, nil).Health(context.Background(), nil)
	if err != nil {
		t.Fatalf("Health returned transport error: %v", err)
	}
	if resp.GetBase().GetCode() != errno.ErrInvalidParam.Code {
		t.Fatalf("code = %d, want %d", resp.GetBase().GetCode(), errno.ErrInvalidParam.Code)
	}
}

func TestAssetImpl_CreateAssetTypeRequiresUserID(t *testing.T) {
	store := &handlerAssetStore{}
	impl := NewAssetImpl(assetbiz.NewHealthBiz(), assetbiz.NewAssetTypeBiz(store, store), nil, nil)
	resp, err := impl.CreateAssetType(context.Background(), &assetgen.CreateAssetTypeReq{
		Base: &assetbase.BaseReq{},
		Name: "Character",
		Code: "character",
	})
	if err != nil {
		t.Fatalf("CreateAssetType returned transport error: %v", err)
	}
	if resp.GetBase().GetCode() != errno.ErrInvalidParam.Code {
		t.Fatalf("code = %d, want %d", resp.GetBase().GetCode(), errno.ErrInvalidParam.Code)
	}
}

func TestAssetImpl_CreateAssetTypeMapsDTO(t *testing.T) {
	store := &handlerAssetStore{}
	userID := primitive.NewObjectID().Hex()
	impl := NewAssetImpl(assetbiz.NewHealthBiz(), assetbiz.NewAssetTypeBiz(store, store), nil, nil)
	desc := "Playable character"
	resp, err := impl.CreateAssetType(context.Background(), &assetgen.CreateAssetTypeReq{
		Base:        &assetbase.BaseReq{UserID: &userID},
		Name:        "Character",
		Code:        "character",
		Description: &desc,
		PartSchemas: []*assetgen.AssetPartSchemaDTO{{
			Key:               "face",
			Name:              "Face",
			AllowedValueKinds: []assetgen.AssetValueKind{assetgen.AssetValueKind_MIXED},
			Multiple:          true,
			Required:          true,
			SortOrder:         1,
		}},
	})
	if err != nil {
		t.Fatalf("CreateAssetType returned transport error: %v", err)
	}
	if resp.GetBase().GetCode() != 0 {
		t.Fatalf("code = %d, want 0: %s", resp.GetBase().GetCode(), resp.GetBase().GetMessage())
	}
	got := resp.GetAssetType()
	if got.GetWorkspaceID() != userID {
		t.Fatalf("workspace_id = %q, want %q", got.GetWorkspaceID(), userID)
	}
	if got.GetCode() != "character" || got.GetDescription() != desc {
		t.Fatalf("asset type dto = %#v", got)
	}
	if len(got.GetPartSchemas()) != 1 || got.GetPartSchemas()[0].GetAllowedValueKinds()[0] != assetgen.AssetValueKind_MIXED {
		t.Fatalf("part schemas = %#v", got.GetPartSchemas())
	}
}

type handlerAssetStore struct {
	assetType *assetmodel.AssetType
	category  *assetmodel.AssetCategory
	asset     *assetmodel.Asset
	version   *assetmodel.AssetVersion
}

func (s *handlerAssetStore) CreateAssetType(_ context.Context, doc *assetmodel.AssetType) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := *doc
	cp.ID = id
	s.assetType = &cp
	return id, nil
}

func (s *handlerAssetStore) FindAssetTypeByID(_ context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.AssetType, error) {
	if s.assetType == nil || s.assetType.WorkspaceID != workspaceID || s.assetType.ID != id {
		return nil, errno.ErrAssetTypeNotFound
	}
	cp := *s.assetType
	return &cp, nil
}

func (s *handlerAssetStore) ListAssetTypes(_ context.Context, workspaceID string, pageNum, pageSize int32) ([]*assetmodel.AssetType, int64, error) {
	if s.assetType == nil || s.assetType.WorkspaceID != workspaceID {
		return nil, 0, nil
	}
	cp := *s.assetType
	return []*assetmodel.AssetType{&cp}, 1, nil
}

func (s *handlerAssetStore) UpdateAssetType(_ context.Context, doc *assetmodel.AssetType) error {
	s.assetType = doc
	return nil
}

func (s *handlerAssetStore) DeleteAssetType(_ context.Context, workspaceID string, id primitive.ObjectID) error {
	s.assetType = nil
	return nil
}

func (s *handlerAssetStore) CreateAssetCategory(_ context.Context, doc *assetmodel.AssetCategory) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := *doc
	cp.ID = id
	s.category = &cp
	return id, nil
}

func (s *handlerAssetStore) FindAssetCategoryByID(_ context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.AssetCategory, error) {
	if s.category == nil || s.category.WorkspaceID != workspaceID || s.category.ID != id {
		return nil, errno.ErrAssetCategoryNotFound
	}
	cp := *s.category
	return &cp, nil
}

func (s *handlerAssetStore) ListAssetCategories(_ context.Context, workspaceID string) ([]*assetmodel.AssetCategory, error) {
	if s.category == nil || s.category.WorkspaceID != workspaceID {
		return nil, nil
	}
	cp := *s.category
	return []*assetmodel.AssetCategory{&cp}, nil
}

func (s *handlerAssetStore) UpdateAssetCategory(_ context.Context, doc *assetmodel.AssetCategory) error {
	s.category = doc
	return nil
}

func (s *handlerAssetStore) DeleteAssetCategory(_ context.Context, workspaceID string, id primitive.ObjectID) error {
	s.category = nil
	return nil
}

func (s *handlerAssetStore) CreateAsset(_ context.Context, doc *assetmodel.Asset) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := *doc
	cp.ID = id
	s.asset = &cp
	return id, nil
}
func (s *handlerAssetStore) FindAssetByID(_ context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.Asset, error) {
	if s.asset == nil || s.asset.WorkspaceID != workspaceID || s.asset.ID != id {
		return nil, errno.ErrAssetNotFound
	}
	cp := *s.asset
	return &cp, nil
}
func (s *handlerAssetStore) ListAssets(_ context.Context, workspaceID string, pageNum, pageSize int32, typeID, categoryID primitive.ObjectID, savedToLibrary *bool) ([]*assetmodel.Asset, int64, error) {
	return nil, 0, nil
}
func (s *handlerAssetStore) UpdateAsset(_ context.Context, doc *assetmodel.Asset) error {
	s.asset = doc
	return nil
}
func (s *handlerAssetStore) SetAssetLibraryState(_ context.Context, workspaceID string, id primitive.ObjectID, saved bool) error {
	return errno.ErrNotImplemented
}
func (s *handlerAssetStore) DeleteAsset(_ context.Context, workspaceID string, id primitive.ObjectID) error {
	return errno.ErrNotImplemented
}
func (s *handlerAssetStore) CountAssetsByType(_ context.Context, workspaceID string, typeID primitive.ObjectID) (int64, error) {
	return 0, nil
}
func (s *handlerAssetStore) CountChildCategories(_ context.Context, workspaceID string, parentID primitive.ObjectID) (int64, error) {
	return 0, nil
}
func (s *handlerAssetStore) CountAssetsByCategory(_ context.Context, workspaceID string, categoryID primitive.ObjectID) (int64, error) {
	return 0, nil
}

func (s *handlerAssetStore) CreateAssetVersion(_ context.Context, doc *assetmodel.AssetVersion) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := *doc
	cp.ID = id
	cp.Parts = cloneHandlerParts(doc.Parts)
	s.version = &cp
	return id, nil
}

func (s *handlerAssetStore) FindAssetVersion(_ context.Context, assetID primitive.ObjectID, version int32) (*assetmodel.AssetVersion, error) {
	if s.version == nil || s.version.AssetID != assetID || s.version.Version != version {
		return nil, errno.ErrAssetVersionNotFound
	}
	cp := *s.version
	cp.Parts = cloneHandlerParts(s.version.Parts)
	return &cp, nil
}

func (s *handlerAssetStore) ListAssetVersions(_ context.Context, assetID primitive.ObjectID, pageNum, pageSize int32) ([]*assetmodel.AssetVersion, int64, error) {
	if s.version == nil || s.version.AssetID != assetID {
		return nil, 0, nil
	}
	cp := *s.version
	cp.Parts = cloneHandlerParts(s.version.Parts)
	return []*assetmodel.AssetVersion{&cp}, 1, nil
}

func (s *handlerAssetStore) NextAssetVersionNumber(_ context.Context, assetID primitive.ObjectID) (int32, error) {
	if s.version == nil || s.version.AssetID != assetID {
		return 1, nil
	}
	return s.version.Version + 1, nil
}

func (s *handlerAssetStore) SetAssetCurrentVersion(_ context.Context, workspaceID string, assetID primitive.ObjectID, version int32) error {
	if s.asset == nil || s.asset.WorkspaceID != workspaceID || s.asset.ID != assetID {
		return errno.ErrAssetNotFound
	}
	s.asset.CurrentVersion = version
	return nil
}

func cloneHandlerParts(in map[string]assetmodel.AssetPartValue) map[string]assetmodel.AssetPartValue {
	out := make(map[string]assetmodel.AssetPartValue, len(in))
	for key, value := range in {
		value.MediaIDs = append([]primitive.ObjectID(nil), value.MediaIDs...)
		out[key] = value
	}
	return out
}
