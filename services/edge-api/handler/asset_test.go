package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/kitex/client/callopt"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	edgeasset "github.com/castlexu/micro-service/services/edge-api/kitex_gen/asset"
	edgebase "github.com/castlexu/micro-service/services/edge-api/kitex_gen/base"
)

func TestAssetHandler_CreateAssetTypePropagatesUserID(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	client := &fakeAssetClient{
		createAssetTypeResp: &edgeasset.CreateAssetTypeResp{
			Base: &edgebase.BaseResp{Code: 0, Message: "ok"},
			AssetType: &edgeasset.AssetTypeDTO{
				AssetTypeID: primitive.NewObjectID().Hex(),
				WorkspaceID: userID,
				Name:        "Character",
				Code:        "character",
				PartSchemas: []*edgeasset.AssetPartSchemaDTO{{
					Key:               "face",
					Name:              "Face",
					AllowedValueKinds: []edgeasset.AssetValueKind{edgeasset.AssetValueKind_MIXED},
				}},
				CreatedBy: userID,
			},
		},
	}
	h := NewAssetHandler(client)
	body := `{
		"name":"Character",
		"code":"character",
		"part_schemas":[{"key":"face","name":"Face","allowed_value_kinds":[4]}]
	}`
	ctx := ut.CreateUtRequestContext(http.MethodPost, "/api/v1/assets/types", &ut.Body{Body: strings.NewReader(body), Len: len(body)}, ut.Header{Key: "Content-Type", Value: "application/json"})
	ctx.Set("auth_user_id", userID)

	h.CreateAssetType(context.Background(), ctx)

	if ctx.Response.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, body=%s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if client.createAssetTypeReq == nil {
		t.Fatal("asset client was not called")
	}
	if got := client.createAssetTypeReq.GetBase().GetUserID(); got != userID {
		t.Fatalf("Base.UserID = %q, want %q", got, userID)
	}
	if got := client.createAssetTypeReq.GetPartSchemas()[0].GetAllowedValueKinds()[0]; got != edgeasset.AssetValueKind_MIXED {
		t.Fatalf("allowed kind = %v, want MIXED", got)
	}
}

