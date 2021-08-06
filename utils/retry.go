package utils

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// BackoffRetry retries up to `maxAttempts` times, and the interval will grow exponentially
func BackoffRetry(ctx context.Context, maxAttempts int, f func() error) error {
	t := time.NewTimer(0)
	var err error
	// make sure to execute at least once
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	interval := 1

	for i := 0; i < maxAttempts; i++ {
		select {
		case <-t.C:
			if err = f(); err == nil {
				return nil
			}
			log.Debugf("[backoffRetry] will retry after %d seconds", interval)
			t.Reset(time.Duration(interval) * time.Second)
			interval *= 2
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}
