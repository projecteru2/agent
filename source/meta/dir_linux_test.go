//go:build linux

package meta

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
)

const (
	watchTimeout = 5 * time.Second

	armAttempts = 100
	armInterval = 50 * time.Millisecond
)

func TestWatchReportsThisDirsOwnKind(t *testing.T) {
	dir := t.TempDir()
	events := startWatch(t, dir, KindProcess)

	write(t, dir, vmWorkload, KindVM)
	write(t, dir, cniWorkload, KindProcess)

	assert.Equal(t, watched{cniWorkload, common.StatusStart}, next(t, events))
}

func TestWatchIgnoresANameThatIsNotAWorkloadID(t *testing.T) {
	dir := t.TempDir()
	events := startWatch(t, dir, KindProcess)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "notanid.json"), []byte("{}"), 0o600))
	write(t, dir, cniWorkload, KindProcess)

	assert.Equal(t, watched{cniWorkload, common.StatusStart}, next(t, events))
}

func TestWatchReportsARemovalItClaimed(t *testing.T) {
	dir := t.TempDir()
	events := startWatch(t, dir, KindProcess)

	write(t, dir, cniWorkload, KindProcess)
	assert.Equal(t, watched{cniWorkload, common.StatusStart}, next(t, events))

	require.NoError(t, os.Remove(filepath.Join(dir, cniWorkload+suffix)))
	assert.Equal(t, watched{cniWorkload, common.StatusDie}, next(t, events))
}

func TestWatchIgnoresTheRemovalOfAnotherKind(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, vmWorkload, KindVM)
	events := startWatch(t, dir, KindProcess)

	require.NoError(t, os.Remove(filepath.Join(dir, vmWorkload+suffix)))
	write(t, dir, cniWorkload, KindProcess)

	assert.Equal(t, watched{cniWorkload, common.StatusStart}, next(t, events))
}

type watched struct {
	ID     string
	action string
}

func startWatch(t *testing.T, dir string, kind Kind) <-chan watched {
	t.Helper()
	d, err := NewDir(dir, kind)
	require.NoError(t, err)

	events := make(chan watched, 16)
	reporter := source.NewReporter()
	reporter.Attach(func(ID, action string) { events <- watched{ID, action} })
	go func() {
		_ = d.Watch(t.Context(), reporter)
	}()
	awaitWatchArmed(t, dir, kind, events)
	return events
}

func awaitWatchArmed(t *testing.T, dir string, kind Kind, events <-chan watched) {
	t.Helper()
	for range armAttempts {
		write(t, dir, hostNetWorkload, kind)
		select {
		case event := <-events:
			require.Equal(t, watched{hostNetWorkload, common.StatusStart}, event)
			return
		case <-time.After(armInterval):
		}
	}
	t.Fatal("the watch never armed")
}

func write(t *testing.T, dir, ID string, kind Kind) {
	t.Helper()
	body := `{"id":"` + ID + `","kind":"` + string(kind) + `","log":{"journal_unit":"eru-` + ID + `.service"}}`
	staging := filepath.Join(t.TempDir(), ID+suffix)
	require.NoError(t, os.WriteFile(staging, []byte(body), 0o600))
	require.NoError(t, os.Rename(staging, filepath.Join(dir, ID+suffix)))
}

func next(t *testing.T, events <-chan watched) watched {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(watchTimeout):
		t.Fatal("timed out waiting for a watch event")
		return watched{}
	}
}
