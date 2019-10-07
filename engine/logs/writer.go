package logs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	conn       io.WriteCloser
	connecting bool
	stdout     bool
	encoder    *json.Encoder
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
		w := &Writer{conn: discard{}}
		w.encoder = json.NewEncoder(w.conn)
		return w, nil
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
func (w *Writer) createConn() (io.WriteCloser, error) {
	var err error
	var conn io.WriteCloser
	switch w.scheme {
	case "udp":
		conn, err = w.createUDPConn()
	case "tcp":
		conn, err = w.createTCPConn()
	default:
		return nil, fmt.Errorf("[writer] Invalid scheme: %s", w.scheme)
	}
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (w *Writer) checkError(err error) {
	if err != nil && err != ErrConnecting {
		w.Lock()
		defer w.Unlock()
		log.Errorf("[writer] Sending log failed %s", err)
		if w.conn != nil {
			w.conn.Close()
			w.conn = nil
		}
	}
}

func (w *Writer) checkConn() error {
	if w.conn != nil {
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
				conn, err := w.createConn()
				if err == nil {
					w.Lock()
					w.conn = conn
					w.encoder = json.NewEncoder(conn)
					w.connecting = false
					w.Unlock()
					break
				} else {
					log.Warnf("[writer] Failed to connect to %s: %s", w.addr, err)
					time.Sleep(30 * time.Second)
				}
			}
			if w.conn == nil {
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
		err = w.encoder.Encode(logline)
	}
	w.checkError(err)
	return err
}

func (w *Writer) createUDPConn() (io.WriteCloser, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", w.addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (w *Writer) createTCPConn() (io.WriteCloser, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", w.addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
