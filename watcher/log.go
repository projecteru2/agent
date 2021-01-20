package watcher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/projecteru2/agent/types"
)

// Watcher indicate watcher
type Watcher struct {
	consumer  map[string]map[string]*types.LogConsumer
	LogC      chan *types.Log
	ConsumerC chan *types.LogConsumer
}

// LogMonitor indicate log monitor
var LogMonitor *Watcher

// InitMonitor init a monitor
func InitMonitor() {
	LogMonitor = &Watcher{}
	LogMonitor.consumer = map[string]map[string]*types.LogConsumer{}
	LogMonitor.LogC = make(chan *types.Log)
	LogMonitor.ConsumerC = make(chan *types.LogConsumer)
}

// Serve start monitor
func (w *Watcher) Serve(ctx context.Context) {
	logrus.Info("[logServe] Log monitor started")
	defer logrus.Info("[logServe] Log monitor stopped")
	for {
		select {
		case <-ctx.Done():
			return
		case log := <-w.LogC:
			if consumers, ok := w.consumer[log.Name]; ok {
				data, err := json.Marshal(log)
				if err != nil {
					logrus.Error(err)
					break
				}
				line := fmt.Sprintf("%X\r\n%s\r\n\r\n", len(data)+2, string(data))
				for id, consumer := range consumers {
					if _, err := consumer.Buf.WriteString(line); err != nil {
						logrus.Error(err)
						logrus.Infof("%s %s log detached", consumer.App, consumer.ID)
						consumer.Conn.Close()
						delete(consumers, id)
						if len(w.consumer[log.Name]) == 0 {
							delete(w.consumer, log.Name)
						}
					}
					consumer.Buf.Flush()
				}
			}
		case consumer := <-w.ConsumerC:
			if consumers, ok := w.consumer[consumer.App]; ok {
				consumers[consumer.ID] = consumer
			} else {
				w.consumer[consumer.App] = map[string]*types.LogConsumer{}
				w.consumer[consumer.App][consumer.ID] = consumer
			}
		}
	}
}
