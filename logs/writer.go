package logs

import (
	"context"
	"errors"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/projecteru2/agent/types"

	log "github.com/sirupsen/logrus"
)

// Discard .
const Discard = "__discard__"

// ErrConnecting means writer is in connecting status, waiting to be connected
var ErrConnecting = errors.New("connecting")

// ErrInvalidScheme .
var ErrInvalidScheme = errors.New("invalid scheme")

// KeepaliveInterval .
var KeepaliveInterval = time.Second * 30

// CloseWaitInterval .
var CloseWaitInterval = time.Second * 5

// Writer is a writer!
type Writer struct {
	sync.RWMutex
	addr          string
	scheme        string
	stdout        bool
	enc           Encoder
	needReconnect bool
}

type discard struct {
}

// Write writer
func (d discard) Write(p []byte) (n int, err error) {
	return 0, nil
}

// Close closer
func (d discard) Close() error {
	return nil
}

// NewWriter return writer
func NewWriter(ctx context.Context, addr string, stdout bool) (writer *Writer, err error) {
	if addr == Discard {
		return &Writer{
			enc: NewStreamEncoder(discard{}),
		}, nil
	}

	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	writer = &Writer{addr: u.Host, scheme: u.Scheme, stdout: stdout}
	writer.enc, err = writer.createEncoder()

	switch {
	case err == ErrInvalidScheme:
		log.Infof("[writer] create an empty writer for %s success", addr)
		writer.enc = NewStreamEncoder(discard{})
	case err != nil:
		log.Errorf("[writer] failed to create writer encoder for %s, err: %v, will retry", addr, err)
		writer.needReconnect = true
	default:
		log.Infof("[writer] create writer for %s success", addr)
	}

	go writer.keepalive(ctx)

	return writer, nil
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

// CreateConn create conn
func (w *Writer) createEncoder() (enc Encoder, err error) {
	switch w.scheme {
	case "udp":
		enc, err = w.createUDPEncoder()
	case "tcp":
		enc, err = w.createTCPEncoder()
	case "journal":
		enc, err = CreateJournalEncoder()
	default:
		log.Errorf("[writer] Invalid scheme: %s", w.scheme)
		err = ErrInvalidScheme
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

	log.Debugf("[writer] Reconnecting to %s...", w.addr)
	enc, err := w.createEncoder()
	if err == nil {
		w.withLock(func() {
			w.enc = enc
			w.needReconnect = false
		})
		log.Debugf("[writer] Connect to %s successfully", w.addr)
		return
	}
	log.Warnf("[writer] Failed to connect to %s: %s", w.addr, err)
}

func (w *Writer) keepalive(ctx context.Context) {
	timer := time.NewTimer(KeepaliveInterval)
	for {
		select {
		case <-timer.C:
			w.reconnect()
			timer.Reset(KeepaliveInterval)
		case <-ctx.Done():
			return
		}
	}
}

func (w *Writer) checkError(err error) {
	if err != nil && err != ErrConnecting {
		log.Errorf("[writer] Sending log failed %s", err)
		w.withLock(func() {
			if w.enc != nil {
				w.enc.Close()
				w.enc = nil
				w.needReconnect = true
			}
		})
	}
}

func (w *Writer) checkConn() error {
	var err error
	w.withLock(func() {
		if w.enc == nil {
			err = ErrConnecting
			w.needReconnect = true
		}
	})

	return err
}

// Write write log to remote
func (w *Writer) Write(logline *types.Log) error {
	if w.stdout {
		log.Info(logline)
	}
	if len(w.addr) == 0 && len(w.scheme) == 0 {
		return nil
	}
	var err error
	err = w.checkConn()
	if err == nil {
		w.withRLock(func() {
			err = w.enc.Encode(logline)
		})
	}

	w.checkError(err)
	return err
}

// Close .
func (w *Writer) Close() error {
	// leave some time for the pending writing
	time.Sleep(CloseWaitInterval)
	var err error
	w.withLock(func() {
		if w.enc != nil {
			err = w.enc.Close()
			w.enc = nil
		}
	})
	log.Infof("[writer] writer for %s closed", w.addr)
	return err
}
