package workload

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/alphadose/haxmap"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
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
	subscribersMap *haxmap.Map[string, map[string]*subscriber] // format: map[app string, map[ID string]*subscriber]
}

func newLogBroadcaster() *logBroadcaster {
	return &logBroadcaster{
		logC:           make(chan *types.Log),
		subscribersMap: haxmap.New[string, map[string]*subscriber](),
	}
}

func (l *logBroadcaster) getSubscribers(app string) map[string]*subscriber {
	subs, ok := l.subscribersMap.Get(app)
	if !ok {
		subs = map[string]*subscriber{}
		l.subscribersMap.Set(app, subs)
	}
	return subs
}

func (l *logBroadcaster) deleteSubscribers(app string) {
	l.subscribersMap.Del(app)
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
		_ = utils.Pool.Submit(func() { l.unsubscribe(app, ID) })
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
		ID := ID
		sub := sub
		_ = utils.Pool.Submit(func() {
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
		})
	}
	wg.Wait()
}

func (l *logBroadcaster) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			logrus.Info("[logBroadcaster] stops")
			return
		case log := <-l.logC:
			l.broadcast(log)
		}
	}
}
