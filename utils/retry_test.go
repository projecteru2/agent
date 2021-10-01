package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackoffRetry(t *testing.T) {
	var errNotSuccess = errors.New("not success")
	i := 0
	f := func() error {
		i++
		if i < 4 {
			return errNotSuccess
		}
		return nil
	}
	assert.Nil(t, BackoffRetry(context.Background(), 10, f))
	assert.Equal(t, 4, i)

	i = 0
	assert.Equal(t, errNotSuccess, BackoffRetry(context.Background(), 0, f))
	assert.Equal(t, 1, i)
}

func TestBackoffRetryWithCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var errNotSuccess = errors.New("not success")
	i := 0
	f := func() error {
		i++
		if i < 4 {
			return errNotSuccess
		}
		return nil
	}
	assert.Equal(t, context.DeadlineExceeded, BackoffRetry(ctx, 10, f))
	assert.NotEqual(t, 4, i)
}
