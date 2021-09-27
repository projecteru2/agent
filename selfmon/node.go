package selfmon

import (
	"context"
	"io"
	"math/rand"
	"time"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	log "github.com/sirupsen/logrus"
)

func (m *Selfmon) initNodeStatus(ctx context.Context) {
	log.Debug("[selfmon] init node status started")
	nodes := make(chan *types.Node)

	go func() {
		defer close(nodes)
		// Get all nodes which are active status, and regardless of pod.
		var podNodes []*types.Node
		var err error
		utils.WithTimeout(ctx, m.config.GlobalConnectionTimeout, func(ctx context.Context) {
			podNodes, err = m.store.ListPodNodes(ctx, true, "", nil)
		})
		if err != nil {
			log.Errorf("[selfmon] get pod nodes failed %v", err)
			return
		}

		for _, n := range podNodes {
			log.Debugf("[selfmon] watched %s/%s", n.Name, n.Endpoint)
			nodes <- n
		}
	}()

	for n := range nodes {
		status, err := m.store.GetNodeStatus(ctx, n.Name)
		if err != nil {
			status = &types.NodeStatus{
				Nodename: n.Name,
				Podname:  n.Podname,
				Alive:    false,
			}
		}
		m.dealNodeStatusMessage(ctx, status)
	}
}

func (m *Selfmon) watchNodeStatus(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Infof("[selfmon] %v stop watching node status", m.id)
			return
		default:
			time.Sleep(time.Second)
			go m.initNodeStatus(ctx)
			if m.watch(ctx) != nil {
				log.Debug("[selfmon] retry to watch node status")
				time.Sleep(m.config.GlobalConnectionTimeout)
			}
		}
	}
}

func (m *Selfmon) watch(ctx context.Context) error {
	messageChan, errChan := m.store.NodeStatusStream(ctx)
	log.Debug("[selfmon] watch node status started")
	defer log.Debug("[selfmon] stop watching node status")

	for {
		select {
		case message := <-messageChan:
			go m.dealNodeStatusMessage(ctx, message)
		case err := <-errChan:
			if err == io.EOF {
				log.Debug("[selfmon] server closed the stream")
				return err
			}
			log.Debugf("[selfmon] read node status failed, err: %s", err)
			return err
		}
	}
}

func (m *Selfmon) dealNodeStatusMessage(ctx context.Context, message *types.NodeStatus) {
	if message.Error != nil {
		log.Errorf("[selfmon] deal with node status stream message failed %+v", message)
		return
	}

	lastValue, ok := m.status.Get(message.Nodename)
	if ok {
		last, o := lastValue.(bool)
		if o && last == message.Alive {
			return
		}
	}

	// TODO maybe we need a distributed lock to control concurrency
	var err error
	utils.WithTimeout(ctx, m.config.GlobalConnectionTimeout, func(ctx context.Context) {
		err = m.store.SetNode(ctx, message.Nodename, message.Alive)
	})

	if err != nil {
		log.Errorf("[selfmon] set node %s failed %v", message.Nodename, err)
		m.status.Delete(message.Nodename)
		return
	}
	log.Debugf("[selfmon] set node %s as alive: %v", message.Nodename, message.Alive)

	m.status.Set(message.Nodename, message.Alive, time.Duration(300+rand.Intn(100))*time.Second) // nolint
}
