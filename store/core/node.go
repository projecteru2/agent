package corestore

import (
	"context"

	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
)

// GetNode return a node by core
func (c *CoreStore) GetNode(nodename string) (*types.Node, error) {
	client := c.client.GetRPCClient()
	resp, err := client.GetNode(context.Background(), &pb.GetNodeOptions{Nodename: nodename})
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
	client := c.client.GetRPCClient()
	opts := &pb.SetNodeOptions{
		Nodename: node.Name,
	}
	if node.Available {
		opts.StatusOpt = types.TriTrue
	} else {
		opts.StatusOpt = types.TriFalse
	}
	_, err := client.SetNode(context.Background(), opts)
	return err
}
