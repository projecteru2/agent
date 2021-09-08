package workload

import (
	"context"
	"sync"

	coreutils "github.com/projecteru2/core/utils"

	log "github.com/sirupsen/logrus"
)

func (m *Manager) load(ctx context.Context) error {
	log.Info("[load] Load workloads")
	workloadIDs, err := m.runtimeClient.ListWorkloadIDs(ctx, true, nil)
	if err != nil {
		return err
	}

	wg := &sync.WaitGroup{}
	for _, wid := range workloadIDs {
		log.Debugf("[load] detect workload %s", coreutils.ShortID(wid))
		wg.Add(1)
		go func(ID string) {
			defer wg.Done()
			workloadStatus, err := m.runtimeClient.GetStatus(ctx, ID, true)
			if err != nil {
				log.Errorf("[load] get workload status failed %v", err)
				return
			}

			if workloadStatus.Running {
				log.Debugf("[load] workload %s is running", workloadStatus.ID)
				go m.attach(ctx, ID)
			}

			// no health check here
			if err := m.setWorkloadStatus(ctx, workloadStatus); err != nil {
				log.Errorf("[load] update deploy status failed %v", err)
			}
		}(wid)
	}
	wg.Wait()
	return nil
}
