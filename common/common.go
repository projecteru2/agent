package common

const (
	StatusDie   = "die"
	StatusStart = "start"

	DateTimeFormat = "2006-01-02 15:04:05.999999"

	DOCKERIZED = "AGENT_IN_DOCKER"

	LocalIP = "127.0.0.1"

	// JournalIdentifier is the one term the journal reader matches, so every eru workload logs under it.
	JournalIdentifier = "eru"

	// NetworkLabelPrefix is where the oci hook writes a cni address and the containerd source reads it.
	NetworkLabelPrefix = "eru.network."

	ContainerdSocket    = "/run/containerd/containerd.sock"
	ContainerdNamespace = "eru"

	CocoonSocket = "/run/cocoond.sock"

	GRPCStore  = "grpc"
	MocksStore = "mocks"
)
