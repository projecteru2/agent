package types

import (
	"bufio"
	"net"
)

type Log struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	EntryPoint string `json:"entrypoint"`
	Ident      string `json:"ident"`
	Data       string `json:"data"`
	Datetime   string `json:"datetime"`
	Zone       string `json:"zone"`
}

type LogConsumer struct {
	ID   string
	App  string
	Conn net.Conn
	Buf  *bufio.ReadWriter
}
