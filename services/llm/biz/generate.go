package biz

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	einojsonschema "github.com/eino-contrib/jsonschema"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/utils"
	"github.com/castlexu/micro-service/services/llm/component"
	llmmodel "github.com/castlexu/micro-service/services/llm/dal/model"
	"github.com/castlexu/micro-service/services/llm/security"
)

const (
	StreamEventReasoningDelta   = "reasoning_delta"
	StreamEventContentDelta     = "content_delta"
	StreamEventToolCallDelta    = "tool_call_delta"
	StreamEventMessageCompleted = "message_completed"
	StreamEventUsage            = "usage"
	StreamEventDone             = "done"
	StreamEventError            = "error"
)

// Message is the HTTP/business representation of an LLM message.
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	ToolName         string     `json:"tool_name,omitempty"`
}

// ToolCall is the structured assistant function call representation.
type ToolCall struct {
	Index    int          `json:"index,omitempty"`
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function FunctionCall `json:"function"`
}

// FunctionCall is a model-emitted function call.
type FunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// Tool is an OpenAI-compatible function tool definition.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a callable function.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ResponseFormat reserves output-format controls for provider implementations.
type ResponseFormat struct {
	Type string `json:"type"`
}

// GenerateReq is shared by non-streaming and streaming generation.
type GenerateReq struct {
	ModelRef       string          `json:"model_ref"`
	Caller         string          `json:"caller,omitempty"`
	UserID         string          `json:"user_id,omitempty"`
	TenantID       string          `json:"tenant_id,omitempty"`
	Messages       []Message       `json:"messages"`
	Tools          []Tool          `json:"tools,omitempty"`
	ToolChoice     string          `json:"tool_choice,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Temperature    *float32        `json:"temperature,omitempty"`
	MaxTokens      *int            `json:"max_tokens,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
}

// Usage is token accounting returned by upstream providers.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// GenerateResp is the non-streaming response body.
type GenerateResp struct {
	RequestID    string  `json:"request_id"`
	Message      Message `json:"message"`
	Usage        Usage   `json:"usage,omitempty"`
	FinishReason string  `json:"finish_reason,omitempty"`
	ModelRef     string  `json:"model_ref"`
}

// StreamEvent is the internal event representation later serialized as SSE.
type StreamEvent struct {
	Type           string     `json:"-"`
	RequestID      string     `json:"request_id,omitempty"`
	Content        string     `json:"content,omitempty"`
	Index          int        `json:"index,omitempty"`
	ID             string     `json:"id,omitempty"`
	Name           string     `json:"name,omitempty"`
	ArgumentsDelta string     `json:"arguments_delta,omitempty"`
	Message        *Message   `json:"message,omitempty"`
	Usage          Usage      `json:"usage,omitempty"`
	FinishReason   string     `json:"finish_reason,omitempty"`
	ModelRef       string     `json:"model_ref,omitempty"`
	Error          *ErrorBody `json:"error,omitempty"`
}

