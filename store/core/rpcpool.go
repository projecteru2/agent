package corestore

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/core/client"
	pb "github.com/projecteru2/core/rpc/gen"

	log "github.com/sirupsen/logrus"
)

// RPCClientPool pool of core rpc client
type RPCClientPool interface {
	GetClient() pb.CoreRPCClient
}

type clientWithStatus struct {
	client pb.CoreRPCClient
	addr   string
	alive  bool
}

// CoreRPCClientPool implement of RPCClientPool
type CoreRPCClientPool struct {
	rpcClients []*clientWithStatus
}

func checkAlive(ctx context.Context, rpc *clientWithStatus, timeout time.Duration) bool {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := rpc.client.Info(cctx, &pb.Empty{})
	if err != nil {
		log.Errorf("[CoreRPCClientPool] connect to %s failed, err: %s", rpc.addr, err)
		return false
	}
	log.Debugf("[CoreRPCClientPool] connect to %s success", rpc.addr)
	return true
}

// NewCoreRPCClientPool .
func NewCoreRPCClientPool(ctx context.Context, config *types.Config) (*CoreRPCClientPool, error) {
	if len(config.Core) == 0 {
		return nil, errors.New("core addr not set")
	}
	c := &CoreRPCClientPool{rpcClients: []*clientWithStatus{}}
	for _, addr := range config.Core {
		cctx, cancel := context.WithTimeout(ctx, config.GlobalConnectionTimeout)
		defer cancel()

		rpc, err := client.NewClient(cctx, addr, config.Auth)
		if err != nil {
			log.Errorf("[NewCoreRPCClientPool] connect to %s failed, err: %s", addr, err)
			continue
		}
		rpcClient := rpc.GetRPCClient()
		c.rpcClients = append(c.rpcClients, &clientWithStatus{client: rpcClient, addr: addr})
		// update client status synchronously
		c.updateClientsStatus(ctx, config.GlobalConnectionTimeout)
	}

	allFailed := true
	for _, rpc := range c.rpcClients {
		if rpc.alive {
			allFailed = false
		}
	}

	if allFailed {
		log.Errorf("[NewCoreRPCClientPool] all connections failed")
		return nil, errors.New("all connections failed")
	}

	go func() {
		ticker := time.NewTicker(config.GlobalConnectionTimeout * 2)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.updateClientsStatus(ctx, config.GlobalConnectionTimeout)
			case <-ctx.Done():
				return
			}
		}
	}()

	return c, nil
}

func (c *CoreRPCClientPool) updateClientsStatus(ctx context.Context, timeout time.Duration) {
	wg := &sync.WaitGroup{}
	for _, rpc := range c.rpcClients {
		wg.Add(1)
		go func(r *clientWithStatus) {
			defer wg.Done()
			r.alive = checkAlive(ctx, r, timeout)
		}(rpc)
	}
	wg.Wait()
}

// GetClient finds the first *client.Client instance with an active connection. If all connections are dead, returns the first one.
func (c *CoreRPCClientPool) GetClient() pb.CoreRPCClient {
	for _, rpc := range c.rpcClients {
		if rpc.alive {
			return rpc.client
		}
	}
	return c.rpcClients[0].client
}
