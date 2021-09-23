package workload

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/projecteru2/agent/types"
	coreutils "github.com/projecteru2/core/utils"

	"github.com/sirupsen/logrus"
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

// logBroadcaster receives log and broadcasts to subscribers
type logBroadcaster struct {
	sync.RWMutex
	logC           chan *types.Log
	subscribersMap sync.Map // format: map[app string]map[ID string]*subscriber
}

func newLogBroadcaster() *logBroadcaster {
	return &logBroadcaster{
		logC:           make(chan *types.Log),
		subscribersMap: sync.Map{},
	}
}

func (l *logBroadcaster) getSubscribers(app string) map[string]*subscriber {
	v, _ := l.subscribersMap.LoadOrStore(app, map[string]*subscriber{})
	return v.(map[string]*subscriber)
}

func (l *logBroadcaster) deleteSubscribers(app string) {
	l.subscribersMap.Delete(app)
}

// subscribe subscribes logs of the specific app.
func (l *logBroadcaster) subscribe(ctx context.Context, app string, buf *bufio.ReadWriter) (string, chan error, func()) {
	l.Lock()
	defer l.Unlock()

	subscribers := l.getSubscribers(app)
	ID := coreutils.RandomString(8)
	ctx, cancel := context.WithCancel(ctx)
	errChan := make(chan error)

	subscribers[ID] = &subscriber{
		ctx:     ctx,
		cancel:  cancel,
		buf:     buf,
		errChan: errChan,
	}

	logrus.Infof("%s %s log subscribed", app, ID)
	return ID, errChan, func() {
		cancel()
		go l.unsubscribe(app, ID)
	}
}

func (l *logBroadcaster) unsubscribe(app string, ID string) {
	l.Lock()
	defer l.Unlock()

	subscribers := l.getSubscribers(app)

	subscriber, ok := subscribers[ID]
	if ok {
		close(subscriber.errChan)
	}

	delete(subscribers, ID)

	logrus.Infof("%s %s detached", app, ID)

	// if no subscribers for this app, remove the key
	if len(subscribers) == 0 {
		l.deleteSubscribers(app)
	}
}

func (l *logBroadcaster) broadcast(log *types.Log) {
	l.RLock()
	defer l.RUnlock()

	subscribers := l.getSubscribers(log.Name)
	if len(subscribers) == 0 {
		return
	}
	data, err := json.Marshal(log)
	if err != nil {
		logrus.Error(err)
		return
	}
	line := fmt.Sprintf("%X\r\n%s\r\n\r\n", len(data)+2, string(data))

	// use wait group to make sure the logs are ordered
	wg := &sync.WaitGroup{}
	wg.Add(len(subscribers))
	for ID, sub := range subscribers {
		go func(ID string, sub *subscriber) {
			defer wg.Done()
			if sub.isDone() {
				return
			}

			if _, err := sub.buf.Write([]byte(line)); err != nil {
				logrus.Debugf("[broadcast] failed to write into %v, err: %v", ID, err)
				sub.cancel()
				sub.errChan <- err
				return
			}

			sub.buf.Flush()
		}(ID, sub)
	}
	wg.Wait()
}

func (l *logBroadcaster) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			logrus.Infof("[logBroadcaster] stops")
			return
		case log := <-l.logC:
			l.broadcast(log)
		}
	}
}
