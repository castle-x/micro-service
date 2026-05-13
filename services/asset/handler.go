package main

import (
	"context"
	"errors"
	"math"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	assetbiz "github.com/castlexu/micro-service/services/asset/biz"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
	assetgen "github.com/castlexu/micro-service/services/asset/kitex_gen/asset"
	assetbase "github.com/castlexu/micro-service/services/asset/kitex_gen/base"
)

// AssetImpl 实现 Kitex 生成的 AssetService 接口。
type AssetImpl struct {
	healthBiz   *assetbiz.HealthBiz
	typeBiz     *assetbiz.AssetTypeBiz
	categoryBiz *assetbiz.AssetCategoryBiz
	assetBiz    *assetbiz.AssetBiz
	versionBiz  *assetbiz.AssetVersionBiz
}

// NewAssetImpl 构造 AssetImpl。
func NewAssetImpl(
	healthBiz *assetbiz.HealthBiz,
	typeBiz *assetbiz.AssetTypeBiz,
	categoryBiz *assetbiz.AssetCategoryBiz,
	assetBiz *assetbiz.AssetBiz,
	versionBiz ...*assetbiz.AssetVersionBiz,
) *AssetImpl {
	var vb *assetbiz.AssetVersionBiz
	if len(versionBiz) > 0 {
		vb = versionBiz[0]
	}
	return &AssetImpl{
		healthBiz:   healthBiz,
		typeBiz:     typeBiz,
		categoryBiz: categoryBiz,
		assetBiz:    assetBiz,
		versionBiz:  vb,
	}
}

