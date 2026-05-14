package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route/param"
	"github.com/cloudwego/kitex/client/callopt"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	edgeasset "github.com/castlexu/micro-service/services/edge-api/kitex_gen/asset"
	edgebase "github.com/castlexu/micro-service/services/edge-api/kitex_gen/base"
)

func TestAssetHandler_CreateStorageUploadSessionBindsJSONAndPropagatesUserID(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	client := &fakeMediaAssetClient{
		createUploadResp: &edgeasset.CreateStorageUploadSessionResp{
			Base:    &edgebase.BaseResp{Code: 0, Message: "ok"},
			Session: edgeStorageSessionDTO(userID),
			Upload:  edgePresignedURLDTO("PUT"),
		},
	}
	h := NewAssetHandler(client)
	body := `{"content_type":"image/png","size":1024,"filename":"avatar.png","sha256":"declared-sha"}`
	ctx := ut.CreateUtRequestContext(http.MethodPost, "/api/v1/assets/media/upload-sessions", &ut.Body{Body: strings.NewReader(body), Len: len(body)}, ut.Header{Key: "Content-Type", Value: "application/json"})
	ctx.Set("auth_user_id", userID)

	h.CreateStorageUploadSession(context.Background(), ctx)

	if ctx.Response.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, body=%s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if client.createUploadReq == nil {
		t.Fatal("asset client was not called")
	}
	if got := client.createUploadReq.GetBase().GetUserID(); got != userID {
		t.Fatalf("Base.UserID = %q, want %q", got, userID)
	}
	if client.createUploadReq.GetContentType() != "image/png" || client.createUploadReq.GetSize() != 1024 || client.createUploadReq.GetFilename() != "avatar.png" || client.createUploadReq.GetSHA256() != "declared-sha" {
		t.Fatalf("create upload req = %#v", client.createUploadReq)
	}
	var resp apiResp
	if err := json.Unmarshal(ctx.Response.Body(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("body code = %d, want 0", resp.Code)
	}
}

func TestAssetHandler_FinalizeStorageUploadSessionBindsPathBodyAndPropagatesUserID(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	sessionID := primitive.NewObjectID().Hex()
	client := &fakeMediaAssetClient{
		finalizeResp: &edgeasset.FinalizeStorageUploadSessionResp{
			Base:    &edgebase.BaseResp{Code: 0, Message: "ok"},
			Session: edgeStorageSessionDTO(userID),
			Media:   edgeMediaDTO(userID),
		},
	}
	h := NewAssetHandler(client)
	body := `{"sha256":"declared-sha","width":32,"height":16}`
	ctx := ut.CreateUtRequestContext(http.MethodPost, "/api/v1/assets/media/upload-sessions/"+sessionID+"/finalize", &ut.Body{Body: strings.NewReader(body), Len: len(body)}, ut.Header{Key: "Content-Type", Value: "application/json"})
	ctx.Params = append(ctx.Params, param.Param{Key: "session_id", Value: sessionID})
	ctx.Set("auth_user_id", userID)

	h.FinalizeStorageUploadSession(context.Background(), ctx)

	if ctx.Response.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, body=%s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if client.finalizeReq == nil {
		t.Fatal("asset client was not called")
	}
	if client.finalizeReq.GetBase().GetUserID() != userID || client.finalizeReq.GetSessionID() != sessionID {
		t.Fatalf("finalize req = %#v", client.finalizeReq)
	}
	if client.finalizeReq.GetSHA256() != "declared-sha" || client.finalizeReq.GetWidth() != 32 || client.finalizeReq.GetHeight() != 16 {
		t.Fatalf("finalize body fields = %#v", client.finalizeReq)
	}
}

func TestAssetHandler_ListMediaObjectsNormalizesPaginationAndFilters(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	client := &fakeMediaAssetClient{
		listMediaResp: &edgeasset.ListMediaObjectsResp{
			Base:  &edgebase.BaseResp{Code: 0, Message: "ok"},
			Media: []*edgeasset.MediaObjectDTO{edgeMediaDTO(userID)},
			Page:  &edgebase.PageResp{Total: 1, PageNum: 1, PageSize: 20, TotalPages: 1},
		},
	}
	h := NewAssetHandler(client)
	ctx := ut.CreateUtRequestContext(http.MethodGet, "/api/v1/assets/media?page=0&page_size=999&source=1&content_type=image/png", nil)
	ctx.Set("auth_user_id", userID)

	h.ListMediaObjects(context.Background(), ctx)

	if ctx.Response.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, body=%s", ctx.Response.StatusCode(), string(ctx.Response.Body()))
	}
	if client.listMediaReq == nil {
		t.Fatal("asset client was not called")
	}
	if client.listMediaReq.GetBase().GetUserID() != userID {
		t.Fatalf("Base.UserID = %q, want %q", client.listMediaReq.GetBase().GetUserID(), userID)
	}
	if client.listMediaReq.GetPage().GetPageNum() != 1 || client.listMediaReq.GetPage().GetPageSize() != 20 {
		t.Fatalf("page = %#v, want page=1 size=20", client.listMediaReq.GetPage())
	}
	if client.listMediaReq.GetSource() != edgeasset.AssetSource_UPLOAD || client.listMediaReq.GetContentType() != "image/png" {
		t.Fatalf("filters = source:%v content_type:%q", client.listMediaReq.GetSource(), client.listMediaReq.GetContentType())
	}
}

