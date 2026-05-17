package mongo

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/db"
	"github.com/castlexu/micro-service/pkg/errno"
	llmmodel "github.com/castlexu/micro-service/services/llm/dal/model"
)

// RequestLogRepo wraps llm_request_logs writes and idempotency lookups.
type RequestLogRepo struct {
	repo *db.Repository[llmmodel.RequestLog]
}

// NewRequestLogRepo constructs RequestLogRepo.
func NewRequestLogRepo(client *db.Client) *RequestLogRepo {
	return &RequestLogRepo{repo: db.NewRepository[llmmodel.RequestLog](client, llmmodel.RequestLogCollection)}
}

// EnsureIndexes creates request log indexes.
func (r *RequestLogRepo) EnsureIndexes(ctx context.Context, client *db.Client) error {
	if err := client.CreateIndexes(ctx, llmmodel.RequestLogCollection, []string{"request_id"}, true); err != nil {
		return err
	}
	if err := client.CreateIndexes(ctx, llmmodel.RequestLogCollection, []string{"user_id", "model_ref", "idempotency_key"}, false); err != nil {
		return err
	}
	return client.CreateIndexes(ctx, llmmodel.RequestLogCollection, []string{"created_at:-1"}, false)
}

// Insert stores a request log.
func (r *RequestLogRepo) Insert(ctx context.Context, log *llmmodel.RequestLog) error {
	if _, err := r.repo.InsertOne(ctx, log); err != nil {
		return errno.ErrInternal.WithMessagef("llm: insert request log: %v", err)
	}
	return nil
}

// FindSuccessful returns a prior successful non-stream response for idempotency.
func (r *RequestLogRepo) FindSuccessful(ctx context.Context, userID, modelRef, idempotencyKey string) (*llmmodel.RequestLog, error) {
	if strings.TrimSpace(userID) == "" || idempotencyKey == "" {
		return nil, errno.ErrNotFound
	}
	log, err := r.repo.FindOne(ctx, bson.D{
		{Key: "user_id", Value: userID},
		{Key: "model_ref", Value: modelRef},
		{Key: "idempotency_key", Value: idempotencyKey},
		{Key: "stream", Value: false},
		{Key: "status", Value: "success"},
	})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, errno.ErrNotFound
		}
		return nil, errno.ErrInternal.WithMessagef("llm: find request log: %v", err)
	}
	if log.ID == primitive.NilObjectID {
		return nil, errno.ErrNotFound
	}
	return log, nil
}
