package biz

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

func TestAssetVersionBiz_CreateVersionsAdvancesCurrentVersion(t *testing.T) {
	ctx := context.Background()
	store := newMemoryAssetStore()
	typeBiz := NewAssetTypeBiz(store, store)
	assetBiz := NewAssetBiz(store, store, store)
	versionBiz := NewAssetVersionBiz(store, store, store)
	userID := primitive.NewObjectID().Hex()
	asset := createVersionedAsset(t, ctx, typeBiz, assetBiz, userID)

	v1, err := versionBiz.Create(ctx, userID, asset.ID.Hex(), AssetVersionInput{
		Parts: map[string]assetmodel.AssetPartValue{
			"name": textPart("Hero"),
			"bio":  jsonPart(`{"level":1}`),
		},
		ChangeReason: "initial",
	})
	if err != nil {
		t.Fatalf("Create v1: %v", err)
	}
	v2, err := versionBiz.Create(ctx, userID, asset.ID.Hex(), AssetVersionInput{
		Parts: map[string]assetmodel.AssetPartValue{
			"name": textPart("Hero Prime"),
			"bio":  jsonPart(`{"level":2}`),
		},
		ChangeReason: "upgrade",
	})
	if err != nil {
		t.Fatalf("Create v2: %v", err)
	}
	if v1.Version != 1 || v2.Version != 2 {
		t.Fatalf("versions = (%d, %d), want (1, 2)", v1.Version, v2.Version)
	}
	gotAsset, err := assetBiz.Get(ctx, userID, asset.ID.Hex())
	if err != nil {
		t.Fatalf("Get asset after versions: %v", err)
	}
	if gotAsset.CurrentVersion != 2 {
		t.Fatalf("current_version = %d, want 2", gotAsset.CurrentVersion)
	}
}

func TestAssetVersionBiz_ValidatesPartsAgainstCurrentTypeSchema(t *testing.T) {
	ctx := context.Background()
	store := newMemoryAssetStore()
	typeBiz := NewAssetTypeBiz(store, store)
	assetBiz := NewAssetBiz(store, store, store)
	versionBiz := NewAssetVersionBiz(store, store, store)
	userID := primitive.NewObjectID().Hex()
	asset := createVersionedAsset(t, ctx, typeBiz, assetBiz, userID)

	tests := []struct {
		name  string
		parts map[string]assetmodel.AssetPartValue
	}{
		{
			name: "unknown part",
			parts: map[string]assetmodel.AssetPartValue{
				"name":    textPart("Hero"),
				"bio":     jsonPart(`{"ok":true}`),
				"unknown": textPart("nope"),
			},
		},
		{
			name: "missing required",
			parts: map[string]assetmodel.AssetPartValue{
				"bio": jsonPart(`{"ok":true}`),
			},
		},
		{
			name: "invalid json",
			parts: map[string]assetmodel.AssetPartValue{
				"name": textPart("Hero"),
				"bio":  jsonPart(`{"ok":`),
			},
		},
		{
			name: "invalid media id",
			parts: map[string]assetmodel.AssetPartValue{
				"name":    textPart("Hero"),
				"bio":     jsonPart(`{"ok":true}`),
				"gallery": mediaPart("not-an-object-id"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := versionBiz.Create(ctx, userID, asset.ID.Hex(), AssetVersionInput{Parts: tt.parts})
			if !errors.Is(err, errno.ErrAssetInvalidPart) {
				t.Fatalf("Create err = %v, want ErrAssetInvalidPart", err)
			}
		})
	}
}

func TestAssetVersionBiz_GetCurrentWhenUnsetReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	store := newMemoryAssetStore()
	typeBiz := NewAssetTypeBiz(store, store)
	assetBiz := NewAssetBiz(store, store, store)
	versionBiz := NewAssetVersionBiz(store, store, store)
	userID := primitive.NewObjectID().Hex()
	asset := createVersionedAsset(t, ctx, typeBiz, assetBiz, userID)

	_, err := versionBiz.GetCurrent(ctx, userID, asset.ID.Hex())
	if !errors.Is(err, errno.ErrAssetVersionNotFound) {
		t.Fatalf("GetCurrent err = %v, want ErrAssetVersionNotFound", err)
	}
}

