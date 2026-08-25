package logs

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
)

func TestNewWriterWithUDP(t *testing.T) {
	ctx := t.Context()
	addr := "udp://127.0.0.1:23456"
	w, err := NewWriter(ctx, addr, true)
	assert.NoError(t, err)
	assert.NoError(t, w.Write(ctx, &types.Log{}))
}

func TestNewWriterWithTCP(t *testing.T) {
	ctx := t.Context()

	tcpL, err := net.Listen("tcp", ":34567")
	require.NoError(t, err)
	defer tcpL.Close()

	addr := "tcp://127.0.0.1:34567"
	w, err := NewWriter(ctx, addr, true)
	assert.NoError(t, err)
	assert.NoError(t, w.Write(ctx, &types.Log{}))
}

func TestNewWriterWithJournal(t *testing.T) {
	ctx := t.Context()
	addr := "journal://system"
	enc, err := CreateJournalEncoder()
	if errors.Is(err, common.ErrJournalDisabled) {
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
		"invalid://hhh":         nil,
	}
	tcpL, err := net.Listen("tcp", ":34567")
	assert.NoError(t, err)
	defer tcpL.Close()

	ctx := t.Context()

	for addr, expectedErr := range cases {
		go func(addr string, expectedErr error) {
			writer, err := NewWriter(ctx, addr, false)
			assert.Equal(t, expectedErr, err)
			if expectedErr != nil {
				return
			}
			assert.NoError(t, err)
			err = writer.Write(ctx, &types.Log{})
			assert.NoError(t, err)
		}(addr, expectedErr)
	}
	time.Sleep(closeWaitInterval + 2*time.Second)
}

func TestDeadlineConnFailsAWriteNobodyReads(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	conn := &deadlineConn{Conn: client, timeout: 50 * time.Millisecond}
	_, err := conn.Write([]byte("nobody is reading this"))

	var timeout net.Error
	require.ErrorAs(t, err, &timeout)
	assert.True(t, timeout.Timeout())
}

func TestReconnect(t *testing.T) {
	ctx := t.Context()

	addr := "tcp://127.0.0.1:34567"
	writer, err := NewWriter(ctx, addr, false)
	assert.NoError(t, err)
	assert.Nil(t, writer.enc)
	assert.Equal(t, writer.needReconnect, true)

	tcpL, err := net.Listen("tcp", ":34567")
	assert.NoError(t, err)
	defer tcpL.Close()

	writer.reconnect(ctx)
	assert.NoError(t, writer.Write(ctx, &types.Log{}))
}
