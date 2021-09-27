package selfmon

import (
	"context"
	"time"

	coretypes "github.com/projecteru2/core/types"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// WithActiveLock acquires the active lock synchronously
func (m *Selfmon) WithActiveLock(parentCtx context.Context, f func(ctx context.Context)) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	var expiry <-chan struct{}
	var unregister func()
	defer func() {
		if unregister != nil {
			log.Infof("[Register] %v unregisters", m.id)
			unregister()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.Info("[Register] context canceled")
			return
		case <-m.Exit():
			log.Infof("[Register] selfmon %v closed", m.id)
			return
		default:
		}

		// try to get the lock
		if ne, un, err := m.register(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Info("[Register] context canceled")
				return
			} else if !errors.Is(err, coretypes.ErrKeyExists) {
				log.Errorf("[Register] failed to re-register: %v", err)
				time.Sleep(time.Second)
				continue
			}
			log.Infof("[Register] %v there has been another active selfmon", m.id)
			time.Sleep(time.Second)
		} else {
			log.Infof("[Register] the agent %v has been active", m.id)
			expiry = ne
			unregister = un
			break
		}
	}

	// cancel the ctx when: 1. selfmon closed 2. lost the active lock
	go func() {
		defer cancel()

		select {
		case <-ctx.Done():
			log.Info("[Register] context canceled")
			return
		case <-m.Exit():
			log.Infof("[Register] selfmon %v closed", m.id)
			return
		case <-expiry:
			log.Info("[Register] lock expired")
			return
		}
	}()

	f(ctx)
}

func (m *Selfmon) register(ctx context.Context) (<-chan struct{}, func(), error) {
	return m.kv.StartEphemeral(ctx, ActiveKey, m.config.HAKeepaliveInterval)
}
