package adapter

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const llmOperationName = "chat.completions"

var (
	llmMeter = otel.Meter("github.com/castlexu/micro-service/services/model/adapter")

	llmRequestDuration metric.Float64Histogram
	llmFirstToken      metric.Float64Histogram
	llmRequestCount    metric.Int64Counter
	llmTokenCount      metric.Int64Counter
)

func init() {
	llmRequestDuration, _ = llmMeter.Float64Histogram(
		"llm.request.duration",
		metric.WithUnit("ms"),
		metric.WithDescription("LLM provider request duration"),
	)
	llmFirstToken, _ = llmMeter.Float64Histogram(
		"llm.first_token.duration",
		metric.WithUnit("ms"),
		metric.WithDescription("LLM stream first token duration"),
	)
	llmRequestCount, _ = llmMeter.Int64Counter(
		"llm.request.count",
		metric.WithDescription("LLM provider request count"),
	)
	llmTokenCount, _ = llmMeter.Int64Counter(
		"llm.token.count",
		metric.WithDescription("LLM token count reported by provider"),
	)
}

func startLLMRequest(ctx context.Context, provider, model string, stream bool) (context.Context, trace.Span, func(error, *streamUsage, int64), time.Time) {
	start := time.Now()
	attrs := []attribute.KeyValue{
		attribute.String("gen_ai.system", provider),
		attribute.String("gen_ai.request.model", model),
		attribute.String("gen_ai.operation.name", llmOperationName),
		attribute.Bool("stream", stream),
	}
	ctx, span := otel.Tracer("github.com/castlexu/micro-service/services/model/adapter").Start(
		ctx,
		"LLM "+llmOperationName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)

	finished := false
	finish := func(err error, usage *streamUsage, firstTokenDurationMS int64) {
		if finished {
			return
		}
		finished = true

		status := "ok"
		if err != nil {
			status = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		recordAttrs := append(attrs, attribute.String("status", status))
		span.SetAttributes(attribute.String("status", status))

		durationMS := float64(time.Since(start).Milliseconds())
		llmRequestDuration.Record(ctx, durationMS, metric.WithAttributes(recordAttrs...))
		llmRequestCount.Add(ctx, 1, metric.WithAttributes(recordAttrs...))
		if firstTokenDurationMS > 0 {
			llmFirstToken.Record(ctx, float64(firstTokenDurationMS), metric.WithAttributes(recordAttrs...))
		}
		if usage != nil {
			recordLLMUsage(ctx, span, recordAttrs, usage)
		}
	}

	return ctx, span, finish, start
}

func recordLLMUsage(ctx context.Context, span trace.Span, attrs []attribute.KeyValue, usage *streamUsage) {
	if usage.PromptTokens > 0 {
		span.SetAttributes(attribute.Int("llm.token.input", usage.PromptTokens))
		llmTokenCount.Add(ctx, int64(usage.PromptTokens), metric.WithAttributes(append(attrs, attribute.String("token.type", "input"))...))
	}
	if usage.CompletionTokens > 0 {
		span.SetAttributes(attribute.Int("llm.token.output", usage.CompletionTokens))
		llmTokenCount.Add(ctx, int64(usage.CompletionTokens), metric.WithAttributes(append(attrs, attribute.String("token.type", "output"))...))
	}
	if usage.TotalTokens > 0 {
		span.SetAttributes(attribute.Int("llm.token.total", usage.TotalTokens))
		llmTokenCount.Add(ctx, int64(usage.TotalTokens), metric.WithAttributes(append(attrs, attribute.String("token.type", "total"))...))
	}
}
