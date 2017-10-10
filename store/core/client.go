package corestore

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/projecteru2/agent/types"
	"google.golang.org/grpc"
)

type Client struct {
	addr string
	conn *grpc.ClientConn
}

func NewClient(config *types.Config) (*Client, error) {
	if config.Core == "" {
		return nil, fmt.Errorf("Core addr not set")
	}
	conn := connect(config.Core)

	return &Client{addr: config.Core, conn: conn}, nil
}

func connect(server string) *grpc.ClientConn {
	conn, err := grpc.Dial(server, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("[ConnectEru] Can not connect %v", err)
	}
	log.Debugf("[ConnectEru] Init eru connection %s", server)
	return conn
}
