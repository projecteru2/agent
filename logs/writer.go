package logs

import (
	"context"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	"github.com/projecteru2/core/log"
)

const Discard = "__discard__"

var (
	KeepaliveInterval = time.Second * 30

	CloseWaitInterval = time.Second * 5
)

type Writer struct {
	sync.RWMutex
	addr          string
	scheme        string
	stdout        bool
	enc           Encoder
	needReconnect bool
}

func NewWriter(ctx context.Context, addr string, stdout bool) (writer *Writer, err error) {
	if addr == Discard {
		return &Writer{
			enc: NewStreamEncoder(discard{}),
		}, nil
	}
	logger := log.WithFunc("NewWriter")

	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	writer = &Writer{addr: u.Host, scheme: u.Scheme, stdout: stdout}
	writer.enc, err = writer.createEncoder()

	switch {
	case err == common.ErrInvalidScheme:
		logger.Infof(ctx, "create an empty writer for %s success", addr)
		writer.enc = NewStreamEncoder(discard{})
	case err == common.ErrJournalDisable:
		return nil, err
	case err != nil:
		logger.Errorf(ctx, err, "failed to create writer encoder for %s, will retry", addr)
		writer.needReconnect = true
	default:
		logger.Infof(ctx, "create writer for %s success", addr)
	}

	_ = utils.Pool.Submit(func() { writer.keepalive(ctx) })
	return writer, nil
}

func (w *Writer) Write(logline *types.Log) error {
	if w.stdout {
		log.WithFunc("Write").Info(nil, logline) //nolint
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

	w.checkError(err)
	return err
}

func (w *Writer) close() error {
	var err error
	w.withLock(func() {
		if w.enc != nil {
			err = w.enc.Close()
			w.enc = nil
		}
	})
	log.WithFunc("close").Infof(nil, "writer for %s closed", w.addr) //nolint
	return err
}

func (w *Writer) withLock(f func()) {
	w.Lock()
	defer w.Unlock()
	f()
}

func (w *Writer) withRLock(f func()) {
	w.RLock()
	defer w.RUnlock()
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

func (w *Writer) createEncoder() (enc Encoder, err error) {
	switch w.scheme {
	case "udp":
		enc, err = w.createUDPEncoder()
	case "tcp":
		enc, err = w.createTCPEncoder()
	case "journal":
		enc, err = CreateJournalEncoder()
	default:
		log.WithFunc("createEncoder").Errorf(nil, err, "Invalid scheme: %s", w.scheme) //nolint
		err = common.ErrInvalidScheme
	}
	return enc, err
}

func (w *Writer) reconnect() {
	needReconnect := false
	w.withRLock(func() {
		needReconnect = w.needReconnect
	})
	if !needReconnect {
		return
	}
	logger := log.WithFunc("reconnect")

	logger.Debugf(nil, "Reconnecting to %s...", w.addr) //nolint
	enc, err := w.createEncoder()
	if err == nil {
		w.withLock(func() {
			w.enc = enc
			w.needReconnect = false
		})
		logger.Debugf(nil, "Connect to %s successfully", w.addr) //nolint
		return
	}
	logger.Warnf(nil, "Failed to connect to %s: %s", w.addr, err) //nolint
}

func (w *Writer) keepalive(ctx context.Context) {
	timer := time.NewTimer(KeepaliveInterval)
	for {
		select {
		case <-timer.C:
			w.reconnect()
			timer.Reset(KeepaliveInterval)
		case <-ctx.Done():
			// give the pending writes a chance to drain before closing
			time.Sleep(CloseWaitInterval)
			if err := w.close(); err != nil {
				log.WithFunc("keepalive").Errorf(nil, err, "failed to close writer %s", w.addr) //nolint
			}
			return
		}
	}
}

func (w *Writer) checkError(err error) {
	if err != nil && err != common.ErrConnecting {
		log.WithFunc("checkError").Error(nil, err, "Sending log failed") //nolint
		w.withLock(func() {
			if w.enc != nil {
				w.enc.Close()
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
