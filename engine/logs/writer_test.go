package logs

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/agent/types"
)

func TestNewWriter(t *testing.T) {
	// udp writer
	addr := "udp://127.0.0.1:23456"
	w, err := NewWriter(addr, true)
	assert.NoError(t, err)

	err = w.createUDPConn()
	assert.NoError(t, err)

	w.Write(&types.Log{
		ID:   "testID",
		Name: "hello",
	})
	w.Close()

	// tcp writer
	addr = "tcp://127.0.0.1:34567"
	tcpL, err := net.Listen("tcp", ":34567")
	assert.NoError(t, err)

	defer tcpL.Close()
	w, err = NewWriter(addr, true)
	assert.NoError(t, err)

	err = w.createTCPConn()
	assert.NoError(t, err)

	w.Close()
}
