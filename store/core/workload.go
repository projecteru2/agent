package core

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	pb "github.com/projecteru2/core/rpc/gen"
)

func getCacheTTL(ttl int) time.Duration {
	delta := rand.Intn(ttl) / 4 //nolint
	ttl = ttl - ttl/8 + delta
	return time.Duration(ttl) * time.Second
}

// SetWorkloadStatus deploy containers
func (c *Store) SetWorkloadStatus(ctx context.Context, status *types.WorkloadStatus, ttl int64) error {
	workloadStatus := fmt.Sprintf("%+v", status)
	if ttl == 0 {
		cached, ok := c.cache.Get(status.ID)
		if ok {
			str := cached.(string)
			if str == workloadStatus {
				return nil
			}
		}
	}

	statusPb := &pb.WorkloadStatus{
		Id:        status.ID,
		Running:   status.Running,
		Healthy:   status.Healthy,
		Networks:  status.Networks,
		Extension: status.Extension,
		Ttl:       ttl,

		Appname:    status.Appname,
		Entrypoint: status.Entrypoint,
		Nodename:   c.config.HostName,
	}

	opts := &pb.SetWorkloadsStatusOptions{
		Status: []*pb.WorkloadStatus{statusPb},
	}

	var err error
	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		_, err = c.GetClient().SetWorkloadsStatus(ctx, opts)
	})

	if ttl == 0 {
		if err != nil {
			c.cache.Delete(status.ID)
		} else {
			c.cache.Set(status.ID, workloadStatus, getCacheTTL(c.config.HealthCheck.CacheTTL))
		}
	}

	return err
}