// ErrorBody is the JSON-safe error shape for stream error events.
type ErrorBody struct {
	Code      int32  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// ChatModelFactory is the component factory seam used by GenerateBiz.
type ChatModelFactory interface {
	Build(ctx context.Context, modelRef string) (einomodel.ToolCallingChatModel, *component.ResolvedModel, error)
}

// RequestLogRepository stores usage and idempotency records.
type RequestLogRepository interface {
	Insert(ctx context.Context, log *llmmodel.RequestLog) error
	FindSuccessful(ctx context.Context, userID, modelRef, idempotencyKey string) (*llmmodel.RequestLog, error)
}

// GenerateBiz orchestrates Generate and Stream calls.
type GenerateBiz struct {
	factory ChatModelFactory
	logs    RequestLogRepository
}

// NewGenerateBiz constructs GenerateBiz.
func NewGenerateBiz(factory ChatModelFactory, logs RequestLogRepository) *GenerateBiz {
	return &GenerateBiz{factory: factory, logs: logs}
}

// Generate performs a non-streaming model call.
func (b *GenerateBiz) Generate(ctx context.Context, req GenerateReq) (*GenerateResp, error) {
	if cached := b.cachedResponse(ctx, req); cached != nil {
		return cached, nil
	}
	requestID := "llmreq_" + utils.NewTraceID()
	chatModel, resolved, input, tools, opts, err := b.prepare(ctx, req)
	if err != nil {
		b.record(ctx, requestID, req, resolved, false, "failed", Usage{}, nil, err)
		return nil, err
	}
	chatModel, err = component.BindTools(chatModel, resolved, tools)
	if err != nil {
		b.record(ctx, requestID, req, resolved, false, "failed", Usage{}, nil, err)
		return nil, err
	}
	out, err := chatModel.Generate(ctx, input, opts...)
	if err != nil {
		err = errno.ErrLLMUpstream.WithMessagef("llm generate failed: %s", security.RedactText(err.Error()))
		b.record(ctx, requestID, req, resolved, false, "failed", Usage{}, nil, err)
		return nil, err
	}
	resp := &GenerateResp{
		RequestID: requestID,
		Message:   messageFromSchema(out),
		ModelRef:  resolved.Ref,
	}
	if out != nil && out.ResponseMeta != nil {
		resp.FinishReason = out.ResponseMeta.FinishReason
		resp.Usage = usageFromSchema(out.ResponseMeta.Usage)
	}
	b.record(ctx, requestID, req, resolved, false, "success", resp.Usage, resp, nil)
	return resp, nil
}

func (b *GenerateBiz) cachedResponse(ctx context.Context, req GenerateReq) *GenerateResp {
	if b == nil || b.logs == nil || req.IdempotencyKey == "" || strings.TrimSpace(req.UserID) == "" {
		return nil
	}
	log, err := b.logs.FindSuccessful(ctx, req.UserID, req.ModelRef, req.IdempotencyKey)
	if err != nil || log == nil || log.ResponseJSON == "" {
		return nil
	}
	var resp GenerateResp
	if err := json.Unmarshal([]byte(log.ResponseJSON), &resp); err != nil {
		return nil
	}
	return &resp
}

// Stream performs a streaming model call and converts chunks into service events.
func (b *GenerateBiz) Stream(ctx context.Context, req GenerateReq) (<-chan StreamEvent, error) {
	requestID := "llmreq_" + utils.NewTraceID()
	chatModel, resolved, input, tools, opts, err := b.prepare(ctx, req)
	if err != nil {
		b.record(ctx, requestID, req, resolved, true, "failed", Usage{}, nil, err)
		return nil, err
	}
	chatModel, err = component.BindTools(chatModel, resolved, tools)
	if err != nil {
		b.record(ctx, requestID, req, resolved, true, "failed", Usage{}, nil, err)
		return nil, err
	}
	reader, err := chatModel.Stream(ctx, input, opts...)
	if err != nil {
		events := make(chan StreamEvent, 1)
		err = errno.ErrLLMUpstream.WithMessagef("llm stream failed: %s", security.RedactText(err.Error()))
		b.record(ctx, requestID, req, resolved, true, "failed", Usage{}, nil, err)
		events <- StreamEvent{Type: StreamEventError, RequestID: requestID, Error: errorBody(requestID, err)}
		close(events)
		return events, nil
	}

	events := make(chan StreamEvent, 16)
	go func() {
		defer close(events)
		defer reader.Close()

		var chunks []*schema.Message
		var usage Usage
		var finishReason string
		var failed error
		for {
			chunk, recvErr := reader.Recv()
			if errors.Is(recvErr, io.EOF) {
				break
			}
			if recvErr != nil {
				failed = errno.ErrLLMUpstream.WithMessagef("llm stream recv failed: %s", security.RedactText(recvErr.Error()))
				events <- StreamEvent{Type: StreamEventError, RequestID: requestID, Error: errorBody(requestID, failed)}
				break
			}
			if chunk == nil {
				continue
			}
			chunks = append(chunks, chunk)
			if chunk.ReasoningContent != "" {
				events <- StreamEvent{Type: StreamEventReasoningDelta, RequestID: requestID, Content: chunk.ReasoningContent}
			}
			if chunk.Content != "" {
				events <- StreamEvent{Type: StreamEventContentDelta, RequestID: requestID, Content: chunk.Content}
			}
			for _, tc := range chunk.ToolCalls {
				events <- StreamEvent{
					Type:           StreamEventToolCallDelta,
					RequestID:      requestID,
					Index:          toolCallIndex(tc),
					ID:             tc.ID,
					Name:           tc.Function.Name,
					ArgumentsDelta: tc.Function.Arguments,
				}
			}
			if chunk.ResponseMeta != nil {
				finishReason = chunk.ResponseMeta.FinishReason
				usage = usageFromSchema(chunk.ResponseMeta.Usage)
			}
		}
		if failed != nil {
			b.record(ctx, requestID, req, resolved, true, "failed", usage, nil, failed)
			return
		}
		completed := concatStreamMessage(chunks)
		events <- StreamEvent{Type: StreamEventMessageCompleted, RequestID: requestID, Message: &completed}
		if usage != (Usage{}) {
			events <- StreamEvent{Type: StreamEventUsage, RequestID: requestID, Usage: usage}
		}
		events <- StreamEvent{Type: StreamEventDone, RequestID: requestID, FinishReason: finishReason, ModelRef: resolved.Ref}
		resp := &GenerateResp{RequestID: requestID, Message: completed, Usage: usage, FinishReason: finishReason, ModelRef: resolved.Ref}
		b.record(ctx, requestID, req, resolved, true, "success", usage, resp, nil)
	}()
	return events, nil
}

func (b *GenerateBiz) prepare(ctx context.Context, req GenerateReq) (einomodel.ToolCallingChatModel, *component.ResolvedModel, []*schema.Message, []*schema.ToolInfo, []einomodel.Option, error) {
	if b == nil || b.factory == nil {
		return nil, nil, nil, nil, nil, errno.ErrInvalidParam.WithMessage("llm generate factory required")
	}
	req.ModelRef = strings.TrimSpace(req.ModelRef)
	if req.ModelRef == "" {
		return nil, nil, nil, nil, nil, errno.ErrInvalidParam.WithMessage("model_ref required")
	}
	input, err := schemaMessages(req.Messages)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	chatModel, resolved, err := b.factory.Build(ctx, req.ModelRef)
	if err != nil {
		return nil, resolved, nil, nil, nil, err
	}
	if resolved == nil {
		return nil, nil, nil, nil, nil, errno.ErrLLMModelNotFound
	}
	if req.MaxTokens != nil && resolved.MaxOutputTokens > 0 && *req.MaxTokens > resolved.MaxOutputTokens {
		return nil, resolved, nil, nil, nil, errno.ErrLLMInvalidMessage.WithMessage("max_tokens exceeds model max_output_tokens")
	}
	if err := ValidateLimits(limitRequest(req, resolved), LimitConfig{}); err != nil {
		return nil, resolved, nil, nil, nil, err
	}
	tools, err := schemaTools(req.Tools)
	if err != nil {
		return nil, resolved, nil, nil, nil, err
	}
	opts := modelOptions(req)
	return chatModel, resolved, input, tools, opts, nil
}

func limitRequest(req GenerateReq, resolved *component.ResolvedModel) LimitRequest {
	messages := make([]LimitMessage, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, LimitMessage{Role: msg.Role, Content: msg.Content})
	}
	toolSchemas := make([][]byte, 0, len(req.Tools))
	for _, tool := range req.Tools {
		if len(tool.Function.Parameters) > 0 {
			toolSchemas = append(toolSchemas, []byte(tool.Function.Parameters))
		}
	}
	maxTokens := 0
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	modelMax := 0
	if resolved != nil {
		modelMax = resolved.MaxOutputTokens
	}
	return LimitRequest{
		Messages:             messages,
		ToolSchemas:          toolSchemas,
		MaxTokens:            maxTokens,
		ModelMaxOutputTokens: modelMax,
	}
}

