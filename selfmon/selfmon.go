package selfmon

import (
	"context"
	"net"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/projecteru2/core/client"
	pb "github.com/projecteru2/core/rpc/gen"

	"github.com/projecteru2/agent/types"
)

// Selfmon .
type Selfmon struct {
	config *types.Config
	nodes  sync.Map
	deads  sync.Map
	core   *client.Client

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
	mon.core, err = client.NewClient(context.Background(), mon.config.Core, config.Auth)
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
		go m.detector(i, ch)
	}

	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	dispatch := func() {
		timer.Reset(time.Second * 16)

		m.nodes.Range(func(key, value interface{}) bool {
			if node, ok := value.(*pb.Node); ok {
				ch <- node
			} else {
				log.Errorf("[selfmon] %p is not a *pb.Node, but %v", value, value)
			}
			return true
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

func (m *Selfmon) detector(ident int, recv <-chan *pb.Node) {
	for {
		select {
		case node := <-recv:
			log.Debugf("[self] detector %d recv node %s/%s", ident, node.Name, node.Endpoint)
			if err := m.detect(node); err != nil {
				m.deads.Store(node.Name, node)
			}

		case <-m.exit.C:
			return
		}
	}
}

func (m *Selfmon) report() {
	cli := m.core.GetRPCClient()

	names := []string{}

	m.deads.Range(func(key, value interface{}) bool {
		if nm, ok := key.(string); ok {
			names = append(names, nm)
		} else {
			log.Errorf("[selfmon] %v is not a string", key)
			return true
		}

		node, ok := value.(*pb.Node)
		if !ok {
			log.Errorf("[selfmon] %p is not a *pb.Node, but %v", value, value)
			return true
		}

		if _, err := cli.SetNode(context.Background(), &pb.SetNodeOptions{
			Nodename:       node.Name,
			StatusOpt:      pb.TriOpt_FALSE,
			ContainersDown: true,
		}); err != nil {
			log.Errorf("[selfmon] set node %s down failed %v", node.Name, err)
			return true
		}

		log.Infof("[selfmon] report %s/%s is dead", node.Name, node.Endpoint)
		return true
	})

	// Clearing them all out after the report.
	for _, nm := range names {
		m.deads.Delete(nm)
	}
}

func (m *Selfmon) detect(node *pb.Node) error {
	timeout := time.Second * time.Duration(m.config.HealthCheckTimeout)

	addr, err := m.parseEndpoint(node.Endpoint)
	if err != nil {
		return err
	}

	dial := func() error {
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err == nil {
			conn.Close()
		}
		return err
	}

	last := 3
	for i := 0; i <= last; i++ {
		if err = dial(); err == nil {
			log.Debugf("[selfmon] dial %s/%s ok", node.Name, node.Endpoint)
			break
		}

		log.Debugf("[selfmon] %d dial %s failed %v", i, node.Name, err)

		if i < last {
			time.Sleep(time.Second * (1 << i))
		}
	}

	return err
}

func (m *Selfmon) parseEndpoint(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}

func (m *Selfmon) watch() {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	watch := func() {
		defer timer.Reset(time.Second * 8)

		// Get all nodes which are active status, and regardless of pod.
		podNodes, err := m.core.GetRPCClient().ListPodNodes(context.Background(), &pb.ListNodesOptions{})
		if err != nil {
			log.Errorf("[selfmon] get pod nodes from %s failed %v", m.config.Core, err)
			return
		}

		var count int
		for _, n := range podNodes.Nodes {
			log.Debugf("[selfmon] watched %s/%s", n.Name, n.Endpoint)
			m.nodes.LoadOrStore(n.Name, n)
			count++
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

// Monitor .
func Monitor(config *types.Config) error {
	mon, err := New(config)
	if err != nil {
		return err
	}

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
