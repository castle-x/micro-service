package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/logger"
	edgeasset "github.com/castlexu/micro-service/services/edge-api/kitex_gen/asset"
	assetclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/asset/assetservice"
	edgebase "github.com/castlexu/micro-service/services/edge-api/kitex_gen/base"
	edgemw "github.com/castlexu/micro-service/services/edge-api/middleware"
)

// AssetHandler 处理 /api/v1/assets/* 路由。
type AssetHandler struct {
	assetClient assetclient.Client
}

// NewAssetHandler 构造 AssetHandler。
func NewAssetHandler(assetClient assetclient.Client) *AssetHandler {
	return &AssetHandler{assetClient: assetClient}
}

type assetPartSchemaReq struct {
	Key               string  `json:"key"`
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	AllowedValueKinds []int32 `json:"allowed_value_kinds"`
	Multiple          bool    `json:"multiple"`
	Required          bool    `json:"required"`
	SortOrder         int32   `json:"sort_order"`
}

type createAssetTypeReq struct {
	Name        string               `json:"name"`
	Code        string               `json:"code"`
	Description string               `json:"description"`
	PartSchemas []assetPartSchemaReq `json:"part_schemas"`
}

type updateAssetTypeReq struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	PartSchemas []assetPartSchemaReq `json:"part_schemas"`
}

type assetCategoryReq struct {
	Name      string `json:"name"`
	ParentID  string `json:"parent_id"`
	SortOrder int32  `json:"sort_order"`
}

type createAssetReq struct {
	TypeID         string                   `json:"type_id"`
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	SavedToLibrary bool                     `json:"saved_to_library"`
	CategoryID     string                   `json:"category_id"`
	CoverMediaID   string                   `json:"cover_media_id"`
	Source         *edgeasset.AssetSource   `json:"source"`
	Provenance     *edgeasset.ProvenanceDTO `json:"provenance"`
}

type updateAssetReq struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	CategoryID   *string                  `json:"category_id"`
	CoverMediaID *string                  `json:"cover_media_id"`
	Source       *edgeasset.AssetSource   `json:"source"`
	Provenance   *edgeasset.ProvenanceDTO `json:"provenance"`
}

type assetLibraryStateReq struct {
	SavedToLibrary bool `json:"saved_to_library"`
}

type assetPartValueReq struct {
	ValueKind int32    `json:"value_kind"`
	Text      string   `json:"text"`
	JSON      string   `json:"json"`
	MediaIDs  []string `json:"media_ids"`
}

type createAssetVersionReq struct {
	Parts        map[string]assetPartValueReq `json:"parts"`
	ChangeReason string                       `json:"change_reason"`
	Provenance   *edgeasset.ProvenanceDTO     `json:"provenance"`
}

type copyAssetVersionReq struct {
	PartOverrides map[string]assetPartValueReq `json:"part_overrides"`
	ChangeReason  string                       `json:"change_reason"`
	Provenance    *edgeasset.ProvenanceDTO     `json:"provenance"`
}

type setCurrentAssetVersionReq struct {
	Version int32 `json:"version"`
}

