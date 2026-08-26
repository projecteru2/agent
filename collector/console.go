package collector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
)

const (
	consoleRetryMin = 100 * time.Millisecond
	consoleRetryMax = 5 * time.Second
)

type journalFunc func(message string, priority journal.Priority, vars map[string]string) error

// pathFunc answers where the vm's console is now: a restart moves it, and core rewrites the meta file.
type pathFunc func() (string, error)

// Console follows a vm's serial console: every line is forwarded live and journaled for history.
type Console struct {
	workloadID string
	path       pathFunc
	send       journalFunc
	vars       map[string]string

	dropped int
}

func NewConsole(workloadID, appname string, path pathFunc) *Console {
	return &Console{
		workloadID: workloadID,
		path:       path,
		send:       journal.Send,
		vars: map[string]string{
			common.FieldIdentifier: common.JournalIdentifier,
			common.FieldID:         workloadID,
			common.FieldName:       appname,
			common.FieldStream:     common.StreamConsole,
		},
	}
}

// Read follows the console until ctx is done, reconnecting so a vm that restarts comes back on its own.
func (c *Console) Read(ctx context.Context, handle func(*Entry)) {
	logger := log.WithFunc("collector.Read").WithField("ID", c.workloadID)

	backoff := consoleRetryMin
	for {
		delivered, err := c.pump(ctx, handle)
		if ctx.Err() != nil {
			return
		}
		if delivered {
			backoff = consoleRetryMin
		}
		// the console goes away with the vm and comes back with it, so a closed one is not a failure
		logger.Debugf(ctx, "console stopped: %v", err)
		if c.dropped > 0 {
			logger.Warnf(ctx, "the journal refused %d console lines", c.dropped)
			c.dropped = 0
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, consoleRetryMax)
	}
}

// pump reads one console session, reporting whether it delivered a line so a retry can back off.
func (c *Console) pump(ctx context.Context, handle func(*Entry)) (bool, error) {
	conn, err := c.open(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	delivered := false
	reader := bufio.NewReaderSize(conn, scanBufferSize)
	for {
		line, _, err := reader.ReadLine()
		if text := strings.TrimRight(string(line), "\r"); text != "" {
			c.emit(text, handle)
			delivered = true
		}
		if err != nil {
			return delivered, err
		}
	}
}

// open dials the console: cloud hypervisor serves a socket for a uefi guest and a pty for a direct boot one.
func (c *Console) open(ctx context.Context) (io.ReadCloser, error) {
	path, err := c.path()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	switch mode := info.Mode(); {
	case mode&os.ModeSocket != 0:
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	case mode&os.ModeCharDevice != 0:
		return os.OpenFile(path, os.O_RDONLY|syscall.O_NOCTTY, 0) //nolint:gosec // the path comes from the meta file core wrote
	default:
		return nil, fmt.Errorf("%s is neither a socket nor a console device", path)
	}
}

func (c *Console) emit(line string, handle func(*Entry)) {
	handle(&Entry{WorkloadID: c.workloadID, Stream: common.StreamConsole, Data: line, Time: time.Now()})
	// journald holds the history core reads back over ssh, the same way it does for a container
	if err := c.send(line, journal.PriInfo, c.vars); err != nil {
		c.dropped++
	}
}
