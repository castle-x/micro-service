package security

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactNestedSensitiveValues(t *testing.T) {
	type credentials struct {
		APIKey string `json:"api_key"`
		Public string `json:"public"`
	}

	input := map[string]any{
		"Authorization": "Bearer plaintext",
		"nested": []any{
			map[string]any{"password": "secret-password", "safe": "visible"},
			credentials{APIKey: "plain-api-key", Public: "ok"},
		},
		"safe": "kept",
	}

	got := Redact(input)
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal(Redact()) error = %v", err)
	}
	text := string(raw)
	for _, leaked := range []string{"Bearer plaintext", "secret-password", "plain-api-key"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("Redact() leaked %q in %s", leaked, text)
		}
	}
	if !strings.Contains(text, "[REDACTED]") {
		t.Fatalf("Redact() output = %s, want redacted marker", text)
	}
	if !strings.Contains(text, "visible") || !strings.Contains(text, "kept") || !strings.Contains(text, "ok") {
		t.Fatalf("Redact() output = %s, want non-sensitive values preserved", text)
	}
}

func TestRedactJSONBytesRedactsNestedFields(t *testing.T) {
	input := []byte(`{"messages":[{"content":"hello","metadata":{"access_token":"token-value"}}],"safe":true}`)

	got := string(RedactJSONBytes(input))
	if strings.Contains(got, "token-value") {
		t.Fatalf("RedactJSONBytes() leaked token: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") || !strings.Contains(got, "hello") {
		t.Fatalf("RedactJSONBytes() = %s, want redacted token and preserved safe content", got)
	}
}

func TestRedactJSONBytesMalformedJSONDoesNotLeakPlaintext(t *testing.T) {
	input := []byte(`{"api_key":"plaintext"`)

	got := string(RedactJSONBytes(input))
	if strings.Contains(got, "plaintext") || strings.Contains(got, "api_key") {
		t.Fatalf("RedactJSONBytes() leaked malformed JSON plaintext: %s", got)
	}
	if got != RedactedValue {
		t.Fatalf("RedactJSONBytes() = %q, want %q", got, RedactedValue)
	}
}

func TestRedactTextPlainErrorString(t *testing.T) {
	input := `upstream 401 Authorization: Bearer sk-live api_key=sk-other password: p@ss token "jwt-value" safe message`

	got := RedactText(input, "sk-live", "jwt-value")
	for _, leaked := range []string{"sk-live", "sk-other", "p@ss", "jwt-value", "Bearer"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("RedactText() leaked %q in %s", leaked, got)
		}
	}
	if !strings.Contains(got, RedactedValue) || !strings.Contains(got, "safe message") {
		t.Fatalf("RedactText() = %s, want redacted secrets and preserved safe text", got)
	}
}
