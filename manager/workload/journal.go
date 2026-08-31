package workload

import (
	"cmp"
	"context"
	"errors"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/collector"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/logs"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

var droppedByForward = collector.LogLinesDropped.WithLabelValues(collector.DropPointForward)

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

// forwardConsole reads a vm's serial console, which journald only holds once this reader has written it there.
func (m *Manager) forwardConsole(ctx context.Context, w *source.Workload) {
	// a restarted vm gets a new console, so the path is read back per attempt rather than held from here
	console := collector.NewConsole(w.ID, w.Meta.Appname, func() (string, error) {
		fresh := m.refreshed(ctx, w.ID)
		if fresh == nil {
			return "", source.ErrUnknownWorkload
		}
		return fresh.Log.ConsoleSocket, nil
	})
	console.Read(ctx, func(entry *collector.Entry) { m.forward(ctx, entry) })
}

func (m *Manager) startForwarding(ctx context.Context, w *source.Workload) {
	logger := log.WithFunc("workload.startForwarding").WithField("ID", w.ID)
	if m.forwardedWorkload(w.ID) != nil {
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
	if !m.registerTarget(w, &logTarget{workload: w, writer: writer, extra: w.LogFields(), cancel: cancel}) {
		cancel()
		return
	}

	if w.Log.ConsoleSocket != "" {
		go m.forwardConsole(ctx, w)
		logger.Infof(ctx, "forwarding %s logs from the console at %s", w.Meta.Appname, w.Log.ConsoleSocket)
		return
	}
	logger.Infof(ctx, "forwarding %s logs from the journal", w.Meta.Appname)
}

func (m *Manager) registerTarget(w *source.Workload, target *logTarget) bool {
	m.logMutex.Lock()
	defer m.logMutex.Unlock()
	if _, ok := m.logTargets[w.ID]; ok {
		return false
	}
	for _, key := range logKeys(w) {
		m.logTargets[key] = target
	}
	return true
}

func (m *Manager) forwardedWorkload(ID string) *source.Workload {
	m.logMutex.RLock()
	defer m.logMutex.RUnlock()
	if target, ok := m.logTargets[ID]; ok {
		return target.workload
	}
	return nil
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
	m.logBroadcaster.broadcast(ctx, l)
	err := target.writer.Write(ctx, l)
	if err == nil {
		return
	}
	droppedByForward.Inc()
	if !errors.Is(err, common.ErrConnecting) {
		logger := log.WithFunc("workload.forward").WithField("ID", w.ID)
		logger.Errorf(ctx, err, "%s workload %s write failed", w.Meta.Appname, w.Meta.Entrypoint)
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