func TestAssetHandler_GetMediaObjectAndAccessURLBindPathAndQuery(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	mediaID := primitive.NewObjectID().Hex()
	client := &fakeMediaAssetClient{
		getMediaResp:    &edgeasset.GetMediaObjectResp{Base: &edgebase.BaseResp{Code: 0, Message: "ok"}, Media: edgeMediaDTO(userID)},
		mediaAccessResp: &edgeasset.GetMediaObjectAccessURLResp{Base: &edgebase.BaseResp{Code: 0, Message: "ok"}, Media: edgeMediaDTO(userID), Access: edgePresignedURLDTO("GET")},
	}
	h := NewAssetHandler(client)

	getCtx := ut.CreateUtRequestContext(http.MethodGet, "/api/v1/assets/media/"+mediaID, nil)
	setMediaIDParam(getCtx, mediaID)
	getCtx.Set("auth_user_id", userID)
	h.GetMediaObject(context.Background(), getCtx)
	if getCtx.Response.StatusCode() != http.StatusOK {
		t.Fatalf("get status = %d, body=%s", getCtx.Response.StatusCode(), string(getCtx.Response.Body()))
	}
	if client.getMediaReq == nil || client.getMediaReq.GetBase().GetUserID() != userID || client.getMediaReq.GetMediaID() != mediaID {
		t.Fatalf("get media req = %#v", client.getMediaReq)
	}

	accessCtx := ut.CreateUtRequestContext(http.MethodGet, "/api/v1/assets/media/"+mediaID+"/access-url?expires_in_seconds=60", nil)
	setMediaIDParam(accessCtx, mediaID)
	accessCtx.Set("auth_user_id", userID)
	h.GetMediaObjectAccessURL(context.Background(), accessCtx)
	if accessCtx.Response.StatusCode() != http.StatusOK {
		t.Fatalf("access status = %d, body=%s", accessCtx.Response.StatusCode(), string(accessCtx.Response.Body()))
	}
	if client.mediaAccessReq == nil || client.mediaAccessReq.GetMediaID() != mediaID || client.mediaAccessReq.GetExpiresInSeconds() != 60 {
		t.Fatalf("access req = %#v", client.mediaAccessReq)
	}
}

