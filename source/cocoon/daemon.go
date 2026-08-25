package cocoon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// daemonHost is a placeholder: the transport dials the unix socket and ignores the authority.
	daemonHost = "http://cocoond"

	healthPath = "/healthz"
	vmsPath    = "/v1/vms"
	eventsPath = "/v1/events"

	syncEvent   = "sync"
	changeEvent = "change"
	eventField  = "event: "
	dataField   = "data: "

	stateRunning  = "running"
	changeDeleted = "DELETED"

	streamBufferSize = 64 << 10
	streamLineMax    = 1 << 20
)

// vmStatus is one supervised vm; its name is the workload id eru created the vm under.
type vmStatus struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Live  bool   `json:"live"`
}

func (s vmStatus) running() bool {
	return s.State == stateRunning && s.Live
}

type vmsResponse struct {
	VMs []vmStatus `json:"vms"`
}

type changeMessage struct {
	Kind   string   `json:"type"`
	Status vmStatus `json:"vm"`
}

// daemon is the cocoon supervisor's read-only api on its unix socket; a node may run without one.
type daemon struct {
	socket  string
	timeout time.Duration
	client  *http.Client
}

func newDaemon(socket string, timeout time.Duration) *daemon {
	dialer := &net.Dialer{}
	return &daemon{
		socket:  socket,
		timeout: timeout,
		client: &http.Client{Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, "unix", socket)
			},
		}},
	}
}

func (d *daemon) health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	resp, err := d.get(ctx, healthPath)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// vms answers which workloads are running, keyed by the id the vm carries as its name.
func (d *daemon) vms(ctx context.Context) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	resp, err := d.get(ctx, vmsPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body := vmsResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	live := make(map[string]bool, len(body.VMs))
	for _, vm := range body.VMs {
		live[vm.Name] = vm.running()
	}
	return live, nil
}

// events streams the daemon's changes after its opening snapshot; it replays nothing, so the snapshot is the resync.
func (d *daemon) events(ctx context.Context, handle func(ID string, running, gone bool)) error {
	resp, err := d.get(ctx, eventsPath)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, streamBufferSize), streamLineMax)

	event := ""
	for scanner.Scan() {
		line := scanner.Text()
		if name, ok := strings.CutPrefix(line, eventField); ok {
			event = name
			continue
		}
		data, ok := strings.CutPrefix(line, dataField)
		if !ok {
			continue
		}
		if err := dispatch(event, []byte(data), handle); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (d *daemon) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, daemonHost+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%s on %s answered %s", path, d.socket, resp.Status)
	}
	return resp, nil
}

func dispatch(event string, data []byte, handle func(ID string, running, gone bool)) error {
	switch event {
	case syncEvent:
		body := vmsResponse{}
		if err := json.Unmarshal(data, &body); err != nil {
			return err
		}
		for _, vm := range body.VMs {
			handle(vm.Name, vm.running(), false)
		}
	case changeEvent:
		msg := changeMessage{}
		if err := json.Unmarshal(data, &msg); err != nil {
			return err
		}
		handle(msg.Status.Name, msg.Status.running(), msg.Kind == changeDeleted)
	}
	return nil
}
