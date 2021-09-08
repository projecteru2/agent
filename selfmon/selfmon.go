package selfmon

import (
	"context"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/store"
	corestore "github.com/projecteru2/agent/store/core"
	storemocks "github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"
	coremeta "github.com/projecteru2/core/store/etcdv3/meta"

	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// ActiveKey .
const ActiveKey = "/selfmon/active"

// Selfmon .
type Selfmon struct {
	config *types.Config
	status *cache.Cache
	store  store.Store
	kv     coremeta.KV
	id     int64

	exit struct {
		sync.Once
		C chan struct{}
	}
}

// New .
func New(ctx context.Context, config *types.Config) (mon *Selfmon, err error) {
	mon = &Selfmon{}
	mon.config = config
	mon.status = cache.New(time.Minute*5, time.Minute*15)
	mon.exit.C = make(chan struct{}, 1)
	mon.id = time.Now().UnixNano() / 1000 % 10000

	switch config.KV {
	case common.ETCDKV:
		if mon.kv, err = coremeta.NewETCD(config.Etcd, nil); err != nil {
			log.Errorf("[selfmon] failed to get etcd client, err: %s", err)
			return nil, err
		}
	case common.MocksKV:
		log.Debugf("[selfmon] use embedded ETCD")
		mon.kv = nil
	default:
		return nil, errors.New("unknown kv type")
	}

	switch config.Store {
	case common.GRPCStore:
		corestore.Init(ctx, config)
		mon.store = corestore.Get()
		if mon.store == nil {
			log.Errorf("[selfmon] failed to get core store")
			return nil, errors.New("failed to get core store")
		}
	case common.MocksStore:
		mon.store = storemocks.FromTemplate()
	default:
		return nil, errors.New("unknown store type")
	}

	return mon, nil
}

// Monitor .
func (m *Selfmon) Monitor(ctx context.Context) {
	go m.watchNodeStatus(ctx)
	log.Infof("[selfmon] selfmon %v is running", m.id)
	<-ctx.Done()
	log.Warnf("[selfmon] m %v monitor stops", m.id)
}

// Run .
func (m *Selfmon) Run(ctx context.Context) error {
	go m.handleSignals(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-m.Exit():
			return nil
		default:
			m.WithActiveLock(ctx, func(ctx context.Context) {
				m.Monitor(ctx)
			})
		}
	}
}

// Exit .
func (m *Selfmon) Exit() <-chan struct{} {
	return m.exit.C
}

// Close .
func (m *Selfmon) Close() {
	m.exit.Do(func() {
		close(m.exit.C)
	})
}

// Reload .
func (m *Selfmon) Reload() error {
	return nil
}

// handleSignals .
func (m *Selfmon) handleSignals(ctx context.Context) {
	var reloadCtx context.Context
	var cancel1 context.CancelFunc
	defer func() {
		log.Warnf("[selfmon] %v signals handler exit", m.id)
		cancel1()
		m.Close()
	}()

	reloadCtx, cancel1 = signal.NotifyContext(ctx, syscall.SIGHUP, syscall.SIGUSR2)
	exitCtx, cancel2 := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel2()

	for {
		select {
		case <-m.Exit():
			log.Warnf("[selfmon] recv from m %v exit ch", m.id)
			return
		case <-exitCtx.Done():
			log.Warn("[selfmon] recv signal to exit")
			return
		case <-reloadCtx.Done():
			log.Warn("[selfmon] recv signal to reload")
			if err := m.Reload(); err != nil {
				log.Errorf("[selfmon] reload %v failed %v", m.id, err)
			}
			reloadCtx, cancel1 = signal.NotifyContext(ctx, syscall.SIGHUP, syscall.SIGUSR2)
		}
	}
}
