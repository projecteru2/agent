package workload

import (
	"cmp"
	"context"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/collector"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/logs"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

type logTarget struct {
	workload *source.Workload
	writer   *logs.Writer
	extra    map[string]string
	cancel   context.CancelFunc
}

func (m *Manager) forwardJournal(ctx context.Context) {
	logger := log.WithFunc("workload.forwardJournal")
	for {
		err := m.journal.Read(ctx, func(entry *collector.Entry) { m.forward(ctx, entry) })
		if ctx.Err() != nil {
			logger.Info(ctx, "context canceled, stop reading the journal")
			return
		}
		logger.Error(ctx, err, "journal reader stopped, will retry")
		time.Sleep(m.config.GlobalConnectionTimeout)
	}
}

func (m *Manager) startForwarding(ctx context.Context, w *source.Workload) {
	logger := log.WithFunc("workload.startForwarding").WithField("ID", w.ID)

	m.logMutex.Lock()
	defer m.logMutex.Unlock()
	if _, ok := m.logTargets[w.ID]; ok {
		return
	}

	transfer := cmp.Or(m.forwards.Get(w.ID, 0), logs.Discard)
	ctx, cancel := context.WithCancel(ctx)
	writer, err := logs.NewWriter(ctx, transfer, m.config.Log.Stdout)
	if err != nil {
		cancel()
		logger.Errorf(ctx, err, "create log forward %s failed", transfer)
		return
	}

	target := &logTarget{workload: w, writer: writer, extra: w.LogFields(), cancel: cancel}
	for _, key := range logKeys(w) {
		m.logTargets[key] = target
	}
	logger.Infof(ctx, "forwarding %s logs from the journal", w.Meta.Appname)
}

func (m *Manager) stopForwarding(ID string) {
	m.logMutex.Lock()
	defer m.logMutex.Unlock()

	target, ok := m.logTargets[ID]
	if !ok {
		return
	}
	target.cancel()
	for _, key := range logKeys(target.workload) {
		delete(m.logTargets, key)
	}
}

func (m *Manager) forward(ctx context.Context, entry *collector.Entry) {
	target := m.logTarget(entry)
	if target == nil {
		return
	}

	w := target.workload
	l := &types.Log{
		ID:         w.ID,
		Name:       w.Meta.Appname,
		Type:       entry.Stream,
		EntryPoint: w.Meta.Entrypoint,
		Ident:      w.Meta.Ident,
		Data:       utils.ReplaceNonUtf8(entry.Data),
		Datetime:   entry.Time.Format(common.DateTimeFormat),
		Extra:      target.extra,
	}
	m.logBroadcaster.logC <- l
	if err := target.writer.Write(ctx, l); err != nil {
		log.WithFunc("workload.forward").WithField("ID", w.ID).Errorf(ctx, err, "%s workload %s write failed", w.Meta.Appname, w.Meta.Entrypoint)
	}
}

func (m *Manager) logTarget(entry *collector.Entry) *logTarget {
	m.logMutex.RLock()
	defer m.logMutex.RUnlock()

	if target, ok := m.logTargets[entry.WorkloadID]; ok {
		return target
	}
	return m.logTargets[entry.Unit]
}

func logKeys(w *source.Workload) []string {
	keys := []string{w.ID}
	if w.Log.JournalUnit != "" {
		keys = append(keys, w.Log.JournalUnit)
	}
	return keys
}
