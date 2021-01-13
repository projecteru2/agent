package selfmon

import (
	"context"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	etcdtypes "go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/mvcc/mvccpb"

	"github.com/projecteru2/core/client"
	pb "github.com/projecteru2/core/rpc/gen"
	coremeta "github.com/projecteru2/core/store/etcdv3/meta"
	coretypes "github.com/projecteru2/core/types"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

// ActiveKey .
const ActiveKey = "/selfmon/active"

// Selfmon .
type Selfmon struct {
	config        *types.Config
	nodes         sync.Map
	deads         sync.Map
	rpc           pb.CoreRPCClient
	etcd          coremeta.KV
	active        utils.AtomicBool
	checkInterval time.Duration
	detector      Detector

	exit struct {
		sync.Once
		C chan struct{}
	}
}

// New .
func New(config *types.Config) (mon *Selfmon, err error) {
	mon = &Selfmon{}
	mon.checkInterval = time.Second * 8
	mon.config = config
	mon.detector = coreDetector{mon}
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
	go m.watch()

	ch := make(chan *pb.Node)
	for i := 0; i < 10; i++ {
		go m.detect(i, ch)
	}

	timer := time.NewTimer(1)
	defer timer.Stop()

	dispatch := func() {
		timer.Reset(m.checkInterval)

		m.nodes.Range(func(key, value interface{}) bool {
			node, ok := value.(*pb.Node)
			if !ok {
				log.Errorf("[selfmon] %p is not a *pb.Node, but %v", value, value)
				return true
			}

			select {
			case ch <- node:
				return true
			case <-m.exit.C:
				return false
			}
		})

		m.report()
	}

	for {
		select {
		case <-timer.C:
			dispatch()

		case <-m.exit.C:
			log.Warnf("[selfmon] exit from %p main loop", m)
			return
		}
	}
}

func (m *Selfmon) detect(ident int, recv <-chan *pb.Node) {
	for {
		select {
		case node := <-recv:
			log.Debugf("[selfmon] detector %d recv node %s/%s", ident, node.Name, node.Endpoint)
			if err := m.detector.Detect(node); err != nil {
				m.deads.Store(node.Name, node)
			}

		case <-m.exit.C:
			log.Warnf("[selfmon] exit from detector %d", ident)
			return
		}
	}
}

func (m *Selfmon) report() {
	keys := []interface{}{}

	m.deads.Range(func(key, value interface{}) bool {
		keys = append(keys, key)
		if _, ok := key.(string); !ok {
			log.Errorf("[selfmon] %v is not a string", key)
			return true
		}
		if !m.active.Bool() {
			log.Errorf("[selfmon] %p is not active yet", m)
			return true
		}

		node, ok := value.(*pb.Node)
		if !ok {
			log.Errorf("[selfmon] %p is not a *pb.Node, but %v", value, value)
			return true
		}

		if _, err := m.rpc.SetNode(context.Background(), &pb.SetNodeOptions{
			Nodename:      node.Name,
			StatusOpt:     pb.TriOpt_FALSE,
			WorkloadsDown: true,
		}); err != nil {
			log.Errorf("[selfmon] set node %s down failed %v", node.Name, err)
			return true
		}

		log.Infof("[selfmon] report %s/%s is dead", node.Name, node.Endpoint)
		return true
	})

	// Clearing them all out after the report.
	for _, nm := range keys {
		m.deads.Delete(nm)
	}
}

func (m *Selfmon) parseEndpoint(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}

func (m *Selfmon) watch() {
	timer := time.NewTimer(1)
	defer timer.Stop()

	watch := func() {
		defer timer.Reset(m.checkInterval * 8)

		// Get all nodes which are active status, and regardless of pod.
		podNodes, err := m.rpc.ListPodNodes(context.Background(), &pb.ListNodesOptions{})
		if err != nil {
			log.Errorf("[selfmon] get pod nodes from %s failed %v", m.config.Core, err)
			return
		}

		for _, n := range podNodes.Nodes {
			log.Debugf("[selfmon] watched %s/%s", n.Name, n.Endpoint)
			m.nodes.LoadOrStore(n.Name, n)
		}
	}

	for {
		select {
		case <-timer.C:
			watch()
		case <-m.exit.C:
			log.Warnf("[selfmon] exit from %p watch", m)
			return
		}
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
				if err != coretypes.ErrKeyExists {
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
