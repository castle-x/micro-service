package biz

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

// AssetVersionInput 是创建资产版本的业务输入。
type AssetVersionInput struct {
	Parts        map[string]assetmodel.AssetPartValue
	ChangeReason string
	Provenance   *assetmodel.Provenance
}

// AssetVersionCopyInput 是复制历史版本并覆盖部分 parts 的业务输入。
type AssetVersionCopyInput struct {
	PartOverrides map[string]assetmodel.AssetPartValue
	ChangeReason  string
	Provenance    *assetmodel.Provenance
}

type AssetVersionRepository interface {
	CreateAssetVersion(ctx context.Context, doc *assetmodel.AssetVersion) (primitive.ObjectID, error)
	FindAssetVersion(ctx context.Context, assetID primitive.ObjectID, version int32) (*assetmodel.AssetVersion, error)
	ListAssetVersions(ctx context.Context, assetID primitive.ObjectID, pageNum, pageSize int32) ([]*assetmodel.AssetVersion, int64, error)
	NextAssetVersionNumber(ctx context.Context, assetID primitive.ObjectID) (int32, error)
}

// AssetVersionBiz 处理资产版本快照。
type AssetVersionBiz struct {
	versionRepo AssetVersionRepository
	assetRepo   AssetRepository
	typeRepo    AssetTypeRepository
}

func NewAssetVersionBiz(versionRepo AssetVersionRepository, assetRepo AssetRepository, typeRepo AssetTypeRepository) *AssetVersionBiz {
	return &AssetVersionBiz{versionRepo: versionRepo, assetRepo: assetRepo, typeRepo: typeRepo}
}

func (b *AssetVersionBiz) Create(ctx context.Context, userID, assetID string, input AssetVersionInput) (*assetmodel.AssetVersion, error) {
	workspaceID, createdBy, asset, assetType, err := b.loadAssetAndType(ctx, userID, assetID)
	if err != nil {
		return nil, err
	}
	parts := cloneParts(input.Parts)
	if err := validateAssetParts(assetType.PartSchemas, parts); err != nil {
		return nil, err
	}
	version, err := b.versionRepo.NextAssetVersionNumber(ctx, asset.ID)
	if err != nil {
		return nil, mapAssetVersionRepoErr(err)
	}
	doc := &assetmodel.AssetVersion{
		AssetID:      asset.ID,
		Version:      version,
		Parts:        parts,
		ChangeReason: input.ChangeReason,
		Provenance:   input.Provenance,
		CreatedBy:    createdBy,
	}
	id, err := b.versionRepo.CreateAssetVersion(ctx, doc)
	if err != nil {
		return nil, mapAssetVersionRepoErr(err)
	}
	doc.ID = id
	if err := b.assetRepo.SetAssetCurrentVersion(ctx, workspaceID, asset.ID, version); err != nil {
		return nil, mapAssetVersionRepoErr(err)
	}
	return doc, nil
}

func (b *AssetVersionBiz) Copy(ctx context.Context, userID, assetID string, fromVersion int32, input AssetVersionCopyInput) (*assetmodel.AssetVersion, error) {
	if fromVersion < 1 {
		return nil, errno.ErrInvalidParam.WithMessage("asset: version must be positive")
	}
	workspaceID, createdBy, asset, assetType, err := b.loadAssetAndType(ctx, userID, assetID)
	if err != nil {
		return nil, err
	}
	source, err := b.versionRepo.FindAssetVersion(ctx, asset.ID, fromVersion)
	if err != nil {
		return nil, mapAssetVersionRepoErr(err)
	}
	parts := cloneParts(source.Parts)
	for key, value := range input.PartOverrides {
		parts[key] = clonePartValue(value)
	}
	if err := validateAssetParts(assetType.PartSchemas, parts); err != nil {
		return nil, err
	}
	version, err := b.versionRepo.NextAssetVersionNumber(ctx, asset.ID)
	if err != nil {
		return nil, mapAssetVersionRepoErr(err)
	}
	doc := &assetmodel.AssetVersion{
		AssetID:      asset.ID,
		Version:      version,
		Parts:        parts,
		ChangeReason: input.ChangeReason,
		Provenance:   input.Provenance,
		CreatedBy:    createdBy,
	}
	id, err := b.versionRepo.CreateAssetVersion(ctx, doc)
	if err != nil {
		return nil, mapAssetVersionRepoErr(err)
	}
	doc.ID = id
	if err := b.assetRepo.SetAssetCurrentVersion(ctx, workspaceID, asset.ID, version); err != nil {
		return nil, mapAssetVersionRepoErr(err)
	}
	return doc, nil
}

func (b *AssetVersionBiz) Get(ctx context.Context, userID, assetID string, version int32) (*assetmodel.AssetVersion, error) {
	if version < 1 {
		return nil, errno.ErrInvalidParam.WithMessage("asset: version must be positive")
	}
	_, _, asset, _, err := b.loadAssetAndType(ctx, userID, assetID)
	if err != nil {
		return nil, err
	}
	doc, err := b.versionRepo.FindAssetVersion(ctx, asset.ID, version)
	if err != nil {
		return nil, mapAssetVersionRepoErr(err)
	}
	return doc, nil
}

func (b *AssetVersionBiz) GetCurrent(ctx context.Context, userID, assetID string) (*assetmodel.AssetVersion, error) {
	_, _, asset, _, err := b.loadAssetAndType(ctx, userID, assetID)
	if err != nil {
		return nil, err
	}
	if asset.CurrentVersion == 0 {
		return nil, errno.ErrAssetVersionNotFound
	}
	doc, err := b.versionRepo.FindAssetVersion(ctx, asset.ID, asset.CurrentVersion)
	if err != nil {
		return nil, mapAssetVersionRepoErr(err)
	}
	return doc, nil
}

