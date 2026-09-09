package core

import (
	"context"

	pb "github.com/projecteru2/core/rpc/gen"

	"github.com/projecteru2/agent/types"
)

func (s *Store) GetNode(ctx context.Context, nodename string) (*types.Node, error) {
	resp, err := call(ctx, s, func(ctx context.Context) (*pb.Node, error) {
		return s.client().GetNode(ctx, &pb.GetNodeOptions{Nodename: nodename})
	})
	if err != nil {
		return nil, err
	}
	return &types.Node{Endpoint: resp.Endpoint}, nil
}

// SetNodeStatus reports the node alive under core's ttl.
func (s *Store) SetNodeStatus(ctx context.Context, ttl int64) error {
	opts := &pb.SetNodeStatusOptions{
		Nodename: s.config.HostName,
		Ttl:      ttl,
	}
	_, err := call(ctx, s, func(ctx context.Context) (*pb.Empty, error) {
		return s.client().SetNodeStatus(ctx, opts)
	})
	return err
}
