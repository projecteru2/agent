//go:build linux

package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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

func requireEvent(t *testing.T, events <-chan watchEvent, want watchEvent) {
	t.Helper()
	select {
	case got := <-events:
		require.Equal(t, want, got)
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %+v", want)
	}
}
