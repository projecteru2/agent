package logshim

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/urfave/cli/v3"

	"github.com/projecteru2/agent/common"
)

const (
	envContainerID = "CONTAINER_ID"

	stdoutFD = 3
	stderrFD = 4
	readyFD  = 5

	lineMax = 64 << 10
)

type sendFunc func(message string, priority journal.Priority, vars map[string]string) error

type task struct {
	id     string
	stdout io.Reader
	stderr io.Reader
	ready  io.WriteCloser
}

// Command returns the binary logger containerd execs per task, as binary:///usr/local/bin/eru-agent?log-shim.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "log-shim",
		Usage: "journal a containerd task's output, one process per task",
		Action: func(context.Context, *cli.Command) error {
			// containerd terms the logger while the task is still draining into it
			signal.Ignore(syscall.SIGTERM)
			return run(journal.Send, task{
				id:     os.Getenv(envContainerID),
				stdout: os.NewFile(stdoutFD, common.StreamStdout),
				stderr: os.NewFile(stderrFD, common.StreamStderr),
				ready:  os.NewFile(readyFD, "ready"),
			})
		},
	}
}

func run(send sendFunc, t task) error {
	if err := signalReady(t.ready); err != nil {
		return err
	}

	stdout := &stream{send: send, id: t.id, name: common.StreamStdout, priority: journal.PriInfo}
	stderr := &stream{send: send, id: t.id, name: common.StreamStderr, priority: journal.PriErr}

	var wg sync.WaitGroup
	wg.Go(func() { stdout.pump(t.stdout) })
	wg.Go(func() { stderr.pump(t.stderr) })
	wg.Wait()

	if dropped := stdout.dropped + stderr.dropped; dropped > 0 {
		return fmt.Errorf("dropped %d lines of %s: %w", dropped, t.id, cmp.Or(stdout.err, stderr.err))
	}
	return nil
}

// signalReady writes the byte containerd waits for before it starts the container, then closes the pipe.
func signalReady(ready io.WriteCloser) error {
	if _, err := ready.Write([]byte{0}); err != nil {
		return errors.Join(err, ready.Close())
	}
	return ready.Close()
}

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
		common.FieldIdentifier: common.JournalIdentifier,
		common.FieldID:         s.id,
		common.FieldStream:     s.name,
	}

	buf := bufio.NewReaderSize(reader, lineMax)
	for {
		line, _, err := buf.ReadLine()
		if err != nil {
			return
		}
		// a journal that refuses a line must not fail the container, so the line is lost
		if sendErr := s.send(string(line), s.priority, vars); sendErr != nil {
			s.dropped++
			s.err = sendErr
		}
	}
}
