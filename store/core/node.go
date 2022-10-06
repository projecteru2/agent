package core

import (
	"context"
	"errors"
	"io"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	pb "github.com/projecteru2/core/rpc/gen"
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
	ch, err := c.listPodeNodes(ctx, &pb.ListNodesOptions{
		Podname: podname,
		All:     all,
		Labels:  labels,
	})
	if err != nil {
		return nil, err
	}

	nodes := []*types.Node{}
	for n := range ch {
		nodes = append(nodes, &types.Node{
			Name:     n.Name,
			Endpoint: n.Endpoint,
			Podname:  n.Podname,
			Labels:   n.Labels,
		})
	}
	return nodes, nil
}

func (c *Store) listPodeNodes(ctx context.Context, opt *pb.ListNodesOptions) (ch chan *pb.Node, err error) {
	ch = make(chan *pb.Node)

	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		var stream pb.CoreRPC_ListPodNodesClient
		if stream, err = c.GetClient().ListPodNodes(ctx, opt); err != nil {
			return
		}

		go func() {
			defer close(ch)
			for {
				node, err := stream.Recv()
				if err != nil {
					if err != io.EOF {
						// TODO:
						// log it
					}
					return
				}
				ch <- node
			}
		}()
	})

	return ch, nil
}
