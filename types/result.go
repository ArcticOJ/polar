package types

type (
	ResultType   uint8
	CaseVerdict  int8
	FinalVerdict int8
	CaseResult   struct {
		CaseID   uint16
		Duration float32
		Memory   uint32
		Message  string
		Verdict  CaseVerdict
	}
	FinalResult struct {
		Points           float64
		MaxPoints        float64
		CompilerOutput   string
		LastNonACVerdict CaseVerdict
		Verdict          FinalVerdict
	}
)

const (
	CaseVerdictNone CaseVerdict = iota
	CaseVerdictAccepted
	CaseVerdictWrongAnswer
	CaseVerdictInternalError
	CaseVerdictTimeLimitExceeded
	CaseVerdictMemoryLimitExceeded
	CaseVerdictOutputLimitExceeded
	CaseVerdictRuntimeError
)

const (
	FinalVerdictNormal FinalVerdict = iota
	FinalVerdictShortCircuit
	FinalVerdictRejected
	FinalVerdictCancelled
	FinalCompileError
	FinalVerdictInitializationError
)

const (
	ResultNone ResultType = iota + 1
	ResultCase
	ResultFinal
	ResultAnnouncement
)
