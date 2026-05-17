package biz

import (
	"context"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

const maxPageSize = 100

// PageInput 是 AS-02 CRUD 列表接口的分页输入。
type PageInput struct {
	PageNum  int32
	PageSize int32
}

// AssetTypeInput 是创建资产类型的业务输入。
type AssetTypeInput struct {
	Name        string
	Code        string
	Description string
	PartSchemas []assetmodel.AssetPartSchema
}

// AssetTypeUpdate 是更新资产类型的业务输入。Code 创建后不可变。
type AssetTypeUpdate struct {
	Name        string
	Description string
	PartSchemas []assetmodel.AssetPartSchema
}

// AssetCategoryInput 是创建或更新分类的业务输入。
type AssetCategoryInput struct {
	Name      string
	ParentID  string
	SortOrder int32
}

// AssetInput 是创建资产实例的业务输入。
type AssetInput struct {
	TypeID         string
	Name           string
	Description    string
	SavedToLibrary bool
	CategoryID     string
	CoverMediaID   string
	Source         assetmodel.AssetSource
	Provenance     *assetmodel.Provenance
}

// AssetUpdate 是更新资产实例的业务输入。
type AssetUpdate struct {
	Name         string
	Description  string
	CategoryID   *string
	CoverMediaID *string
	Source       *assetmodel.AssetSource
	Provenance   *assetmodel.Provenance
}

// AssetListFilter 是资产实例列表筛选条件。
type AssetListFilter struct {
	Page           PageInput
	TypeID         string
	CategoryID     string
	SavedToLibrary *bool
}

type AssetTypeRepository interface {
	CreateAssetType(ctx context.Context, doc *assetmodel.AssetType) (primitive.ObjectID, error)
	FindAssetTypeByID(ctx context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.AssetType, error)
	ListAssetTypes(ctx context.Context, workspaceID string, pageNum, pageSize int32) ([]*assetmodel.AssetType, int64, error)
	UpdateAssetType(ctx context.Context, doc *assetmodel.AssetType) error
	DeleteAssetType(ctx context.Context, workspaceID string, id primitive.ObjectID) error
}

type AssetCategoryRepository interface {
	CreateAssetCategory(ctx context.Context, doc *assetmodel.AssetCategory) (primitive.ObjectID, error)
	FindAssetCategoryByID(ctx context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.AssetCategory, error)
	ListAssetCategories(ctx context.Context, workspaceID string) ([]*assetmodel.AssetCategory, error)
	UpdateAssetCategory(ctx context.Context, doc *assetmodel.AssetCategory) error
	DeleteAssetCategory(ctx context.Context, workspaceID string, id primitive.ObjectID) error
	CountChildCategories(ctx context.Context, workspaceID string, parentID primitive.ObjectID) (int64, error)
}

type AssetRepository interface {
	CreateAsset(ctx context.Context, doc *assetmodel.Asset) (primitive.ObjectID, error)
	FindAssetByID(ctx context.Context, workspaceID string, id primitive.ObjectID) (*assetmodel.Asset, error)
	ListAssets(ctx context.Context, workspaceID string, pageNum, pageSize int32, typeID, categoryID primitive.ObjectID, savedToLibrary *bool) ([]*assetmodel.Asset, int64, error)
	UpdateAsset(ctx context.Context, doc *assetmodel.Asset) error
	SetAssetLibraryState(ctx context.Context, workspaceID string, id primitive.ObjectID, saved bool) error
	SetAssetCurrentVersion(ctx context.Context, workspaceID string, id primitive.ObjectID, version int32) error
	DeleteAsset(ctx context.Context, workspaceID string, id primitive.ObjectID) error
	CountAssetsByType(ctx context.Context, workspaceID string, typeID primitive.ObjectID) (int64, error)
	CountAssetsByCategory(ctx context.Context, workspaceID string, categoryID primitive.ObjectID) (int64, error)
}

// AssetTypeBiz 处理资产类型与 part schema 的业务规则。
type AssetTypeBiz struct {
	typeRepo  AssetTypeRepository
	assetRepo AssetRepository
}

func NewAssetTypeBiz(typeRepo AssetTypeRepository, assetRepo AssetRepository) *AssetTypeBiz {
	return &AssetTypeBiz{typeRepo: typeRepo, assetRepo: assetRepo}
}

func (b *AssetTypeBiz) Create(ctx context.Context, userID string, input AssetTypeInput) (*assetmodel.AssetType, error) {
	workspaceID, createdBy, err := workspaceFromUser(userID)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	code := strings.TrimSpace(input.Code)
	if name == "" {
		return nil, errno.ErrInvalidParam.WithMessage("asset: name is required")
	}
	if code == "" {
		return nil, errno.ErrInvalidParam.WithMessage("asset: code is required")
	}
	doc := &assetmodel.AssetType{
		WorkspaceID: workspaceID,
		Name:        name,
		Code:        code,
		Description: input.Description,
		PartSchemas: normalizePartSchemas(input.PartSchemas),
		CreatedBy:   createdBy,
	}
	id, err := b.typeRepo.CreateAssetType(ctx, doc)
	if err != nil {
		return nil, mapAssetRepoErr(err)
	}
	doc.ID = id
	return doc, nil
}

func (b *AssetTypeBiz) Update(ctx context.Context, userID, assetTypeID string, input AssetTypeUpdate) (*assetmodel.AssetType, error) {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, err
	}
	id, err := parseObjectID(assetTypeID, "asset_type_id")
	if err != nil {
		return nil, err
	}
	existing, err := b.typeRepo.FindAssetTypeByID(ctx, workspaceID, id)
	if err != nil {
		return nil, mapAssetRepoErr(err)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errno.ErrInvalidParam.WithMessage("asset: name is required")
	}
	existing.Name = name
	existing.Description = input.Description
	existing.PartSchemas = normalizePartSchemas(input.PartSchemas)
	if err := b.typeRepo.UpdateAssetType(ctx, existing); err != nil {
		return nil, mapAssetRepoErr(err)
	}
	return b.typeRepo.FindAssetTypeByID(ctx, workspaceID, id)
}

