package core

import (
	"context"
	"errors"

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

	node := &types.Node{
		Name:      resp.Name,
		Podname:   resp.Podname,
		Endpoint:  resp.Endpoint,
		Available: resp.Available,
	}
	return node, nil
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

func (c *Store) GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error) {
	var resp *pb.NodeStatusStreamMessage
	var err error

	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		resp, err = c.GetClient().GetNodeStatus(ctx, &pb.GetNodeStatusOptions{Nodename: nodename})
	})

	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		err = errors.New(resp.Error)
	}

	status := &types.NodeStatus{
		Nodename: resp.Nodename,
		Podname:  resp.Podname,
		Alive:    resp.Alive,
		Error:    err,
	}
	return status, nil
}
