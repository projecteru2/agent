package core

import (
	"context"
	"errors"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	pb "github.com/projecteru2/core/rpc/gen"
	coretypes "github.com/projecteru2/core/types"
)

// GetNode return a node by core
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

// UpdateNode update node status
func (c *Store) UpdateNode(ctx context.Context, node *types.Node) error {
	opts := &pb.SetNodeOptions{
		Nodename:  node.Name,
		StatusOpt: coretypes.TriFalse,
	}
	if node.Available {
		opts.StatusOpt = coretypes.TriTrue
	}

	var err error
	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		_, err = c.GetClient().SetNode(ctx, opts)
	})

	return err
}

// SetNodeStatus reports the status of node
// SetNodeStatus always reports alive status,
// when not alive, TTL will cause expiration of node
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

// SetNode sets node
func (c *Store) SetNode(ctx context.Context, node string, status bool) error {
	statusOpt := pb.TriOpt_TRUE
	if !status {
		statusOpt = pb.TriOpt_FALSE
	}

	var err error
	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		_, err = c.GetClient().SetNode(ctx, &pb.SetNodeOptions{
			Nodename:      node,
			StatusOpt:     statusOpt,
			WorkloadsDown: !status,
		})
	})

	return err
}

// GetNodeStatus gets the status of node
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

// NodeStatusStream watches the changes of node status
func (c *Store) NodeStatusStream(ctx context.Context) (<-chan *types.NodeStatus, <-chan error) {
	msgChan := make(chan *types.NodeStatus)
	errChan := make(chan error)

	go func() {
		defer close(msgChan)
		defer close(errChan)

		client, err := c.GetClient().NodeStatusStream(ctx, &pb.Empty{})
		if err != nil {
			errChan <- err
			return
		}

		for {
			message, err := client.Recv()
			if err != nil {
				errChan <- err
				return
			}
			nodeStatus := &types.NodeStatus{
				Nodename: message.Nodename,
				Podname:  message.Podname,
				Alive:    message.Alive,
				Error:    nil,
			}
			if message.Error != "" {
				nodeStatus.Error = errors.New(message.Error)
			}
			msgChan <- nodeStatus
		}
	}()

	return msgChan, errChan
}

// ListPodNodes list nodes by given conditions, note that not all the fields are filled.
func (c *Store) ListPodNodes(ctx context.Context, all bool, podname string, labels map[string]string) ([]*types.Node, error) {
	var resp *pb.Nodes
	var err error
	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		resp, err = c.GetClient().ListPodNodes(ctx, &pb.ListNodesOptions{
			Podname: podname,
			All:     all,
			Labels:  labels,
		})
	})

	if err != nil {
		return nil, err
	}

	nodes := make([]*types.Node, 0, len(resp.Nodes))
	for _, n := range resp.Nodes {
		nodes = append(nodes, &types.Node{
			Name:     n.Name,
			Endpoint: n.Endpoint,
			Podname:  n.Podname,
			Labels:   n.Labels,
		})
	}
	return nodes, nil
}