func (b *AssetVersionBiz) List(ctx context.Context, userID, assetID string, page PageInput) ([]*assetmodel.AssetVersion, int64, error) {
	_, _, asset, _, err := b.loadAssetAndType(ctx, userID, assetID)
	if err != nil {
		return nil, 0, err
	}
	page = normalizePage(page)
	docs, total, err := b.versionRepo.ListAssetVersions(ctx, asset.ID, page.PageNum, page.PageSize)
	if err != nil {
		return nil, 0, mapAssetVersionRepoErr(err)
	}
	return docs, total, nil
}

func (b *AssetVersionBiz) SetCurrent(ctx context.Context, userID, assetID string, version int32) error {
	if version < 1 {
		return errno.ErrInvalidParam.WithMessage("asset: version must be positive")
	}
	workspaceID, _, asset, _, err := b.loadAssetAndType(ctx, userID, assetID)
	if err != nil {
		return err
	}
	if _, err := b.versionRepo.FindAssetVersion(ctx, asset.ID, version); err != nil {
		return mapAssetVersionRepoErr(err)
	}
	return mapAssetVersionRepoErr(b.assetRepo.SetAssetCurrentVersion(ctx, workspaceID, asset.ID, version))
}

func (b *AssetVersionBiz) loadAssetAndType(ctx context.Context, userID, assetID string) (string, primitive.ObjectID, *assetmodel.Asset, *assetmodel.AssetType, error) {
	workspaceID, createdBy, err := workspaceFromUser(userID)
	if err != nil {
		return "", primitive.NilObjectID, nil, nil, err
	}
	id, err := parseObjectID(assetID, "asset_id")
	if err != nil {
		return "", primitive.NilObjectID, nil, nil, err
	}
	asset, err := b.assetRepo.FindAssetByID(ctx, workspaceID, id)
	if err != nil {
		return "", primitive.NilObjectID, nil, nil, mapAssetVersionRepoErr(err)
	}
	assetType, err := b.typeRepo.FindAssetTypeByID(ctx, workspaceID, asset.TypeID)
	if err != nil {
		return "", primitive.NilObjectID, nil, nil, mapAssetVersionRepoErr(err)
	}
	return workspaceID, createdBy, asset, assetType, nil
}

func validateAssetParts(schemas []assetmodel.AssetPartSchema, parts map[string]assetmodel.AssetPartValue) error {
	byKey := make(map[string]assetmodel.AssetPartSchema, len(schemas))
	for _, schema := range schemas {
		byKey[schema.Key] = schema
	}
	for key, value := range parts {
		schema, ok := byKey[key]
		if !ok || !partKindAllowed(schema.AllowedValueKinds, value.ValueKind) || !partValueValid(value) {
			return errno.ErrAssetInvalidPart
		}
	}
	for _, schema := range schemas {
		if schema.Required {
			if _, ok := parts[schema.Key]; !ok {
				return errno.ErrAssetInvalidPart
			}
		}
	}
	return nil
}

func partKindAllowed(allowed []assetmodel.AssetValueKind, kind assetmodel.AssetValueKind) bool {
	if kind == assetmodel.AssetValueKindUnknown {
		return false
	}
	for _, item := range allowed {
		if item == kind {
			return true
		}
	}
	return false
}

func partValueValid(value assetmodel.AssetPartValue) bool {
	switch value.ValueKind {
	case assetmodel.AssetValueKindText:
		return strings.TrimSpace(value.Text) != ""
	case assetmodel.AssetValueKindJSON:
		return validJSON(value.JSON)
	case assetmodel.AssetValueKindMedia:
		return validMediaIDs(value.MediaIDs)
	case assetmodel.AssetValueKindMixed:
		hasText := strings.TrimSpace(value.Text) != ""
		hasJSON := strings.TrimSpace(value.JSON) != ""
		hasMedia := len(value.MediaIDs) > 0
		if !hasText && !hasJSON && !hasMedia {
			return false
		}
		if hasJSON && !validJSON(value.JSON) {
			return false
		}
		if hasMedia && !validMediaIDs(value.MediaIDs) {
			return false
		}
		return true
	default:
		return false
	}
}

func validJSON(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	return json.Valid([]byte(raw))
}

func validMediaIDs(ids []primitive.ObjectID) bool {
	if len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		if id.IsZero() {
			return false
		}
	}
	return true
}

func cloneParts(in map[string]assetmodel.AssetPartValue) map[string]assetmodel.AssetPartValue {
	out := make(map[string]assetmodel.AssetPartValue, len(in))
	for key, value := range in {
		out[key] = clonePartValue(value)
	}
	return out
}

func clonePartValue(value assetmodel.AssetPartValue) assetmodel.AssetPartValue {
	value.MediaIDs = append([]primitive.ObjectID(nil), value.MediaIDs...)
	return value
}

func mapAssetVersionRepoErr(err error) error {
	if err == nil {
		return nil
	}
	for _, target := range []error{
		errno.ErrAssetTypeNotFound,
		errno.ErrAssetCategoryNotFound,
		errno.ErrAssetNotFound,
		errno.ErrAssetVersionNotFound,
		errno.ErrAssetConflict,
		errno.ErrAssetInvalidPart,
		errno.ErrInvalidParam,
	} {
		if errors.Is(err, target) {
			return err
		}
	}
	return errno.ErrInternal.WithMessagef("asset: repository error: %v", err)
}
