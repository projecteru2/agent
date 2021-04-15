package selfmon

import (
	"context"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	etcdtypes "go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/mvcc/mvccpb"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	"github.com/projecteru2/core/client"
	pb "github.com/projecteru2/core/rpc/gen"
	coremeta "github.com/projecteru2/core/store/etcdv3/meta"
	coretypes "github.com/projecteru2/core/types"
)

// ActiveKey .
const ActiveKey = "/selfmon/active"

// Selfmon .
type Selfmon struct {
	config *types.Config
	status sync.Map
	rpc    pb.CoreRPCClient
	etcd   coremeta.KV
	active utils.AtomicBool

	exit struct {
		sync.Once
		C chan struct{}
	}
}

// New .
func New(config *types.Config) (mon *Selfmon, err error) {
	mon = &Selfmon{}
	mon.config = config
	mon.exit.C = make(chan struct{}, 1)
	if mon.etcd, err = coremeta.NewETCD(config.Etcd, false); err != nil {
		return
	}

	var cc *client.Client
	if cc, err = client.NewClient(context.Background(), mon.config.Core, config.Auth); err != nil {
		return
	}
	mon.rpc = cc.GetRPCClient()

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
func (m *Selfmon) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go m.initNodeStatus(ctx)
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
		podNodes, err := m.rpc.ListPodNodes(cctx, &pb.ListNodesOptions{})
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
		status, err := m.rpc.GetNodeStatus(ctx, &pb.GetNodeStatusOptions{Nodename: n.Name})
		fakeMessage := &pb.NodeStatusStreamMessage{
			Nodename: n.Name,
			Podname:  n.Podname,
		}
		if err != nil || status == nil {
			fakeMessage.Alive = false
		} else {
			fakeMessage.Alive = status.Alive
		}
		m.dealNodeStatusMessage(fakeMessage)
	}
}

func (m *Selfmon) watchNodeStatus(ctx context.Context) {
	client, err := m.rpc.NodeStatusStream(ctx, &pb.Empty{})
	if err != nil {
		log.Errorf("[selfmon] watch node status failed %v", err)
		return
	}

	for {
		message, err := client.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf("[selfmon] read node events failed %v", err)
			return
		}
		go m.dealNodeStatusMessage(message)
	}
}

func (m *Selfmon) dealNodeStatusMessage(message *pb.NodeStatusStreamMessage) {
	if message.Error != "" {
		log.Errorf("[selfmon] deal with node status stream message failed %v", message.Error)
		return
	}

	defer m.status.Store(message.Nodename, message.Alive)

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
	ctx, cancel := context.WithTimeout(context.Background(), m.config.GlobalConnectionTimeout)
	defer cancel()
	if _, err := m.rpc.SetNode(ctx, &pb.SetNodeOptions{
		Nodename:      message.Nodename,
		StatusOpt:     opt,
		WorkloadsDown: !message.Alive,
	}); err != nil {
		log.Errorf("[selfmon] set node %s down failed %v", message.Nodename, err)
		return
	}
}

// Register .
func (m *Selfmon) Register() (func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
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

			if ne, un, err := m.register(); err != nil {
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

func (m *Selfmon) register() (<-chan struct{}, func(), error) {
	return m.etcd.StartEphemeral(context.Background(), ActiveKey, time.Second*16)
}

// Monitor .
func Monitor(config *types.Config) error {
	mon, err := New(config)
	if err != nil {
		return err
	}

	unregister, err := mon.Register()
	if err != nil {
		return err
	}
	defer unregister()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleSignals(mon)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		mon.Run()
	}()

	log.Infof("[selfmon] selfmon %p is running", mon)
	wg.Wait()

	log.Infof("[selfmon] selfmon %p is terminated", mon)
	return nil
}

// handleSignals .
func handleSignals(mon *Selfmon) {
	defer func() {
		log.Warnf("[selfmon] %p signals handler exit", mon)
		mon.Close()
	}()

	sch := make(chan os.Signal, 1)
	signal.Notify(sch, []os.Signal{
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGUSR2,
	}...)

	for {
		select {
		case sign := <-sch:
			switch sign {
			case syscall.SIGHUP, syscall.SIGUSR2:
				log.Warnf("[selfmon] recv signal %d to reload", sign)
				if err := mon.Reload(); err != nil {
					log.Errorf("[selfmon] reload %p failed %v", mon, err)
				}

			case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				log.Warnf("[selfmon] recv signal %d to exit", sign)
				return

			default:
				log.Warnf("[selfmon] recv signal %d to ignore", sign)
			}

		case <-mon.Exit():
			log.Warnf("[selfmon] recv from mon %p exit ch", mon)
			return
		}
	}
}
