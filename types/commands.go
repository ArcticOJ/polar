package types

type Command = string

const (
	// Register a judge (consumer-side) or register a producer (producer-side)
	CommandRegister Command = "register"
	// Pop a submission from queue and mark as pending (consumer-side)
	CommandConsume = "consume"
	// Reject a submission
	CommandReject = "reject"

	// Report either test case result or final result (producer-side)
	CommandReport = "report"
	// Announce stages of submission process (compilation, completion, cancellation) (producer-side)
	CommandAnnounce = "announce"
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
