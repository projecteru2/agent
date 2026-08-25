package types

type Node struct {
	Name     string
	Endpoint string
}

type NodeStatus struct {
	Nodename string
	Podname  string
	Alive    bool
	Error    error
}
