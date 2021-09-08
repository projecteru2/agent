package logs

import (
	"net"
	"testing"

	"github.com/projecteru2/agent/types"

	"github.com/stretchr/testify/assert"
)

func TestNewWriterWithUDP(t *testing.T) {
	// udp writer
	addr := "udp://127.0.0.1:23456"
	w, err := NewWriter(addr, true)
	assert.NoError(t, err)

	enc, err := w.createUDPEncoder()
	assert.NoError(t, err)

	w.enc = enc
	w.Write(&types.Log{})
}

func TestNewWriterWithTCP(t *testing.T) {
	// tcp writer
	addr := "tcp://127.0.0.1:34567"
	tcpL, err := net.Listen("tcp", ":34567")
	assert.NoError(t, err)

	defer tcpL.Close()
	w, err := NewWriter(addr, true)
	assert.NoError(t, err)

	enc, err := w.createTCPEncoder()
	assert.NoError(t, err)

	w.enc = enc
	w.enc.Encode(&types.Log{})
}

// func TestNewWriterWithJournal(t *testing.T) {
// 	addr := "journal://system"
// 	enc, err := CreateJournalEncoder()
// 	assert.NoError(t, err)
// 	defer enc.Close()

// 	w, err := NewWriter(addr, true)
// 	assert.NoError(t, err)

// 	w.enc = enc
// 	err = w.enc.Encode(&types.Log{
// 		ID: "id",
// 		Name: "name",
// 		Type: "type",
// 		EntryPoint: "entrypoint",
// 		Ident: "ident",
// 		Data: "data",
// 		Datetime: "datetime",
// 		Extra: map[string]string{"a": "1", "b": "2"},
// 	})
// 	assert.NoError(t, err)
// }
