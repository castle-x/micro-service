package cloudwego

import (
	"errors"
	"sort"
	"strings"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var sharedEtcdClients = struct {
	sync.Mutex
	clients map[string]*clientv3.Client
}{
	clients: make(map[string]*clientv3.Client),
}

// SharedEtcdClient returns a cached etcd client for CloudWeGo health checks.
func SharedEtcdClient(endpoints []string) (*clientv3.Client, error) {
	normalized, key, err := normalizeEtcdEndpoints(endpoints)
	if err != nil {
		return nil, err
	}

	sharedEtcdClients.Lock()
	defer sharedEtcdClients.Unlock()

	if client := sharedEtcdClients.clients[key]; client != nil {
		return client, nil
	}
	client, err := clientv3.New(clientv3.Config{Endpoints: normalized})
	if err != nil {
		return nil, err
	}
	sharedEtcdClients.clients[key] = client
	return client, nil
}

func normalizeEtcdEndpoints(endpoints []string) ([]string, string, error) {
	normalized := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		endpoint = strings.TrimSpace(endpoint)
		if endpoint != "" {
			normalized = append(normalized, endpoint)
		}
	}
	if len(normalized) == 0 {
		return nil, "", errors.New("etcd endpoints are required")
	}
	sort.Strings(normalized)
	return normalized, strings.Join(normalized, ","), nil
}