func TestAssetVersionBiz_SetCurrentToOldVersionDoesNotCreateVersion(t *testing.T) {
	ctx := context.Background()
	store := newMemoryAssetStore()
	typeBiz := NewAssetTypeBiz(store, store)
	assetBiz := NewAssetBiz(store, store, store)
	versionBiz := NewAssetVersionBiz(store, store, store)
	userID := primitive.NewObjectID().Hex()
	asset := createVersionedAsset(t, ctx, typeBiz, assetBiz, userID)

	if _, err := versionBiz.Create(ctx, userID, asset.ID.Hex(), AssetVersionInput{Parts: validParts("Hero", 1)}); err != nil {
		t.Fatalf("Create v1: %v", err)
	}
	if _, err := versionBiz.Create(ctx, userID, asset.ID.Hex(), AssetVersionInput{Parts: validParts("Hero Prime", 2)}); err != nil {
		t.Fatalf("Create v2: %v", err)
	}
	if err := versionBiz.SetCurrent(ctx, userID, asset.ID.Hex(), 1); err != nil {
		t.Fatalf("SetCurrent to v1: %v", err)
	}
	items, total, err := versionBiz.List(ctx, userID, asset.ID.Hex(), PageInput{})
	if err != nil {
		t.Fatalf("List versions: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("versions after SetCurrent = (%d, %d items), want exactly 2", total, len(items))
	}
	current, err := versionBiz.GetCurrent(ctx, userID, asset.ID.Hex())
	if err != nil {
		t.Fatalf("GetCurrent: %v", err)
	}
	if current.Version != 1 {
		t.Fatalf("current version = %d, want 1", current.Version)
	}
}

func TestAssetVersionBiz_CopyOldVersionWithOverridesCreatesNewImmutableSnapshot(t *testing.T) {
	ctx := context.Background()
	store := newMemoryAssetStore()
	typeBiz := NewAssetTypeBiz(store, store)
	assetBiz := NewAssetBiz(store, store, store)
	versionBiz := NewAssetVersionBiz(store, store, store)
	userID := primitive.NewObjectID().Hex()
	asset := createVersionedAsset(t, ctx, typeBiz, assetBiz, userID)

	v1, err := versionBiz.Create(ctx, userID, asset.ID.Hex(), AssetVersionInput{Parts: validParts("Hero", 1)})
	if err != nil {
		t.Fatalf("Create v1: %v", err)
	}
	if _, err := versionBiz.Create(ctx, userID, asset.ID.Hex(), AssetVersionInput{Parts: validParts("Hero Prime", 2)}); err != nil {
		t.Fatalf("Create v2: %v", err)
	}
	v3, err := versionBiz.Copy(ctx, userID, asset.ID.Hex(), 1, AssetVersionCopyInput{
		PartOverrides: map[string]assetmodel.AssetPartValue{
			"name": textPart("Hero Copy"),
		},
		ChangeReason: "branch from v1",
	})
	if err != nil {
		t.Fatalf("Copy v1: %v", err)
	}
	if v3.Version != 3 {
		t.Fatalf("copied version = %d, want 3", v3.Version)
	}
	if got := v3.Parts["name"].Text; got != "Hero Copy" {
		t.Fatalf("copied name = %q, want override", got)
	}
	unchanged, err := versionBiz.Get(ctx, userID, asset.ID.Hex(), v1.Version)
	if err != nil {
		t.Fatalf("Get v1: %v", err)
	}
	if got := unchanged.Parts["name"].Text; got != "Hero" {
		t.Fatalf("v1 name = %q, want old snapshot unchanged", got)
	}
}

func TestAssetVersionBiz_ReadsAreIsolatedByWorkspace(t *testing.T) {
	ctx := context.Background()
	store := newMemoryAssetStore()
	typeBiz := NewAssetTypeBiz(store, store)
	assetBiz := NewAssetBiz(store, store, store)
	versionBiz := NewAssetVersionBiz(store, store, store)
	userA := primitive.NewObjectID().Hex()
	userB := primitive.NewObjectID().Hex()
	asset := createVersionedAsset(t, ctx, typeBiz, assetBiz, userA)

	if _, err := versionBiz.Create(ctx, userA, asset.ID.Hex(), AssetVersionInput{Parts: validParts("Hero", 1)}); err != nil {
		t.Fatalf("Create version: %v", err)
	}
	if _, err := versionBiz.Get(ctx, userB, asset.ID.Hex(), 1); !errors.Is(err, errno.ErrAssetNotFound) {
		t.Fatalf("cross-user Get err = %v, want ErrAssetNotFound", err)
	}
	if _, _, err := versionBiz.List(ctx, userB, asset.ID.Hex(), PageInput{}); !errors.Is(err, errno.ErrAssetNotFound) {
		t.Fatalf("cross-user List err = %v, want ErrAssetNotFound", err)
	}
}

func createVersionedAsset(t *testing.T, ctx context.Context, typeBiz *AssetTypeBiz, assetBiz *AssetBiz, userID string) *assetmodel.Asset {
	t.Helper()
	assetType, err := typeBiz.Create(ctx, userID, AssetTypeInput{
		Name: "Character",
		Code: "character",
		PartSchemas: []assetmodel.AssetPartSchema{
			{
				Key:               "name",
				Name:              "Name",
				AllowedValueKinds: []assetmodel.AssetValueKind{assetmodel.AssetValueKindText},
				Required:          true,
				SortOrder:         1,
			},
			{
				Key:               "bio",
				Name:              "Bio",
				AllowedValueKinds: []assetmodel.AssetValueKind{assetmodel.AssetValueKindJSON, assetmodel.AssetValueKindMixed},
				Required:          true,
				SortOrder:         2,
			},
			{
				Key:               "gallery",
				Name:              "Gallery",
				AllowedValueKinds: []assetmodel.AssetValueKind{assetmodel.AssetValueKindMedia, assetmodel.AssetValueKindMixed},
				SortOrder:         3,
			},
		},
	})
	if err != nil {
		t.Fatalf("Create asset type: %v", err)
	}
	asset, err := assetBiz.Create(ctx, userID, AssetInput{
		TypeID:         assetType.ID.Hex(),
		Name:           "Hero",
		SavedToLibrary: true,
	})
	if err != nil {
		t.Fatalf("Create asset: %v", err)
	}
	return asset
}

func validParts(name string, level int) map[string]assetmodel.AssetPartValue {
	return map[string]assetmodel.AssetPartValue{
		"name": textPart(name),
		"bio":  jsonPart(`{"level":` + string(rune('0'+level)) + `}`),
	}
}

func textPart(text string) assetmodel.AssetPartValue {
	return assetmodel.AssetPartValue{ValueKind: assetmodel.AssetValueKindText, Text: text}
}

func jsonPart(json string) assetmodel.AssetPartValue {
	return assetmodel.AssetPartValue{ValueKind: assetmodel.AssetValueKindJSON, JSON: json}
}

func mediaPart(ids ...string) assetmodel.AssetPartValue {
	out := make([]primitive.ObjectID, 0, len(ids))
	for _, id := range ids {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			out = append(out, primitive.NilObjectID)
			continue
		}
		out = append(out, oid)
	}
	return assetmodel.AssetPartValue{ValueKind: assetmodel.AssetValueKindMedia, MediaIDs: out}
}

