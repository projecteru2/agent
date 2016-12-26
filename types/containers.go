package types

type Container struct {
	ID         string
	Pid        int
	Alive      bool
	Healthy    bool
	Name       string
	EntryPoint string
	Ident      string
	Version    string
	CPUQuota   int64
	Extend     map[string]string
}