func TestAssetHandler_ListAssetsNormalizesPaginationAndMapsConflict(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	client := &fakeAssetClient{
		listAssetsResp: &edgeasset.ListAssetsResp{
			Base: &edgebase.BaseResp{Code: errno.ErrAssetConflict.Code, Message: "asset conflict"},
		},
	}
	h := NewAssetHandler(client)
	ctx := ut.CreateUtRequestContext(http.MethodGet, "/api/v1/assets?page=0&page_size=999&saved_to_library=true", nil)
	ctx.Set("auth_user_id", userID)

	h.ListAssets(context.Background(), ctx)

	if ctx.Response.StatusCode() != http.StatusConflict {
		t.Fatalf("status = %d, want 409 body=%s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if client.listAssetsReq == nil {
		t.Fatal("asset client was not called")
	}
	if got := client.listAssetsReq.GetBase().GetUserID(); got != userID {
		t.Fatalf("Base.UserID = %q, want %q", got, userID)
	}
	if client.listAssetsReq.GetPage().GetPageNum() != 1 || client.listAssetsReq.GetPage().GetPageSize() != 20 {
		t.Fatalf("page = %#v, want page=1 size=20", client.listAssetsReq.GetPage())
	}
	if !client.listAssetsReq.IsSetSavedToLibrary() || !client.listAssetsReq.GetSavedToLibrary() {
		t.Fatalf("saved_to_library filter was not propagated")
	}
	var body apiResp
	if err := json.Unmarshal(ctx.Response.Body(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Code != errno.ErrAssetConflict.Code {
		t.Fatalf("body code = %d, want %d", body.Code, errno.ErrAssetConflict.Code)
	}
}

type fakeAssetClient struct {
	createAssetTypeReq  *edgeasset.CreateAssetTypeReq
	createAssetTypeResp *edgeasset.CreateAssetTypeResp
	listAssetsReq       *edgeasset.ListAssetsReq
	listAssetsResp      *edgeasset.ListAssetsResp
	createVersionReq    *edgeasset.CreateAssetVersionReq
	createVersionResp   *edgeasset.CreateAssetVersionResp
	copyVersionReq      *edgeasset.CopyAssetVersionReq
	copyVersionResp     *edgeasset.CopyAssetVersionResp
	listVersionsReq     *edgeasset.ListAssetVersionsReq
	listVersionsResp    *edgeasset.ListAssetVersionsResp
	currentVersionReq   *edgeasset.GetCurrentAssetVersionReq
	currentVersionResp  *edgeasset.GetCurrentAssetVersionResp
	getVersionReq       *edgeasset.GetAssetVersionReq
	getVersionResp      *edgeasset.GetAssetVersionResp
}

func (f *fakeAssetClient) Health(ctx context.Context, req *edgeasset.HealthReq, callOptions ...callopt.Option) (*edgeasset.HealthResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) CreateAssetType(ctx context.Context, req *edgeasset.CreateAssetTypeReq, callOptions ...callopt.Option) (*edgeasset.CreateAssetTypeResp, error) {
	f.createAssetTypeReq = req
	return f.createAssetTypeResp, nil
}
func (f *fakeAssetClient) UpdateAssetType(ctx context.Context, req *edgeasset.UpdateAssetTypeReq, callOptions ...callopt.Option) (*edgeasset.UpdateAssetTypeResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) GetAssetType(ctx context.Context, req *edgeasset.GetAssetTypeReq, callOptions ...callopt.Option) (*edgeasset.GetAssetTypeResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) ListAssetTypes(ctx context.Context, req *edgeasset.ListAssetTypesReq, callOptions ...callopt.Option) (*edgeasset.ListAssetTypesResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) DeleteAssetType(ctx context.Context, req *edgeasset.DeleteAssetTypeReq, callOptions ...callopt.Option) (*edgeasset.DeleteAssetTypeResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) CreateAssetCategory(ctx context.Context, req *edgeasset.CreateAssetCategoryReq, callOptions ...callopt.Option) (*edgeasset.CreateAssetCategoryResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) UpdateAssetCategory(ctx context.Context, req *edgeasset.UpdateAssetCategoryReq, callOptions ...callopt.Option) (*edgeasset.UpdateAssetCategoryResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) ListAssetCategories(ctx context.Context, req *edgeasset.ListAssetCategoriesReq, callOptions ...callopt.Option) (*edgeasset.ListAssetCategoriesResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) DeleteAssetCategory(ctx context.Context, req *edgeasset.DeleteAssetCategoryReq, callOptions ...callopt.Option) (*edgeasset.DeleteAssetCategoryResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) CreateAsset(ctx context.Context, req *edgeasset.CreateAssetReq, callOptions ...callopt.Option) (*edgeasset.CreateAssetResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) UpdateAsset(ctx context.Context, req *edgeasset.UpdateAssetReq, callOptions ...callopt.Option) (*edgeasset.UpdateAssetResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) GetAsset(ctx context.Context, req *edgeasset.GetAssetReq, callOptions ...callopt.Option) (*edgeasset.GetAssetResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) ListAssets(ctx context.Context, req *edgeasset.ListAssetsReq, callOptions ...callopt.Option) (*edgeasset.ListAssetsResp, error) {
	f.listAssetsReq = req
	return f.listAssetsResp, nil
}
func (f *fakeAssetClient) SetAssetLibraryState(ctx context.Context, req *edgeasset.SetAssetLibraryStateReq, callOptions ...callopt.Option) (*edgeasset.SetAssetLibraryStateResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) DeleteAsset(ctx context.Context, req *edgeasset.DeleteAssetReq, callOptions ...callopt.Option) (*edgeasset.DeleteAssetResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) CreateAssetVersion(ctx context.Context, req *edgeasset.CreateAssetVersionReq, callOptions ...callopt.Option) (*edgeasset.CreateAssetVersionResp, error) {
	f.createVersionReq = req
	return f.createVersionResp, nil
}
func (f *fakeAssetClient) GetAssetVersion(ctx context.Context, req *edgeasset.GetAssetVersionReq, callOptions ...callopt.Option) (*edgeasset.GetAssetVersionResp, error) {
	f.getVersionReq = req
	return f.getVersionResp, nil
}
func (f *fakeAssetClient) ListAssetVersions(ctx context.Context, req *edgeasset.ListAssetVersionsReq, callOptions ...callopt.Option) (*edgeasset.ListAssetVersionsResp, error) {
	f.listVersionsReq = req
	return f.listVersionsResp, nil
}
func (f *fakeAssetClient) GetCurrentAssetVersion(ctx context.Context, req *edgeasset.GetCurrentAssetVersionReq, callOptions ...callopt.Option) (*edgeasset.GetCurrentAssetVersionResp, error) {
	f.currentVersionReq = req
	return f.currentVersionResp, nil
}
func (f *fakeAssetClient) SetCurrentAssetVersion(ctx context.Context, req *edgeasset.SetCurrentAssetVersionReq, callOptions ...callopt.Option) (*edgeasset.SetCurrentAssetVersionResp, error) {
	return nil, errno.ErrNotImplemented
}
func (f *fakeAssetClient) CopyAssetVersion(ctx context.Context, req *edgeasset.CopyAssetVersionReq, callOptions ...callopt.Option) (*edgeasset.CopyAssetVersionResp, error) {
	f.copyVersionReq = req
	return f.copyVersionResp, nil
}
