package types

type Node struct {
	Endpoint string
}

type NodeStatus struct {
	Nodename string
	Alive    bool
}
