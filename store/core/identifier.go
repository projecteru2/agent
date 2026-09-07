package core

import (
	"context"

	pb "github.com/projecteru2/core/rpc/gen"
)

func (s *Store) GetIdentifier(ctx context.Context) string {
	resp, err := call(ctx, s, func(ctx context.Context) (*pb.CoreInfo, error) {
		return s.GetClient().Info(ctx, &pb.Empty{})
	})
	if err != nil {
		return ""
	}
	return resp.Identifier
}
