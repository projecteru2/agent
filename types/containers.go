package types

import "sync"

type Container struct {
	sync.Mutex
	Cid        string
	Pid        int
	Alive      bool
	Name       string
	EntryPoint string
	Ident      string
	Extend     map[string]interface{}
}
