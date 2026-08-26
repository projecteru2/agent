package core

import (
	"context"

	pb "github.com/projecteru2/core/rpc/gen"
)

func (c *Store) GetIdentifier(ctx context.Context) string {
	resp, err := call(ctx, c, func(ctx context.Context) (*pb.CoreInfo, error) {
		return c.GetClient().Info(ctx, &pb.Empty{})
	})
	if err != nil {
		return ""
	}
	return resp.Identifier
}
