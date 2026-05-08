package mq

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/castlexu/micro-service/pkg/errno"
)

func TestNotImplementedProducer(t *testing.T) {
	var p Producer = NotImplementedProducer{}
	err := p.Publish(context.Background(), "t", []byte("x"))
	assert.True(t, errors.Is(err, errno.ErrNotImplemented))
	assert.NoError(t, p.Close())
}

func TestNotImplementedConsumer(t *testing.T) {
	var c Consumer = NotImplementedConsumer{}
	err := c.Subscribe(context.Background(), "t", func(context.Context, *Message) error { return nil })
	assert.True(t, errors.Is(err, errno.ErrNotImplemented))
	assert.NoError(t, c.Close())
}
