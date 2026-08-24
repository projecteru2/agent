package utils

import (
	"github.com/panjf2000/ants/v2"
)

const size = 10000

var Pool *ants.Pool

func init() { //nolint
	Pool, _ = ants.NewPool(size, ants.WithNonblocking(true))
}
