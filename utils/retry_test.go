package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBackoffRetry(t *testing.T) {
	i := 0
	f := func() error {
		i++
		if i < 4 {
			return errors.New("xxx")
		}
		return nil
	}
	assert.Nil(t, BackoffRetry(context.Background(), 10, f))
	assert.Equal(t, 4, i)
}
