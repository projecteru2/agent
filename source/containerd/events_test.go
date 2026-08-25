package containerd

import (
	"errors"
	"testing"
	"time"

	apievents "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/v2/core/events"
	"github.com/containerd/typeurl/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
)

func TestTranslateTheWorkloadLifecycle(t *testing.T) {
	tests := []struct {
		name   string
		event  any
		ID     string
		action string
	}{
		{
			name:   "a task starting starts the workload",
			event:  &apievents.TaskStart{ContainerID: "myapp_web_EAXPcM", Pid: 42},
			ID:     "myapp_web_EAXPcM",
			action: common.StatusStart,
		},
		{
			name:   "the init process exiting kills the workload",
			event:  &apievents.TaskExit{ContainerID: "myapp_web_EAXPcM", ID: "myapp_web_EAXPcM", ExitStatus: 1},
			ID:     "myapp_web_EAXPcM",
			action: common.StatusDie,
		},
		{
			name:   "a deleted container kills the workload",
			event:  &apievents.ContainerDelete{ID: "myapp_web_EAXPcM"},
			ID:     "myapp_web_EAXPcM",
			action: common.StatusDie,
		},
		{
			name:   "an updated container is a new set of facts",
			event:  &apievents.ContainerUpdate{ID: "myapp_web_EAXPcM"},
			ID:     "myapp_web_EAXPcM",
			action: common.StatusStart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := translate(t.Context(), envelope(t, tt.event))
			require.NotNil(t, message)
			assert.Equal(t, tt.ID, message.ID)
			assert.Equal(t, tt.action, message.Action)
			assert.Equal(t, eventType, message.Type)
			assert.NotZero(t, message.TimeNano)
		})
	}
}

func TestTranslateIgnoresWhatIsNotTheWorkload(t *testing.T) {
	exec := &apievents.TaskExit{ContainerID: "myapp_web_EAXPcM", ID: "an-exec", ExitStatus: 1}
	assert.Nil(t, translate(t.Context(), envelope(t, exec)))

	assert.Nil(t, translate(t.Context(), envelope(t, &apievents.TaskOOM{ContainerID: "myapp_web_EAXPcM"})))
	assert.Nil(t, translate(t.Context(), &events.Envelope{Topic: "/tasks/start", Event: unknownEvent{}}))
}

func TestEventFiltersScopeEveryTopicToTheNamespace(t *testing.T) {
	assert.Equal(t, []string{
		`topic=="/tasks/start",namespace=="eru"`,
		`topic=="/tasks/exit",namespace=="eru"`,
		`topic=="/containers/delete",namespace=="eru"`,
		`topic=="/containers/update",namespace=="eru"`,
	}, eventFilters("eru"))
}

func TestRelayForwardsEveryTranslatedEvent(t *testing.T) {
	envelopes := make(chan *events.Envelope, 2)
	envelopes <- envelope(t, &apievents.TaskOOM{ContainerID: "myapp_web_EAXPcM"})
	envelopes <- envelope(t, &apievents.TaskStart{ContainerID: "myapp_web_EAXPcM"})

	eventChan, _ := relay(t.Context(), envelopes, make(chan error))
	select {
	case message := <-eventChan:
		assert.Equal(t, "myapp_web_EAXPcM", message.ID)
		assert.Equal(t, common.StatusStart, message.Action)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the relayed event")
	}
}

func TestRelayStopsOnASubscriptionError(t *testing.T) {
	broken := errors.New("containerd is down")
	errs := make(chan error, 1)
	errs <- broken

	eventChan, errChan := relay(t.Context(), make(chan *events.Envelope), errs)
	select {
	case err := <-errChan:
		assert.ErrorIs(t, err, broken)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the relayed error")
	}
	assertClosed(t, eventChan)
}

func TestRelayStopsWhenTheSubscriptionCloses(t *testing.T) {
	eventChan, errChan := relay(t.Context(), make(chan *events.Envelope), closedErrs())
	assertClosed(t, eventChan)
	assertClosed(t, errChan)
}

type unknownEvent struct{}

func (unknownEvent) GetTypeUrl() string {
	return "types.eru.io/not-a-registered-event"
}

func (unknownEvent) GetValue() []byte {
	return nil
}

func envelope(t *testing.T, event any) *events.Envelope {
	t.Helper()
	payload, err := typeurl.MarshalAny(event)
	require.NoError(t, err)
	return &events.Envelope{Timestamp: time.Now(), Namespace: "eru", Event: payload}
}

func closedErrs() chan error {
	errs := make(chan error)
	close(errs)
	return errs
}

func assertClosed[T any](t *testing.T, c <-chan T) {
	t.Helper()
	select {
	case _, ok := <-c:
		assert.False(t, ok)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the channel to close")
	}
}
