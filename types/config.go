package types

type DockerConfig struct {
	Endpoint string `yaml:"endpoint"`
}

type ETCDConfig struct {
	Prefix       string   `yaml:"prefix"`
	EtcdMachines []string `yaml:"etcd"`
}

type MetricsConfig struct {
	Step      int64    `yaml:"step"`
	Transfers []string `yaml:"transfers"`
}

type APIConfig struct {
	Addr string `yaml:"addr"`
}

type LogConfig struct {
	Forwards []string `yaml:"forwards"`
	Stdout   bool     `yaml:"stdout"`
}

type NICConfig struct {
	Physical []string `yaml:"physical"`
}

type Config struct {
	PidFile             string `yaml:"pid"`
	HostName            string
	HealthCheckInterval int `yaml:"health_check_interval"`

	Docker  DockerConfig
	Etcd    ETCDConfig
	Metrics MetricsConfig
	API     APIConfig
	Log     LogConfig
	NIC     NICConfig
}
