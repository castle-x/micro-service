package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	llmmodel "github.com/castlexu/micro-service/services/llm/dal/model"
)

// ProviderRepo wraps CRUD operations for llm_providers.
type ProviderRepo struct {
	repo *db.Repository[llmmodel.Provider]
}

// NewProviderRepo constructs ProviderRepo.
func NewProviderRepo(client *db.Client) *ProviderRepo {
	return &ProviderRepo{repo: db.NewRepository[llmmodel.Provider](client, llmmodel.ProviderCollection)}
}

// EnsureIndexes creates provider indexes.
func (r *ProviderRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	return client.CreateIndexes(ctx, llmmodel.ProviderCollection, []string{"slug"}, true)
}

// Insert inserts a provider.
func (r *ProviderRepo) Insert(ctx context.Context, p *llmmodel.Provider) (primitive.ObjectID, error) {
	id, err := r.repo.InsertOne(ctx, p)
	if err != nil {
		if db.IsDuplicateKey(err) {
			if cleanupErr := r.repo.HardDeleteOne(ctx, deletedProviderSlugFilter(p.Slug)); cleanupErr != nil && !db.IsNotFound(cleanupErr) {
				return primitive.NilObjectID, errno.ErrInternal.WithMessagef("llm: cleanup deleted provider slug: %v", cleanupErr)
			}
			if id, err = r.repo.InsertOne(ctx, p); err == nil {
				return id, nil
			}
			if db.IsDuplicateKey(err) {
				return primitive.NilObjectID, errno.ErrDuplicateKey.WithMessage("llm: provider slug already exists")
			}
			return primitive.NilObjectID, errno.ErrInternal.WithMessagef("llm: insert provider: %v", err)
		}
		return primitive.NilObjectID, errno.ErrInternal.WithMessagef("llm: insert provider: %v", err)
	}
	return id, nil
}

// FindByID finds a provider by ObjectID.
func (r *ProviderRepo) FindByID(ctx context.Context, id primitive.ObjectID) (*llmmodel.Provider, error) {
	p, err := r.repo.FindByID(ctx, id)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrLLMProviderNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("llm: find provider: %v", err)
	}
	return p, nil
}

// FindBySlug finds a provider by slug.
func (r *ProviderRepo) FindBySlug(ctx context.Context, slug string) (*llmmodel.Provider, error) {
	p, err := r.repo.FindOne(ctx, bson.D{{Key: "slug", Value: slug}})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrLLMProviderNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("llm: find provider by slug: %v", err)
	}
	return p, nil
}

// List returns all providers.
func (r *ProviderRepo) List(ctx context.Context) ([]*llmmodel.Provider, error) {
	items, err := r.repo.Find(ctx, bson.D{}, db.FindOptions{
		Sort: bson.D{{Key: "created_at", Value: 1}},
	})
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("llm: list providers: %v", err)
	}
	return items, nil
}

// Update updates writable provider fields and returns the updated document.
func (r *ProviderRepo) Update(ctx context.Context, id primitive.ObjectID, patch llmmodel.ProviderUpdatePatch) (*llmmodel.Provider, error) {
	set := bson.D{}
	if patch.Name != nil {
		set = append(set, bson.E{Key: "name", Value: *patch.Name})
	}
	if patch.Vendor != nil {
		set = append(set, bson.E{Key: "vendor", Value: *patch.Vendor})
	}
	if patch.BaseURL != nil {
		set = append(set, bson.E{Key: "base_url", Value: *patch.BaseURL})
	}
	if patch.DefaultModelRef != nil {
		set = append(set, bson.E{Key: "default_model_ref", Value: *patch.DefaultModelRef})
	}
	if patch.ExtraJSON != nil {
		set = append(set, bson.E{Key: "extra_json", Value: *patch.ExtraJSON})
	}
	if len(set) == 0 {
		return r.FindByID(ctx, id)
	}
	p, err := r.repo.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: set}},
		db.FindAndUpdateOptions{ReturnNew: true},
	)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrLLMProviderNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("llm: update provider: %v", err)
	}
	return p, nil
}

// UpdateAPIKey updates the encrypted API key.
func (r *ProviderRepo) UpdateAPIKey(ctx context.Context, id primitive.ObjectID, apiKeyCipher string) error {
	_, err := r.repo.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "api_key_cipher", Value: apiKeyCipher}}}},
	)
	if err != nil {
		if db.IsNotFound(err) {
			return errno.ErrLLMProviderNotFound
		}
		return errno.ErrInternal.WithMessagef("llm: update provider api_key_cipher: %v", err)
	}
	return nil
}

// UpdateEnabled updates enabled and returns the updated provider.
func (r *ProviderRepo) UpdateEnabled(ctx context.Context, id primitive.ObjectID, enabled bool) (*llmmodel.Provider, error) {
	p, err := r.repo.FindOneAndUpdate(ctx,
		bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "enabled", Value: enabled}}}},
		db.FindAndUpdateOptions{ReturnNew: true},
	)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrLLMProviderNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("llm: update provider enabled: %v", err)
	}
	return p, nil
}

// Delete physically removes a provider so the admin can recreate the same slug.
func (r *ProviderRepo) Delete(ctx context.Context, id primitive.ObjectID) error {
	if err := r.repo.HardDeleteOne(ctx, bson.D{{Key: "_id", Value: id}}); err != nil {
		if db.IsNotFound(err) {
			return errno.ErrLLMProviderNotFound
		}
		return errno.ErrInternal.WithMessagef("llm: delete provider: %v", err)
	}
	return nil
}

func deletedProviderSlugFilter(slug string) bson.D {
	return bson.D{
		{Key: "slug", Value: slug},
		{Key: "deleted_at", Value: bson.D{{Key: "$exists", Value: true}}},
	}
}
