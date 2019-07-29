package types

import (
	"bufio"
	"net"
)

// Log for log
type Log struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	EntryPoint string            `json:"entrypoint"`
	Ident      string            `json:"ident"`
	Data       string            `json:"data"`
	Datetime   string            `json:"datetime"`
	Extra      map[string]string `json:"extra"`
}

// LogConsumer for log consumer
type LogConsumer struct {
	ID   string
	App  string
	Conn net.Conn
	Buf  *bufio.ReadWriter
}
