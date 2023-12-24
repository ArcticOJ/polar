package types

type Command = string

const (
	// Register a judge / consumer / producer
	CommandRegister Command = "register"
	// Pop a submission from queue and mark as pending (consumer-side)
	CommandConsume Command = "consume"
	// Report either acknowledgement, test case result or final result (producer-side)
	CommandReport Command = "report"
)

type (
	RegisterArgs struct {
		Type    ConnType
		Secret  string
		JudgeID string
		Data    interface{}
	}
	ReportArgs struct {
		Type ResultType
		Data interface{}
	}
)
