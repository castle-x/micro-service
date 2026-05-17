package model_test

import (
	"testing"

	"github.com/castlexu/micro-service/pkg/db"
	assetmodel "github.com/castlexu/micro-service/services/asset/dal/model"
)

func TestDocumentsImplementBaseDocument(t *testing.T) {
	var docs = []db.BaseDocument{
		&assetmodel.AssetType{},
		&assetmodel.Asset{},
		&assetmodel.AssetVersion{},
		&assetmodel.MediaObject{},
		&assetmodel.AssetCategory{},
		&assetmodel.StorageUploadSession{},
	}
	for _, doc := range docs {
		doc.SetTimestamps(100)
		if doc.GetCreatedAt() != 100 {
			t.Fatalf("created_at = %d, want 100", doc.GetCreatedAt())
		}
	}
}

func TestCollectionNames(t *testing.T) {
	cases := map[string]string{
		"asset_types":             assetmodel.AssetTypeCollection,
		"assets":                  assetmodel.AssetCollection,
		"asset_versions":          assetmodel.AssetVersionCollection,
		"media_objects":           assetmodel.MediaObjectCollection,
		"asset_categories":        assetmodel.AssetCategoryCollection,
		"storage_upload_sessions": assetmodel.StorageUploadSessionCollection,
	}
	for want, got := range cases {
		if got != want {
			t.Fatalf("collection = %q, want %q", got, want)
		}
	}
}

func TestEnumValuesMatchIDL(t *testing.T) {
	if assetmodel.AssetValueKindUnknown != 0 || assetmodel.AssetValueKindMixed != 4 {
		t.Fatalf("asset value kind enum drifted")
	}
	if assetmodel.AssetSourceUnknown != 0 || assetmodel.AssetSourceImport != 4 {
		t.Fatalf("asset source enum drifted")
	}
	if assetmodel.StorageProviderUnknown != 0 || assetmodel.StorageProviderTencentCOS != 4 {
		t.Fatalf("storage provider enum drifted")
	}
	if assetmodel.URLVisibilityUnknown != 0 || assetmodel.URLVisibilitySigned != 3 {
		t.Fatalf("url visibility enum drifted")
	}
	if assetmodel.UploadSessionStatusUnknown != 0 || assetmodel.UploadSessionStatusCancelled != 4 {
		t.Fatalf("upload session status enum drifted")
	}
}
