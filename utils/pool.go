package utils

import "github.com/alphadose/itogami"

const size = 10000

// Pool .
var Pool *itogami.Pool

func init() { //nolint
	Pool = itogami.NewPool(size)
}
