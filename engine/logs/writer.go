package logs

import (
	"encoding/json"
	"io"
	"net"
	"net/url"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/agent/types"
)

type Writer struct {
	addr    string
	scheme  string
	conn    io.Writer
	stdout  bool
	encoder *json.Encoder
	Close   func() error
}

func NewWriter(addr string, stdout bool) (*Writer, error) {
	u, err := url.Parse(addr)
	if err != nil {
		log.Errorf("Parse forward addr failed %s", err)
		return nil, err
	}
	writer := &Writer{addr: u.Host, scheme: u.Scheme}
	writer.stdout = stdout
	switch {
	case u.Scheme == "udp":
		err = writer.createUDPConn()
		return writer, err
	case u.Scheme == "tcp":
		err = writer.createTCPConn()
		return writer, err
	}
	return nil, nil
}

func (w *Writer) Write(logline *types.Log) error {
	if w.stdout {
		log.Info(logline)
	}
	return w.encoder.Encode(logline)
}

func (w *Writer) createUDPConn() error {
	udpAddr, err := net.ResolveUDPAddr("udp", w.addr)
	if err != nil {
		log.Errorf("Resolve %s failed %s", w.addr, err)
		return err
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		log.Errorf("Connect backend failed %s", err)
		return err
	}
	w.conn = conn
	w.encoder = json.NewEncoder(conn)
	w.Close = conn.Close
	return nil
}

func (w *Writer) createTCPConn() error {
	tcpAddr, err := net.ResolveTCPAddr("tcp", w.addr)
	if err != nil {
		log.Errorf("Resolve %s failed %s", w.addr, err)
		return err
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Errorf("Connect backend failed %s", err)
		return err
	}
	w.conn = conn
	w.encoder = json.NewEncoder(conn)
	w.Close = conn.Close
	return nil
}
