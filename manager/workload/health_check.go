package workload

import (
	"context"
	"errors"
	"time"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	log "github.com/sirupsen/logrus"
)

func (m *Manager) healthCheck(ctx context.Context) {
	tick := time.NewTicker(time.Duration(m.config.HealthCheck.Interval) * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			go m.checkAllWorkloads(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// 检查全部 label 为ERU=1的workload
// 这里需要 list all，原因是 monitor 检测到 die 的时候已经标记为 false 了
// 但是这时候 health check 刚返回 true 回来并写入 core
// 为了保证最终数据一致性这里也要检测
func (m *Manager) checkAllWorkloads(ctx context.Context) {
	log.Debugf("[checkAllWorkloads] health check begin")
	workloadIDs, err := m.runtimeClient.ListWorkloadIDs(ctx, true, nil)
	if err != nil {
		log.Errorf("[checkAllWorkloads] Error when list all workloads with label \"ERU=1\": %v", err)
		return
	}

	for _, wid := range workloadIDs {
		go m.checkOneWorkload(ctx, wid)
	}
}

// 检查并保存一个workload的状态，最后返回workload是否healthy。
// 返回healthy是为了重试用的，没啥别的意义。
func (m *Manager) checkOneWorkload(ctx context.Context, ID string) bool {
	workloadStatus, err := m.runtimeClient.GetStatus(ctx, ID, true)
	if err != nil {
		log.Errorf("[checkOneWorkload] failed to get status of workload %s, err: %v", ID, err)
		return false
	}

	if err = m.setWorkloadStatus(ctx, workloadStatus); err != nil {
		log.Errorf("[checkOneWorkload] update workload status failed, err: %v", err)
	}
	return workloadStatus.Healthy
}

// 设置workload状态，允许重试，带timeout控制
func (m *Manager) setWorkloadStatus(ctx context.Context, status *types.WorkloadStatus) error {
	return utils.BackoffRetry(ctx, 3, func() error {
		var err error
		utils.WithTimeout(ctx, m.config.GlobalConnectionTimeout, func(ctx context.Context) {
			err = m.store.SetWorkloadStatus(ctx, status, m.config.GetHealthCheckStatusTTL())
		})
		return err
	})
}

// 检查一个workload，允许重试
func (m *Manager) checkOneWorkloadWithBackoffRetry(ctx context.Context, ID string) {
	log.Debugf("[checkOneWorkloadWithBackoffRetry] check workload %s", ID)
	err := utils.BackoffRetry(ctx, utils.GetMaxAttemptsByTTL(m.config.GetHealthCheckStatusTTL()), func() error {
		if !m.checkOneWorkload(ctx, ID) {
			// 这个err就是用来判断要不要继续的，不用打在日志里
			return errors.New("not healthy")
		}
		return nil
	})

	if err != nil {
		log.Debugf("[checkOneWorkloadWithBackoffRetry] workload %s still not healthy", ID)
	}
}
