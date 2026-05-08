package nsq

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/mq"
)

func TestProducer_Placeholder(t *testing.T) {
	p, err := NewProducer(Config{NSQDAddrs: []string{"127.0.0.1:4150"}})
	require.NoError(t, err)

	err = p.Publish(context.Background(), "orders", []byte("{}"))
	assert.True(t, errors.Is(err, errno.ErrNotImplemented))
	assert.NoError(t, p.Close())
}

func TestConsumer_Placeholder(t *testing.T) {
	c, err := NewConsumer(Config{LookupdAddrs: []string{"127.0.0.1:4161"}})
	require.NoError(t, err)

	err = c.Subscribe(context.Background(), "orders", func(context.Context, *mq.Message) error { return nil })
	assert.True(t, errors.Is(err, errno.ErrNotImplemented))
	assert.NoError(t, c.Close())
}
