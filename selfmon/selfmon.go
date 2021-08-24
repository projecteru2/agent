package selfmon

import (
	"context"
	"io"
	"os/signal"
	"sync"
	"syscall"
	"time"

	corestore "github.com/projecteru2/agent/store/core"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	pb "github.com/projecteru2/core/rpc/gen"
	coremeta "github.com/projecteru2/core/store/etcdv3/meta"
	coretypes "github.com/projecteru2/core/types"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"go.etcd.io/etcd/api/v3/mvccpb"
	etcdtypes "go.etcd.io/etcd/client/v3"
)

// ActiveKey .
const ActiveKey = "/selfmon/active"

// Selfmon .
type Selfmon struct {
	config *types.Config
	status sync.Map
	rpc    corestore.RPCClientPool
	etcd   coremeta.KV
	active utils.AtomicBool

	exit struct {
		sync.Once
		C chan struct{}
	}
}

// New .
func New(ctx context.Context, config *types.Config) (mon *Selfmon, err error) {
	mon = &Selfmon{}
	mon.config = config
	mon.exit.C = make(chan struct{}, 1)
	if mon.etcd, err = coremeta.NewETCD(config.Etcd, nil); err != nil {
		return
	}

	if mon.rpc, err = corestore.NewCoreRPCClientPool(ctx, mon.config); err != nil {
		log.Errorf("[selfmon] no core rpc connection")
		return
	}

	return
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

// Run .
func (m *Selfmon) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go m.watchNodeStatus(ctx)

	<-m.Exit()
	log.Warnf("[selfmon] exit from %p main loop", m)
}

func (m *Selfmon) initNodeStatus(ctx context.Context) {
	nodes := make(chan *pb.Node)

	go func() {
		defer close(nodes)
		// Get all nodes which are active status, and regardless of pod.
		cctx, cancel := context.WithTimeout(ctx, m.config.GlobalConnectionTimeout)
		defer cancel()
		podNodes, err := m.rpc.GetClient().ListPodNodes(cctx, &pb.ListNodesOptions{All: true})
		if err != nil {
			log.Errorf("[selfmon] get pod nodes from %s failed %v", m.config.Core, err)
			return
		}

		for _, n := range podNodes.Nodes {
			log.Debugf("[selfmon] watched %s/%s", n.Name, n.Endpoint)
			nodes <- n
		}
	}()

	for n := range nodes {
		status, err := m.rpc.GetClient().GetNodeStatus(ctx, &pb.GetNodeStatusOptions{Nodename: n.Name})
		fakeMessage := &pb.NodeStatusStreamMessage{
			Nodename: n.Name,
			Podname:  n.Podname,
		}
		fakeMessage.Alive = !(err != nil || status == nil) && status.Alive
		m.dealNodeStatusMessage(ctx, fakeMessage)
	}
}

func (m *Selfmon) watchNodeStatus(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Infof("[selfmon] stop watching node status")
			return
		default:
			go m.initNodeStatus(ctx)
			if m.watch(ctx) != nil {
				log.Debugf("[selfmon] retry to watch node status")
				time.Sleep(m.config.GlobalConnectionTimeout)
			}
		}
	}
}

func (m *Selfmon) watch(ctx context.Context) error {
	client, err := m.rpc.GetClient().NodeStatusStream(ctx, &pb.Empty{})
	if err != nil {
		log.Errorf("[selfmon] watch node status failed %v", err)
		return err
	}
	log.Debugf("[selfmon] watch node status started")
	defer log.Debugf("[selfmon] stop watching node status")

	for {
		message, err := client.Recv()
		if err == io.EOF {
			log.Debugf("[selfmon] server closed the stream")
			return err
		}
		if err != nil {
			log.Errorf("[selfmon] read node events failed %v", err)
			return err
		}
		go m.dealNodeStatusMessage(ctx, message)
	}
}

func (m *Selfmon) dealNodeStatusMessage(ctx context.Context, message *pb.NodeStatusStreamMessage) {
	if message.Error != "" {
		log.Errorf("[selfmon] deal with node status stream message failed %v", message.Error)
		return
	}

	lastValue, ok := m.status.Load(message.Nodename)
	if ok {
		last, o := lastValue.(bool)
		if o && last == message.Alive {
			return
		}
	}

	var opt pb.TriOpt
	if message.Alive {
		opt = pb.TriOpt_TRUE
	} else {
		opt = pb.TriOpt_FALSE
	}

	// TODO maybe we need a distributed lock to control concurrency
	ctx, cancel := context.WithTimeout(ctx, m.config.GlobalConnectionTimeout)
	defer cancel()
	if _, err := m.rpc.GetClient().SetNode(ctx, &pb.SetNodeOptions{
		Nodename:      message.Nodename,
		StatusOpt:     opt,
		WorkloadsDown: !message.Alive,
	}); err != nil {
		log.Errorf("[selfmon] set node %s failed %v", message.Nodename, err)
		return
	}

	m.status.Store(message.Nodename, message.Alive)
}

