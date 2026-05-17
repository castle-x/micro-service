package biz

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

func TestAssetTypeBiz_AllowsSchemaUpdateAfterAssetExists(t *testing.T) {
	ctx := context.Background()
	store := newMemoryAssetStore()
	typeBiz := NewAssetTypeBiz(store, store)
	assetBiz := NewAssetBiz(store, store, store)
	userID := primitive.NewObjectID().Hex()

	assetType, err := typeBiz.Create(ctx, userID, AssetTypeInput{
		Name: "Character",
		Code: "character",
		PartSchemas: []assetmodel.AssetPartSchema{{
			Key:               "face",
			Name:              "Face",
			AllowedValueKinds: []assetmodel.AssetValueKind{assetmodel.AssetValueKindMixed},
			SortOrder:         1,
		}},
	})
	if err != nil {
		t.Fatalf("Create asset type: %v", err)
	}
	if _, err := assetBiz.Create(ctx, userID, AssetInput{
		TypeID:         assetType.ID.Hex(),
		Name:           "Hero",
		SavedToLibrary: true,
	}); err != nil {
		t.Fatalf("Create asset: %v", err)
	}

	updated, err := typeBiz.Update(ctx, userID, assetType.ID.Hex(), AssetTypeUpdate{
		Name: "Character Updated",
		PartSchemas: []assetmodel.AssetPartSchema{{
			Key:               "body",
			Name:              "Body",
			AllowedValueKinds: []assetmodel.AssetValueKind{assetmodel.AssetValueKindText},
			SortOrder:         2,
		}},
	})
	if err != nil {
		t.Fatalf("Update asset type after use: %v", err)
	}
	if got := updated.PartSchemas[0].Key; got != "body" {
		t.Fatalf("schema key = %q, want body", got)
	}
}

