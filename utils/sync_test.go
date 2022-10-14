package utils

import (
	"fmt"
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

func TestCASConccurently(t *testing.T) {
	var wg sync.WaitGroup
	cas := NewGroupCAS()

	n := 1000
	key := "key"
	var sum int32
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()

			_, acq := cas.Acquire(fmt.Sprintf("%d", idx))
			require.True(t, acq)

			free, acquired := cas.Acquire(key)
			t.Log(acquired)
			if !acquired {
				return
			}

			// makes sure that there're only one thread has been acquired.
			require.True(t, atomic.CompareAndSwapInt32(&sum, 0, 1), atomic.LoadInt32(&sum))
			// marks there's no thread is acquired in advance.
			require.True(t, atomic.CompareAndSwapInt32(&sum, 1, 0))

			free()
		}(i)
	}

	wg.Wait()
}
