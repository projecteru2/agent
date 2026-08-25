package logs

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
)

const Discard = "__discard__"

var (
	KeepaliveInterval = time.Second * 30

	CloseWaitInterval = time.Second * 5
)

type Writer struct {
	mu            sync.RWMutex
	addr          string
	scheme        string
	stdout        bool
	enc           Encoder
	needReconnect bool
}

func NewWriter(ctx context.Context, addr string, stdout bool) (writer *Writer, err error) {
	if addr == Discard {
		return &Writer{
			stdout: stdout,
			enc:    NewStreamEncoder(discard{}),
		}, nil
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
		writer.enc = NewStreamEncoder(discard{})
	case errors.Is(err, common.ErrJournalDisabled):
		return nil, err
	case err != nil:
		logger.Errorf(ctx, err, "failed to create writer encoder for %s, will retry", addr)
		writer.needReconnect = true
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
			w.needReconnect = true
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

func (w *Writer) createUDPEncoder() (Encoder, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", w.addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}
	return NewStreamEncoder(conn), nil
}

func (w *Writer) createTCPEncoder() (Encoder, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", w.addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, err
	}
	return NewStreamEncoder(conn), nil
}

func (w *Writer) createEncoder(ctx context.Context) (enc Encoder, err error) {
	switch w.scheme {
	case "udp":
		enc, err = w.createUDPEncoder()
	case "tcp":
		enc, err = w.createTCPEncoder()
	case "journal":
		enc, err = CreateJournalEncoder()
	default:
		log.WithFunc("logs.createEncoder").Warnf(ctx, "invalid scheme %s", w.scheme)
		err = common.ErrInvalidScheme
	}
	return enc, err
}

func (w *Writer) reconnect(ctx context.Context) {
	needReconnect := false
	w.withRLock(func() {
		needReconnect = w.needReconnect
	})
	if !needReconnect {
		return
	}
	logger := log.WithFunc("logs.reconnect")

	logger.Debugf(ctx, "reconnecting to %s", w.addr)
	enc, err := w.createEncoder(ctx)
	if err == nil {
		w.withLock(func() {
			w.enc = enc
			w.needReconnect = false
		})
		logger.Debugf(ctx, "connected to %s", w.addr)
		return
	}
	logger.Warnf(ctx, "failed to connect to %s: %v", w.addr, err)
}

func (w *Writer) keepalive(ctx context.Context) {
	timer := time.NewTimer(KeepaliveInterval)
	for {
		select {
		case <-timer.C:
			w.reconnect(ctx)
			timer.Reset(KeepaliveInterval)
		case <-ctx.Done():
			// give the pending writes a chance to drain before closing
			time.Sleep(CloseWaitInterval)
			if err := w.close(ctx); err != nil {
				log.WithFunc("logs.keepalive").Errorf(ctx, err, "failed to close writer %s", w.addr)
			}
			return
		}
	}
}

func (w *Writer) checkError(ctx context.Context, err error) {
	if err != nil && !errors.Is(err, common.ErrConnecting) {
		log.WithFunc("logs.checkError").Error(ctx, err, "failed to send log")
		w.withLock(func() {
			if w.enc != nil {
				_ = w.enc.Close()
				w.enc = nil
				w.needReconnect = true
			}
		})
	}
}

type discard struct{}

func (d discard) Write([]byte) (n int, err error) {
	return 0, nil
}

func (d discard) Close() error {
	return nil
}
