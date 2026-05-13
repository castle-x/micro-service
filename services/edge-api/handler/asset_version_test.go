package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route/param"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	edgeasset "github.com/castlexu/micro-service/services/edge-api/kitex_gen/asset"
	edgebase "github.com/castlexu/micro-service/services/edge-api/kitex_gen/base"
)

func TestAssetHandler_CreateAssetVersionBindsPartsAndPropagatesUserID(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	assetID := primitive.NewObjectID().Hex()
	mediaID := primitive.NewObjectID().Hex()
	client := &fakeAssetClient{
		createVersionResp: &edgeasset.CreateAssetVersionResp{
			Base:    &edgebase.BaseResp{Code: 0, Message: "ok"},
			Version: edgeVersionDTO(assetID, 1),
		},
	}
	h := NewAssetHandler(client)
	body := `{
		"parts":{
			"title":{"value_kind":1,"text":"Hero"},
			"profile":{"value_kind":3,"json":"{\"class\":\"mage\"}"},
			"cover":{"value_kind":2,"media_ids":["` + mediaID + `"]},
			"mixed":{"value_kind":4,"text":"caption","json":"{\"x\":1}","media_ids":["` + mediaID + `"]}
		},
		"change_reason":"initial"
	}`
	ctx := ut.CreateUtRequestContext(http.MethodPost, "/api/v1/assets/"+assetID+"/versions", &ut.Body{Body: strings.NewReader(body), Len: len(body)}, ut.Header{Key: "Content-Type", Value: "application/json"})
	setAssetVersionParams(ctx, assetID, "")
	ctx.Set("auth_user_id", userID)

	h.CreateAssetVersion(context.Background(), ctx)

	if ctx.Response.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, body=%s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if client.createVersionReq == nil {
		t.Fatal("asset client was not called")
	}
	if got := client.createVersionReq.GetBase().GetUserID(); got != userID {
		t.Fatalf("Base.UserID = %q, want %q", got, userID)
	}
	if got := client.createVersionReq.GetAssetID(); got != assetID {
		t.Fatalf("AssetID = %q, want %q", got, assetID)
	}
	parts := client.createVersionReq.GetParts()
	if parts["title"].GetText() != "Hero" || parts["profile"].GetJSON() != `{"class":"mage"}` {
		t.Fatalf("parts = %#v", parts)
	}
	if got := parts["cover"].GetMediaIDs()[0]; got != mediaID {
		t.Fatalf("media id = %q, want %q", got, mediaID)
	}
	if parts["mixed"].GetValueKind() != edgeasset.AssetValueKind_MIXED || parts["mixed"].GetText() != "caption" {
		t.Fatalf("mixed = %#v", parts["mixed"])
	}
}

func TestAssetHandler_CopyAssetVersionBindsPartOverrides(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	assetID := primitive.NewObjectID().Hex()
	client := &fakeAssetClient{
		copyVersionResp: &edgeasset.CopyAssetVersionResp{
			Base:    &edgebase.BaseResp{Code: 0, Message: "ok"},
			Version: edgeVersionDTO(assetID, 3),
		},
	}
	h := NewAssetHandler(client)
	body := `{"part_overrides":{"title":{"value_kind":1,"text":"Hero Copy"}},"change_reason":"branch"}`
	ctx := ut.CreateUtRequestContext(http.MethodPost, "/api/v1/assets/"+assetID+"/versions/1/copy", &ut.Body{Body: strings.NewReader(body), Len: len(body)}, ut.Header{Key: "Content-Type", Value: "application/json"})
	setAssetVersionParams(ctx, assetID, "1")
	ctx.Set("auth_user_id", userID)

	h.CopyAssetVersion(context.Background(), ctx)

	if ctx.Response.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, body=%s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if client.copyVersionReq == nil {
		t.Fatal("asset client was not called")
	}
	if client.copyVersionReq.GetBase().GetUserID() != userID || client.copyVersionReq.GetAssetID() != assetID || client.copyVersionReq.GetFromVersion() != 1 {
		t.Fatalf("copy req = %#v", client.copyVersionReq)
	}
	if got := client.copyVersionReq.GetPartOverrides()["title"].GetText(); got != "Hero Copy" {
		t.Fatalf("override title = %q, want Hero Copy", got)
	}
}

func TestAssetHandler_ListAssetVersionsNormalizesPagination(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	assetID := primitive.NewObjectID().Hex()
	client := &fakeAssetClient{
		listVersionsResp: &edgeasset.ListAssetVersionsResp{
			Base:     &edgebase.BaseResp{Code: 0, Message: "ok"},
			Versions: []*edgeasset.AssetVersionDTO{edgeVersionDTO(assetID, 1)},
			Page:     &edgebase.PageResp{Total: 1, PageNum: 1, PageSize: 20, TotalPages: 1},
		},
	}
	h := NewAssetHandler(client)
	ctx := ut.CreateUtRequestContext(http.MethodGet, "/api/v1/assets/"+assetID+"/versions?page=0&page_size=999", nil)
	setAssetVersionParams(ctx, assetID, "")
	ctx.Set("auth_user_id", userID)

	h.ListAssetVersions(context.Background(), ctx)

	if ctx.Response.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, body=%s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if client.listVersionsReq == nil {
		t.Fatal("asset client was not called")
	}
	if client.listVersionsReq.GetPage().GetPageNum() != 1 || client.listVersionsReq.GetPage().GetPageSize() != 20 {
		t.Fatalf("page = %#v, want page=1 size=20", client.listVersionsReq.GetPage())
	}
}