func (b *AssetTypeBiz) Get(ctx context.Context, userID, assetTypeID string) (*assetmodel.AssetType, error) {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, err
	}
	id, err := parseObjectID(assetTypeID, "asset_type_id")
	if err != nil {
		return nil, err
	}
	return b.typeRepo.FindAssetTypeByID(ctx, workspaceID, id)
}

func (b *AssetTypeBiz) List(ctx context.Context, userID string, page PageInput) ([]*assetmodel.AssetType, int64, error) {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, 0, err
	}
	page = normalizePage(page)
	return b.typeRepo.ListAssetTypes(ctx, workspaceID, page.PageNum, page.PageSize)
}

func (b *AssetTypeBiz) Delete(ctx context.Context, userID, assetTypeID string) error {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return err
	}
	id, err := parseObjectID(assetTypeID, "asset_type_id")
	if err != nil {
		return err
	}
	if _, err := b.typeRepo.FindAssetTypeByID(ctx, workspaceID, id); err != nil {
		return mapAssetRepoErr(err)
	}
	count, err := b.assetRepo.CountAssetsByType(ctx, workspaceID, id)
	if err != nil {
		return mapAssetRepoErr(err)
	}
	if count > 0 {
		return errno.ErrAssetConflict.WithMessage("asset: asset type is in use")
	}
	return mapAssetRepoErr(b.typeRepo.DeleteAssetType(ctx, workspaceID, id))
}

// AssetCategoryBiz 处理个人资产库分类。
type AssetCategoryBiz struct {
	categoryRepo AssetCategoryRepository
	assetRepo    AssetRepository
}

func NewAssetCategoryBiz(categoryRepo AssetCategoryRepository, assetRepo AssetRepository) *AssetCategoryBiz {
	return &AssetCategoryBiz{categoryRepo: categoryRepo, assetRepo: assetRepo}
}

