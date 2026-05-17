package biz

import (
	"fmt"

	"github.com/castlexu/micro-service/pkg/errno"
)

// LimitConfig keeps request and response payloads within bounded sizes.
type LimitConfig struct {
	MaxMessages              int
	MaxMessageContentBytes   int
	MaxToolSchemaBytes       int
	MaxStreamEventBytes      int
	DefaultModelOutputTokens int
}

// LimitMessage is the minimal message shape needed for preflight validation.
type LimitMessage struct {
	Role    string
	Content string
}

// LimitRequest is the minimal LLM generation request shape for limit checks.
type LimitRequest struct {
	Messages             []LimitMessage
	ToolSchemas          [][]byte
	MaxTokens            int
	ModelMaxOutputTokens int
}

func DefaultLimitConfig() LimitConfig {
	return LimitConfig{
		MaxMessages:              64,
		MaxMessageContentBytes:   64 * 1024,
		MaxToolSchemaBytes:       128 * 1024,
		MaxStreamEventBytes:      64 * 1024,
		DefaultModelOutputTokens: 4096,
	}
}

func ValidateLimits(req LimitRequest, cfg LimitConfig) error {
	cfg = normalizeLimitConfig(cfg)
	if len(req.Messages) == 0 {
		return errno.ErrLLMInvalidMessage.WithMessage("messages required")
	}
	if len(req.Messages) > cfg.MaxMessages {
		return errno.ErrLLMInvalidMessage.WithMessagef("messages count exceeds limit: %d > %d", len(req.Messages), cfg.MaxMessages)
	}
	for i, msg := range req.Messages {
		if len([]byte(msg.Content)) > cfg.MaxMessageContentBytes {
			return errno.ErrLLMInvalidMessage.WithMessagef("message content exceeds limit at index %d", i)
		}
	}
	totalToolSchemaBytes := 0
	for _, schema := range req.ToolSchemas {
		totalToolSchemaBytes += len(schema)
		if totalToolSchemaBytes > cfg.MaxToolSchemaBytes {
			return errno.ErrInvalidParam.WithMessagef("tool schema bytes exceed limit: %d > %d", totalToolSchemaBytes, cfg.MaxToolSchemaBytes)
		}
	}
	if req.MaxTokens < 0 {
		return errno.ErrLLMInvalidMessage.WithMessage("max_tokens must be non-negative")
	}
	if req.MaxTokens > 0 {
		modelMax := req.ModelMaxOutputTokens
		if modelMax <= 0 {
			modelMax = cfg.DefaultModelOutputTokens
		}
		if req.MaxTokens > modelMax {
			return errno.ErrLLMModelCapabilityUnsupported.WithMessage(fmt.Sprintf("max_tokens exceeds model output limit: %d > %d", req.MaxTokens, modelMax))
		}
	}
	return nil
}

func ValidateStreamEventLimit(event []byte, cfg LimitConfig) error {
	cfg = normalizeLimitConfig(cfg)
	if len(event) > cfg.MaxStreamEventBytes {
		return errno.ErrLLMInvalidMessage.WithMessagef("stream event bytes exceed limit: %d > %d", len(event), cfg.MaxStreamEventBytes)
	}
	return nil
}

func normalizeLimitConfig(cfg LimitConfig) LimitConfig {
	defaults := DefaultLimitConfig()
	if cfg.MaxMessages <= 0 {
		cfg.MaxMessages = defaults.MaxMessages
	}
	if cfg.MaxMessageContentBytes <= 0 {
		cfg.MaxMessageContentBytes = defaults.MaxMessageContentBytes
	}
	if cfg.MaxToolSchemaBytes <= 0 {
		cfg.MaxToolSchemaBytes = defaults.MaxToolSchemaBytes
	}
	if cfg.MaxStreamEventBytes <= 0 {
		cfg.MaxStreamEventBytes = defaults.MaxStreamEventBytes
	}
	if cfg.DefaultModelOutputTokens <= 0 {
		cfg.DefaultModelOutputTokens = defaults.DefaultModelOutputTokens
	}
	return cfg
}
