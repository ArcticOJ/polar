package types

type Command = string

const (
	// Register a judge (consumer-side) or register a producer cluster (producer-side)
	CommandRegister Command = "register"
	// Pop a submission from queue and mark as pending (consumer-side)
	CommandConsume Command = "consume"
	// Reject a submission
	CommandReject Command = "reject"

	// Report either acknowledgement, test case result or final result (producer-side)
	CommandReport Command = "report"
)

type (
	RegisterArgs struct {
		Type ConnType
		Data interface{}
	}
	ReportArgs struct {
		Type ResultType
		Data interface{}
	}
)
