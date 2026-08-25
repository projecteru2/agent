//go:build linux

package utils

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const settleTimeout = 500 * time.Millisecond

func TestDirWatcherReportsCreationAndRemoval(t *testing.T) {
	dir := t.TempDir()
	events := startDirWatch(t, dir)

	path := filepath.Join(dir, "abc123.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))
	requireEvent(t, events, watchEvent{name: "abc123.json", created: true})

	require.NoError(t, os.Remove(path))
	requireEvent(t, events, watchEvent{name: "abc123.json", created: false})
}

func TestDirWatcherReportsARenameIntoTheDir(t *testing.T) {
	dir := t.TempDir()
	staging := t.TempDir()
	events := startDirWatch(t, dir)

	tmp := filepath.Join(staging, "renamed.json.tmp")
	require.NoError(t, os.WriteFile(tmp, []byte("{}"), 0o600))
	require.NoError(t, os.Rename(tmp, filepath.Join(dir, "renamed.json")))

	requireEvent(t, events, watchEvent{name: "renamed.json", created: true})
}

func TestDirWatcherReportsARenameOutOfTheDir(t *testing.T) {
	dir := t.TempDir()
	staging := t.TempDir()
	events := startDirWatch(t, dir)

	path := filepath.Join(dir, "moved.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))
	requireEvent(t, events, watchEvent{name: "moved.json", created: true})

	require.NoError(t, os.Rename(path, filepath.Join(staging, "moved.json")))
	requireEvent(t, events, watchEvent{name: "moved.json", created: false})
}

func TestDirWatcherIgnoresAWriteToAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settled.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))
	events := startDirWatch(t, dir)

	require.NoError(t, os.WriteFile(path, []byte("{\"a\":1}"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "next.json"), []byte("{}"), 0o600))

	requireEvent(t, events, watchEvent{name: "next.json", created: true})
}

func TestNewDirWatcherFailsOnAMissingDir(t *testing.T) {
	_, err := NewDirWatcher(filepath.Join(t.TempDir(), "absent"))
	require.Error(t, err)
}

func TestFileWatcherWakesOnWritesToItsFileOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "console.log")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	wakes := startFileWatch(t, path)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.log"), []byte("noise\n"), 0o600))
	requireNoWake(t, wakes)

	require.NoError(t, appendLine(path, "boot\n"))
	requireWake(t, wakes)
}

func TestFileWatcherWakesWhenItsFileIsReplaced(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "console.log")
	require.NoError(t, os.WriteFile(path, []byte("old\n"), 0o600))
	wakes := startFileWatch(t, path)

	require.NoError(t, os.Remove(path))
	requireWake(t, wakes)

	require.NoError(t, os.WriteFile(path, []byte("new\n"), 0o600))
	requireWake(t, wakes)
}

func TestNewFileWatcherFailsOnAMissingDir(t *testing.T) {
	_, err := NewFileWatcher(filepath.Join(t.TempDir(), "absent", "console.log"))
	require.Error(t, err)
}

type watchEvent struct {
	name    string
	created bool
}

func startDirWatch(t *testing.T, dir string) <-chan watchEvent {
	t.Helper()
	watcher, err := NewDirWatcher(dir)
	require.NoError(t, err)

	events := make(chan watchEvent, 16)
	go func() {
		_ = watcher.Run(t.Context(), func(name string, created bool) {
			events <- watchEvent{name: name, created: created}
		})
	}()
	return events
}

func startFileWatch(t *testing.T, path string) <-chan struct{} {
	t.Helper()
	watcher, err := NewFileWatcher(path)
	require.NoError(t, err)

	wakes := make(chan struct{}, 16)
	go func() {
		_ = watcher.Run(t.Context(), func() { wakes <- struct{}{} })
	}()
	return wakes
}

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.WriteString(line)
	return errors.Join(err, f.Close())
}

func requireEvent(t *testing.T, events <-chan watchEvent, want watchEvent) {
	t.Helper()
	select {
	case got := <-events:
		require.Equal(t, want, got)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %+v", want)
	}
}

func requireWake(t *testing.T, wakes <-chan struct{}) {
	t.Helper()
	select {
	case <-wakes:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a wake")
	}
}

func requireNoWake(t *testing.T, wakes <-chan struct{}) {
	t.Helper()
	select {
	case <-wakes:
		t.Fatal("woke on a change to another file")
	case <-time.After(settleTimeout):
	}
}
