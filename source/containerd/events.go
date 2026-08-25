package containerd

import (
	"context"

	apievents "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/typeurl/v2"
	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
)

const eventType = "container"

// containerd ors its filters, so the agent subscribes with one term per topic it reacts to
var eventFilters = []string{
	`topic=="/tasks/start"`,
	`topic=="/tasks/exit"`,
	`topic=="/containers/delete"`,
	`topic=="/containers/update"`,
}

// relay turns the daemon's event stream into the workload events the manager handles.
func relay(ctx context.Context, envelopes <-chan *events.Envelope, errs <-chan error) (<-chan *types.WorkloadEventMessage, <-chan error) {
	eventChan := make(chan *types.WorkloadEventMessage)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		for {
			select {
			case envelope, ok := <-envelopes:
				if !ok {
					return
				}
				message := translate(ctx, envelope)
				if message == nil {
					continue
				}
				select {
				case eventChan <- message:
				case <-ctx.Done():
					return
				}
			case err, ok := <-errs:
				if !ok {
					return
				}
				select {
				case errChan <- err:
				default:
				}
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, errChan
}

func translate(ctx context.Context, envelope *events.Envelope) *types.WorkloadEventMessage {
	event, err := typeurl.UnmarshalAny(envelope.Event)
	if err != nil {
		log.WithFunc("containerd.translate").Errorf(ctx, err, "failed to decode a %s event", envelope.Topic)
		return nil
	}

	var ID, action string
	switch e := event.(type) {
	case *apievents.TaskStart:
		ID, action = e.ContainerID, common.StatusStart
	case *apievents.TaskExit:
		// an exec process exiting is not the workload exiting
		if e.ID != e.ContainerID {
			return nil
		}
		ID, action = e.ContainerID, common.StatusDie
	case *apievents.ContainerDelete:
		ID, action = e.ID, common.StatusDie
	case *apievents.ContainerUpdate:
		// the oci hook writes the cni addresses back as labels, so an update is a new set of facts
		ID, action = e.ID, common.StatusStart
	default:
		return nil
	}
	return &types.WorkloadEventMessage{ID: ID, Type: eventType, Action: action, TimeNano: envelope.Timestamp.UnixNano()}
}
