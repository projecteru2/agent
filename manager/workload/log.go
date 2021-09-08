package workload

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"

	"github.com/projecteru2/agent/types"
	coreutils "github.com/projecteru2/core/utils"

	"github.com/sirupsen/logrus"
)

type subscriber struct {
	buf         *bufio.ReadWriter
	unsubscribe func()
}

// logBroadcaster receives log and broadcasts to subscribers
type logBroadcaster struct {
	logC        chan *types.Log
	subscribers map[string]map[string]*subscriber
}

func newLogBroadcaster() *logBroadcaster {
	return &logBroadcaster{
		logC:        make(chan *types.Log),
		subscribers: map[string]map[string]*subscriber{},
	}
}

// subscribe subscribes logs of the specific app.
func (l *logBroadcaster) subscribe(app string, buf *bufio.ReadWriter) {
	if _, ok := l.subscribers[app]; !ok {
		l.subscribers[app] = map[string]*subscriber{}
	}

	ID := coreutils.RandomString(8)
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	l.subscribers[app][ID] = &subscriber{buf, cancel}
	logrus.Infof("%s %s log subscribed", app, ID)
	<-ctx.Done()

	delete(l.subscribers[app], ID)
	if len(l.subscribers[app]) == 0 {
		delete(l.subscribers, app)
	}
}

func (l *logBroadcaster) broadcast(log *types.Log) {
	if _, ok := l.subscribers[log.Name]; !ok {
		return
	}
	data, err := json.Marshal(log)
	if err != nil {
		logrus.Error(err)
		return
	}
	line := fmt.Sprintf("%X\r\n%s\r\n\r\n", len(data)+2, string(data))
	for ID, subscriber := range l.subscribers[log.Name] {
		if _, err := subscriber.buf.WriteString(line); err != nil {
			logrus.Error(err)
			logrus.Infof("%s %s detached", log.Name, ID)
			subscriber.unsubscribe()
		}
		subscriber.buf.Flush()
		logrus.Debugf("sub %s get %s", ID, line)
	}
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
