package workload

import (
	"bufio"
	"io"
	"testing"

	"github.com/projecteru2/agent/types"
)

func BenchmarkBroadcast(b *testing.B) {
	l := newLogBroadcaster()
	for range 2 {
		buf := bufio.NewReadWriter(bufio.NewReader(nil), bufio.NewWriter(io.Discard))
		l.subscribe(b.Context(), "nerv", buf)
	}
	log := &types.Log{
		ID: "cid0123456789", Name: "nerv", Type: "stdout", EntryPoint: "eva0",
		Ident: "abc", Datetime: "2026-08-25 10:00:00", Data: "a fairly ordinary log line from a workload",
		Extra: map[string]string{"podname": "eru", "nodename": "node-1"},
	}

	b.ReportAllocs()
	for b.Loop() {
		l.broadcast(b.Context(), log)
	}
}
