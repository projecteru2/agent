package logshim

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/containerd/containerd/v2/core/runtime/v2/logging"
	"github.com/coreos/go-systemd/v22/journal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
)

func TestRunJournalsOneEntryPerLine(t *testing.T) {
	sender := &fakeJournal{}
	config := &logging.Config{
		ID:     "myapp_web_EAXPcM",
		Stdout: strings.NewReader("first\r\nsecond\nno trailing newline"),
		Stderr: strings.NewReader("boom\n"),
	}

	require.NoError(t, run(sender.send, config, ready(nil)))

	assert.Equal(t, []string{"first", "second", "no trailing newline"}, sender.messages(streamStdout))
	assert.Equal(t, []string{"boom"}, sender.messages(streamStderr))
	assert.Equal(t, map[string]string{
		"SYSLOG_IDENTIFIER": common.JournalIdentifier,
		"ERU_ID":            "myapp_web_EAXPcM",
		"ERU_STREAM":        streamStdout,
	}, sender.varsOf("first"))
	assert.Equal(t, journal.PriInfo, sender.priorityOf("first"))
	assert.Equal(t, journal.PriErr, sender.priorityOf("boom"))
}

func TestRunJournalsNothingForAnEmptyStream(t *testing.T) {
	sender := &fakeJournal{}
	config := &logging.Config{ID: "myapp_web_EAXPcM", Stdout: strings.NewReader(""), Stderr: strings.NewReader("")}

	require.NoError(t, run(sender.send, config, ready(nil)))
	assert.Empty(t, sender.records)
}

func TestRunDropsWhatTheJournalRefuses(t *testing.T) {
	full := errors.New("journal socket is full")
	sender := &fakeJournal{err: full}
	config := &logging.Config{
		ID:     "myapp_web_EAXPcM",
		Stdout: strings.NewReader("first\nsecond\n"),
		Stderr: strings.NewReader("boom\n"),
	}

	err := run(sender.send, config, ready(nil))
	assert.ErrorIs(t, err, full)
	assert.ErrorContains(t, err, "dropped 3 lines of myapp_web_EAXPcM")
}

func TestRunStopsWhenItCannotSignalReadiness(t *testing.T) {
	notReady := errors.New("the shim closed the wait pipe")
	sender := &fakeJournal{}
	config := &logging.Config{ID: "myapp_web_EAXPcM", Stdout: strings.NewReader("first\n"), Stderr: strings.NewReader("")}

	assert.ErrorIs(t, run(sender.send, config, ready(notReady)), notReady)
	assert.Empty(t, sender.records)
}

type record struct {
	message  string
	priority journal.Priority
	vars     map[string]string
}

type fakeJournal struct {
	mutex   sync.Mutex
	records []record
	err     error
}

func (f *fakeJournal) send(message string, priority journal.Priority, vars map[string]string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.records = append(f.records, record{message: message, priority: priority, vars: vars})
	return f.err
}

func (f *fakeJournal) messages(stream string) []string {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	var messages []string
	for _, r := range f.records {
		if r.vars["ERU_STREAM"] == stream {
			messages = append(messages, r.message)
		}
	}
	return messages
}

func (f *fakeJournal) varsOf(message string) map[string]string {
	return f.find(message).vars
}

func (f *fakeJournal) priorityOf(message string) journal.Priority {
	return f.find(message).priority
}

func (f *fakeJournal) find(message string) record {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	for _, r := range f.records {
		if r.message == message {
			return r
		}
	}
	return record{}
}

func ready(err error) func() error {
	return func() error { return err }
}
