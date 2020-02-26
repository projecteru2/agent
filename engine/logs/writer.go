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

// ErrConnecting means writer is in connecting status, waiting to be connected
var ErrConnecting = errors.New("Connecting")

// Writer is a writer!
type Writer struct {
	sync.Mutex
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
	if addr == "__discard__" {
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
	writer.checkConn()
	return writer, err
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
		w.Lock()
		defer w.Unlock()
		log.Errorf("[writer] Sending log failed %s", err)
		if w.enc != nil {
			w.enc.Close()
			w.enc = nil
		}
	}
}

func (w *Writer) checkConn() error {
	if w.enc != nil {
		// normal
		return nil
	}
	if w.connecting == false {
		w.Lock()
		defer w.Unlock()
		// double check
		if w.connecting == true {
			return ErrConnecting
		}
		w.connecting = true
		go func() {
			log.Debugf("[writer] Begin trying to connect to %s", w.addr)
			// retrying up to 4 times to prevent infinite loop
			for i := 0; i < 4; i++ {
				enc, err := w.createEncoder()
				if err == nil {
					w.Lock()
					w.enc = enc
					w.connecting = false
					w.Unlock()
					break
				} else {
					log.Warnf("[writer] Failed to connect to %s: %s", w.addr, err)
					time.Sleep(30 * time.Second)
				}
			}
			if w.enc == nil {
				log.Warnf("[writer] Connect to %s failed for 4 times", w.addr)
				w.Lock()
				w.connecting = false
				w.Unlock()
			} else {
				log.Debugf("[writer] Connect to %s successfully", w.addr)
			}
		}()
	}
	return ErrConnecting
}

// Write write log to remote
func (w *Writer) Write(logline *types.Log) error {
	if w.stdout {
		log.Info(logline)
	}
	err := w.checkConn()
	if err == nil {
		err = w.enc.Encode(logline)
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
