package mongo

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestDeletedProviderSlugFilterOnlyTargetsSoftDeletedSlug(t *testing.T) {
	got := deletedProviderSlugFilter("deepseek")
	want := bson.D{
		{Key: "slug", Value: "deepseek"},
		{Key: "deleted_at", Value: bson.D{{Key: "$exists", Value: true}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}