func (b *AssetCategoryBiz) Create(ctx context.Context, userID string, input AssetCategoryInput) (*assetmodel.AssetCategory, error) {
	workspaceID, createdBy, err := workspaceFromUser(userID)
	if err != nil {
		return nil, err
	}
	parentID, err := b.parseOptionalParent(ctx, workspaceID, input.ParentID, primitive.NilObjectID)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errno.ErrInvalidParam.WithMessage("asset: category name is required")
	}
	doc := &assetmodel.AssetCategory{
		WorkspaceID: workspaceID,
		Name:        name,
		ParentID:    parentID,
		SortOrder:   input.SortOrder,
		CreatedBy:   createdBy,
	}
	id, err := b.categoryRepo.CreateAssetCategory(ctx, doc)
	if err != nil {
		return nil, mapAssetRepoErr(err)
	}
	doc.ID = id
	return doc, nil
}

func (b *AssetCategoryBiz) Update(ctx context.Context, userID, categoryID string, input AssetCategoryInput) (*assetmodel.AssetCategory, error) {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, err
	}
	id, err := parseObjectID(categoryID, "category_id")
	if err != nil {
		return nil, err
	}
	existing, err := b.categoryRepo.FindAssetCategoryByID(ctx, workspaceID, id)
	if err != nil {
		return nil, mapAssetRepoErr(err)
	}
	parentID, err := b.parseOptionalParent(ctx, workspaceID, input.ParentID, id)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errno.ErrInvalidParam.WithMessage("asset: category name is required")
	}
	existing.Name = name
	existing.ParentID = parentID
	existing.SortOrder = input.SortOrder
	if err := b.categoryRepo.UpdateAssetCategory(ctx, existing); err != nil {
		return nil, mapAssetRepoErr(err)
	}
	return b.categoryRepo.FindAssetCategoryByID(ctx, workspaceID, id)
}

func (b *AssetCategoryBiz) List(ctx context.Context, userID string) ([]*assetmodel.AssetCategory, error) {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, err
	}
	return b.categoryRepo.ListAssetCategories(ctx, workspaceID)
}

func (b *AssetCategoryBiz) Delete(ctx context.Context, userID, categoryID string) error {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return err
	}
	id, err := parseObjectID(categoryID, "category_id")
	if err != nil {
		return err
	}
	if _, err := b.categoryRepo.FindAssetCategoryByID(ctx, workspaceID, id); err != nil {
		return mapAssetRepoErr(err)
	}
	children, err := b.categoryRepo.CountChildCategories(ctx, workspaceID, id)
	if err != nil {
		return mapAssetRepoErr(err)
	}
	if children > 0 {
		return errno.ErrAssetConflict.WithMessage("asset: category has child categories")
	}
	assets, err := b.assetRepo.CountAssetsByCategory(ctx, workspaceID, id)
	if err != nil {
		return mapAssetRepoErr(err)
	}
	if assets > 0 {
		return errno.ErrAssetConflict.WithMessage("asset: category has assets")
	}
	return mapAssetRepoErr(b.categoryRepo.DeleteAssetCategory(ctx, workspaceID, id))
}

func (b *AssetCategoryBiz) parseOptionalParent(ctx context.Context, workspaceID, raw string, self primitive.ObjectID) (primitive.ObjectID, error) {
	if strings.TrimSpace(raw) == "" {
		return primitive.NilObjectID, nil
	}
	parentID, err := parseObjectID(raw, "parent_id")
	if err != nil {
		return primitive.NilObjectID, err
	}
	if parentID == self {
		return primitive.NilObjectID, errno.ErrAssetConflict.WithMessage("asset: category parent cannot be itself")
	}
	if _, err := b.categoryRepo.FindAssetCategoryByID(ctx, workspaceID, parentID); err != nil {
		return primitive.NilObjectID, mapAssetRepoErr(err)
	}
	return parentID, nil
}

// AssetBiz 处理资产实例 CRUD。
type AssetBiz struct {
	assetRepo    AssetRepository
	typeRepo     AssetTypeRepository
	categoryRepo AssetCategoryRepository
	mediaRepo    MediaObjectRepository
}

func NewAssetBiz(assetRepo AssetRepository, typeRepo AssetTypeRepository, categoryRepo AssetCategoryRepository, mediaRepo ...MediaObjectRepository) *AssetBiz {
	var mr MediaObjectRepository
	if len(mediaRepo) > 0 {
		mr = mediaRepo[0]
	}
	return &AssetBiz{assetRepo: assetRepo, typeRepo: typeRepo, categoryRepo: categoryRepo, mediaRepo: mr}
}

