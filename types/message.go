package types

type WorkloadEventMessage struct {
	ID       string
	Type     string
	Action   string
	TimeNano int64
}
