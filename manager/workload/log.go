package workload

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	corelog "github.com/projecteru2/core/log"
	coreutils "github.com/projecteru2/core/utils"

	"github.com/projecteru2/agent/collector"
	"github.com/projecteru2/agent/types"
)

const subscriberDepth = 256

var droppedBySubscriber = collector.LogLinesDropped.WithLabelValues(collector.DropPointSubscriber)

type subscriber struct {
	ctx     context.Context
	cancel  context.CancelFunc
	buf     *bufio.ReadWriter
	lines   chan []byte
	errChan chan error
	dropped atomic.Int64
}

func (s *subscriber) isDone() bool {
	select {
	case <-s.ctx.Done():
		return true
	default:
		return false
	}
}

// send never blocks: one client that stopped reading must not stall the node's forwarding.
func (s *subscriber) send(line []byte) {
	select {
	case s.lines <- line:
	default:
		s.dropped.Add(1)
		droppedBySubscriber.Inc()
	}
}

// pump writes off the broadcast path, so a stalled socket stalls only its own subscriber.
func (s *subscriber) pump() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case line := <-s.lines:
			if _, err := s.buf.Write(line); err != nil {
				s.cancel()
				select {
				case s.errChan <- err:
				default:
				}
				return
			}
			_ = s.buf.Flush()
		}
	}
}

type logBroadcaster struct {
	mu             sync.RWMutex
	subscribersMap map[string]map[string]*subscriber
}

func newLogBroadcaster() *logBroadcaster {
	return &logBroadcaster{subscribersMap: map[string]map[string]*subscriber{}}
}

func (l *logBroadcaster) subscribe(ctx context.Context, app string, buf *bufio.ReadWriter) (string, chan error, func()) {
	l.mu.Lock()
	defer l.mu.Unlock()

	subscribers := l.subscribersMap[app]
	if subscribers == nil {
		subscribers = map[string]*subscriber{}
		l.subscribersMap[app] = subscribers
	}
	ID := coreutils.RandomString(8)
	subCtx, cancel := context.WithCancel(ctx)
	errChan := make(chan error)

	sub := &subscriber{
		ctx:     subCtx,
		cancel:  cancel,
		buf:     buf,
		lines:   make(chan []byte, subscriberDepth),
		errChan: errChan,
	}
	subscribers[ID] = sub
	go sub.pump()

	corelog.WithFunc("workload.subscribe").Infof(ctx, "%s %s log subscribed", app, ID)
	return ID, errChan, func() {
		cancel()
		go l.unsubscribe(ctx, app, ID)
	}
}

func (l *logBroadcaster) unsubscribe(ctx context.Context, app, ID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	logger := corelog.WithFunc("workload.unsubscribe")
	subscribers := l.subscribersMap[app]
	if sub, ok := subscribers[ID]; ok {
		if dropped := sub.dropped.Load(); dropped > 0 {
			logger.Warnf(ctx, "%s %s could not keep up, %d log lines dropped", app, ID, dropped)
		}
	}
	delete(subscribers, ID)

	logger.Infof(ctx, "%s %s detached", app, ID)

	if len(subscribers) == 0 {
		delete(l.subscribersMap, app)
	}
}

func (l *logBroadcaster) broadcast(ctx context.Context, log *types.Log) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	subscribers := l.subscribersMap[log.Name]
	if len(subscribers) == 0 {
		return
	}
	data, err := json.Marshal(log)
	if err != nil {
		corelog.WithFunc("workload.broadcast").Error(ctx, err, "failed to marshal log")
		return
	}
	line := fmt.Appendf(nil, "%X\r\n%s\r\n\r\n", len(data)+2, data)

	for _, sub := range subscribers {
		if !sub.isDone() {
			sub.send(line)
		}
	}
}