func TestAssetHandler_GetCurrentAssetVersionPropagatesUserID(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	assetID := primitive.NewObjectID().Hex()
	client := &fakeAssetClient{
		currentVersionResp: &edgeasset.GetCurrentAssetVersionResp{
			Base:    &edgebase.BaseResp{Code: 0, Message: "ok"},
			Version: edgeVersionDTO(assetID, 2),
		},
	}
	h := NewAssetHandler(client)
	ctx := ut.CreateUtRequestContext(http.MethodGet, "/api/v1/assets/"+assetID+"/versions/current", nil)
	setAssetVersionParams(ctx, assetID, "")
	ctx.Set("auth_user_id", userID)

	h.GetCurrentAssetVersion(context.Background(), ctx)

	if ctx.Response.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, body=%s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if client.currentVersionReq == nil {
		t.Fatal("asset client was not called")
	}
	if got := client.currentVersionReq.GetBase().GetUserID(); got != userID {
		t.Fatalf("Base.UserID = %q, want %q", got, userID)
	}
	if got := client.currentVersionReq.GetAssetID(); got != assetID {
		t.Fatalf("AssetID = %q, want %q", got, assetID)
	}
}

func TestAssetHandler_AssetVersionErrorMappings(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	assetID := primitive.NewObjectID().Hex()
	tests := []struct {
		name string
		run  func(*AssetHandler, *app.RequestContext)
		resp func() *fakeAssetClient
		want int
		code int32
	}{
		{
			name: "invalid part maps to bad request",
			run: func(h *AssetHandler, ctx *app.RequestContext) {
				h.CreateAssetVersion(context.Background(), ctx)
			},
			resp: func() *fakeAssetClient {
				return &fakeAssetClient{createVersionResp: &edgeasset.CreateAssetVersionResp{Base: &edgebase.BaseResp{Code: errno.ErrAssetInvalidPart.Code, Message: "invalid part"}}}
			},
			want: http.StatusBadRequest,
			code: errno.ErrAssetInvalidPart.Code,
		},
		{
			name: "version not found maps to not found",
			run: func(h *AssetHandler, ctx *app.RequestContext) {
				h.GetAssetVersion(context.Background(), ctx)
			},
			resp: func() *fakeAssetClient {
				return &fakeAssetClient{getVersionResp: &edgeasset.GetAssetVersionResp{Base: &edgebase.BaseResp{Code: errno.ErrAssetVersionNotFound.Code, Message: "missing"}}}
			},
			want: http.StatusNotFound,
			code: errno.ErrAssetVersionNotFound.Code,
		},
		{
			name: "conflict maps to conflict",
			run: func(h *AssetHandler, ctx *app.RequestContext) {
				h.CopyAssetVersion(context.Background(), ctx)
			},
			resp: func() *fakeAssetClient {
				return &fakeAssetClient{copyVersionResp: &edgeasset.CopyAssetVersionResp{Base: &edgebase.BaseResp{Code: errno.ErrAssetConflict.Code, Message: "conflict"}}}
			},
			want: http.StatusConflict,
			code: errno.ErrAssetConflict.Code,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.resp()
			h := NewAssetHandler(client)
			body := `{"parts":{"title":{"value_kind":1,"text":"Hero"}},"part_overrides":{"title":{"value_kind":1,"text":"Hero"}}}`
			ctx := ut.CreateUtRequestContext(http.MethodPost, "/api/v1/assets/"+assetID+"/versions/1", &ut.Body{Body: strings.NewReader(body), Len: len(body)}, ut.Header{Key: "Content-Type", Value: "application/json"})
			setAssetVersionParams(ctx, assetID, "1")
			ctx.Set("auth_user_id", userID)

			tt.run(h, ctx)

			if ctx.Response.StatusCode() != tt.want {
				t.Fatalf("status = %d, want %d body=%s", ctx.Response.StatusCode(), tt.want, string(ctx.Response.Body()))
			}
			var bodyResp apiResp
			if err := json.Unmarshal(ctx.Response.Body(), &bodyResp); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if bodyResp.Code != tt.code {
				t.Fatalf("body code = %d, want %d", bodyResp.Code, tt.code)
			}
		})
	}
}

func setAssetVersionParams(ctx *app.RequestContext, assetID, version string) {
	ctx.Params = append(ctx.Params, param.Param{Key: "id", Value: assetID}, param.Param{Key: "asset_id", Value: assetID})
	if version != "" {
		ctx.Params = append(ctx.Params, param.Param{Key: "version", Value: version})
	}
}

func edgeVersionDTO(assetID string, version int32) *edgeasset.AssetVersionDTO {
	return &edgeasset.AssetVersionDTO{
		VersionID: primitive.NewObjectID().Hex(),
		AssetID:   assetID,
		Version:   version,
		Parts: map[string]*edgeasset.AssetPartValueDTO{
			"title": {ValueKind: edgeasset.AssetValueKind_TEXT},
		},
		CreatedBy: primitive.NewObjectID().Hex(),
		CreatedAt: 1,
	}
}
