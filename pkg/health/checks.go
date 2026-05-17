package health

import (
	"context"
	"errors"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/castlexu/micro-service/pkg/db"
	pkgredis "github.com/castlexu/micro-service/pkg/redis"
)

// MongoCheck returns a readiness check for the shared MongoDB client.
func MongoCheck(client *db.Client) CheckFunc {
	return func(ctx context.Context) error {
		if client == nil {
			return errors.New("mongo client is nil")
		}
		return client.Ping(ctx)
	}
}

// RedisCheck returns a readiness check for the shared Redis client.
func RedisCheck(client *pkgredis.Client) CheckFunc {
	return func(ctx context.Context) error {
		if client == nil {
			return errors.New("redis client is nil")
		}
		return client.Ping(ctx)
	}
}

// EtcdCheck returns a readiness check for an etcd client.
func EtcdCheck(client *clientv3.Client) CheckFunc {
	return func(ctx context.Context) error {
		if client == nil {
			return errors.New("etcd client is nil")
		}
		endpoints := client.Endpoints()
		if len(endpoints) == 0 {
			return errors.New("etcd endpoints are empty")
		}
		_, err := client.Status(ctx, endpoints[0])
		return err
	}
}
