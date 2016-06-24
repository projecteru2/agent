package types

type Container struct {
	ID         string
	Pid        int
	Alive      bool
	Name       string
	EntryPoint string
	Ident      string
	Version    string
	Extend     map[string]interface{}
}
