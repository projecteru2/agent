package core

import (
	"context"

	pb "github.com/projecteru2/core/rpc/gen"

	"github.com/projecteru2/agent/types"
)

func (c *Store) GetNode(ctx context.Context, nodename string) (*types.Node, error) {
	resp, err := call(ctx, c, func(ctx context.Context) (*pb.Node, error) {
		return c.GetClient().GetNode(ctx, &pb.GetNodeOptions{Nodename: nodename})
	})
	if err != nil {
		return nil, err
	}
	return &types.Node{Endpoint: resp.Endpoint}, nil
}

// SetNodeStatus always reports the node as alive, core expires the status by ttl.
func (c *Store) SetNodeStatus(ctx context.Context, ttl int64) error {
	opts := &pb.SetNodeStatusOptions{
		Nodename: c.config.HostName,
		Ttl:      ttl,
	}
	_, err := call(ctx, c, func(ctx context.Context) (*pb.Empty, error) {
		return c.GetClient().SetNodeStatus(ctx, opts)
	})
	return err
}
