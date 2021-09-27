package common

const (
	// DockerCliVersion for docker cli version
	DockerCliVersion = "1.35"

	// StatusDie for die status
	StatusDie = "die"
	// StatusStart for start status
	StatusStart = "start"

	// DateTimeFormat for datetime format
	DateTimeFormat = "2006-01-02 15:04:05.999999"

	// DOCKERIZED detect agent in docker
	DOCKERIZED = "AGENT_IN_DOCKER"

	// LocalIP .
	LocalIP = "127.0.0.1"

	// DockerRuntime use docker as runtime
	DockerRuntime = "docker"
	// YavirtRuntime use yavirt as runtime
	YavirtRuntime = "yavirt"
	// MocksRuntime use the mock runtime
	MocksRuntime = "mocks"

	// GRPCStore use gRPC as store
	GRPCStore = "grpc"
	// MocksStore use the mock store
	MocksStore = "mocks"

	// ETCDKV use ETCD as KV
	ETCDKV = "etcd"
	// MocksKV use the mock KV
	MocksKV = "mocks"

	// ERUNodeName key of workload's name label
	ERUNodeName = "eru.nodename"
	// ERUCoreID key of workload's core ID label
	ERUCoreID = "eru.coreid"
)
