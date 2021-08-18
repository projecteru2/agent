package corestore

import (
	"context"

	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
)

// GetNode return a node by core
func (c *CoreStore) GetNode(nodename string) (*types.Node, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.config.GlobalConnectionTimeout)
	defer cancel()
	resp, err := c.GetClient().GetNode(ctx, &pb.GetNodeOptions{Nodename: nodename})
	if err != nil {
		return nil, err
	}

	cpus := types.CPUMap{}
	for k, v := range resp.Cpu {
		cpus[k] = int64(v)
	}

	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     resp.Name,
			Podname:  resp.Podname,
			Endpoint: resp.Endpoint,
			CPU:      cpus,
			MemCap:   resp.Memory,
		},
		Available: resp.Available,
	}
	return node, nil
}

// UpdateNode update node status
func (c *CoreStore) UpdateNode(node *types.Node) error {
	opts := &pb.SetNodeOptions{
		Nodename: node.Name,
	}
	if node.Available {
		opts.StatusOpt = types.TriTrue
	} else {
		opts.StatusOpt = types.TriFalse
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.config.GlobalConnectionTimeout)
	defer cancel()
	_, err := c.GetClient().SetNode(ctx, opts)
	return err
}

// SetNodeStatus reports the status of node
// SetNodeStatus always reports alive status,
// when not alive, TTL will cause expiration of node
func (c *CoreStore) SetNodeStatus(ctx context.Context, ttl int64) error {
	opts := &pb.SetNodeStatusOptions{
		Nodename: c.config.HostName,
		Ttl:      ttl,
	}
	_, err := c.GetClient().SetNodeStatus(ctx, opts)
	return err
}
