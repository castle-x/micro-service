package main

import "testing"

func TestResolveEncryptKeyPrefersEnv(t *testing.T) {
	envKey := "env-encrypt-key-32-bytes-pad-123"
	cfgKey := "cfg-encrypt-key-32-bytes-pad-456"

	got, err := resolveEncryptKey(envKey, cfgKey)
	if err != nil {
		t.Fatalf("resolveEncryptKey returned error: %v", err)
	}
	if string(got) != envKey[:32] {
		t.Fatalf("got %q, want env key prefix %q", string(got), envKey[:32])
	}
}

func TestResolveEncryptKeyFallsBackToConfig(t *testing.T) {
	cfgKey := "cfg-encrypt-key-32-bytes-pad-456"

	got, err := resolveEncryptKey("", cfgKey)
	if err != nil {
		t.Fatalf("resolveEncryptKey returned error: %v", err)
	}
	if string(got) != cfgKey[:32] {
		t.Fatalf("got %q, want config key prefix %q", string(got), cfgKey[:32])
	}
}

func TestResolveEncryptKeyRejectsMissingOrShortKey(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
		cfgKey string
	}{
		{name: "missing", envKey: "", cfgKey: ""},
		{name: "env short", envKey: "short", cfgKey: ""},
		{name: "config short", envKey: "", cfgKey: "short"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := resolveEncryptKey(tt.envKey, tt.cfgKey); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