func (s *memoryAssetStore) CreateAssetVersion(_ context.Context, doc *assetmodel.AssetVersion) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := cloneVersion(doc)
	cp.ID = id
	s.versions[id] = cp
	return id, nil
}

func (s *memoryAssetStore) FindAssetVersion(_ context.Context, assetID primitive.ObjectID, version int32) (*assetmodel.AssetVersion, error) {
	for _, doc := range s.versions {
		if doc.DeletedAt == nil && doc.AssetID == assetID && doc.Version == version {
			return cloneVersion(doc), nil
		}
	}
	return nil, errno.ErrAssetVersionNotFound
}

func (s *memoryAssetStore) ListAssetVersions(_ context.Context, assetID primitive.ObjectID, pageNum, pageSize int32) ([]*assetmodel.AssetVersion, int64, error) {
	out := make([]*assetmodel.AssetVersion, 0)
	for _, doc := range s.versions {
		if doc.DeletedAt == nil && doc.AssetID == assetID {
			out = append(out, cloneVersion(doc))
		}
	}
	return out, int64(len(out)), nil
}

func (s *memoryAssetStore) NextAssetVersionNumber(_ context.Context, assetID primitive.ObjectID) (int32, error) {
	var max int32
	for _, doc := range s.versions {
		if doc.DeletedAt == nil && doc.AssetID == assetID && doc.Version > max {
			max = doc.Version
		}
	}
	return max + 1, nil
}

func (s *memoryAssetStore) SetAssetCurrentVersion(_ context.Context, workspaceID string, assetID primitive.ObjectID, version int32) error {
	doc, ok := s.assets[assetID]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return errno.ErrAssetNotFound
	}
	doc.CurrentVersion = version
	return nil
}

func cloneVersion(in *assetmodel.AssetVersion) *assetmodel.AssetVersion {
	if in == nil {
		return nil
	}
	cp := *in
	cp.Parts = make(map[string]assetmodel.AssetPartValue, len(in.Parts))
	for key, value := range in.Parts {
		value.MediaIDs = append([]primitive.ObjectID(nil), value.MediaIDs...)
		cp.Parts[key] = value
	}
	return &cp
}