// Register .
func (m *Selfmon) Register(ctx context.Context) (func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	del := make(chan struct{}, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	// Watching the active key permanently.
	go func() {
		defer wg.Done()
		defer close(del)

		handleResp := func(resp etcdtypes.WatchResponse) {
			if err := resp.Err(); err != nil {
				if resp.Canceled {
					log.Infof("[Register] watching is canceled")
					return
				}
				log.Errorf("[Register] watch failed: %v", err)
				time.Sleep(time.Second)
				return
			}

			for _, ev := range resp.Events {
				if ev.Type == mvccpb.DELETE {
					select {
					case del <- struct{}{}:
					case <-ctx.Done():
						return
					}
				}
			}
		}

		for {
			select {
			case <-ctx.Done():
				log.Infof("[Register] watching done")
				return
			case resp := <-m.etcd.Watch(ctx, ActiveKey):
				handleResp(resp)
			}
		}
	}()

	wg.Add(1)
	// Always trying to register if the selfmon is alive.
	go func() {
		var expiry <-chan struct{}
		unregister := func() {}

		defer func() {
			m.active.Unset()
			unregister()
			wg.Done()
		}()

		for {
			m.active.Unset()

			// We have to put a single <-ctx.Done() here to avoid it may be starved
			// while it combines with <-expiry and <-del.
			select {
			case <-ctx.Done():
				log.Infof("[Register] register done: %v", ctx.Err())
				return
			default:
			}

			if ne, un, err := m.etcd.StartEphemeral(ctx, ActiveKey, m.config.HAKeepaliveInterval); err != nil {
				if !errors.Is(err, coretypes.ErrKeyExists) {
					log.Errorf("[Register] failed to re-register: %v", err)
					time.Sleep(time.Second)
					continue
				}
				log.Infof("[Register] there has been another active selfmon")
			} else {
				log.Infof("[Register] the agent has been active")
				expiry = ne
				unregister = un
				m.active.Set()
			}

			// Though there's a standalone <-ctx.Done() above, we still need <-ctx.Done()
			// in this select block to make sure the select could be terminated
			// once the ctx is done during hang together.
			select {
			case <-ctx.Done():
				log.Infof("[Register] register done: %v", ctx.Err())
				return
			case <-expiry:
				log.Infof("[Register] the original active selfmon has been expired")
			case <-del:
				log.Infof("[Register] The original active Selfmon is terminated")
			}
		}
	}()

	return func() {
		cancel()
		wg.Wait()
	}, nil
}

// Monitor .
func Monitor(ctx context.Context, config *types.Config) error {
	mon, err := New(ctx, config)
	if err != nil {
		return err
	}

	unregister, err := mon.Register(ctx)
	if err != nil {
		return err
	}
	defer unregister()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		handleSignals(ctx, mon)
	}()

	go func() {
		defer wg.Done()
		mon.Run(ctx)
	}()

	log.Infof("[selfmon] selfmon %p is running", mon)
	wg.Wait()

	log.Infof("[selfmon] selfmon %p is terminated", mon)
	return nil
}

// handleSignals .
func handleSignals(ctx context.Context, mon *Selfmon) {
	var reloadCtx context.Context
	var cancel1 context.CancelFunc
	defer func() {
		log.Warnf("[selfmon] %p signals handler exit", mon)
		cancel1()
		mon.Close()
	}()

	reloadCtx, cancel1 = signal.NotifyContext(ctx, syscall.SIGHUP, syscall.SIGUSR2)
	exitCtx, cancel2 := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel2()

	for {
		select {
		case <-mon.Exit():
			log.Warnf("[selfmon] recv from mon %p exit ch", mon)
			return
		case <-exitCtx.Done():
			log.Warn("[selfmon] recv signal to exit")
			return
		case <-reloadCtx.Done():
			log.Warn("[selfmon] recv signal to reload")
			if err := mon.Reload(); err != nil {
				log.Errorf("[selfmon] reload %p failed %v", mon, err)
			}
			reloadCtx, cancel1 = signal.NotifyContext(ctx, syscall.SIGHUP, syscall.SIGUSR2)
		}
	}
}
