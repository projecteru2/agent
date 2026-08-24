package utils

import (
	"context"
	"time"

	"github.com/projecteru2/core/log"
)

type RetryTask struct {
	ctx         context.Context
	cancel      context.CancelFunc
	Func        func() error
	MaxAttempts int
}

func NewRetryTask(ctx context.Context, maxAttempts int, f func() error) *RetryTask {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	ctx, cancel := context.WithCancel(ctx)
	return &RetryTask{
		ctx:         ctx,
		cancel:      cancel,
		MaxAttempts: maxAttempts,
		Func:        f,
	}
}

func (r *RetryTask) Run(ctx context.Context) error {
	logger := log.WithFunc("Run")
	logger.Debug(ctx, "start")
	defer r.Stop(ctx)

	var err error
	interval := 1
	timer := time.NewTimer(0)
	defer timer.Stop()

	for range r.MaxAttempts {
		select {
		case <-r.ctx.Done():
			logger.Debug(ctx, "abort")
			return r.ctx.Err()
		case <-timer.C:
			err = r.Func()
			if err == nil {
				return nil
			}
			logger.Debugf(ctx, "will retry after %v seconds", interval)
			timer.Reset(time.Duration(interval) * time.Second)
			interval *= 2
		}
	}
	return err
}

func (r *RetryTask) Stop(context.Context) {
	r.cancel()
}

// BackoffRetry runs f up to maxAttempts times, doubling the wait after each failure.
func BackoffRetry(ctx context.Context, maxAttempts int, f func() error) error {
	retryTask := NewRetryTask(ctx, maxAttempts, f)
	defer retryTask.Stop(ctx)
	return retryTask.Run(ctx)
}
