package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/castlexu/micro-service/pkg/errno"
)

func TestNotImplementedRegistry(t *testing.T) {
	var r Registry = NotImplementedRegistry{}
	err := r.Register(context.Background(), ServiceInfo{Name: "idp"})
	assert.True(t, errors.Is(err, errno.ErrNotImplemented))

	err = r.Deregister(context.Background(), ServiceInfo{Name: "idp"})
	assert.True(t, errors.Is(err, errno.ErrNotImplemented))

	assert.NoError(t, r.Close())
}

func TestNotImplementedResolver(t *testing.T) {
	var r Resolver = NotImplementedResolver{}
	_, err := r.Resolve(context.Background(), "idp")
	assert.True(t, errors.Is(err, errno.ErrNotImplemented))
	assert.NoError(t, r.Close())
}