// Health 返回 AS-01 的最小探活响应。
func (s *AssetImpl) Health(ctx context.Context, req *assetgen.HealthReq) (*assetgen.HealthResp, error) {
	if req == nil {
		return &assetgen.HealthResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	service, status, err := s.healthBiz.Check(ctx)
	if err != nil {
		return &assetgen.HealthResp{Base: errBase(err)}, nil
	}
	return &assetgen.HealthResp{
		Base:    okBase(),
		Service: service,
		Status:  status,
	}, nil
}

// CreateAssetType 创建个人资产类型。
func (s *AssetImpl) CreateAssetType(ctx context.Context, req *assetgen.CreateAssetTypeReq) (*assetgen.CreateAssetTypeResp, error) {
	if req == nil {
		return &assetgen.CreateAssetTypeResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.CreateAssetTypeResp{Base: errBase(err)}, nil
	}
	doc, err := s.typeBiz.Create(ctx, userID, assetbiz.AssetTypeInput{
		Name:        req.GetName(),
		Code:        req.GetCode(),
		Description: req.GetDescription(),
		PartSchemas: partSchemasFromDTO(req.GetPartSchemas()),
	})
	if err != nil {
		return &assetgen.CreateAssetTypeResp{Base: errBase(err)}, nil
	}
	return &assetgen.CreateAssetTypeResp{Base: okBase(), AssetType: assetTypeDTO(doc)}, nil
}

// UpdateAssetType 更新个人资产类型。Code 创建后不可变。
func (s *AssetImpl) UpdateAssetType(ctx context.Context, req *assetgen.UpdateAssetTypeReq) (*assetgen.UpdateAssetTypeResp, error) {
	if req == nil {
		return &assetgen.UpdateAssetTypeResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.UpdateAssetTypeResp{Base: errBase(err)}, nil
	}
	doc, err := s.typeBiz.Update(ctx, userID, req.GetAssetTypeID(), assetbiz.AssetTypeUpdate{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		PartSchemas: partSchemasFromDTO(req.GetPartSchemas()),
	})
	if err != nil {
		return &assetgen.UpdateAssetTypeResp{Base: errBase(err)}, nil
	}
	return &assetgen.UpdateAssetTypeResp{Base: okBase(), AssetType: assetTypeDTO(doc)}, nil
}

// GetAssetType 查询个人资产类型详情。
func (s *AssetImpl) GetAssetType(ctx context.Context, req *assetgen.GetAssetTypeReq) (*assetgen.GetAssetTypeResp, error) {
	if req == nil {
		return &assetgen.GetAssetTypeResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.GetAssetTypeResp{Base: errBase(err)}, nil
	}
	doc, err := s.typeBiz.Get(ctx, userID, req.GetAssetTypeID())
	if err != nil {
		return &assetgen.GetAssetTypeResp{Base: errBase(err)}, nil
	}
	return &assetgen.GetAssetTypeResp{Base: okBase(), AssetType: assetTypeDTO(doc)}, nil
}

// ListAssetTypes 分页查询个人资产类型。
func (s *AssetImpl) ListAssetTypes(ctx context.Context, req *assetgen.ListAssetTypesReq) (*assetgen.ListAssetTypesResp, error) {
	if req == nil {
		return &assetgen.ListAssetTypesResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.ListAssetTypesResp{Base: errBase(err)}, nil
	}
	page := pageInput(req.GetPage())
	docs, total, err := s.typeBiz.List(ctx, userID, page)
	if err != nil {
		return &assetgen.ListAssetTypesResp{Base: errBase(err)}, nil
	}
	return &assetgen.ListAssetTypesResp{Base: okBase(), AssetTypes: assetTypeDTOs(docs), Page: pageResp(page, total)}, nil
}

// DeleteAssetType 删除未使用资产类型。
func (s *AssetImpl) DeleteAssetType(ctx context.Context, req *assetgen.DeleteAssetTypeReq) (*assetgen.DeleteAssetTypeResp, error) {
	if req == nil {
		return &assetgen.DeleteAssetTypeResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.DeleteAssetTypeResp{Base: errBase(err)}, nil
	}
	if err := s.typeBiz.Delete(ctx, userID, req.GetAssetTypeID()); err != nil {
		return &assetgen.DeleteAssetTypeResp{Base: errBase(err)}, nil
	}
	return &assetgen.DeleteAssetTypeResp{Base: okBase()}, nil
}

// CreateAssetCategory 创建个人资产分类。
func (s *AssetImpl) CreateAssetCategory(ctx context.Context, req *assetgen.CreateAssetCategoryReq) (*assetgen.CreateAssetCategoryResp, error) {
	if req == nil {
		return &assetgen.CreateAssetCategoryResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.CreateAssetCategoryResp{Base: errBase(err)}, nil
	}
	doc, err := s.categoryBiz.Create(ctx, userID, assetbiz.AssetCategoryInput{
		Name:      req.GetName(),
		ParentID:  req.GetParentID(),
		SortOrder: req.GetSortOrder(),
	})
	if err != nil {
		return &assetgen.CreateAssetCategoryResp{Base: errBase(err)}, nil
	}
	return &assetgen.CreateAssetCategoryResp{Base: okBase(), Category: categoryDTO(doc)}, nil
}

// UpdateAssetCategory 更新个人资产分类。
func (s *AssetImpl) UpdateAssetCategory(ctx context.Context, req *assetgen.UpdateAssetCategoryReq) (*assetgen.UpdateAssetCategoryResp, error) {
	if req == nil {
		return &assetgen.UpdateAssetCategoryResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.UpdateAssetCategoryResp{Base: errBase(err)}, nil
	}
	doc, err := s.categoryBiz.Update(ctx, userID, req.GetCategoryID(), assetbiz.AssetCategoryInput{
		Name:      req.GetName(),
		ParentID:  req.GetParentID(),
		SortOrder: req.GetSortOrder(),
	})
	if err != nil {
		return &assetgen.UpdateAssetCategoryResp{Base: errBase(err)}, nil
	}
	return &assetgen.UpdateAssetCategoryResp{Base: okBase(), Category: categoryDTO(doc)}, nil
}

// ListAssetCategories 查询个人资产分类。
func (s *AssetImpl) ListAssetCategories(ctx context.Context, req *assetgen.ListAssetCategoriesReq) (*assetgen.ListAssetCategoriesResp, error) {
	if req == nil {
		return &assetgen.ListAssetCategoriesResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.ListAssetCategoriesResp{Base: errBase(err)}, nil
	}
	docs, err := s.categoryBiz.List(ctx, userID)
	if err != nil {
		return &assetgen.ListAssetCategoriesResp{Base: errBase(err)}, nil
	}
	return &assetgen.ListAssetCategoriesResp{Base: okBase(), Categories: categoryDTOs(docs)}, nil
}

// DeleteAssetCategory 删除空分类。
func (s *AssetImpl) DeleteAssetCategory(ctx context.Context, req *assetgen.DeleteAssetCategoryReq) (*assetgen.DeleteAssetCategoryResp, error) {
	if req == nil {
		return &assetgen.DeleteAssetCategoryResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.DeleteAssetCategoryResp{Base: errBase(err)}, nil
	}
	if err := s.categoryBiz.Delete(ctx, userID, req.GetCategoryID()); err != nil {
		return &assetgen.DeleteAssetCategoryResp{Base: errBase(err)}, nil
	}
	return &assetgen.DeleteAssetCategoryResp{Base: okBase()}, nil
}

// CreateAsset 创建个人资产实例。
func (s *AssetImpl) CreateAsset(ctx context.Context, req *assetgen.CreateAssetReq) (*assetgen.CreateAssetResp, error) {
	if req == nil {
		return &assetgen.CreateAssetResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.CreateAssetResp{Base: errBase(err)}, nil
	}
	source := assetmodel.AssetSourceUnknown
	if req.IsSetSource() {
		source = assetmodel.AssetSource(req.GetSource())
	}
	doc, err := s.assetBiz.Create(ctx, userID, assetbiz.AssetInput{
		TypeID:         req.GetTypeID(),
		Name:           req.GetName(),
		Description:    req.GetDescription(),
		SavedToLibrary: req.GetSavedToLibrary(),
		CategoryID:     req.GetCategoryID(),
		CoverMediaID:   req.GetCoverMediaID(),
		Source:         source,
		Provenance:     provenanceFromDTO(req.GetProvenance()),
	})
	if err != nil {
		return &assetgen.CreateAssetResp{Base: errBase(err)}, nil
	}
	return &assetgen.CreateAssetResp{Base: okBase(), Asset: assetDTO(doc)}, nil
}

// UpdateAsset 更新个人资产基础信息。
func (s *AssetImpl) UpdateAsset(ctx context.Context, req *assetgen.UpdateAssetReq) (*assetgen.UpdateAssetResp, error) {
	if req == nil {
		return &assetgen.UpdateAssetResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.UpdateAssetResp{Base: errBase(err)}, nil
	}
	var source *assetmodel.AssetSource
	if req.IsSetSource() {
		src := assetmodel.AssetSource(req.GetSource())
		source = &src
	}
	update := assetbiz.AssetUpdate{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Source:      source,
		Provenance:  provenanceFromDTO(req.GetProvenance()),
	}
	if req.IsSetCategoryID() {
		categoryID := req.GetCategoryID()
		update.CategoryID = &categoryID
	}
	if req.IsSetCoverMediaID() {
		coverMediaID := req.GetCoverMediaID()
		update.CoverMediaID = &coverMediaID
	}
	doc, err := s.assetBiz.Update(ctx, userID, req.GetAssetID(), update)
	if err != nil {
		return &assetgen.UpdateAssetResp{Base: errBase(err)}, nil
	}
	return &assetgen.UpdateAssetResp{Base: okBase(), Asset: assetDTO(doc)}, nil
}

// GetAsset 查询个人资产详情。
func (s *AssetImpl) GetAsset(ctx context.Context, req *assetgen.GetAssetReq) (*assetgen.GetAssetResp, error) {
	if req == nil {
		return &assetgen.GetAssetResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.GetAssetResp{Base: errBase(err)}, nil
	}
	doc, err := s.assetBiz.Get(ctx, userID, req.GetAssetID())
	if err != nil {
		return &assetgen.GetAssetResp{Base: errBase(err)}, nil
	}
	return &assetgen.GetAssetResp{Base: okBase(), Asset: assetDTO(doc)}, nil
}

// ListAssets 分页查询个人资产。
func (s *AssetImpl) ListAssets(ctx context.Context, req *assetgen.ListAssetsReq) (*assetgen.ListAssetsResp, error) {
	if req == nil {
		return &assetgen.ListAssetsResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.ListAssetsResp{Base: errBase(err)}, nil
	}
	page := pageInput(req.GetPage())
	filter := assetbiz.AssetListFilter{
		Page:       page,
		TypeID:     req.GetTypeID(),
		CategoryID: req.GetCategoryID(),
	}
	if req.IsSetSavedToLibrary() {
		saved := req.GetSavedToLibrary()
		filter.SavedToLibrary = &saved
	}
	docs, total, err := s.assetBiz.List(ctx, userID, filter)
	if err != nil {
		return &assetgen.ListAssetsResp{Base: errBase(err)}, nil
	}
	return &assetgen.ListAssetsResp{Base: okBase(), Assets: assetDTOs(docs), Page: pageResp(page, total)}, nil
}

// SetAssetLibraryState 保存到资产库或移出资产库。
func (s *AssetImpl) SetAssetLibraryState(ctx context.Context, req *assetgen.SetAssetLibraryStateReq) (*assetgen.SetAssetLibraryStateResp, error) {
	if req == nil {
		return &assetgen.SetAssetLibraryStateResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.SetAssetLibraryStateResp{Base: errBase(err)}, nil
	}
	doc, err := s.assetBiz.SetLibraryState(ctx, userID, req.GetAssetID(), req.GetSavedToLibrary())
	if err != nil {
		return &assetgen.SetAssetLibraryStateResp{Base: errBase(err)}, nil
	}
	return &assetgen.SetAssetLibraryStateResp{Base: okBase(), Asset: assetDTO(doc)}, nil
}

// DeleteAsset 软删除个人资产。
func (s *AssetImpl) DeleteAsset(ctx context.Context, req *assetgen.DeleteAssetReq) (*assetgen.DeleteAssetResp, error) {
	if req == nil {
		return &assetgen.DeleteAssetResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.DeleteAssetResp{Base: errBase(err)}, nil
	}
	if err := s.assetBiz.Delete(ctx, userID, req.GetAssetID()); err != nil {
		return &assetgen.DeleteAssetResp{Base: errBase(err)}, nil
	}
	return &assetgen.DeleteAssetResp{Base: okBase()}, nil
}

// CreateAssetVersion 创建资产版本快照。
func (s *AssetImpl) CreateAssetVersion(ctx context.Context, req *assetgen.CreateAssetVersionReq) (*assetgen.CreateAssetVersionResp, error) {
	if req == nil {
		return &assetgen.CreateAssetVersionResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.CreateAssetVersionResp{Base: errBase(err)}, nil
	}
	doc, err := s.versionBiz.Create(ctx, userID, req.GetAssetID(), assetbiz.AssetVersionInput{
		Parts:        partValuesFromDTO(req.GetParts()),
		ChangeReason: req.GetChangeReason(),
		Provenance:   provenanceFromDTO(req.GetProvenance()),
	})
	if err != nil {
		return &assetgen.CreateAssetVersionResp{Base: errBase(err)}, nil
	}
	asset, err := s.assetBiz.Get(ctx, userID, req.GetAssetID())
	if err != nil {
		return &assetgen.CreateAssetVersionResp{Base: errBase(err)}, nil
	}
	return &assetgen.CreateAssetVersionResp{Base: okBase(), Asset: assetDTO(asset), Version: assetVersionDTO(doc)}, nil
}

// CopyAssetVersion 从历史版本复制并生成新快照。
func (s *AssetImpl) CopyAssetVersion(ctx context.Context, req *assetgen.CopyAssetVersionReq) (*assetgen.CopyAssetVersionResp, error) {
	if req == nil {
		return &assetgen.CopyAssetVersionResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.CopyAssetVersionResp{Base: errBase(err)}, nil
	}
	doc, err := s.versionBiz.Copy(ctx, userID, req.GetAssetID(), req.GetFromVersion(), assetbiz.AssetVersionCopyInput{
		PartOverrides: partValuesFromDTO(req.GetPartOverrides()),
		ChangeReason:  req.GetChangeReason(),
		Provenance:    provenanceFromDTO(req.GetProvenance()),
	})
	if err != nil {
		return &assetgen.CopyAssetVersionResp{Base: errBase(err)}, nil
	}
	asset, err := s.assetBiz.Get(ctx, userID, req.GetAssetID())
	if err != nil {
		return &assetgen.CopyAssetVersionResp{Base: errBase(err)}, nil
	}
	return &assetgen.CopyAssetVersionResp{Base: okBase(), Asset: assetDTO(asset), Version: assetVersionDTO(doc)}, nil
}

// GetAssetVersion 查询资产指定版本。
func (s *AssetImpl) GetAssetVersion(ctx context.Context, req *assetgen.GetAssetVersionReq) (*assetgen.GetAssetVersionResp, error) {
	if req == nil {
		return &assetgen.GetAssetVersionResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.GetAssetVersionResp{Base: errBase(err)}, nil
	}
	doc, err := s.versionBiz.Get(ctx, userID, req.GetAssetID(), req.GetVersion())
	if err != nil {
		return &assetgen.GetAssetVersionResp{Base: errBase(err)}, nil
	}
	return &assetgen.GetAssetVersionResp{Base: okBase(), Version: assetVersionDTO(doc)}, nil
}

// GetCurrentAssetVersion 查询资产当前版本。
func (s *AssetImpl) GetCurrentAssetVersion(ctx context.Context, req *assetgen.GetCurrentAssetVersionReq) (*assetgen.GetCurrentAssetVersionResp, error) {
	if req == nil {
		return &assetgen.GetCurrentAssetVersionResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.GetCurrentAssetVersionResp{Base: errBase(err)}, nil
	}
	doc, err := s.versionBiz.GetCurrent(ctx, userID, req.GetAssetID())
	if err != nil {
		return &assetgen.GetCurrentAssetVersionResp{Base: errBase(err)}, nil
	}
	return &assetgen.GetCurrentAssetVersionResp{Base: okBase(), Version: assetVersionDTO(doc)}, nil
}

// ListAssetVersions 分页查询资产版本。
func (s *AssetImpl) ListAssetVersions(ctx context.Context, req *assetgen.ListAssetVersionsReq) (*assetgen.ListAssetVersionsResp, error) {
	if req == nil {
		return &assetgen.ListAssetVersionsResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.ListAssetVersionsResp{Base: errBase(err)}, nil
	}
	page := pageInput(req.GetPage())
	docs, total, err := s.versionBiz.List(ctx, userID, req.GetAssetID(), page)
	if err != nil {
		return &assetgen.ListAssetVersionsResp{Base: errBase(err)}, nil
	}
	return &assetgen.ListAssetVersionsResp{Base: okBase(), Versions: assetVersionDTOs(docs), Page: pageResp(page, total)}, nil
}

// SetCurrentAssetVersion 更新资产当前版本指针。
func (s *AssetImpl) SetCurrentAssetVersion(ctx context.Context, req *assetgen.SetCurrentAssetVersionReq) (*assetgen.SetCurrentAssetVersionResp, error) {
	if req == nil {
		return &assetgen.SetCurrentAssetVersionResp{Base: errBase(errno.ErrInvalidParam.WithMessage("request is required"))}, nil
	}
	userID, err := userIDFromBase(req.GetBase())
	if err != nil {
		return &assetgen.SetCurrentAssetVersionResp{Base: errBase(err)}, nil
	}
	if err := s.versionBiz.SetCurrent(ctx, userID, req.GetAssetID(), req.GetVersion()); err != nil {
		return &assetgen.SetCurrentAssetVersionResp{Base: errBase(err)}, nil
	}
	asset, err := s.assetBiz.Get(ctx, userID, req.GetAssetID())
	if err != nil {
		return &assetgen.SetCurrentAssetVersionResp{Base: errBase(err)}, nil
	}
	version, err := s.versionBiz.Get(ctx, userID, req.GetAssetID(), req.GetVersion())
	if err != nil {
		return &assetgen.SetCurrentAssetVersionResp{Base: errBase(err)}, nil
	}
	return &assetgen.SetCurrentAssetVersionResp{Base: okBase(), Asset: assetDTO(asset), Version: assetVersionDTO(version)}, nil
}

func okBase() *assetbase.BaseResp {
	return &assetbase.BaseResp{Code: 0, Message: "ok"}
}

func errBase(err error) *assetbase.BaseResp {
	var e errno.Errno
	if errors.As(err, &e) {
		return &assetbase.BaseResp{Code: e.Code, Message: e.Message}
	}
	return &assetbase.BaseResp{Code: errno.ErrInternal.Code, Message: err.Error()}
}

func userIDFromBase(base *assetbase.BaseReq) (string, error) {
	if base == nil || base.GetUserID() == "" {
		return "", errno.ErrInvalidParam.WithMessage("asset: base.user_id is required")
	}
	return base.GetUserID(), nil
}

func pageInput(page *assetbase.PageReq) assetbiz.PageInput {
	if page == nil {
		return assetbiz.PageInput{}
	}
	return assetbiz.PageInput{PageNum: page.GetPageNum(), PageSize: page.GetPageSize()}
}

func pageResp(page assetbiz.PageInput, total int64) *assetbase.PageResp {
	if page.PageNum < 1 {
		page.PageNum = 1
	}
	if page.PageSize < 1 || page.PageSize > 100 {
		page.PageSize = 20
	}
	totalPages := int32(0)
	if total > 0 {
		totalPages = int32(math.Ceil(float64(total) / float64(page.PageSize)))
	}
	return &assetbase.PageResp{
		Total:      int32Clamp(total),
		PageNum:    page.PageNum,
		PageSize:   page.PageSize,
		TotalPages: totalPages,
	}
}

func int32Clamp(n int64) int32 {
	const maxInt32 = int64(1<<31 - 1)
	const minInt32 = -1 << 31
	if n > maxInt32 {
		return int32(maxInt32)
	}
	if n < minInt32 {
		return int32(minInt32)
	}
	return int32(n)
}

func partSchemasFromDTO(in []*assetgen.AssetPartSchemaDTO) []assetmodel.AssetPartSchema {
	out := make([]assetmodel.AssetPartSchema, 0, len(in))
	for _, item := range in {
		if item == nil {
			continue
		}
		kinds := make([]assetmodel.AssetValueKind, 0, len(item.GetAllowedValueKinds()))
		for _, kind := range item.GetAllowedValueKinds() {
			kinds = append(kinds, assetmodel.AssetValueKind(kind))
		}
		out = append(out, assetmodel.AssetPartSchema{
			Key:               item.GetKey(),
			Name:              item.GetName(),
			Description:       item.GetDescription(),
			AllowedValueKinds: kinds,
			Multiple:          item.GetMultiple(),
			Required:          item.GetRequired(),
			SortOrder:         item.GetSortOrder(),
		})
	}
	return out
}

func partSchemasToDTO(in []assetmodel.AssetPartSchema) []*assetgen.AssetPartSchemaDTO {
	out := make([]*assetgen.AssetPartSchemaDTO, 0, len(in))
	for _, item := range in {
		kinds := make([]assetgen.AssetValueKind, 0, len(item.AllowedValueKinds))
		for _, kind := range item.AllowedValueKinds {
			kinds = append(kinds, assetgen.AssetValueKind(kind))
		}
		out = append(out, &assetgen.AssetPartSchemaDTO{
			Key:               item.Key,
			Name:              item.Name,
			Description:       strPtr(item.Description),
			AllowedValueKinds: kinds,
			Multiple:          item.Multiple,
			Required:          item.Required,
			SortOrder:         item.SortOrder,
		})
	}
	return out
}

func assetTypeDTO(doc *assetmodel.AssetType) *assetgen.AssetTypeDTO {
	if doc == nil {
		return nil
	}
	return &assetgen.AssetTypeDTO{
		AssetTypeID: doc.ID.Hex(),
		WorkspaceID: doc.WorkspaceID,
		Name:        doc.Name,
		Code:        doc.Code,
		Description: strPtr(doc.Description),
		PartSchemas: partSchemasToDTO(doc.PartSchemas),
		CreatedBy:   doc.CreatedBy.Hex(),
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.UpdatedAt,
	}
}

func assetTypeDTOs(docs []*assetmodel.AssetType) []*assetgen.AssetTypeDTO {
	out := make([]*assetgen.AssetTypeDTO, 0, len(docs))
	for _, doc := range docs {
		out = append(out, assetTypeDTO(doc))
	}
	return out
}

func categoryDTO(doc *assetmodel.AssetCategory) *assetgen.AssetCategoryDTO {
	if doc == nil {
		return nil
	}
	return &assetgen.AssetCategoryDTO{
		CategoryID:  doc.ID.Hex(),
		WorkspaceID: doc.WorkspaceID,
		Name:        doc.Name,
		ParentID:    objectIDPtr(doc.ParentID),
		SortOrder:   doc.SortOrder,
		CreatedBy:   doc.CreatedBy.Hex(),
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.UpdatedAt,
	}
}

func categoryDTOs(docs []*assetmodel.AssetCategory) []*assetgen.AssetCategoryDTO {
	out := make([]*assetgen.AssetCategoryDTO, 0, len(docs))
	for _, doc := range docs {
		out = append(out, categoryDTO(doc))
	}
	return out
}

func assetDTO(doc *assetmodel.Asset) *assetgen.AssetDTO {
	if doc == nil {
		return nil
	}
	return &assetgen.AssetDTO{
		AssetID:        doc.ID.Hex(),
		WorkspaceID:    doc.WorkspaceID,
		TypeID:         doc.TypeID.Hex(),
		Name:           doc.Name,
		Description:    strPtr(doc.Description),
		SavedToLibrary: doc.SavedToLibrary,
		CategoryID:     objectIDPtr(doc.CategoryID),
		CurrentVersion: doc.CurrentVersion,
		CoverMediaID:   objectIDPtr(doc.CoverMediaID),
		Source:         assetgen.AssetSource(doc.Source),
		Provenance:     provenanceToDTO(doc.Provenance),
		CreatedBy:      doc.CreatedBy.Hex(),
		CreatedAt:      doc.CreatedAt,
		UpdatedAt:      doc.UpdatedAt,
	}
}

func assetDTOs(docs []*assetmodel.Asset) []*assetgen.AssetDTO {
	out := make([]*assetgen.AssetDTO, 0, len(docs))
	for _, doc := range docs {
		out = append(out, assetDTO(doc))
	}
	return out
}

func partValuesFromDTO(in map[string]*assetgen.AssetPartValueDTO) map[string]assetmodel.AssetPartValue {
	out := make(map[string]assetmodel.AssetPartValue, len(in))
	for key, item := range in {
		if item == nil {
			out[key] = assetmodel.AssetPartValue{}
			continue
		}
		out[key] = assetmodel.AssetPartValue{
			ValueKind: assetmodel.AssetValueKind(item.GetValueKind()),
			Text:      item.GetText(),
			JSON:      item.GetJSON(),
			MediaIDs:  mediaIDsFromStrings(item.GetMediaIDs()),
		}
	}
	return out
}

func partValuesToDTO(in map[string]assetmodel.AssetPartValue) map[string]*assetgen.AssetPartValueDTO {
	out := make(map[string]*assetgen.AssetPartValueDTO, len(in))
	for key, item := range in {
		out[key] = &assetgen.AssetPartValueDTO{
			ValueKind: assetgen.AssetValueKind(item.ValueKind),
			Text:      strPtr(item.Text),
			JSON:      strPtr(item.JSON),
			MediaIDs:  mediaIDsToStrings(item.MediaIDs),
		}
	}
	return out
}

func mediaIDsFromStrings(in []string) []primitive.ObjectID {
	out := make([]primitive.ObjectID, 0, len(in))
	for _, raw := range in {
		id, err := primitive.ObjectIDFromHex(raw)
		if err != nil {
			out = append(out, primitive.NilObjectID)
			continue
		}
		out = append(out, id)
	}
	return out
}

func mediaIDsToStrings(in []primitive.ObjectID) []string {
	out := make([]string, 0, len(in))
	for _, id := range in {
		if id.IsZero() {
			continue
		}
		out = append(out, id.Hex())
	}
	return out
}

func assetVersionDTO(doc *assetmodel.AssetVersion) *assetgen.AssetVersionDTO {
	if doc == nil {
		return nil
	}
	return &assetgen.AssetVersionDTO{
		VersionID:    doc.ID.Hex(),
		AssetID:      doc.AssetID.Hex(),
		Version:      doc.Version,
		Parts:        partValuesToDTO(doc.Parts),
		ChangeReason: strPtr(doc.ChangeReason),
		Provenance:   provenanceToDTO(doc.Provenance),
		CreatedBy:    doc.CreatedBy.Hex(),
		CreatedAt:    doc.CreatedAt,
	}
}

func assetVersionDTOs(docs []*assetmodel.AssetVersion) []*assetgen.AssetVersionDTO {
	out := make([]*assetgen.AssetVersionDTO, 0, len(docs))
	for _, doc := range docs {
		out = append(out, assetVersionDTO(doc))
	}
	return out
}

func provenanceFromDTO(in *assetgen.ProvenanceDTO) *assetmodel.Provenance {
	if in == nil {
		return nil
	}
	return &assetmodel.Provenance{
		WorkflowRunID:   in.GetWorkflowRunID(),
		StepRunID:       in.GetStepRunID(),
		GenerationJobID: in.GetGenerationJobID(),
		PromptID:        in.GetPromptID(),
		Extra:           in.GetExtra(),
	}
}

func provenanceToDTO(in *assetmodel.Provenance) *assetgen.ProvenanceDTO {
	if in == nil {
		return nil
	}
	return &assetgen.ProvenanceDTO{
		WorkflowRunID:   strPtr(in.WorkflowRunID),
		StepRunID:       strPtr(in.StepRunID),
		GenerationJobID: strPtr(in.GenerationJobID),
		PromptID:        strPtr(in.PromptID),
		Extra:           in.Extra,
	}
}

func objectIDPtr(id primitiveLike) *string {
	if id.IsZero() {
		return nil
	}
	s := id.Hex()
	return &s
}

type primitiveLike interface {
	IsZero() bool
	Hex() string
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
