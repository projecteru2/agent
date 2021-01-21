package selfmon

import (
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	pb "github.com/projecteru2/core/rpc/gen"
)

// Detector .
type Detector interface {
	Detect(*pb.Node) error
}

type coreDetector struct {
	m *Selfmon
}

func (d coreDetector) Detect(node *pb.Node) error {
	timeout := time.Second * time.Duration(d.m.config.HealthCheck.Timeout)

	addr, err := d.m.parseEndpoint(node.Endpoint)
	if err != nil {
		return err
	}

	dial := func() error {
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err == nil {
			conn.Close()
		}
		return err
	}

	last := 3
	for i := 0; i <= last; i++ {
		if err = dial(); err == nil {
			log.Debugf("[selfmon] dial %s/%s ok", node.Name, node.Endpoint)
			break
		}

		log.Debugf("[selfmon] %d dial %s failed %v", i, node.Name, err)

		if i < last {
			time.Sleep(time.Second * (1 << i))
		}
	}

	return err

}
