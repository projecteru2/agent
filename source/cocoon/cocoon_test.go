package cocoon

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
)

const (
	vmWorkload    = "3c4d5e6f708192a3b4c5d6e7f8091a2b"
	otherWorkload = "4d5e6f708192a3b4c5d6e7f8091a2b3c"
	cocoonVMID    = "01k3n8qz7m4vx9pbc2ry6dth5w"

	syncFrame   = "event: sync\ndata: {\"vms\":[{\"name\":%q,\"state\":\"running\",\"live\":true}]}\n\n"
	changeFrame = "event: change\ndata: {\"type\":%q,\"vm\":{\"name\":%q,\"state\":%q,\"live\":%t}}\n\n"

	eventTimeout = 5 * time.Second
)

func TestWatchDaemonReportsTransitions(t *testing.T) {
	socket := startDaemon(t, stream(
		fmt.Sprintf(syncFrame, vmWorkload),
		fmt.Sprintf(changeFrame, "MODIFIED", vmWorkload, "running", true),
		fmt.Sprintf(changeFrame, "MODIFIED", vmWorkload, "stopped", false),
		fmt.Sprintf(changeFrame, "DELETED", vmWorkload, "stopped", false),
	))

	events := watch(t, newCocoon(t, socket, t.TempDir()))
	assert.Equal(t, event{vmWorkload, common.StatusStart}, next(t, events))
	assert.Equal(t, event{vmWorkload, common.StatusDie}, next(t, events))
}

func TestWatchDaemonReportsALiveVMOnly(t *testing.T) {
	socket := startDaemon(t, stream(
		fmt.Sprintf(changeFrame, "MODIFIED", vmWorkload, "running", false),
		fmt.Sprintf(changeFrame, "ADDED", otherWorkload, "running", true),
	))

	events := watch(t, newCocoon(t, socket, t.TempDir()))
	assert.Equal(t, event{vmWorkload, common.StatusDie}, next(t, events))
	assert.Equal(t, event{otherWorkload, common.StatusStart}, next(t, events))
}

func TestWatchDaemonIgnoresAForeignVM(t *testing.T) {
	socket := startDaemon(t, stream(
		fmt.Sprintf(changeFrame, "ADDED", "my-laptop-vm", "running", true),
		fmt.Sprintf(changeFrame, "ADDED", vmWorkload, "running", true),
	))

	events := watch(t, newCocoon(t, socket, t.TempDir()))
	assert.Equal(t, event{vmWorkload, common.StatusStart}, next(t, events))
}

func TestListUsesTheDaemonLiveness(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, vmWorkload, scope(t, ""))
	socket := startDaemon(t, vms(fmt.Sprintf(`{"vms":[{"name":%q,"state":"running","live":true}]}`, vmWorkload)))

	workloads, err := newCocoon(t, socket, dir).List(t.Context())
	require.NoError(t, err)
	require.Len(t, workloads, 1)
	assert.True(t, workloads[0].Running)
	assert.Equal(t, "10.0.0.9", workloads[0].LocalIP)
	assert.Equal(t, "tap-"+vmWorkload, workloads[0].HostIface)
	assert.Equal(t, "/var/lib/cocoon/run/ch/"+cocoonVMID+"/console.sock", workloads[0].Log.ConsoleSocket)
}

func TestListFallsBackToTheVMScope(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, vmWorkload, scope(t, "4213\n"))
	writeMeta(t, dir, otherWorkload, scope(t, ""))

	workloads, err := newCocoon(t, absentSocket(t), dir).List(t.Context())
	require.NoError(t, err)
	require.Len(t, workloads, 2)
	assert.True(t, workloads[0].Running)
	assert.False(t, workloads[1].Running)
}

func TestGetFallsBackToTheVMScope(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, vmWorkload, scope(t, "4213\n"))
	c := newCocoon(t, absentSocket(t), dir)

	w, err := c.Get(t.Context(), vmWorkload)
	require.NoError(t, err)
	assert.True(t, w.Running)

	_, err = c.Get(t.Context(), otherWorkload)
	assert.Error(t, err)
}

func TestAliveWithoutADaemon(t *testing.T) {
	c := newCocoon(t, absentSocket(t), t.TempDir())
	assert.True(t, c.Alive(t.Context()))
}

func TestAliveWithAHealthyDaemon(t *testing.T) {
	c := newCocoon(t, startDaemon(t, health(http.StatusOK)), t.TempDir())
	assert.True(t, c.Alive(t.Context()))
}

func TestAliveWithAnUnhealthyDaemon(t *testing.T) {
	c := newCocoon(t, startDaemon(t, health(http.StatusServiceUnavailable)), t.TempDir())
	assert.False(t, c.Alive(t.Context()))
}

type event struct {
	ID     string
	action string
}

func newCocoon(t *testing.T, socket, metaDir string) *Cocoon {
	t.Helper()
	c, err := New(t.Context(), &types.Config{
		MetaDir:                 metaDir,
		Runtimes:                types.RuntimesConfig{Cocoon: &types.CocoonConfig{Socket: socket}},
		GlobalConnectionTimeout: eventTimeout,
	})
	require.NoError(t, err)
	return c
}

func startDaemon(t *testing.T, handler http.Handler) string {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "d")
	ln, err := net.Listen("unix", socket)
	require.NoError(t, err)

	srv := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return socket
}

func absentSocket(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "absent.sock")
}

func stream(frames ...string) http.Handler {
	return route("GET /v1/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		rc := http.NewResponseController(w)
		for _, frame := range frames {
			if _, err := io.WriteString(w, frame); err != nil {
				return
			}
			_ = rc.Flush()
		}
		<-r.Context().Done()
	})
}

func vms(body string) http.Handler {
	return route("GET /v1/vms", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	})
}

func health(status int) http.Handler {
	return route("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	})
}

func route(pattern string, handle http.HandlerFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, handle)
	return mux
}

func watch(t *testing.T, c *Cocoon) <-chan event {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	events := make(chan event, 16)
	go c.watchDaemon(ctx, func(ID, action string) { events <- event{ID, action} })
	return events
}

func next(t *testing.T, events <-chan event) event {
	t.Helper()
	select {
	case got := <-events:
		return got
	case <-time.After(eventTimeout):
		t.Fatal("timed out waiting for an event")
		return event{}
	}
}

func scope(t *testing.T, procs string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, procsFile), []byte(procs), 0o600))
	return dir
}

func writeMeta(t *testing.T, dir, ID, cgroup string) {
	t.Helper()
	body := fmt.Sprintf(
		`{"id":%q,"kind":"vm","appname":"myvm","entrypoint":"web","networks":{"eru-cni":"10.0.0.9"},`+
			`"cgroup":%q,"iface":"tap-%s","log":{"console_socket":"/var/lib/cocoon/run/ch/%s/console.sock"}}`,
		ID, cgroup, ID, cocoonVMID,
	)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ID+".json"), []byte(body), 0o600))
}
