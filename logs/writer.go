package logs

import (
	"errors"
	"fmt"
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
var ErrConnecting = errors.New("Connecting")

// Writer is a writer!
type Writer struct {
	sync.RWMutex
	addr       string
	scheme     string
	connecting bool
	stdout     bool
	enc        Encoder
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
func NewWriter(addr string, stdout bool) (*Writer, error) {
	if addr == Discard {
		return &Writer{
			enc: NewStreamEncoder(discard{}),
		}, nil
	}
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	writer := &Writer{addr: u.Host, scheme: u.Scheme, stdout: stdout}
	// pre-connect and ignore error
	_ = writer.checkConn()
	return writer, err
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
		err = fmt.Errorf("[writer] Invalid scheme: %s", w.scheme)
	}
	return enc, err
}

func (w *Writer) checkError(err error) {
	if err != nil && err != ErrConnecting {
		log.Errorf("[writer] Sending log failed %s", err)
		w.withLock(func() {
			if w.enc != nil {
				w.enc.Close()
				w.enc = nil
			}
		})
	}
}

func (w *Writer) checkConn() error {
	var err error
	w.withLock(func() {
		if w.enc != nil {
			// normal
			return
		}
		if w.connecting {
			err = ErrConnecting
			return
		}
		w.connecting = true
	})

	go func() {
		log.Debugf("[writer] Begin trying to connect to %s", w.addr)
		// retrying up to 4 times to prevent infinite loop
		for i := 0; i < 4; i++ {
			enc, err := w.createEncoder()
			if err == nil {
				w.withLock(func() {
					w.enc = enc
					w.connecting = false
				})
				log.Debugf("[writer] Connect to %s successfully", w.addr)
				return
			}
			log.Warnf("[writer] Failed to connect to %s: %s", w.addr, err)
			time.Sleep(30 * time.Second)
		}
		w.connecting = false
	}()
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
