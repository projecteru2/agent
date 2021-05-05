package corestore

import (
	"context"

	pb "github.com/projecteru2/core/rpc/gen"
)

// GetCoreIdentifier returns the identifier of core
func (c *CoreStore) GetCoreIdentifier() string {
	client := c.client.GetRPCClient()

	ctx, cancel := context.WithTimeout(context.Background(), c.config.GlobalConnectionTimeout)
	defer cancel()

	resp, err := client.Info(ctx, &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Identifier
}
