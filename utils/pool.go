package utils

import (
	"github.com/panjf2000/ants/v2"
)

// TODO configurableu
const size = 10000

// Pool .
var Pool *ants.Pool

func init() { //nolint
	Pool, _ = ants.NewPool(size, ants.WithNonblocking(true))
}
