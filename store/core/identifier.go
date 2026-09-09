package core

import (
	"context"

	"github.com/projecteru2/core/log"
	pb "github.com/projecteru2/core/rpc/gen"
)

func (s *Store) GetIdentifier(ctx context.Context) string {
	resp, err := call(ctx, s, func(ctx context.Context) (*pb.CoreInfo, error) {
		return s.client().Info(ctx, &pb.Empty{})
	})
	if err != nil {
		log.WithFunc("core.GetIdentifier").Error(ctx, err, "failed to read the core identifier")
		return ""
	}
	return resp.Identifier
}
