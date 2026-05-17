package handler

import (
	"context"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func startLLMProxyRequest(ctx context.Context, req *http.Request, upstreamPath string, stream bool) (context.Context, func(int, error)) {
	attrs := []attribute.KeyValue{
		attribute.String("http.request.method", req.Method),
		attribute.String("url.path", upstreamPath),
		attribute.String("server.address", req.URL.Hostname()),
		attribute.Bool("stream", stream),
	}
	ctx, span := otel.Tracer("github.com/castlexu/micro-service/services/edge-api/handler").Start(
		ctx,
		"HTTP "+req.Method+" "+upstreamPath,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
	propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}).
		Inject(ctx, propagation.HeaderCarrier(req.Header))

	start := time.Now()
	finished := false
	finish := func(statusCode int, err error) {
		if finished {
			return
		}
		finished = true
		if statusCode > 0 {
			span.SetAttributes(attribute.Int("http.response.status_code", statusCode))
			if statusCode >= http.StatusInternalServerError && err == nil {
				span.SetStatus(codes.Error, http.StatusText(statusCode))
			}
		}
		span.SetAttributes(attribute.Int64("http.client.duration_ms", time.Since(start).Milliseconds()))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
	return ctx, finish
}

type llmProxyBody struct {
	io.ReadCloser
	finish func(int, error)
	status int
}

func (b *llmProxyBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err == io.EOF {
		b.finish(b.status, nil)
	} else if err != nil {
		b.finish(b.status, err)
	}
	return n, err
}

func (b *llmProxyBody) Close() error {
	err := b.ReadCloser.Close()
	b.finish(b.status, err)
	return err
}
