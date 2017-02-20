package common

const (
	DEFAULT_ETCD_PREFIX = "eru"
	ERU_AGENT_VERSION   = "2.0.9a"
	DOCKER_CLI_VERSION  = "v1.23"

	STATUS_DIE     = "die"
	STATUS_START   = "start"
	STATUS_DESTROY = "destroy"

	DATETIME_FORMAT = "2006-01-02 15:04:05"
	CNAME_NUM       = 3

	CGROUP_BASE_PATH = "/sys/fs/cgroup/%s/docker/%s/%s"

	VLAN_PREFIX = "cali0"
	DEFAULT_BR  = "eth0"
)
