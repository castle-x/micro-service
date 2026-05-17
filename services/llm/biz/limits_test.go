package biz

import (
	"errors"
	"strings"
	"testing"

	"github.com/castlexu/micro-service/pkg/errno"
)

func TestLimitValidateMessagesCountAndContent(t *testing.T) {
	cfg := DefaultLimitConfig()
	cfg.MaxMessages = 2
	cfg.MaxMessageContentBytes = 4

	err := ValidateLimits(LimitRequest{
		Messages: []LimitMessage{{Content: "one"}, {Content: "two"}, {Content: "three"}},
	}, cfg)
	if !errors.Is(err, errno.ErrLLMInvalidMessage) {
		t.Fatalf("ValidateLimits() count error = %v, want ErrLLMInvalidMessage", err)
	}

	err = ValidateLimits(LimitRequest{
		Messages: []LimitMessage{{Content: "12345"}},
	}, cfg)
	if !errors.Is(err, errno.ErrLLMInvalidMessage) {
		t.Fatalf("ValidateLimits() content error = %v, want ErrLLMInvalidMessage", err)
	}
}

func TestLimitValidateToolSchemaBytes(t *testing.T) {
	cfg := DefaultLimitConfig()
	cfg.MaxToolSchemaBytes = 12

	err := ValidateLimits(LimitRequest{
		Messages:    []LimitMessage{{Content: "ok"}},
		ToolSchemas: [][]byte{[]byte(`{"a":"12345"}`), []byte(`{"b":1}`)},
	}, cfg)
	if !errors.Is(err, errno.ErrInvalidParam) {
		t.Fatalf("ValidateLimits() tool schema error = %v, want ErrInvalidParam", err)
	}
}

func TestLimitValidateStreamEventBytes(t *testing.T) {
	cfg := DefaultLimitConfig()
	cfg.MaxStreamEventBytes = 8

	err := ValidateStreamEventLimit([]byte("123456789"), cfg)
	if !errors.Is(err, errno.ErrLLMInvalidMessage) {
		t.Fatalf("ValidateStreamEventLimit() error = %v, want ErrLLMInvalidMessage", err)
	}
}

func TestLimitValidateMaxTokensAgainstModel(t *testing.T) {
	cfg := DefaultLimitConfig()

	err := ValidateLimits(LimitRequest{
		Messages:             []LimitMessage{{Content: "ok"}},
		MaxTokens:            129,
		ModelMaxOutputTokens: 128,
	}, cfg)
	if !errors.Is(err, errno.ErrLLMModelCapabilityUnsupported) {
		t.Fatalf("ValidateLimits() max_tokens error = %v, want ErrLLMModelCapabilityUnsupported", err)
	}
}

func TestLimitDefaultConfigIsConservativeAndUsable(t *testing.T) {
	cfg := DefaultLimitConfig()

	err := ValidateLimits(LimitRequest{
		Messages: []LimitMessage{
			{Role: "user", Content: strings.Repeat("a", 1024)},
		},
		ToolSchemas:          [][]byte{[]byte(`{"type":"object"}`)},
		MaxTokens:            128,
		ModelMaxOutputTokens: 512,
	}, cfg)
	if err != nil {
		t.Fatalf("ValidateLimits() default config error = %v", err)
	}

	if err := ValidateStreamEventLimit([]byte(strings.Repeat("x", 1024)), cfg); err != nil {
		t.Fatalf("ValidateStreamEventLimit() default config error = %v", err)
	}
}