func (b *AssetBiz) Create(ctx context.Context, userID string, input AssetInput) (*assetmodel.Asset, error) {
	workspaceID, createdBy, err := workspaceFromUser(userID)
	if err != nil {
		return nil, err
	}
	typeID, err := parseObjectID(input.TypeID, "type_id")
	if err != nil {
		return nil, err
	}
	if _, err := b.typeRepo.FindAssetTypeByID(ctx, workspaceID, typeID); err != nil {
		return nil, mapAssetRepoErr(err)
	}
	categoryID, err := b.parseOptionalCategory(ctx, workspaceID, input.CategoryID)
	if err != nil {
		return nil, err
	}
	coverMediaID, err := parseOptionalObjectID(input.CoverMediaID, "cover_media_id")
	if err != nil {
		return nil, err
	}
	if err := b.validateOptionalCoverMedia(ctx, workspaceID, coverMediaID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errno.ErrInvalidParam.WithMessage("asset: name is required")
	}
	doc := &assetmodel.Asset{
		WorkspaceID:    workspaceID,
		TypeID:         typeID,
		Name:           name,
		Description:    input.Description,
		SavedToLibrary: input.SavedToLibrary,
		CategoryID:     categoryID,
		CurrentVersion: 0,
		CoverMediaID:   coverMediaID,
		Source:         input.Source,
		Provenance:     input.Provenance,
		CreatedBy:      createdBy,
	}
	id, err := b.assetRepo.CreateAsset(ctx, doc)
	if err != nil {
		return nil, mapAssetRepoErr(err)
	}
	doc.ID = id
	return doc, nil
}

func (b *AssetBiz) Update(ctx context.Context, userID, assetID string, input AssetUpdate) (*assetmodel.Asset, error) {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, err
	}
	id, err := parseObjectID(assetID, "asset_id")
	if err != nil {
		return nil, err
	}
	existing, err := b.assetRepo.FindAssetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, mapAssetRepoErr(err)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errno.ErrInvalidParam.WithMessage("asset: name is required")
	}
	existing.Name = name
	existing.Description = input.Description
	if input.CategoryID != nil {
		categoryID, err := b.parseOptionalCategory(ctx, workspaceID, *input.CategoryID)
		if err != nil {
			return nil, err
		}
		existing.CategoryID = categoryID
	}
	if input.CoverMediaID != nil {
		coverMediaID, err := parseOptionalObjectID(*input.CoverMediaID, "cover_media_id")
		if err != nil {
			return nil, err
		}
		if err := b.validateOptionalCoverMedia(ctx, workspaceID, coverMediaID); err != nil {
			return nil, err
		}
		existing.CoverMediaID = coverMediaID
	}
	if input.Source != nil {
		existing.Source = *input.Source
	}
	existing.Provenance = input.Provenance
	if err := b.assetRepo.UpdateAsset(ctx, existing); err != nil {
		return nil, mapAssetRepoErr(err)
	}
	return b.assetRepo.FindAssetByID(ctx, workspaceID, id)
}

func (b *AssetBiz) Get(ctx context.Context, userID, assetID string) (*assetmodel.Asset, error) {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, err
	}
	id, err := parseObjectID(assetID, "asset_id")
	if err != nil {
		return nil, err
	}
	return b.assetRepo.FindAssetByID(ctx, workspaceID, id)
}

func (b *AssetBiz) List(ctx context.Context, userID string, filter AssetListFilter) ([]*assetmodel.Asset, int64, error) {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, 0, err
	}
	filter.Page = normalizePage(filter.Page)
	typeID := primitive.NilObjectID
	if filter.TypeID != "" {
		typeID, err = parseObjectID(filter.TypeID, "type_id")
		if err != nil {
			return nil, 0, err
		}
		if _, err := b.typeRepo.FindAssetTypeByID(ctx, workspaceID, typeID); err != nil {
			return nil, 0, mapAssetRepoErr(err)
		}
	}
	if filter.CategoryID != "" {
		categoryID, err := b.parseOptionalCategory(ctx, workspaceID, filter.CategoryID)
		if err != nil {
			return nil, 0, err
		}
		return b.assetRepo.ListAssets(ctx, workspaceID, filter.Page.PageNum, filter.Page.PageSize, typeID, categoryID, filter.SavedToLibrary)
	}
	return b.assetRepo.ListAssets(ctx, workspaceID, filter.Page.PageNum, filter.Page.PageSize, typeID, primitive.NilObjectID, filter.SavedToLibrary)
}