func schemaMessages(messages []Message) ([]*schema.Message, error) {
	if len(messages) == 0 {
		return nil, errno.ErrLLMInvalidMessage.WithMessage("messages required")
	}
	out := make([]*schema.Message, 0, len(messages))
	for _, msg := range messages {
		role := schema.RoleType(strings.TrimSpace(msg.Role))
		switch role {
		case schema.System, schema.User, schema.Assistant, schema.Tool:
		default:
			return nil, errno.ErrLLMInvalidMessage.WithMessagef("unsupported message role %q", msg.Role)
		}
		out = append(out, &schema.Message{
			Role:             role,
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
			ToolCalls:        schemaToolCalls(msg.ToolCalls),
			ToolCallID:       msg.ToolCallID,
			ToolName:         msg.ToolName,
		})
	}
	return out, nil
}

func schemaTools(tools []Tool) ([]*schema.ToolInfo, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	out := make([]*schema.ToolInfo, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "" && tool.Type != "function" {
			return nil, errno.ErrLLMInvalidMessage.WithMessage("only function tools are supported")
		}
		name := strings.TrimSpace(tool.Function.Name)
		if name == "" {
			return nil, errno.ErrLLMInvalidMessage.WithMessage("tool function name required")
		}
		info := &schema.ToolInfo{Name: name, Desc: tool.Function.Description}
		if len(tool.Function.Parameters) > 0 {
			var js einojsonschema.Schema
			if err := json.Unmarshal(tool.Function.Parameters, &js); err != nil {
				return nil, errno.ErrLLMInvalidMessage.WithMessage("tool parameters must be valid JSON schema")
			}
			info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(&js)
		}
		out = append(out, info)
	}
	return out, nil
}

