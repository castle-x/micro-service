package cloudwego

import (
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func resetSharedEtcdClientsForTest(t *testing.T) {
	t.Helper()
	reset := func() {
		sharedEtcdClients.Lock()
		defer sharedEtcdClients.Unlock()
		for _, client := range sharedEtcdClients.clients {
			_ = client.Close()
		}
		sharedEtcdClients.clients = make(map[string]*clientv3.Client)
	}
	reset()
	t.Cleanup(reset)
}

func TestSharedEtcdClientRequiresEndpoints(t *testing.T) {
	_, err := SharedEtcdClient(nil)
	if err == nil {
		t.Fatal("expected missing endpoints error")
	}
}

func TestSharedEtcdClientReusesClientForSameEndpoints(t *testing.T) {
	resetSharedEtcdClientsForTest(t)

	first, err := SharedEtcdClient([]string{"127.0.0.1:12379"})
	if err != nil {
		t.Fatalf("first shared etcd client: %v", err)
	}
	second, err := SharedEtcdClient([]string{"127.0.0.1:12379"})
	if err != nil {
		t.Fatalf("second shared etcd client: %v", err)
	}
	if first != second {
		t.Fatal("expected shared etcd client to be reused")
	}
}
