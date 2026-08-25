package workload

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/logs"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

func (m *Manager) attach(ctx context.Context, w *source.Workload) {
	attacher, ok := m.source.(source.Attacher)
	if !ok {
		return
	}

	logger := log.WithFunc("workload.attach").WithField("ID", w.ID)
	logger.Debug(ctx, "attaching workload")
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	transfer := m.forwards.Get(w.ID, 0)
	if transfer == "" {
		transfer = logs.Discard
	}
	writer, err := logs.NewWriter(ctx, transfer, m.config.Log.Stdout)
	if err != nil {
		logger.Errorf(ctx, err, "create log forward %s failed", transfer)
		return
	}

	outr, errr, err := attacher.Attach(ctx, w.ID)
	if err != nil {
		logger.Errorf(ctx, err, "failed to attach workload %s", w.Meta.Appname)
		return
	}
	logger.Infof(ctx, "attach %s workload success", w.Meta.Appname)

	extra := w.LogFields()

	var wg sync.WaitGroup
	pump := func(typ string, reader io.Reader) {
		logger.Debugf(ctx, "attach pump %s %s start", w.Meta.Appname, typ)
		defer logger.Debugf(ctx, "attach pump %s %s finished", w.Meta.Appname, typ)

		buf := bufio.NewReader(reader)
		for {
			data, err := buf.ReadString('\n')
			if err != nil {
				if !errors.Is(err, io.EOF) {
					logger.Errorf(ctx, err, "attach pump %s %s failed", w.Meta.Appname, typ)
				}
				return
			}
			data = strings.TrimSuffix(data, "\n")
			data = strings.TrimSuffix(data, "\r")
			l := &types.Log{
				ID:         w.ID,
				Name:       w.Meta.Appname,
				Type:       typ,
				EntryPoint: w.Meta.Entrypoint,
				Ident:      w.Meta.Ident,
				Data:       utils.ReplaceNonUtf8(data),
				Datetime:   time.Now().Format(common.DateTimeFormat),
				Extra:      extra,
			}
			m.logBroadcaster.logC <- l
			if err := writer.Write(ctx, l); err != nil && (w.Meta.Entrypoint != "agent" || !utils.IsDockerized()) {
				logger.Errorf(ctx, err, "%s workload %s write failed", w.Meta.Appname, w.Meta.Entrypoint)
			}
		}
	}
	defer wg.Wait()
	wg.Go(func() { pump("stdout", outr) })
	wg.Go(func() { pump("stderr", errr) })
}