func TestAssetTypeBiz_DuplicateCodeAndWorkspaceIsolation(t *testing.T) {
	ctx := context.Background()
	store := newMemoryAssetStore()
	typeBiz := NewAssetTypeBiz(store, store)
	userA := primitive.NewObjectID().Hex()
	userB := primitive.NewObjectID().Hex()

	if _, err := typeBiz.Create(ctx, userA, AssetTypeInput{Name: "Character", Code: "character"}); err != nil {
		t.Fatalf("Create userA type: %v", err)
	}
	if _, err := typeBiz.Create(ctx, userA, AssetTypeInput{Name: "Duplicate", Code: "character"}); !errors.Is(err, errno.ErrAssetConflict) {
		t.Fatalf("duplicate err = %v, want ErrAssetConflict", err)
	}
	if _, err := typeBiz.Create(ctx, userB, AssetTypeInput{Name: "Character", Code: "character"}); err != nil {
		t.Fatalf("same code in another workspace should succeed: %v", err)
	}

	items, total, err := typeBiz.List(ctx, userA, PageInput{})
	if err != nil {
		t.Fatalf("List userA types: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].WorkspaceID != userA {
		t.Fatalf("userA list = (%d, %#v), want one isolated item", total, items)
	}
}

func TestAssetCategoryBiz_DeleteConflictRules(t *testing.T) {
	ctx := context.Background()
	store := newMemoryAssetStore()
	typeBiz := NewAssetTypeBiz(store, store)
	categoryBiz := NewAssetCategoryBiz(store, store)
	assetBiz := NewAssetBiz(store, store, store)
	userID := primitive.NewObjectID().Hex()

	root, err := categoryBiz.Create(ctx, userID, AssetCategoryInput{Name: "Root", SortOrder: 1})
	if err != nil {
		t.Fatalf("Create root category: %v", err)
	}
	child, err := categoryBiz.Create(ctx, userID, AssetCategoryInput{Name: "Child", ParentID: root.ID.Hex(), SortOrder: 2})
	if err != nil {
		t.Fatalf("Create child category: %v", err)
	}
	if err := categoryBiz.Delete(ctx, userID, root.ID.Hex()); !errors.Is(err, errno.ErrAssetConflict) {
		t.Fatalf("delete category with child err = %v, want ErrAssetConflict", err)
	}
	if err := categoryBiz.Delete(ctx, userID, child.ID.Hex()); err != nil {
		t.Fatalf("Delete child category: %v", err)
	}

	assetType, err := typeBiz.Create(ctx, userID, AssetTypeInput{Name: "Character", Code: "character"})
	if err != nil {
		t.Fatalf("Create asset type: %v", err)
	}
	if _, err := assetBiz.Create(ctx, userID, AssetInput{
		TypeID:         assetType.ID.Hex(),
		Name:           "Hero",
		CategoryID:     root.ID.Hex(),
		SavedToLibrary: true,
	}); err != nil {
		t.Fatalf("Create asset in category: %v", err)
	}
	if err := categoryBiz.Delete(ctx, userID, root.ID.Hex()); !errors.Is(err, errno.ErrAssetConflict) {
		t.Fatalf("delete category with asset err = %v, want ErrAssetConflict", err)
	}
}

func TestAssetBiz_ListFiltersLibraryStateAndWorkspace(t *testing.T) {
	ctx := context.Background()
	store := newMemoryAssetStore()
	typeBiz := NewAssetTypeBiz(store, store)
	categoryBiz := NewAssetCategoryBiz(store, store)
	assetBiz := NewAssetBiz(store, store, store)
	userA := primitive.NewObjectID().Hex()
	userB := primitive.NewObjectID().Hex()

	assetType, err := typeBiz.Create(ctx, userA, AssetTypeInput{Name: "Character", Code: "character"})
	if err != nil {
		t.Fatalf("Create type: %v", err)
	}
	category, err := categoryBiz.Create(ctx, userA, AssetCategoryInput{Name: "Library", SortOrder: 1})
	if err != nil {
		t.Fatalf("Create category: %v", err)
	}
	saved, err := assetBiz.Create(ctx, userA, AssetInput{
		TypeID:         assetType.ID.Hex(),
		Name:           "Saved",
		CategoryID:     category.ID.Hex(),
		SavedToLibrary: true,
	})
	if err != nil {
		t.Fatalf("Create saved asset: %v", err)
	}
	if _, err := assetBiz.Create(ctx, userA, AssetInput{
		TypeID:         assetType.ID.Hex(),
		Name:           "History",
		SavedToLibrary: false,
	}); err != nil {
		t.Fatalf("Create history asset: %v", err)
	}
	otherType, err := typeBiz.Create(ctx, userB, AssetTypeInput{Name: "Character", Code: "character"})
	if err != nil {
		t.Fatalf("Create other type: %v", err)
	}
	if _, err := assetBiz.Create(ctx, userB, AssetInput{
		TypeID:         otherType.ID.Hex(),
		Name:           "Other",
		SavedToLibrary: true,
	}); err != nil {
		t.Fatalf("Create other asset: %v", err)
	}

	onlySaved := true
	items, total, err := assetBiz.List(ctx, userA, AssetListFilter{
		Page:           PageInput{},
		TypeID:         assetType.ID.Hex(),
		CategoryID:     category.ID.Hex(),
		SavedToLibrary: &onlySaved,
	})
	if err != nil {
		t.Fatalf("List saved assets: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != saved.ID {
		t.Fatalf("saved list = (%d, %#v), want saved asset only", total, items)
	}
	if _, err := assetBiz.Get(ctx, userB, saved.ID.Hex()); !errors.Is(err, errno.ErrAssetNotFound) {
		t.Fatalf("cross-user get err = %v, want ErrAssetNotFound", err)
	}

	updated, err := assetBiz.SetLibraryState(ctx, userA, saved.ID.Hex(), false)
	if err != nil {
		t.Fatalf("SetLibraryState: %v", err)
	}
	if updated.SavedToLibrary {
		t.Fatal("saved_to_library = true, want false")
	}
	if err := assetBiz.Delete(ctx, userA, saved.ID.Hex()); err != nil {
		t.Fatalf("Delete asset: %v", err)
	}
	if _, err := assetBiz.Get(ctx, userA, saved.ID.Hex()); !errors.Is(err, errno.ErrAssetNotFound) {
		t.Fatalf("get deleted err = %v, want ErrAssetNotFound", err)
	}
}

type memoryAssetStore struct {
	assetTypes map[primitive.ObjectID]*assetmodel.AssetType
	categories map[primitive.ObjectID]*assetmodel.AssetCategory
	assets     map[primitive.ObjectID]*assetmodel.Asset
	versions   map[primitive.ObjectID]*assetmodel.AssetVersion
}

func newMemoryAssetStore() *memoryAssetStore {
	return &memoryAssetStore{
		assetTypes: make(map[primitive.ObjectID]*assetmodel.AssetType),
		categories: make(map[primitive.ObjectID]*assetmodel.AssetCategory),
		assets:     make(map[primitive.ObjectID]*assetmodel.Asset),
		versions:   make(map[primitive.ObjectID]*assetmodel.AssetVersion),
	}
}

func (s *memoryAssetStore) CreateAssetType(_ context.Context, doc *assetmodel.AssetType) (primitive.ObjectID, error) {
	for _, existing := range s.assetTypes {
		if existing.DeletedAt == nil && existing.WorkspaceID == doc.WorkspaceID && existing.Code == doc.Code {
			return primitive.NilObjectID, errno.ErrAssetConflict
		}
	}
	id := primitive.NewObjectID()
	cp := *doc
	cp.ID = id
	s.assetTypes[id] = &cp
	return id, nil
}

func (s *memoryAssetStore) FindAssetTypeByID(_ context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.AssetType, error) {
	doc, ok := s.assetTypes[id]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return nil, errno.ErrAssetTypeNotFound
	}
	cp := *doc
	return &cp, nil
}

func (s *memoryAssetStore) ListAssetTypes(_ context.Context, workspaceID string, pageNum, pageSize int32) ([]*assetmodel.AssetType, int64, error) {
	out := make([]*assetmodel.AssetType, 0)
	for _, doc := range s.assetTypes {
		if doc.DeletedAt == nil && doc.WorkspaceID == workspaceID {
			cp := *doc
			out = append(out, &cp)
		}
	}
	return out, int64(len(out)), nil
}

func (s *memoryAssetStore) UpdateAssetType(_ context.Context, doc *assetmodel.AssetType) error {
	existing, ok := s.assetTypes[doc.ID]
	if !ok || existing.DeletedAt != nil || existing.WorkspaceID != doc.WorkspaceID {
		return errno.ErrAssetTypeNotFound
	}
	existing.Name = doc.Name
	existing.Description = doc.Description
	existing.PartSchemas = doc.PartSchemas
	return nil
}

func (s *memoryAssetStore) DeleteAssetType(_ context.Context, workspaceID string, id primitive.ObjectID) error {
	doc, ok := s.assetTypes[id]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return errno.ErrAssetTypeNotFound
	}
	delete(s.assetTypes, id)
	return nil
}

