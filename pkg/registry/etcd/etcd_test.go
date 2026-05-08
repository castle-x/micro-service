package etcd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/registry"
)

func TestRegistry_Placeholder(t *testing.T) {
	r, err := NewRegistry(Config{Endpoints: []string{"localhost:2379"}})
	require.NoError(t, err)

	err = r.Register(context.Background(), registry.ServiceInfo{Name: "idp"})
	assert.True(t, errors.Is(err, errno.ErrNotImplemented))

	err = r.Deregister(context.Background(), registry.ServiceInfo{Name: "idp"})
	assert.True(t, errors.Is(err, errno.ErrNotImplemented))
	assert.NoError(t, r.Close())
}

func TestResolver_Placeholder(t *testing.T) {
	r, err := NewResolver(Config{Endpoints: []string{"localhost:2379"}})
	require.NoError(t, err)

	_, err = r.Resolve(context.Background(), "idp")
	assert.True(t, errors.Is(err, errno.ErrNotImplemented))
	assert.NoError(t, r.Close())
}
