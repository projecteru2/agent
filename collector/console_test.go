package collector

import (
	"errors"
	"io"
	"maps"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/prometheus/client_golang/prometheus/testutil"
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

func TestConsoleCutsALineLongerThanTheReadBuffer(t *testing.T) {
	blob := strings.Repeat("x", scanBufferSize+scanBufferSize/2)
	console := NewConsole(consoleWorkload, consoleAppname, at(socketConsole(t, blob)))
	entries, _ := readConsole(t, console)

	assert.Len(t, nextEntry(t, entries).Data, scanBufferSize)
	assert.Len(t, nextEntry(t, entries).Data, scanBufferSize/2)
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

func TestConsoleKeepsBackingOffWhenASessionSaysNothing(t *testing.T) {
	console := NewConsole(consoleWorkload, consoleAppname, at(socketConsole(t, "", "", "late boot\n")))
	entries, _ := readConsole(t, console)

	assert.Equal(t, "late boot", nextEntry(t, entries).Data)
}

func TestConsoleOpensACharacterDevice(t *testing.T) {
	console := NewConsole(consoleWorkload, consoleAppname, at(os.DevNull))

	conn, err := console.open(t.Context())
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func TestConsoleRejectsAConsoleThatIsNotADevice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "console.log")
	require.NoError(t, os.WriteFile(path, []byte("not a console\n"), 0o600))
	console := NewConsole(consoleWorkload, consoleAppname, at(path))

	_, err := console.open(t.Context())
	assert.ErrorContains(t, err, "neither a socket nor a console device")
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
	counted := testutil.ToFloat64(droppedByConsole)

	var entries []*Entry
	console.emit("boot", func(e *Entry) { entries = append(entries, e) })
	console.emit("login:", func(e *Entry) { entries = append(entries, e) })

	assert.Len(t, entries, 2)
	assert.Equal(t, 2, console.dropped)
	assert.InDelta(t, counted+2, testutil.ToFloat64(droppedByConsole), 0)
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

func socketConsole(t *testing.T, payloads ...string) string {
	t.Helper()
	path := filepath.Join(shortDir(t), "c")
	serve(t, listenAt(t, path), payloads...)
	return path
}

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