func (s *memoryAssetStore) CreateAssetCategory(_ context.Context, doc *assetmodel.AssetCategory) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := *doc
	cp.ID = id
	s.categories[id] = &cp
	return id, nil
}

func (s *memoryAssetStore) FindAssetCategoryByID(_ context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.AssetCategory, error) {
	doc, ok := s.categories[id]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return nil, errno.ErrAssetCategoryNotFound
	}
	cp := *doc
	return &cp, nil
}

func (s *memoryAssetStore) ListAssetCategories(_ context.Context, workspaceID string) ([]*assetmodel.AssetCategory, error) {
	out := make([]*assetmodel.AssetCategory, 0)
	for _, doc := range s.categories {
		if doc.DeletedAt == nil && doc.WorkspaceID == workspaceID {
			cp := *doc
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *memoryAssetStore) UpdateAssetCategory(_ context.Context, doc *assetmodel.AssetCategory) error {
	existing, ok := s.categories[doc.ID]
	if !ok || existing.DeletedAt != nil || existing.WorkspaceID != doc.WorkspaceID {
		return errno.ErrAssetCategoryNotFound
	}
	existing.Name = doc.Name
	existing.ParentID = doc.ParentID
	existing.SortOrder = doc.SortOrder
	return nil
}

func (s *memoryAssetStore) DeleteAssetCategory(_ context.Context, workspaceID string, id primitive.ObjectID) error {
	doc, ok := s.categories[id]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return errno.ErrAssetCategoryNotFound
	}
	delete(s.categories, id)
	return nil
}

func (s *memoryAssetStore) CountChildCategories(_ context.Context, workspaceID string, parentID primitive.ObjectID) (int64, error) {
	var count int64
	for _, doc := range s.categories {
		if doc.DeletedAt == nil && doc.WorkspaceID == workspaceID && doc.ParentID == parentID {
			count++
		}
	}
	return count, nil
}

func (s *memoryAssetStore) CreateAsset(_ context.Context, doc *assetmodel.Asset) (primitive.ObjectID, error) {
	id := primitive.NewObjectID()
	cp := *doc
	cp.ID = id
	s.assets[id] = &cp
	return id, nil
}

func (s *memoryAssetStore) FindAssetByID(_ context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.Asset, error) {
	doc, ok := s.assets[id]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return nil, errno.ErrAssetNotFound
	}
	cp := *doc
	return &cp, nil
}

