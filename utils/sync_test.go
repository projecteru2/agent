package utils

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCAS(t *testing.T) {
	key := "key"
	cas := NewGroupCAS()

	free, acquired := cas.Acquire(key)
	require.True(t, acquired)
	require.NotNil(t, free)

	free1, acquired1 := cas.Acquire(key)
	require.False(t, acquired1)
	require.Nil(t, free1)

	free()
	free1, acquired1 = cas.Acquire(key)
	require.True(t, acquired1)
	require.NotNil(t, free1)
}

func TestCASConcurrently(t *testing.T) {
	const n = 1000
	key := "key"
	cas := NewGroupCAS()
	start := make(chan struct{})

	var wg sync.WaitGroup
	var held, acquisitions atomic.Int32

	for i := range n {
		wg.Go(func() {
			_, acquiredOwn := cas.Acquire(strconv.Itoa(i))
			require.True(t, acquiredOwn)

			<-start
			free, acquired := cas.Acquire(key)
			if !acquired {
				return
			}
			acquisitions.Add(1)
			require.Equal(t, int32(1), held.Add(1), "two callers hold the same key")
			held.Add(-1)
			free()
		})
	}

	close(start)
	wg.Wait()
	require.Positive(t, acquisitions.Load())
}