func (b *AssetBiz) SetLibraryState(ctx context.Context, userID, assetID string, saved bool) (*assetmodel.Asset, error) {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return nil, err
	}
	id, err := parseObjectID(assetID, "asset_id")
	if err != nil {
		return nil, err
	}
	if err := b.assetRepo.SetAssetLibraryState(ctx, workspaceID, id, saved); err != nil {
		return nil, mapAssetRepoErr(err)
	}
	return b.assetRepo.FindAssetByID(ctx, workspaceID, id)
}

func (b *AssetBiz) Delete(ctx context.Context, userID, assetID string) error {
	workspaceID, _, err := workspaceFromUser(userID)
	if err != nil {
		return err
	}
	id, err := parseObjectID(assetID, "asset_id")
	if err != nil {
		return err
	}
	return mapAssetRepoErr(b.assetRepo.DeleteAsset(ctx, workspaceID, id))
}

func (b *AssetBiz) parseOptionalCategory(ctx context.Context, workspaceID, raw string) (primitive.ObjectID, error) {
	categoryID, err := parseOptionalObjectID(raw, "category_id")
	if err != nil || categoryID.IsZero() {
		return categoryID, err
	}
	if _, err := b.categoryRepo.FindAssetCategoryByID(ctx, workspaceID, categoryID); err != nil {
		return primitive.NilObjectID, mapAssetRepoErr(err)
	}
	return categoryID, nil
}

func (b *AssetBiz) validateOptionalCoverMedia(ctx context.Context, workspaceID string, mediaID primitive.ObjectID) error {
	if mediaID.IsZero() || b.mediaRepo == nil {
		return nil
	}
	if _, err := b.mediaRepo.FindMediaObjectByID(ctx, workspaceID, mediaID); err != nil {
		return mapAssetRepoErr(err)
	}
	return nil
}

func workspaceFromUser(userID string) (workspaceID string, createdBy primitive.ObjectID, err error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", primitive.NilObjectID, errno.ErrInvalidParam.WithMessage("asset: base.user_id is required")
	}
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return "", primitive.NilObjectID, errno.ErrInvalidParam.WithMessagef("asset: invalid user_id %q", userID)
	}
	return userID, oid, nil
}

func parseObjectID(raw string, field string) (primitive.ObjectID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return primitive.NilObjectID, errno.ErrInvalidParam.WithMessagef("asset: %s is required", field)
	}
	id, err := primitive.ObjectIDFromHex(raw)
	if err != nil {
		return primitive.NilObjectID, errno.ErrInvalidParam.WithMessagef("asset: invalid %s %q", field, raw)
	}
	return id, nil
}

func parseOptionalObjectID(raw string, field string) (primitive.ObjectID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return primitive.NilObjectID, nil
	}
	return parseObjectID(raw, field)
}

func normalizePage(page PageInput) PageInput {
	if page.PageNum < 1 {
		page.PageNum = 1
	}
	if page.PageSize < 1 || page.PageSize > maxPageSize {
		page.PageSize = 20
	}
	return page
}

func normalizePartSchemas(in []assetmodel.AssetPartSchema) []assetmodel.AssetPartSchema {
	out := make([]assetmodel.AssetPartSchema, 0, len(in))
	for _, schema := range in {
		schema.Key = strings.TrimSpace(schema.Key)
		schema.Name = strings.TrimSpace(schema.Name)
		out = append(out, schema)
	}
	return out
}

func mapAssetRepoErr(err error) error {
	if err == nil {
		return nil
	}
	for _, target := range []error{
		errno.ErrAssetTypeNotFound,
		errno.ErrAssetCategoryNotFound,
		errno.ErrAssetNotFound,
		errno.ErrMediaObjectNotFound,
		errno.ErrAssetConflict,
		errno.ErrInvalidParam,
	} {
		if errors.Is(err, target) {
			return err
		}
	}
	return errno.ErrInternal.WithMessagef("asset: repository error: %v", err)
}
