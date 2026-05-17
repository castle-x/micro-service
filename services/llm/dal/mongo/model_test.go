package mongo

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	llmmodel "github.com/castlexu/micro-service/services/llm/dal/model"
)

func TestDeletedModelIdentityFilterOnlyTargetsSoftDeletedModel(t *testing.T) {
	providerID := primitive.NewObjectID()
	got := deletedModelIdentityFilter(&llmmodel.Model{
		ProviderID: providerID,
		Model:      "deepseek-v4-flash",
		ModelRef:   "deepseek/deepseek-v4-flash",
	})
	want := bson.D{
		{Key: "deleted_at", Value: bson.D{{Key: "$exists", Value: true}}},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "model_ref", Value: "deepseek/deepseek-v4-flash"}},
			bson.D{{Key: "provider_id", Value: providerID}, {Key: "model", Value: "deepseek-v4-flash"}},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}