func TestAssetHandler_MediaErrorMappings(t *testing.T) {
	userID := primitive.NewObjectID().Hex()
	mediaID := primitive.NewObjectID().Hex()
	sessionID := primitive.NewObjectID().Hex()
	tests := []struct {
		name string
		run  func(*AssetHandler, *app.RequestContext)
		ctx  *app.RequestContext
		resp *fakeMediaAssetClient
		want int
		code int32
	}{
		{
			name: "media not found maps to 404",
			run:  func(h *AssetHandler, ctx *app.RequestContext) { h.GetMediaObject(context.Background(), ctx) },
			ctx:  mediaCtx(http.MethodGet, "/api/v1/assets/media/"+mediaID, userID, mediaID, ""),
			resp: &fakeMediaAssetClient{getMediaResp: &edgeasset.GetMediaObjectResp{Base: &edgebase.BaseResp{Code: errno.ErrMediaObjectNotFound.Code, Message: "missing"}}},
			want: http.StatusNotFound,
			code: errno.ErrMediaObjectNotFound.Code,
		},
		{
			name: "upload session not found maps to 404",
			run: func(h *AssetHandler, ctx *app.RequestContext) {
				h.FinalizeStorageUploadSession(context.Background(), ctx)
			},
			ctx:  mediaCtx(http.MethodPost, "/api/v1/assets/media/upload-sessions/"+sessionID+"/finalize", userID, "", sessionID),
			resp: &fakeMediaAssetClient{finalizeResp: &edgeasset.FinalizeStorageUploadSessionResp{Base: &edgebase.BaseResp{Code: errno.ErrAssetUploadSessionNotFound.Code, Message: "missing"}}},
			want: http.StatusNotFound,
			code: errno.ErrAssetUploadSessionNotFound.Code,
		},
		{
			name: "storage error maps to 502",
			run: func(h *AssetHandler, ctx *app.RequestContext) {
				h.CreateStorageUploadSession(context.Background(), ctx)
			},
			ctx:  mediaCtx(http.MethodPost, "/api/v1/assets/media/upload-sessions", userID, "", ""),
			resp: &fakeMediaAssetClient{createUploadResp: &edgeasset.CreateStorageUploadSessionResp{Base: &edgebase.BaseResp{Code: errno.ErrAssetStorageError.Code, Message: "storage unavailable"}}},
			want: http.StatusBadGateway,
			code: errno.ErrAssetStorageError.Code,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewAssetHandler(tt.resp)
			tt.run(h, tt.ctx)
			if tt.ctx.Response.StatusCode() != tt.want {
				t.Fatalf("status = %d, want %d body=%s", tt.ctx.Response.StatusCode(), tt.want, string(tt.ctx.Response.Body()))
			}
			var body apiResp
			if err := json.Unmarshal(tt.ctx.Response.Body(), &body); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if body.Code != tt.code {
				t.Fatalf("body code = %d, want %d", body.Code, tt.code)
			}
		})
	}
}

