package core

import (
	"context"

	"github.com/projecteru2/agent/utils"
	pb "github.com/projecteru2/core/rpc/gen"
)

// GetIdentifier returns the identifier of core
func (c *Store) GetIdentifier(ctx context.Context) string {
	var resp *pb.CoreInfo
	var err error
	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		resp, err = c.GetClient().Info(ctx, &pb.Empty{})
	})
	if err != nil {
		return ""
	}
	return resp.Identifier
}
