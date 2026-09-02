package logs

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sync"
	"syscall"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
)

const (
	Discard = "__discard__"

	keepaliveInterval = time.Second * 30
	dialTimeout       = time.Second * 5
	writeTimeout      = time.Second * 5
)

type Writer struct {
	mu     sync.RWMutex
	addr   string
	scheme string
	stdout bool
	enc    Encoder
}

func NewWriter(ctx context.Context, addr string, stdout bool) (writer *Writer, err error) {
	if addr == Discard {
		return &Writer{stdout: stdout}, nil
	}
	logger := log.WithFunc("logs.NewWriter")

	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	writer = &Writer{addr: u.Host, scheme: u.Scheme, stdout: stdout}
	writer.enc, err = writer.createEncoder(ctx)

	switch {
	case errors.Is(err, common.ErrInvalidScheme):
		logger.Infof(ctx, "create an empty writer for %s success", addr)
		return &Writer{stdout: stdout}, nil
	case errors.Is(err, common.ErrJournalDisabled):
		return nil, err
	case err != nil:
		logger.Errorf(ctx, err, "failed to create writer encoder for %s, will retry", addr)
	default:
		logger.Infof(ctx, "create writer for %s success", addr)
	}

	go writer.keepalive(ctx)
	return writer, nil
}

func (w *Writer) Write(ctx context.Context, logline *types.Log) error {
	if w.stdout {
		log.WithFunc("logs.Write").Info(ctx, logline)
	}
	if len(w.addr) == 0 && len(w.scheme) == 0 {
		return nil
	}
	var err error
	w.withLock(func() {
		if w.enc == nil {
			err = common.ErrConnecting
			return
		}
		err = w.enc.Encode(logline)
	})

	w.checkError(ctx, err)
	return err
}

func (w *Writer) close(ctx context.Context) error {
	var err error
	w.withLock(func() {
		if w.enc != nil {
			err = w.enc.Close()
			w.enc = nil
		}
	})
	log.WithFunc("logs.close").Infof(ctx, "writer for %s closed", w.addr)
	return err
}

func (w *Writer) withLock(f func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	f()
}

func (w *Writer) withRLock(f func()) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	f()
}

func (w *Writer) createStreamEncoder(network string) (Encoder, error) {
	conn, err := net.DialTimeout(network, w.addr, dialTimeout)
	if err != nil {
		return nil, err
	}
	return NewStreamEncoder(&deadlineConn{Conn: conn, timeout: writeTimeout}), nil
}

func (w *Writer) createEncoder(ctx context.Context) (enc Encoder, err error) {
	switch w.scheme {
	case "udp", "tcp":
		enc, err = w.createStreamEncoder(w.scheme)
	case "journal":
		enc, err = CreateJournalEncoder()
	default:
		log.WithFunc("logs.createEncoder").Warnf(ctx, "invalid scheme %s", w.scheme)
		err = common.ErrInvalidScheme
	}
	return enc, err
}

func (w *Writer) reconnect(ctx context.Context) {
	connected := false
	w.withRLock(func() {
		connected = w.enc != nil
	})
	if connected {
		return
	}
	logger := log.WithFunc("logs.reconnect")

	logger.Debugf(ctx, "reconnecting to %s", w.addr)
	enc, err := w.createEncoder(ctx)
	if err == nil {
		w.withLock(func() {
			w.enc = enc
		})
		logger.Debugf(ctx, "connected to %s", w.addr)
		return
	}
	logger.Warnf(ctx, "failed to connect to %s: %v", w.addr, err)
}

func (w *Writer) keepalive(ctx context.Context) {
	timer := time.NewTimer(keepaliveInterval)
	for {
		select {
		case <-timer.C:
			w.reconnect(ctx)
			timer.Reset(keepaliveInterval)
		case <-ctx.Done():
			if err := w.close(ctx); err != nil {
				log.WithFunc("logs.keepalive").Errorf(ctx, err, "failed to close writer %s", w.addr)
			}
			return
		}
	}
}

func (w *Writer) checkError(ctx context.Context, err error) {
	if err == nil || errors.Is(err, common.ErrConnecting) {
		return
	}
	log.WithFunc("logs.checkError").Error(ctx, err, "failed to send log")
	if errors.Is(err, syscall.EMSGSIZE) {
		return
	}
	w.withLock(func() {
		if w.enc != nil {
			_ = w.enc.Close()
			w.enc = nil
		}
	})
}

// deadlineConn bounds a write, so a target that accepts and then stops reading is retried like a down one.
type deadlineConn struct {
	net.Conn
	timeout time.Duration
}

func (c *deadlineConn) Write(p []byte) (int, error) {
	if err := c.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(p)
}
