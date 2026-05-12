package cloudwego

import "testing"

func TestRegistryEnabledRequiresEndpoints(t *testing.T) {
	_, err := KitexRegistryOptions(RegistryConfig{
		ServiceName: "iam",
		Addr:        "127.0.0.1:38082",
	})
	if err == nil {
		t.Fatal("expected missing endpoints error")
	}
}

func TestRegistryEnabledRequiresServiceName(t *testing.T) {
	_, err := KitexRegistryOptions(RegistryConfig{
		Endpoints: []string{"127.0.0.1:2379"},
		Addr:      "127.0.0.1:38082",
	})
	if err == nil {
		t.Fatal("expected missing service name error")
	}
}

func TestDiscoveryEnabledRequiresEndpoints(t *testing.T) {
	_, err := KitexClientOptions(DiscoveryConfig{})
	if err == nil {
		t.Fatal("expected missing endpoints error")
	}
}

func TestHertzRegistryEnabledRequiresAddr(t *testing.T) {
	_, err := HertzServerOptions(RegistryConfig{
		Endpoints:   []string{"127.0.0.1:2379"},
		ServiceName: "edge-api",
	}, "")
	if err == nil {
		t.Fatal("expected missing registry address error")
	}
}

func TestHertzServiceResolverRequiresServiceName(t *testing.T) {
	_, err := NewHertzServiceResolver(DiscoveryConfig{Endpoints: []string{"127.0.0.1:2379"}}, "")
	if err == nil {
		t.Fatal("expected missing service name error")
	}
}