func (h *AssetHandler) CreateAssetType(c context.Context, ctx *app.RequestContext) {
	var req createAssetTypeReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	resp, err := h.assetClient.CreateAssetType(c, &edgeasset.CreateAssetTypeReq{
		Base:        baseReq(ctx),
		Name:        req.Name,
		Code:        req.Code,
		Description: optionalString(req.Description),
		PartSchemas: partSchemaDTOs(req.PartSchemas),
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.CreateAssetType", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.CreateAssetType", resp.GetBase(), resp.GetAssetType(), err)
}

func (h *AssetHandler) UpdateAssetType(c context.Context, ctx *app.RequestContext) {
	var req updateAssetTypeReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	resp, err := h.assetClient.UpdateAssetType(c, &edgeasset.UpdateAssetTypeReq{
		Base:        baseReq(ctx),
		AssetTypeID: ctx.Param("id"),
		Name:        req.Name,
		Description: optionalString(req.Description),
		PartSchemas: partSchemaDTOs(req.PartSchemas),
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.UpdateAssetType", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.UpdateAssetType", resp.GetBase(), resp.GetAssetType(), err)
}

func (h *AssetHandler) GetAssetType(c context.Context, ctx *app.RequestContext) {
	resp, err := h.assetClient.GetAssetType(c, &edgeasset.GetAssetTypeReq{
		Base:        baseReq(ctx),
		AssetTypeID: ctx.Param("id"),
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.GetAssetType", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.GetAssetType", resp.GetBase(), resp.GetAssetType(), err)
}

func (h *AssetHandler) ListAssetTypes(c context.Context, ctx *app.RequestContext) {
	resp, err := h.assetClient.ListAssetTypes(c, &edgeasset.ListAssetTypesReq{
		Base: baseReq(ctx),
		Page: pageReq(ctx),
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.ListAssetTypes", nil, nil, err)
		return
	}
	if resp.GetBase().GetCode() != 0 {
		writeAssetResp(c, ctx, "asset.ListAssetTypes", resp.GetBase(), nil, err)
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok", Data: map[string]any{
		"asset_types": resp.GetAssetTypes(),
		"page":        resp.GetPage(),
	}})
}

func (h *AssetHandler) DeleteAssetType(c context.Context, ctx *app.RequestContext) {
	resp, err := h.assetClient.DeleteAssetType(c, &edgeasset.DeleteAssetTypeReq{
		Base:        baseReq(ctx),
		AssetTypeID: ctx.Param("id"),
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.DeleteAssetType", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.DeleteAssetType", resp.GetBase(), nil, err)
}

func (h *AssetHandler) CreateAssetCategory(c context.Context, ctx *app.RequestContext) {
	var req assetCategoryReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	resp, err := h.assetClient.CreateAssetCategory(c, &edgeasset.CreateAssetCategoryReq{
		Base:      baseReq(ctx),
		Name:      req.Name,
		ParentID:  optionalString(req.ParentID),
		SortOrder: req.SortOrder,
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.CreateAssetCategory", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.CreateAssetCategory", resp.GetBase(), resp.GetCategory(), err)
}

func (h *AssetHandler) UpdateAssetCategory(c context.Context, ctx *app.RequestContext) {
	var req assetCategoryReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	resp, err := h.assetClient.UpdateAssetCategory(c, &edgeasset.UpdateAssetCategoryReq{
		Base:       baseReq(ctx),
		CategoryID: ctx.Param("id"),
		Name:       req.Name,
		ParentID:   optionalString(req.ParentID),
		SortOrder:  req.SortOrder,
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.UpdateAssetCategory", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.UpdateAssetCategory", resp.GetBase(), resp.GetCategory(), err)
}

func (h *AssetHandler) ListAssetCategories(c context.Context, ctx *app.RequestContext) {
	resp, err := h.assetClient.ListAssetCategories(c, &edgeasset.ListAssetCategoriesReq{Base: baseReq(ctx)})
	if err != nil {
		writeAssetResp(c, ctx, "asset.ListAssetCategories", nil, nil, err)
		return
	}
	if resp.GetBase().GetCode() != 0 {
		writeAssetResp(c, ctx, "asset.ListAssetCategories", resp.GetBase(), nil, err)
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok", Data: resp.GetCategories()})
}

func (h *AssetHandler) DeleteAssetCategory(c context.Context, ctx *app.RequestContext) {
	resp, err := h.assetClient.DeleteAssetCategory(c, &edgeasset.DeleteAssetCategoryReq{
		Base:       baseReq(ctx),
		CategoryID: ctx.Param("id"),
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.DeleteAssetCategory", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.DeleteAssetCategory", resp.GetBase(), nil, err)
}

func (h *AssetHandler) CreateAsset(c context.Context, ctx *app.RequestContext) {
	var req createAssetReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	resp, err := h.assetClient.CreateAsset(c, &edgeasset.CreateAssetReq{
		Base:           baseReq(ctx),
		TypeID:         req.TypeID,
		Name:           req.Name,
		Description:    optionalString(req.Description),
		SavedToLibrary: req.SavedToLibrary,
		CategoryID:     optionalString(req.CategoryID),
		CoverMediaID:   optionalString(req.CoverMediaID),
		Source:         req.Source,
		Provenance:     req.Provenance,
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.CreateAsset", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.CreateAsset", resp.GetBase(), resp.GetAsset(), err)
}

func (h *AssetHandler) UpdateAsset(c context.Context, ctx *app.RequestContext) {
	var req updateAssetReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	resp, err := h.assetClient.UpdateAsset(c, &edgeasset.UpdateAssetReq{
		Base:         baseReq(ctx),
		AssetID:      ctx.Param("id"),
		Name:         req.Name,
		Description:  optionalString(req.Description),
		CategoryID:   req.CategoryID,
		CoverMediaID: req.CoverMediaID,
		Source:       req.Source,
		Provenance:   req.Provenance,
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.UpdateAsset", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.UpdateAsset", resp.GetBase(), resp.GetAsset(), err)
}

func (h *AssetHandler) GetAsset(c context.Context, ctx *app.RequestContext) {
	resp, err := h.assetClient.GetAsset(c, &edgeasset.GetAssetReq{
		Base:    baseReq(ctx),
		AssetID: ctx.Param("id"),
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.GetAsset", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.GetAsset", resp.GetBase(), resp.GetAsset(), err)
}

func (h *AssetHandler) ListAssets(c context.Context, ctx *app.RequestContext) {
	req := &edgeasset.ListAssetsReq{
		Base: baseReq(ctx),
		Page: pageReq(ctx),
	}
	if v := string(ctx.Query("type_id")); v != "" {
		req.TypeID = &v
	}
	if v := string(ctx.Query("category_id")); v != "" {
		req.CategoryID = &v
	}
	if v := string(ctx.Query("saved_to_library")); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "saved_to_library must be boolean"})
			return
		}
		req.SavedToLibrary = &parsed
	}
	resp, err := h.assetClient.ListAssets(c, req)
	if err != nil {
		writeAssetResp(c, ctx, "asset.ListAssets", nil, nil, err)
		return
	}
	if resp.GetBase().GetCode() != 0 {
		writeAssetResp(c, ctx, "asset.ListAssets", resp.GetBase(), nil, err)
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok", Data: map[string]any{
		"assets": resp.GetAssets(),
		"page":   resp.GetPage(),
	}})
}

func (h *AssetHandler) SetAssetLibraryState(c context.Context, ctx *app.RequestContext) {
	var req assetLibraryStateReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	resp, err := h.assetClient.SetAssetLibraryState(c, &edgeasset.SetAssetLibraryStateReq{
		Base:           baseReq(ctx),
		AssetID:        ctx.Param("id"),
		SavedToLibrary: req.SavedToLibrary,
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.SetAssetLibraryState", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.SetAssetLibraryState", resp.GetBase(), resp.GetAsset(), err)
}

func (h *AssetHandler) DeleteAsset(c context.Context, ctx *app.RequestContext) {
	resp, err := h.assetClient.DeleteAsset(c, &edgeasset.DeleteAssetReq{
		Base:    baseReq(ctx),
		AssetID: ctx.Param("id"),
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.DeleteAsset", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.DeleteAsset", resp.GetBase(), nil, err)
}

func (h *AssetHandler) CreateAssetVersion(c context.Context, ctx *app.RequestContext) {
	var req createAssetVersionReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	resp, err := h.assetClient.CreateAssetVersion(c, &edgeasset.CreateAssetVersionReq{
		Base:         baseReq(ctx),
		AssetID:      ctx.Param("id"),
		Parts:        partValueDTOs(req.Parts),
		ChangeReason: optionalString(req.ChangeReason),
		Provenance:   req.Provenance,
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.CreateAssetVersion", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.CreateAssetVersion", resp.GetBase(), assetVersionMutationResp(resp.GetAsset(), resp.GetVersion()), err)
}

func (h *AssetHandler) CopyAssetVersion(c context.Context, ctx *app.RequestContext) {
	version, ok := versionParam(ctx)
	if !ok {
		return
	}
	var req copyAssetVersionReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	resp, err := h.assetClient.CopyAssetVersion(c, &edgeasset.CopyAssetVersionReq{
		Base:          baseReq(ctx),
		AssetID:       ctx.Param("id"),
		FromVersion:   version,
		PartOverrides: partValueDTOs(req.PartOverrides),
		ChangeReason:  optionalString(req.ChangeReason),
		Provenance:    req.Provenance,
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.CopyAssetVersion", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.CopyAssetVersion", resp.GetBase(), assetVersionMutationResp(resp.GetAsset(), resp.GetVersion()), err)
}

func (h *AssetHandler) ListAssetVersions(c context.Context, ctx *app.RequestContext) {
	resp, err := h.assetClient.ListAssetVersions(c, &edgeasset.ListAssetVersionsReq{
		Base:    baseReq(ctx),
		AssetID: ctx.Param("id"),
		Page:    pageReq(ctx),
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.ListAssetVersions", nil, nil, err)
		return
	}
	if resp.GetBase().GetCode() != 0 {
		writeAssetResp(c, ctx, "asset.ListAssetVersions", resp.GetBase(), nil, err)
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok", Data: map[string]any{
		"versions": resp.GetVersions(),
		"page":     resp.GetPage(),
	}})
}

func (h *AssetHandler) GetCurrentAssetVersion(c context.Context, ctx *app.RequestContext) {
	resp, err := h.assetClient.GetCurrentAssetVersion(c, &edgeasset.GetCurrentAssetVersionReq{
		Base:    baseReq(ctx),
		AssetID: ctx.Param("id"),
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.GetCurrentAssetVersion", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.GetCurrentAssetVersion", resp.GetBase(), resp.GetVersion(), err)
}

func (h *AssetHandler) SetCurrentAssetVersion(c context.Context, ctx *app.RequestContext) {
	var req setCurrentAssetVersionReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	if req.Version <= 0 {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "version must be positive"})
		return
	}
	resp, err := h.assetClient.SetCurrentAssetVersion(c, &edgeasset.SetCurrentAssetVersionReq{
		Base:    baseReq(ctx),
		AssetID: ctx.Param("id"),
		Version: req.Version,
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.SetCurrentAssetVersion", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.SetCurrentAssetVersion", resp.GetBase(), assetVersionMutationResp(resp.GetAsset(), resp.GetVersion()), err)
}

func (h *AssetHandler) GetAssetVersion(c context.Context, ctx *app.RequestContext) {
	version, ok := versionParam(ctx)
	if !ok {
		return
	}
	resp, err := h.assetClient.GetAssetVersion(c, &edgeasset.GetAssetVersionReq{
		Base:    baseReq(ctx),
		AssetID: ctx.Param("id"),
		Version: version,
	})
	if err != nil {
		writeAssetResp(c, ctx, "asset.GetAssetVersion", nil, nil, err)
		return
	}
	writeAssetResp(c, ctx, "asset.GetAssetVersion", resp.GetBase(), resp.GetVersion(), err)
}

func baseReq(ctx *app.RequestContext) *edgebase.BaseReq {
	userID := edgemw.GetUserID(ctx)
	return &edgebase.BaseReq{UserID: optionalString(userID)}
}

func pageReq(ctx *app.RequestContext) *edgebase.PageReq {
	page, _ := strconv.Atoi(string(ctx.Query("page")))
	pageSize, _ := strconv.Atoi(string(ctx.Query("page_size")))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return &edgebase.PageReq{PageNum: int32(page), PageSize: int32(pageSize)}
}

func partSchemaDTOs(in []assetPartSchemaReq) []*edgeasset.AssetPartSchemaDTO {
	out := make([]*edgeasset.AssetPartSchemaDTO, 0, len(in))
	for _, item := range in {
		kinds := make([]edgeasset.AssetValueKind, 0, len(item.AllowedValueKinds))
		for _, kind := range item.AllowedValueKinds {
			kinds = append(kinds, edgeasset.AssetValueKind(kind))
		}
		out = append(out, &edgeasset.AssetPartSchemaDTO{
			Key:               item.Key,
			Name:              item.Name,
			Description:       optionalString(item.Description),
			AllowedValueKinds: kinds,
			Multiple:          item.Multiple,
			Required:          item.Required,
			SortOrder:         item.SortOrder,
		})
	}
	return out
}

func partValueDTOs(in map[string]assetPartValueReq) map[string]*edgeasset.AssetPartValueDTO {
	out := make(map[string]*edgeasset.AssetPartValueDTO, len(in))
	for key, item := range in {
		out[key] = &edgeasset.AssetPartValueDTO{
			ValueKind: edgeasset.AssetValueKind(item.ValueKind),
			Text:      optionalString(item.Text),
			JSON:      optionalString(item.JSON),
			MediaIDs:  item.MediaIDs,
		}
	}
	return out
}

func versionParam(ctx *app.RequestContext) (int32, bool) {
	version, err := strconv.Atoi(ctx.Param("version"))
	if err != nil || version <= 0 {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "version must be positive"})
		return 0, false
	}
	return int32(version), true
}

func assetVersionMutationResp(asset *edgeasset.AssetDTO, version *edgeasset.AssetVersionDTO) map[string]any {
	return map[string]any{
		"asset":   asset,
		"version": version,
	}
}

func writeAssetResp(c context.Context, ctx *app.RequestContext, op string, base *edgebase.BaseResp, data any, err error) {
	if err != nil {
		logger.Ctx(c).Error(op+" failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if base != nil && base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(base.Code), apiResp{Code: base.Code, Message: base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok", Data: data})
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
