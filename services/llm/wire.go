package main

import (
	"context"

	"github.com/castlexu/micro-service/pkg/db"
	llmbiz "github.com/castlexu/micro-service/services/llm/biz"
	"github.com/castlexu/micro-service/services/llm/component"
	llmmongo "github.com/castlexu/micro-service/services/llm/dal/mongo"
	llmhandler "github.com/castlexu/micro-service/services/llm/handler"
)

func init() {
	ensureServiceIndexes = func(ctx context.Context, client *db.Client) error {
		providerRepo := llmmongo.NewProviderRepo(client)
		if err := providerRepo.EnsureIndexes(ctx, client); err != nil {
			return err
		}
		modelRepo := llmmongo.NewModelRepo(client)
		if err := modelRepo.EnsureIndexes(ctx, client); err != nil {
			return err
		}
		requestLogRepo := llmmongo.NewRequestLogRepo(client)
		return requestLogRepo.EnsureIndexes(ctx, client)
	}

	buildRouteHandlers = func(_ context.Context, deps ServiceDeps) (ProviderHandler, ModelHandler, GenerateHandler, error) {
		providerRepo := llmmongo.NewProviderRepo(deps.Mongo)
		modelRepo := llmmongo.NewModelRepo(deps.Mongo)
		requestLogRepo := llmmongo.NewRequestLogRepo(deps.Mongo)
		providerBiz := llmbiz.NewProviderBiz(providerRepo, deps.EncryptKey, modelRepo)
		modelBiz := llmbiz.NewModelBiz(modelRepo, providerRepo)
		factory := component.NewFactory(modelBiz, providerBiz)
		generateBiz := llmbiz.NewGenerateBiz(factory, requestLogRepo)
		return llmhandler.NewProviderHandler(providerBiz),
			llmhandler.NewModelHandler(modelBiz),
			llmhandler.NewGenerateHandler(generateBiz),
			nil
	}
}