func TestAssetMediaRoutesAreRegisteredBeforeDynamicAssetIDRoute(t *testing.T) {
	source, err := os.ReadFile("../router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	text := string(source)
	mediaIdx := strings.Index(text, `assets.POST("/media/upload-sessions"`)
	dynamicIdx := strings.Index(text, `assets.GET("/:id"`)
	if mediaIdx < 0 {
		t.Fatal("media upload route is not registered")
	}
	if dynamicIdx < 0 {
		t.Fatal("dynamic asset detail route is not registered")
	}
	if mediaIdx > dynamicIdx {
		t.Fatalf("media route index %d is after dynamic /:id route index %d", mediaIdx, dynamicIdx)
	}
}

type fakeMediaAssetClient struct {
	fakeAssetClient

	createUploadReq  *edgeasset.CreateStorageUploadSessionReq
	createUploadResp *edgeasset.CreateStorageUploadSessionResp
	finalizeReq      *edgeasset.FinalizeStorageUploadSessionReq
	finalizeResp     *edgeasset.FinalizeStorageUploadSessionResp
	getMediaReq      *edgeasset.GetMediaObjectReq
	getMediaResp     *edgeasset.GetMediaObjectResp
	listMediaReq     *edgeasset.ListMediaObjectsReq
	listMediaResp    *edgeasset.ListMediaObjectsResp
	mediaAccessReq   *edgeasset.GetMediaObjectAccessURLReq
	mediaAccessResp  *edgeasset.GetMediaObjectAccessURLResp
}

func (f *fakeMediaAssetClient) CreateStorageUploadSession(ctx context.Context, req *edgeasset.CreateStorageUploadSessionReq, callOptions ...callopt.Option) (*edgeasset.CreateStorageUploadSessionResp, error) {
	f.createUploadReq = req
	return f.createUploadResp, nil
}

func (f *fakeMediaAssetClient) FinalizeStorageUploadSession(ctx context.Context, req *edgeasset.FinalizeStorageUploadSessionReq, callOptions ...callopt.Option) (*edgeasset.FinalizeStorageUploadSessionResp, error) {
	f.finalizeReq = req
	return f.finalizeResp, nil
}

func (f *fakeMediaAssetClient) GetMediaObject(ctx context.Context, req *edgeasset.GetMediaObjectReq, callOptions ...callopt.Option) (*edgeasset.GetMediaObjectResp, error) {
	f.getMediaReq = req
	return f.getMediaResp, nil
}

func (f *fakeMediaAssetClient) ListMediaObjects(ctx context.Context, req *edgeasset.ListMediaObjectsReq, callOptions ...callopt.Option) (*edgeasset.ListMediaObjectsResp, error) {
	f.listMediaReq = req
	return f.listMediaResp, nil
}

func (f *fakeMediaAssetClient) GetMediaObjectAccessURL(ctx context.Context, req *edgeasset.GetMediaObjectAccessURLReq, callOptions ...callopt.Option) (*edgeasset.GetMediaObjectAccessURLResp, error) {
	f.mediaAccessReq = req
	return f.mediaAccessResp, nil
}

func (f *fakeAssetClient) CreateStorageUploadSession(ctx context.Context, req *edgeasset.CreateStorageUploadSessionReq, callOptions ...callopt.Option) (*edgeasset.CreateStorageUploadSessionResp, error) {
	return nil, errno.ErrNotImplemented
}

func (f *fakeAssetClient) FinalizeStorageUploadSession(ctx context.Context, req *edgeasset.FinalizeStorageUploadSessionReq, callOptions ...callopt.Option) (*edgeasset.FinalizeStorageUploadSessionResp, error) {
	return nil, errno.ErrNotImplemented
}

func (f *fakeAssetClient) GetMediaObject(ctx context.Context, req *edgeasset.GetMediaObjectReq, callOptions ...callopt.Option) (*edgeasset.GetMediaObjectResp, error) {
	return nil, errno.ErrNotImplemented
}

func (f *fakeAssetClient) ListMediaObjects(ctx context.Context, req *edgeasset.ListMediaObjectsReq, callOptions ...callopt.Option) (*edgeasset.ListMediaObjectsResp, error) {
	return nil, errno.ErrNotImplemented
}

func (f *fakeAssetClient) GetMediaObjectAccessURL(ctx context.Context, req *edgeasset.GetMediaObjectAccessURLReq, callOptions ...callopt.Option) (*edgeasset.GetMediaObjectAccessURLResp, error) {
	return nil, errno.ErrNotImplemented
}

func mediaCtx(method, path, userID, mediaID, sessionID string) *app.RequestContext {
	body := `{"content_type":"image/png","size":1}`
	ctx := ut.CreateUtRequestContext(method, path, &ut.Body{Body: strings.NewReader(body), Len: len(body)}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if mediaID != "" {
		setMediaIDParam(ctx, mediaID)
	}
	if sessionID != "" {
		ctx.Params = append(ctx.Params, param.Param{Key: "session_id", Value: sessionID})
	}
	ctx.Set("auth_user_id", userID)
	return ctx
}

func setMediaIDParam(ctx *app.RequestContext, mediaID string) {
	ctx.Params = append(ctx.Params, param.Param{Key: "id", Value: mediaID})
}

func edgeStorageSessionDTO(workspaceID string) *edgeasset.StorageUploadSessionDTO {
	return &edgeasset.StorageUploadSessionDTO{
		SessionID:   primitive.NewObjectID().Hex(),
		WorkspaceID: workspaceID,
		Provider:    edgeasset.StorageProvider_ALIYUN_OSS,
		Bucket:      "asset-test-bucket",
		ObjectKey:   "assets-test/" + workspaceID + "/uploads/2026/05/14/session/original.png",
		Status:      edgeasset.UploadSessionStatus_CREATED,
		ExpiresAt:   1778755200,
		CreatedBy:   workspaceID,
		CreatedAt:   1778754300,
		ContentType: edgeStringPtr("image/png"),
		Size:        edgeInt64Ptr(1024),
	}
}

func edgeMediaDTO(workspaceID string) *edgeasset.MediaObjectDTO {
	return &edgeasset.MediaObjectDTO{
		MediaID:       primitive.NewObjectID().Hex(),
		WorkspaceID:   workspaceID,
		Provider:      edgeasset.StorageProvider_ALIYUN_OSS,
		Bucket:        "asset-test-bucket",
		ObjectKey:     "assets-test/" + workspaceID + "/uploads/2026/05/14/session/original.png",
		URLVisibility: edgeasset.URLVisibility_SIGNED,
		ContentType:   "image/png",
		Size:          1024,
		Source:        edgeasset.AssetSource_UPLOAD,
		CreatedBy:     workspaceID,
		CreatedAt:     1778754300,
	}
}

func edgePresignedURLDTO(method string) *edgeasset.StoragePresignedURLDTO {
	return &edgeasset.StoragePresignedURLDTO{
		Method:    method,
		URL:       "https://oss.example.test/signed",
		Headers:   map[string]string{"Content-Type": "image/png"},
		ExpiresAt: 1778755200,
	}
}

func edgeStringPtr(s string) *string {
	return &s
}

func edgeInt64Ptr(v int64) *int64 {
	return &v
}
