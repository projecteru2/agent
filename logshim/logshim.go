package logshim

import (
	"bufio"
	"cmp"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/containerd/containerd/v2/core/runtime/v2/logging"
	"github.com/coreos/go-systemd/v22/journal"
	"github.com/urfave/cli/v3"

	"github.com/projecteru2/agent/common"
)

const (
	fieldIdentifier = "SYSLOG_IDENTIFIER"
	fieldID         = "ERU_ID"
	fieldStream     = "ERU_STREAM"

	streamStdout = "stdout"
	streamStderr = "stderr"
)

// sendFunc writes one record to the local journal.
type sendFunc func(message string, priority journal.Priority, vars map[string]string) error

// Command returns the containerd binary logger mode, which core points a task's cio.LogURI at.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "log-shim",
		Usage: "journal a containerd task's output, one process per task",
		Action: func(context.Context, *cli.Command) error {
			logging.Run(func(_ context.Context, config *logging.Config, ready func() error) error {
				return run(journal.Send, config, ready)
			})
			return nil
		},
	}
}

func run(send sendFunc, config *logging.Config, ready func() error) error {
	if err := ready(); err != nil {
		return err
	}

	stdout := &stream{send: send, id: config.ID, name: streamStdout, priority: journal.PriInfo}
	stderr := &stream{send: send, id: config.ID, name: streamStderr, priority: journal.PriErr}

	var wg sync.WaitGroup
	wg.Go(func() { stdout.pump(config.Stdout) })
	wg.Go(func() { stderr.pump(config.Stderr) })
	wg.Wait()

	if dropped := stdout.dropped + stderr.dropped; dropped > 0 {
		return fmt.Errorf("dropped %d lines of %s: %w", dropped, config.ID, cmp.Or(stdout.err, stderr.err))
	}
	return nil
}

// stream journals one of the task's two output pipes.
type stream struct {
	send     sendFunc
	id       string
	name     string
	priority journal.Priority

	dropped int
	err     error
}

func (s *stream) pump(reader io.Reader) {
	vars := map[string]string{
		fieldIdentifier: common.JournalIdentifier,
		fieldID:         s.id,
		fieldStream:     s.name,
	}

	buf := bufio.NewReader(reader)
	for {
		line, err := buf.ReadString('\n')
		if line != "" {
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			// a journal that cannot keep up must not block the container, so a refused line is lost
			if sendErr := s.send(line, s.priority, vars); sendErr != nil {
				s.dropped++
				s.err = sendErr
			}
		}
		if err != nil {
			return
		}
	}
}
