package core

import (
	"context"

	pb "github.com/projecteru2/core/rpc/gen"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

func (c *Store) GetNode(ctx context.Context, nodename string) (*types.Node, error) {
	var resp *pb.Node
	var err error

	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		resp, err = c.GetClient().GetNode(ctx, &pb.GetNodeOptions{Nodename: nodename})
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
	var err error
	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		_, err = c.GetClient().SetNodeStatus(ctx, opts)
	})

	return err
}
