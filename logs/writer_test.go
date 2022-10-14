package logs

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"

	"github.com/stretchr/testify/assert"
)

func TestNewWriterWithUDP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	utils.NewPool(1000)
	defer cancel()
	// udp writer
	addr := "udp://127.0.0.1:23456"
	w, err := NewWriter(ctx, addr, true)
	assert.NoError(t, err)
	assert.NoError(t, w.Write(&types.Log{}))
}

func TestNewWriterWithTCP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// tcp writer

	tcpL, err := net.Listen("tcp", ":34567")
	defer tcpL.Close()
	addr := "tcp://127.0.0.1:34567"
	w, err := NewWriter(ctx, addr, true)
	assert.NoError(t, err)
	assert.NoError(t, w.Write(&types.Log{}))
}

func TestNewWriterWithJournal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addr := "journal://system"
	enc, err := CreateJournalEncoder()
	if err == errJournalDisabled {
		return
	}
	assert.NoError(t, err)
	defer enc.Close()

	w, err := NewWriter(ctx, addr, true)
	assert.NoError(t, err)

	w.enc = enc
	err = w.enc.Encode(&types.Log{
		ID:         "id",
		Name:       "name",
		Type:       "type",
		EntryPoint: "entrypoint",
		Ident:      "ident",
		Data:       "data",
		Datetime:   "datetime",
		Extra:      map[string]string{"a": "1", "b": "2"},
	})
	assert.NoError(t, err)
}

func TestNewWriters(t *testing.T) {
	cases := map[string]error{
		Discard:                 nil,
		"udp://127.0.0.1:23456": nil,
		"tcp://127.0.0.1:34567": nil,
		// journal if enabled totally depends upon the system settings,
		// "journal://system":      errJournalDisabled,
		"invalid://hhh": nil,
	}
	tcpL, err := net.Listen("tcp", ":34567")
	assert.NoError(t, err)
	defer tcpL.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for addr, expectedErr := range cases {
		go func(addr string, expectedErr error) {
			writer, err := NewWriter(ctx, addr, false)
			assert.Equal(t, expectedErr, err)
			if expectedErr != nil {
				return
			}
			assert.NoError(t, err)
			err = writer.Write(&types.Log{})
			assert.NoError(t, err)
		}(addr, expectedErr)
	}
	// wait for closing all writers
	time.Sleep(CloseWaitInterval + 2*time.Second)
}

func TestReconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := "tcp://127.0.0.1:34567"
	writer, err := NewWriter(ctx, addr, false)
	assert.NoError(t, err)
	assert.Nil(t, writer.enc)
	assert.Equal(t, writer.needReconnect, true)

	tcpL, err := net.Listen("tcp", ":34567")
	assert.NoError(t, err)
	defer tcpL.Close()

	writer.reconnect()
	assert.NoError(t, writer.Write(&types.Log{}))
}