func modelOptions(req GenerateReq) []einomodel.Option {
	opts := make([]einomodel.Option, 0, 3)
	if req.Temperature != nil {
		opts = append(opts, einomodel.WithTemperature(*req.Temperature))
	}
	if req.MaxTokens != nil {
		opts = append(opts, einomodel.WithMaxTokens(*req.MaxTokens))
	}
	switch req.ToolChoice {
	case "none":
		opts = append(opts, einomodel.WithToolChoice(schema.ToolChoiceForbidden))
	case "required":
		opts = append(opts, einomodel.WithToolChoice(schema.ToolChoiceForced))
	case "auto", "":
		if len(req.Tools) > 0 {
			opts = append(opts, einomodel.WithToolChoice(schema.ToolChoiceAllowed))
		}
	}
	return opts
}

func messageFromSchema(msg *schema.Message) Message {
	if msg == nil {
		return Message{Role: string(schema.Assistant)}
	}
	return Message{
		Role:             string(msg.Role),
		Content:          msg.Content,
		ReasoningContent: msg.ReasoningContent,
		ToolCalls:        toolCallsFromSchema(msg.ToolCalls),
		ToolCallID:       msg.ToolCallID,
		ToolName:         msg.ToolName,
	}
}

func schemaToolCalls(calls []ToolCall) []schema.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]schema.ToolCall, 0, len(calls))
	for _, tc := range calls {
		idx := tc.Index
		out = append(out, schema.ToolCall{
			Index: &idx,
			ID:    tc.ID,
			Type:  tc.Type,
			Function: schema.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return out
}

func toolCallsFromSchema(calls []schema.ToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]ToolCall, 0, len(calls))
	for _, tc := range calls {
		out = append(out, ToolCall{
			Index: toolCallIndex(tc),
			ID:    tc.ID,
			Type:  tc.Type,
			Function: FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}
	return out
}

func toolCallIndex(tc schema.ToolCall) int {
	if tc.Index == nil {
		return 0
	}
	return *tc.Index
}

func usageFromSchema(usage *schema.TokenUsage) Usage {
	if usage == nil {
		return Usage{}
	}
	return Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func concatStreamMessage(chunks []*schema.Message) Message {
	filtered := make([]*schema.Message, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk == nil || (chunk.Content == "" && chunk.ReasoningContent == "" && len(chunk.ToolCalls) == 0) {
			continue
		}
		filtered = append(filtered, chunk)
	}
	if len(filtered) == 0 {
		return Message{Role: string(schema.Assistant)}
	}
	msg, err := schema.ConcatMessages(filtered)
	if err != nil {
		return messageFromSchema(filtered[len(filtered)-1])
	}
	return messageFromSchema(msg)
}

func errorBody(requestID string, err error) *ErrorBody {
	var e errno.Errno
	if errors.As(err, &e) {
		return &ErrorBody{Code: e.Code, Message: security.RedactText(e.Message), RequestID: requestID}
	}
	return &ErrorBody{Code: errno.ErrInternal.Code, Message: "internal server error", RequestID: requestID}
}

func (b *GenerateBiz) record(ctx context.Context, requestID string, req GenerateReq, resolved *component.ResolvedModel, stream bool, status string, usage Usage, resp *GenerateResp, err error) {
	if b == nil || b.logs == nil {
		return
	}
	log := &llmmodel.RequestLog{
		RequestID:      requestID,
		Caller:         req.Caller,
		UserID:         req.UserID,
		TenantID:       req.TenantID,
		ModelRef:       req.ModelRef,
		Stream:         stream,
		Status:         status,
		IdempotencyKey: req.IdempotencyKey,
		Usage: llmmodel.RequestUsage{
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
		},
	}
	if resolved != nil {
		log.ModelRef = resolved.Ref
		log.ProviderSlug = resolved.ProviderSlug
	}
	if resp != nil {
		if raw, marshalErr := json.Marshal(resp); marshalErr == nil {
			log.ResponseJSON = string(raw)
		}
	}
	if err != nil {
		var e errno.Errno
		if errors.As(err, &e) {
			log.ErrorCode = e.Code
			log.ErrorMessage = security.RedactText(e.Message)
		} else {
			log.ErrorCode = errno.ErrInternal.Code
			log.ErrorMessage = "internal server error"
		}
	}
	_ = b.logs.Insert(ctx, log)
}
