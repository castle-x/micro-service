package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	llmmodel "github.com/castlexu/micro-service/services/llm/dal/model"
)

// ModelRepo wraps CRUD operations for llm_models.
type ModelRepo struct {
	repo *db.Repository[llmmodel.Model]
}

// NewModelRepo constructs ModelRepo.
func NewModelRepo(client *db.Client) *ModelRepo {
	return &ModelRepo{repo: db.NewRepository[llmmodel.Model](client, llmmodel.ModelCollection)}
}

// EnsureIndexes creates model indexes.
func (r *ModelRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	if err := client.CreateIndexes(ctx, llmmodel.ModelCollection, []string{"model_ref"}, true); err != nil {
		return err
	}
	return client.CreateIndexes(ctx, llmmodel.ModelCollection, []string{"provider_id", "model"}, true)
}

// Insert inserts a model.
func (r *ModelRepo) Insert(ctx context.Context, m *llmmodel.Model) (primitive.ObjectID, error) {
	id, err := r.repo.InsertOne(ctx, m)
	if err != nil {
		if db.IsDuplicateKey(err) {
			if cleanupErr := r.repo.HardDeleteOne(ctx, deletedModelIdentityFilter(m)); cleanupErr != nil && !db.IsNotFound(cleanupErr) {
				return primitive.NilObjectID, errno.ErrInternal.WithMessagef("llm: cleanup deleted model: %v", cleanupErr)
			}
			if id, err = r.repo.InsertOne(ctx, m); err == nil {
				return id, nil
			}
			if db.IsDuplicateKey(err) {
				return primitive.NilObjectID, errno.ErrDuplicateKey.WithMessage("llm: model already exists")
			}
			return primitive.NilObjectID, errno.ErrInternal.WithMessagef("llm: insert model: %v", err)
		}
		return primitive.NilObjectID, errno.ErrInternal.WithMessagef("llm: insert model: %v", err)
	}
	return id, nil
}

// FindByID finds a model by ObjectID.
func (r *ModelRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*llmmodel.Model, error) {
	m, err := r.repo.FindByID(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrLLMModelNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("llm: find model: %v", err)
	}
	return m, nil
}

// FindByRef finds a model by model_ref.
func (r *ModelRepo) FindByRef(ctx context.Context, modelRef string) (*llmmodel.Model, error) {
	m, err := r.repo.FindOne(ctx, bson.D{{Key: "model_ref", Value: modelRef}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrLLMModelNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("llm: find model by ref: %v", err)
	}
	return m, nil
}

// List returns models, optionally filtered by provider slug and enabled state.
func (r *ModelRepo) List(ctx context.Context, providerSlug string, enabled *bool) ([]*llmmodel.Model, error) {
	filter := bson.D{}
	if providerSlug != "" {
		filter = append(filter, bson.E{Key: "provider_slug", Value: providerSlug})
	}
	if enabled != nil {
		filter = append(filter, bson.E{Key: "enabled", Value: *enabled})
	}
	items, err := r.repo.Find(ctx, filter, db.FindOptions{
		Sort: bson.D{{Key: "provider_slug", Value: 1}, {Key: "model", Value: 1}},
	})
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("llm: list models: %v", err)
	}
	return items, nil
}

// Update updates writable model fields and returns the updated document.
func (r *ModelRepo) Update(ctx context.Context, id primitive.ObjectID, patch llmmodel.ModelUpdatePatch) (*llmmodel.Model, error) {
	set := bson.D{}
	if patch.DisplayName != nil {
		set = append(set, bson.E{Key: "display_name", Value: *patch.DisplayName})
	}
	if patch.Capabilities != nil {
		set = append(set, bson.E{Key: "capabilities", Value: *patch.Capabilities})
	}
	if patch.ContextWindow != nil {
		set = append(set, bson.E{Key: "context_window", Value: *patch.ContextWindow})
	}
	if patch.MaxOutputTokens != nil {
		set = append(set, bson.E{Key: "max_output_tokens", Value: *patch.MaxOutputTokens})
	}
	if patch.DefaultParametersJSON != nil {
		set = append(set, bson.E{Key: "default_parameters_json", Value: *patch.DefaultParametersJSON})
	}
	if len(set) == 0 {
		return r.FindByID(ctx, id)
	}
	m, err := r.repo.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: set}},
		db.FindAndUpdateOptions{ReturnNew: true},
	)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrLLMModelNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("llm: update model: %v", err)
	}
	return m, nil
}

// UpdateEnabled updates enabled and returns the updated model.
func (r *ModelRepo) UpdateEnabled(ctx context.Context, id primitive.ObjectID, enabled bool) (*llmmodel.Model, error) {
	m, err := r.repo.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "enabled", Value: enabled}}}},
		db.FindAndUpdateOptions{ReturnNew: true},
	)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrLLMModelNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("llm: update model enabled: %v", err)
	}
	return m, nil
}

// Delete physically removes a model so the admin can recreate the same model_ref.
func (r *ModelRepo) Delete(ctx context.Context, id primitive.ObjectID) error {
	if err := r.repo.HardDeleteOne(ctx, bson.D{{Key: "_id", Value: id}}); err != nil {
		if db.IsNotFound(err) {
			return errno.ErrLLMModelNotFound
		}
		return errno.ErrInternal.WithMessagef("llm: delete model: %v", err)
	}
	return nil
}

func deletedModelIdentityFilter(m *llmmodel.Model) bson.D {
	return bson.D{
		{Key: "deleted_at", Value: bson.D{{Key: "$exists", Value: true}}},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "model_ref", Value: m.ModelRef}},
			bson.D{{Key: "provider_id", Value: m.ProviderID}, {Key: "model", Value: m.Model}},
		}},
	}
}
