package collector

import (
	"errors"
	"io"
	"maps"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
)

const (
	consoleWorkload = "3c4d5e6f708192a3b4c5d6e7f8091a2b"
	consoleAppname  = "myvm"

	consoleTimeout = 5 * time.Second
)

var (
	errJournalRefused = errors.New("journal refused the line")
	errNoPath         = errors.New("no console path")
)

func TestConsoleForwardsEveryLineOfASession(t *testing.T) {
	console := NewConsole(consoleWorkload, consoleAppname, at(socketConsole(t, "boot\r\nlogin:\n")))
	entries, lines := readConsole(t, console)

	assert.Equal(t, "boot", nextEntry(t, entries).Data)
	assert.Equal(t, "login:", nextEntry(t, entries).Data)
	assert.Equal(t, "boot", nextLine(t, lines).message)
	assert.Equal(t, "login:", nextLine(t, lines).message)
}

func TestConsoleJournalsTheFieldsCoreReadsBack(t *testing.T) {
	console := NewConsole(consoleWorkload, consoleAppname, at(socketConsole(t, "boot\n")))
	_, lines := readConsole(t, console)

	assert.Equal(t, map[string]string{
		common.FieldIdentifier: common.JournalIdentifier,
		common.FieldID:         consoleWorkload,
		common.FieldName:       consoleAppname,
		common.FieldStream:     common.StreamConsole,
	}, nextLine(t, lines).vars)
}

func TestConsoleEmitsATrailingLineWithoutANewline(t *testing.T) {
	console := NewConsole(consoleWorkload, consoleAppname, at(socketConsole(t, "no newline here")))
	entries, _ := readConsole(t, console)

	assert.Equal(t, "no newline here", nextEntry(t, entries).Data)
}

func TestConsoleReconnectsWhenTheVMComesBack(t *testing.T) {
	console := NewConsole(consoleWorkload, consoleAppname, at(socketConsole(t, "first boot\n", "second boot\n")))
	entries, _ := readConsole(t, console)

	assert.Equal(t, "first boot", nextEntry(t, entries).Data)
	assert.Equal(t, "second boot", nextEntry(t, entries).Data)
}

func TestConsoleFollowsAConsoleThatMoved(t *testing.T) {
	paths := []string{socketConsole(t, "first boot\n"), socketConsole(t, "second boot\n")}
	attempt := 0
	console := NewConsole(consoleWorkload, consoleAppname, func() (string, error) {
		path := paths[min(attempt, len(paths)-1)]
		attempt++
		return path, nil
	})
	entries, _ := readConsole(t, console)

	assert.Equal(t, "first boot", nextEntry(t, entries).Data)
	assert.Equal(t, "second boot", nextEntry(t, entries).Data)
}

func TestConsoleRetriesWhenThePathCannotBeRead(t *testing.T) {
	path := socketConsole(t, "boot\n")
	attempt := 0
	console := NewConsole(consoleWorkload, consoleAppname, func() (string, error) {
		attempt++
		if attempt == 1 {
			return "", errNoPath
		}
		return path, nil
	})
	entries, _ := readConsole(t, console)

	assert.Equal(t, "boot", nextEntry(t, entries).Data)
}

func TestConsoleReadsAConsoleThatIsNotASocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console.pty")
	require.NoError(t, os.WriteFile(path, []byte("direct boot\n"), 0o600))
	console := NewConsole(consoleWorkload, consoleAppname, at(path))
	entries, _ := readConsole(t, console)

	assert.Equal(t, "direct boot", nextEntry(t, entries).Data)
}

func TestConsoleWaitsForAConsoleThatIsNotThereYet(t *testing.T) {
	dir := shortDir(t)
	console := NewConsole(consoleWorkload, consoleAppname, at(filepath.Join(dir, "c")))
	entries, _ := readConsole(t, console)

	serve(t, listenAt(t, filepath.Join(dir, "c")), "late boot\n")
	assert.Equal(t, "late boot", nextEntry(t, entries).Data)
}

func TestEmitStampsTheWorkloadAndTheConsoleStream(t *testing.T) {
	console := NewConsole(consoleWorkload, consoleAppname, at("unused"))
	console.send = func(string, journal.Priority, map[string]string) error { return nil }

	var entries []*Entry
	console.emit("boot", func(e *Entry) { entries = append(entries, e) })

	require.Len(t, entries, 1)
	assert.Equal(t, consoleWorkload, entries[0].WorkloadID)
	assert.Equal(t, common.StreamConsole, entries[0].Stream)
	assert.False(t, entries[0].Time.IsZero())
}

func TestEmitKeepsForwardingWhenTheJournalRefuses(t *testing.T) {
	console := NewConsole(consoleWorkload, consoleAppname, at("unused"))
	console.send = func(string, journal.Priority, map[string]string) error { return errJournalRefused }

	var entries []*Entry
	console.emit("boot", func(e *Entry) { entries = append(entries, e) })
	console.emit("login:", func(e *Entry) { entries = append(entries, e) })

	assert.Len(t, entries, 2)
	assert.Equal(t, 2, console.dropped)
}

func at(path string) pathFunc {
	return func() (string, error) { return path, nil }
}

type journalLine struct {
	message string
	vars    map[string]string
}

func listenAt(t *testing.T, path string) net.Listener {
	t.Helper()
	ln, err := net.Listen("unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

// socketConsole serves one payload per connection, so a payload list is a list of vm lifetimes.
func socketConsole(t *testing.T, payloads ...string) string {
	t.Helper()
	path := filepath.Join(shortDir(t), "c")
	serve(t, listenAt(t, path), payloads...)
	return path
}

// shortDir keeps a socket path inside the 104 byte sockaddr_un limit, which a test name blows past.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "eru")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func serve(t *testing.T, ln net.Listener, payloads ...string) {
	t.Helper()
	go func() {
		for _, payload := range payloads {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = io.WriteString(conn, payload)
			_ = conn.Close()
		}
	}()
}

func readConsole(t *testing.T, console *Console) (<-chan *Entry, <-chan journalLine) {
	t.Helper()
	entries := make(chan *Entry, 32)
	lines := make(chan journalLine, 32)
	console.send = func(message string, _ journal.Priority, vars map[string]string) error {
		lines <- journalLine{message: message, vars: maps.Clone(vars)}
		return nil
	}

	go console.Read(t.Context(), func(e *Entry) { entries <- e })
	return entries, lines
}

func nextEntry(t *testing.T, entries <-chan *Entry) *Entry {
	t.Helper()
	select {
	case entry := <-entries:
		return entry
	case <-time.After(consoleTimeout):
		t.Fatal("timed out waiting for a console line")
		return nil
	}
}

func nextLine(t *testing.T, lines <-chan journalLine) journalLine {
	t.Helper()
	select {
	case line := <-lines:
		return line
	case <-time.After(consoleTimeout):
		t.Fatal("timed out waiting for a journal line")
		return journalLine{}
	}
}
