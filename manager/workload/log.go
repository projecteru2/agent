package workload

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	corelog "github.com/projecteru2/core/log"
	coreutils "github.com/projecteru2/core/utils"

	"github.com/projecteru2/agent/types"
)

type subscriber struct {
	ctx     context.Context
	cancel  context.CancelFunc
	buf     *bufio.ReadWriter
	errChan chan error
}

func (s *subscriber) isDone() bool {
	select {
	case <-s.ctx.Done():
		return true
	default:
		return false
	}
}

type logBroadcaster struct {
	sync.RWMutex
	logC           chan *types.Log
	subscribersMap map[string]map[string]*subscriber
}

func newLogBroadcaster() *logBroadcaster {
	return &logBroadcaster{
		logC:           make(chan *types.Log),
		subscribersMap: map[string]map[string]*subscriber{},
	}
}

func (l *logBroadcaster) subscribe(ctx context.Context, app string, buf *bufio.ReadWriter) (string, chan error, func()) {
	l.Lock()
	defer l.Unlock()

	subscribers := l.subscribersMap[app]
	if subscribers == nil {
		subscribers = map[string]*subscriber{}
		l.subscribersMap[app] = subscribers
	}
	ID := coreutils.RandomString(8)
	ctx, cancel := context.WithCancel(ctx)
	errChan := make(chan error)

	subscribers[ID] = &subscriber{
		ctx:     ctx,
		cancel:  cancel,
		buf:     buf,
		errChan: errChan,
	}

	corelog.Infof(ctx, "%s %s log subscribed", app, ID)
	return ID, errChan, func() {
		cancel()
		go l.unsubscribe(app, ID)
	}
}

func (l *logBroadcaster) unsubscribe(app, ID string) {
	l.Lock()
	defer l.Unlock()

	subscribers := l.subscribersMap[app]
	if subscriber, ok := subscribers[ID]; ok {
		close(subscriber.errChan)
	}
	delete(subscribers, ID)

	corelog.Infof(nil, "%s %s detached", app, ID) //nolint

	if len(subscribers) == 0 {
		delete(l.subscribersMap, app)
	}
}

func (l *logBroadcaster) broadcast(log *types.Log) {
	l.RLock()
	defer l.RUnlock()

	subscribers := l.subscribersMap[log.Name]
	if len(subscribers) == 0 {
		return
	}
	data, err := json.Marshal(log)
	if err != nil {
		corelog.Error(nil, err) //nolint
		return
	}
	line := fmt.Sprintf("%X\r\n%s\r\n\r\n", len(data)+2, string(data))

	// waiting here keeps the log lines ordered across subscribers
	var wg sync.WaitGroup
	for ID, sub := range subscribers {
		wg.Go(func() {
			if sub.isDone() {
				return
			}
			if _, err := sub.buf.Write([]byte(line)); err != nil {
				corelog.Debugf(nil, "[broadcast] failed to write into %v, err: %v", ID, err) //nolint
				sub.cancel()
				sub.errChan <- err
				return
			}
			_ = sub.buf.Flush()
		})
	}
	wg.Wait()
}

func (l *logBroadcaster) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			corelog.Info(ctx, "[logBroadcaster] stops")
			return
		case log := <-l.logC:
			l.broadcast(log)
		}
	}
}
