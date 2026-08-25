package logshim

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
)

func TestRunJournalsOneEntryPerLine(t *testing.T) {
	sender := &fakeJournal{}
	task := task{
		id:     "myapp_web_EAXPcM",
		stdout: strings.NewReader("first\r\nsecond\nno trailing newline"),
		stderr: strings.NewReader("boom\n"),
		ready:  &fakePipe{},
	}

	require.NoError(t, run(sender.send, task))

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

func TestRunSignalsReadinessBeforeItReads(t *testing.T) {
	sender := &fakeJournal{}
	ready := &fakePipe{}

	require.NoError(t, run(sender.send, task{id: "myapp_web_EAXPcM", stdout: empty(), stderr: empty(), ready: ready}))

	assert.Equal(t, []byte{0}, ready.written)
	assert.True(t, ready.closed)
}

func TestRunSplitsALineLongerThanTheBuffer(t *testing.T) {
	sender := &fakeJournal{}
	// a progress bar redraws with \r and never terminates a line, so the reader must cut it up itself
	blob := strings.Repeat("x", lineMax+lineMax/2)
	task := task{id: "myapp_web_EAXPcM", stdout: strings.NewReader(blob), stderr: empty(), ready: &fakePipe{}}

	require.NoError(t, run(sender.send, task))

	messages := sender.messages(streamStdout)
	require.Len(t, messages, 2)
	assert.Len(t, messages[0], lineMax)
	assert.Len(t, messages[1], lineMax/2)
}

func TestRunJournalsNothingForAnEmptyStream(t *testing.T) {
	sender := &fakeJournal{}

	require.NoError(t, run(sender.send, task{id: "myapp_web_EAXPcM", stdout: empty(), stderr: empty(), ready: &fakePipe{}}))
	assert.Empty(t, sender.records)
}

func TestRunDropsWhatTheJournalRefuses(t *testing.T) {
	full := errors.New("journal socket is full")
	sender := &fakeJournal{err: full}
	task := task{
		id:     "myapp_web_EAXPcM",
		stdout: strings.NewReader("first\nsecond\n"),
		stderr: strings.NewReader("boom\n"),
		ready:  &fakePipe{},
	}

	err := run(sender.send, task)
	assert.ErrorIs(t, err, full)
	assert.ErrorContains(t, err, "dropped 3 lines of myapp_web_EAXPcM")
}

func TestRunStopsWhenItCannotSignalReadiness(t *testing.T) {
	broken := errors.New("the shim closed the wait pipe")
	sender := &fakeJournal{}
	task := task{id: "myapp_web_EAXPcM", stdout: strings.NewReader("first\n"), stderr: empty(), ready: &fakePipe{err: broken}}

	assert.ErrorIs(t, run(sender.send, task), broken)
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

type fakePipe struct {
	written []byte
	closed  bool
	err     error
}

func (f *fakePipe) Write(p []byte) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.written = append(f.written, p...)
	return len(p), nil
}

func (f *fakePipe) Close() error {
	f.closed = true
	return nil
}

func empty() io.Reader {
	return strings.NewReader("")
}
