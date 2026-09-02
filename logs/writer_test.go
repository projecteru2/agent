package logs

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
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

	var wg sync.WaitGroup
	for addr, expectedErr := range cases {
		wg.Go(func() {
			writer, err := NewWriter(ctx, addr, false)
			assert.Equal(t, expectedErr, err)
			if expectedErr != nil {
				return
			}
			assert.NoError(t, writer.Write(ctx, &types.Log{}))
		})
	}
	wg.Wait()
}

func TestWriteKeepsTheEncoderWhenADatagramIsTooBig(t *testing.T) {
	ctx := t.Context()
	w, err := NewWriter(ctx, "udp://127.0.0.1:23456", false)
	require.NoError(t, err)

	assert.Error(t, w.Write(ctx, &types.Log{Data: strings.Repeat("x", 128<<10)}))
	assert.NoError(t, w.Write(ctx, &types.Log{Data: "small"}))
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

	tcpL, err := net.Listen("tcp", ":34567")
	assert.NoError(t, err)
	defer tcpL.Close()

	writer.reconnect(ctx)
	assert.NoError(t, writer.Write(ctx, &types.Log{}))
}

func BenchmarkWriterWrite(b *testing.B) {
	line := &types.Log{
		ID:         "0123456789abcdef0123456789abcdef",
		Name:       "app",
		Type:       common.StreamStdout,
		EntryPoint: "web",
		Ident:      "ident",
		Data:       strings.Repeat("x", 200),
		Datetime:   "2026-08-31T00:00:00.000000000Z",
		Extra:      map[string]string{"zone": "z", "node": "n"},
	}
	ctx := b.Context()
	writers := map[string]*Writer{"encode": {addr: "encode", scheme: "tcp", enc: NewStreamEncoder(nopSink{})}}
	for name, addr := range map[string]string{"tcp": drainingTCP(b), "udp": drainingUDP(b)} {
		w, err := NewWriter(ctx, addr, false)
		require.NoError(b, err)
		writers[name] = w
	}
	for name, w := range writers {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := w.Write(ctx, line); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

type nopSink struct{}

func (nopSink) Write(p []byte) (int, error) { return len(p), nil }

func (nopSink) Close() error { return nil }

func drainingTCP(b *testing.B) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(b, err)
	b.Cleanup(func() { _ = l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(io.Discard, conn) }()
		}
	}()
	return "tcp://" + l.Addr().String()
}

func drainingUDP(b *testing.B) string {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(b, err)
	b.Cleanup(func() { _ = conn.Close() })
	go func() {
		buf := make([]byte, 64<<10)
		for {
			if _, _, err := conn.ReadFrom(buf); err != nil {
				return
			}
		}
	}()
	return "udp://" + conn.LocalAddr().String()
}
