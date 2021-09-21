package workload

import (
	"bufio"
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/logs"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	log "github.com/sirupsen/logrus"
)

func (m *Manager) attach(ctx context.Context, ID string) {
	log.Debugf("[attach] attaching workload %v", ID)
	transfer := m.forwards.Get(ID, 0)
	if transfer == "" {
		transfer = logs.Discard
	}
	writer, err := logs.NewWriter(transfer, m.config.Log.Stdout)
	if err != nil {
		log.Errorf("[attach] Create log forward failed %s", err)
		return
	}

	// get app info
	workloadName, err := m.runtimeClient.GetWorkloadName(ctx, ID)
	if err != nil {
		if err != common.ErrNotImplemented {
			log.Errorf("[attach] failed to get workload name, id: %v, err: %v", ID, err)
		} else {
			log.Debugf("[attach] should ignore this workload")
		}
		return
	}

	name, entryPoint, ident, err := utils.GetAppInfo(workloadName)
	if err != nil {
		log.Errorf("[attach] invalid workload name %s, err: %v", workloadName, err)
		return
	}

	// attach workload
	outr, errr, err := m.runtimeClient.AttachWorkload(ctx, ID)
	if err != nil {
		log.Errorf("[attach] failed to attach workload %s, err: %v", workloadName, err)
		return
	}
	log.Infof("[attach] attach %s workload %s success", workloadName, ID)

	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// attach metrics
	go m.runtimeClient.CollectWorkloadMetrics(cancelCtx, ID)

	extra, err := m.runtimeClient.LogFieldsExtra(ctx, ID)
	if err != nil {
		log.Errorf("[attach] failed to get log fields extra, err: %v", err)
	}

	wg := &sync.WaitGroup{}
	pump := func(typ string, source io.Reader) {
		defer wg.Done()
		log.Debugf("[attach] attach pump %s %s %s start", workloadName, ID, typ)
		defer log.Debugf("[attach] attach pump %s %s %s finished", workloadName, ID, typ)

		buf := bufio.NewReader(source)
		for {
			data, err := buf.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Errorf("[attach] attach pump %s %s %s failed, err: %v", workloadName, ID, typ, err)
				}
				return
			}
			data = strings.TrimSuffix(data, "\n")
			data = strings.TrimSuffix(data, "\r")
			l := &types.Log{
				ID:         ID,
				Name:       name,
				Type:       typ,
				EntryPoint: entryPoint,
				Ident:      ident,
				Data:       utils.ReplaceNonUtf8(data),
				Datetime:   time.Now().Format(common.DateTimeFormat),
				Extra:      extra,
			}
			if m.logBroadcaster != nil && m.logBroadcaster.logC != nil {
				m.logBroadcaster.logC <- l
			}
			if err := writer.Write(l); err != nil && !(entryPoint == "agent" && utils.IsDockerized()) {
				log.Errorf("[attach] %s workload %s_%s write failed %v", workloadName, entryPoint, ID, err)
				log.Errorf("[attach] %s", data)
			}
		}
	}
	wg.Add(2)
	go pump("stdout", outr)
	go pump("stderr", errr)
	wg.Wait()
}
