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

func TestAssetImpl_CreateAssetVersionRequiresUserID(t *testing.T) {
	store := &handlerAssetStore{}
	impl := &AssetImpl{versionBiz: assetbiz.NewAssetVersionBiz(store, store, store)}

	resp, err := impl.CreateAssetVersion(context.Background(), &assetgen.CreateAssetVersionReq{
		Base:    &assetbase.BaseReq{},
		AssetID: primitive.NewObjectID().Hex(),
		Parts:   map[string]*assetgen.AssetPartValueDTO{"name": handlerTextDTO("Hero")},
	})
	if err != nil {
		t.Fatalf("CreateAssetVersion returned transport error: %v", err)
	}
	if resp.GetBase().GetCode() != errno.ErrInvalidParam.Code {
		t.Fatalf("code = %d, want %d", resp.GetBase().GetCode(), errno.ErrInvalidParam.Code)
	}
}

func TestAssetImpl_AssetVersionRejectsInvalidAssetIDAndVersion(t *testing.T) {
	store := &handlerAssetStore{}
	impl := &AssetImpl{versionBiz: assetbiz.NewAssetVersionBiz(store, store, store)}
	userID := primitive.NewObjectID().Hex()

	createResp, err := impl.CreateAssetVersion(context.Background(), &assetgen.CreateAssetVersionReq{
		Base:    &assetbase.BaseReq{UserID: &userID},
		AssetID: "bad-asset-id",
		Parts:   map[string]*assetgen.AssetPartValueDTO{"name": handlerTextDTO("Hero")},
	})
	if err != nil {
		t.Fatalf("CreateAssetVersion returned transport error: %v", err)
	}
	if createResp.GetBase().GetCode() != errno.ErrInvalidParam.Code {
		t.Fatalf("create code = %d, want %d", createResp.GetBase().GetCode(), errno.ErrInvalidParam.Code)
	}

	getResp, err := impl.GetAssetVersion(context.Background(), &assetgen.GetAssetVersionReq{
		Base:    &assetbase.BaseReq{UserID: &userID},
		AssetID: primitive.NewObjectID().Hex(),
		Version: 0,
	})
	if err != nil {
		t.Fatalf("GetAssetVersion returned transport error: %v", err)
	}
	if getResp.GetBase().GetCode() != errno.ErrInvalidParam.Code {
		t.Fatalf("get code = %d, want %d", getResp.GetBase().GetCode(), errno.ErrInvalidParam.Code)
	}
}

func TestAssetImpl_AssetVersionMapsPartValueDTOs(t *testing.T) {
	ctx := context.Background()
	store := &handlerAssetStore{}
	typeBiz := assetbiz.NewAssetTypeBiz(store, store)
	assetBiz := assetbiz.NewAssetBiz(store, store, store)
	versionBiz := assetbiz.NewAssetVersionBiz(store, store, store)
	impl := &AssetImpl{assetBiz: assetBiz, versionBiz: versionBiz}
	userID := primitive.NewObjectID().Hex()
	asset := createHandlerVersionedAsset(t, ctx, typeBiz, assetBiz, userID)
	mediaID := primitive.NewObjectID().Hex()

	resp, err := impl.CreateAssetVersion(ctx, &assetgen.CreateAssetVersionReq{
		Base:    &assetbase.BaseReq{UserID: &userID},
		AssetID: asset.ID.Hex(),
		Parts: map[string]*assetgen.AssetPartValueDTO{
			"title":   handlerTextDTO("Hero"),
			"profile": handlerJSONDTO(`{"class":"mage"}`),
			"cover":   handlerMediaDTO(mediaID),
			"mixed":   handlerMixedDTO("caption", `{"x":1}`, mediaID),
		},
	})
	if err != nil {
		t.Fatalf("CreateAssetVersion returned transport error: %v", err)
	}
	if resp.GetBase().GetCode() != 0 {
		t.Fatalf("code = %d, want 0: %s", resp.GetBase().GetCode(), resp.GetBase().GetMessage())
	}
	got := resp.GetVersion()
	if got.GetVersion() != 1 {
		t.Fatalf("version = %d, want 1", got.GetVersion())
	}
	if got.GetParts()["title"].GetValueKind() != assetgen.AssetValueKind_TEXT || got.GetParts()["title"].GetText() != "Hero" {
		t.Fatalf("text part = %#v", got.GetParts()["title"])
	}
	if got.GetParts()["profile"].GetJSON() != `{"class":"mage"}` {
		t.Fatalf("json part = %#v", got.GetParts()["profile"])
	}
	if got.GetParts()["cover"].GetMediaIDs()[0] != mediaID {
		t.Fatalf("media part = %#v", got.GetParts()["cover"])
	}
	mixed := got.GetParts()["mixed"]
	if mixed.GetValueKind() != assetgen.AssetValueKind_MIXED || mixed.GetText() != "caption" || mixed.GetJSON() != `{"x":1}` || mixed.GetMediaIDs()[0] != mediaID {
		t.Fatalf("mixed part = %#v", mixed)
	}
}

func createHandlerVersionedAsset(t *testing.T, ctx context.Context, typeBiz *assetbiz.AssetTypeBiz, assetBiz *assetbiz.AssetBiz, userID string) *assetmodel.Asset {
	t.Helper()
	assetType, err := typeBiz.Create(ctx, userID, assetbiz.AssetTypeInput{
		Name: "Character",
		Code: "character",
		PartSchemas: []assetmodel.AssetPartSchema{
			{Key: "title", Name: "Title", AllowedValueKinds: []assetmodel.AssetValueKind{assetmodel.AssetValueKindText}, Required: true},
			{Key: "profile", Name: "Profile", AllowedValueKinds: []assetmodel.AssetValueKind{assetmodel.AssetValueKindJSON}, Required: true},
			{Key: "cover", Name: "Cover", AllowedValueKinds: []assetmodel.AssetValueKind{assetmodel.AssetValueKindMedia}, Required: true},
			{Key: "mixed", Name: "Mixed", AllowedValueKinds: []assetmodel.AssetValueKind{assetmodel.AssetValueKindMixed}, Required: true},
		},
	})
	if err != nil {
		t.Fatalf("Create asset type: %v", err)
	}
	asset, err := assetBiz.Create(ctx, userID, assetbiz.AssetInput{TypeID: assetType.ID.Hex(), Name: "Hero", SavedToLibrary: true})
	if err != nil {
		t.Fatalf("Create asset: %v", err)
	}
	return asset
}

func handlerTextDTO(text string) *assetgen.AssetPartValueDTO {
	return &assetgen.AssetPartValueDTO{ValueKind: assetgen.AssetValueKind_TEXT, Text: &text}
}

func handlerJSONDTO(json string) *assetgen.AssetPartValueDTO {
	return &assetgen.AssetPartValueDTO{ValueKind: assetgen.AssetValueKind_JSON, JSON: &json}
}

func handlerMediaDTO(ids ...string) *assetgen.AssetPartValueDTO {
	return &assetgen.AssetPartValueDTO{ValueKind: assetgen.AssetValueKind_MEDIA, MediaIDs: ids}
}

func handlerMixedDTO(text string, json string, ids ...string) *assetgen.AssetPartValueDTO {
	return &assetgen.AssetPartValueDTO{ValueKind: assetgen.AssetValueKind_MIXED, Text: &text, JSON: &json, MediaIDs: ids}
}
