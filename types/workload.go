package types

// WorkloadStatus .
type WorkloadStatus struct {
	ID         string
	Running    bool
	Healthy    bool
	Networks   map[string]string
	Extension  []byte
	Appname    string
	Nodename   string
	Entrypoint string
}
