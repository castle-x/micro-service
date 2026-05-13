package biz

import (
	"context"
	"testing"
)

func TestHealthBiz_Check(t *testing.T) {
	service, status, err := NewHealthBiz().Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if service != ServiceName {
		t.Fatalf("service = %q, want %q", service, ServiceName)
	}
	if status != HealthStatusOK {
		t.Fatalf("status = %q, want %q", status, HealthStatusOK)
	}
}
