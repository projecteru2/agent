package logs

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"sync"

	"github.com/projecteru2/agent/types"
	log "github.com/sirupsen/logrus"
)

// Writer is a writer!
type Writer struct {
	sync.Mutex
	addr    string
	scheme  string
	conn    io.WriteCloser
	stdout  bool
	encoder *json.Encoder
}

// NewWriter return writer
func NewWriter(addr string, stdout bool) (*Writer, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	writer := &Writer{addr: u.Host, scheme: u.Scheme}
	writer.stdout = stdout
	err = writer.CreateConn()
	return writer, err
}

// CreateConn create conn
func (w *Writer) CreateConn() error {
	var err error
	var conn io.WriteCloser
	switch w.scheme {
	case "udp":
		conn, err = w.createUDPConn()
	case "tcp":
		conn, err = w.createTCPConn()
	default:
		return fmt.Errorf("Invalid scheme: %s", w.scheme)
	}
	w.conn = conn
	w.encoder = json.NewEncoder(conn)
	return err
}

// Write write log to remote
func (w *Writer) Write(logline *types.Log) error {
	w.Lock()
	defer w.Unlock()
	if w.stdout {
		log.Info(logline)
	}
	err := w.encoder.Encode(logline)
	if err != nil {
		log.Error(err)
		err = w.conn.Close()
		w.CreateConn()
	}
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
