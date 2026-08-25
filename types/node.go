package types

type Node struct {
	Name      string
	Endpoint  string
	Podname   string
	Labels    map[string]string
	Available bool
}

type NodeStatus struct {
	Nodename string
	Podname  string
	Alive    bool
	Error    error
}