func (s *memoryAssetStore) ListAssets(_ context.Context, workspaceID string, pageNum, pageSize int32, typeID, categoryID primitive.ObjectID, savedToLibrary *bool) ([]*assetmodel.Asset, int64, error) {
	out := make([]*assetmodel.Asset, 0)
	for _, doc := range s.assets {
		if doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
			continue
		}
		if !typeID.IsZero() && doc.TypeID != typeID {
			continue
		}
		if !categoryID.IsZero() && doc.CategoryID != categoryID {
			continue
		}
		if savedToLibrary != nil && doc.SavedToLibrary != *savedToLibrary {
			continue
		}
		cp := *doc
		out = append(out, &cp)
	}
	return out, int64(len(out)), nil
}

func (s *memoryAssetStore) UpdateAsset(_ context.Context, doc *assetmodel.Asset) error {
	existing, ok := s.assets[doc.ID]
	if !ok || existing.DeletedAt != nil || existing.WorkspaceID != doc.WorkspaceID {
		return errno.ErrAssetNotFound
	}
	existing.Name = doc.Name
	existing.Description = doc.Description
	existing.CategoryID = doc.CategoryID
	existing.CoverMediaID = doc.CoverMediaID
	existing.Source = doc.Source
	existing.Provenance = doc.Provenance
	return nil
}

func (s *memoryAssetStore) SetAssetLibraryState(_ context.Context, workspaceID string, id primitive.ObjectID, saved bool) error {
	doc, ok := s.assets[id]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return errno.ErrAssetNotFound
	}
	doc.SavedToLibrary = saved
	return nil
}

func (s *memoryAssetStore) DeleteAsset(_ context.Context, workspaceID string, id primitive.ObjectID) error {
	doc, ok := s.assets[id]
	if !ok || doc.DeletedAt != nil || doc.WorkspaceID != workspaceID {
		return errno.ErrAssetNotFound
	}
	deletedAt := int64(1)
	doc.DeletedAt = &deletedAt
	return nil
}

func (s *memoryAssetStore) CountAssetsByType(_ context.Context, workspaceID string, typeID primitive.ObjectID) (int64, error) {
	var count int64
	for _, doc := range s.assets {
		if doc.DeletedAt == nil && doc.WorkspaceID == workspaceID && doc.TypeID == typeID {
			count++
		}
	}
	return count, nil
}

func (s *memoryAssetStore) CountAssetsByCategory(_ context.Context, workspaceID string, categoryID primitive.ObjectID) (int64, error) {
	var count int64
	for _, doc := range s.assets {
		if doc.DeletedAt == nil && doc.WorkspaceID == workspaceID && doc.CategoryID == categoryID {
			count++
		}
	}
	return count, nil
}
