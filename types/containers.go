package types

type Container struct {
	ID         string
	Pid        int
	Healthy    bool
	Name       string
	EntryPoint string
	Ident      string
	Version    string
	CPUQuota   int64
	CPUPeriod  int64
	CPUShares  int64
	Memory     int64
	Extend     map[string]string
}
