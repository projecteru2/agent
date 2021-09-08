package types

// Node .
type Node struct {
	Name      string
	Endpoint  string
	Podname   string
	Labels    map[string]string
	Available bool
}

// NodeStatus .
type NodeStatus struct {
	Nodename string
	Podname  string
	Alive    bool
	Error    error
}
