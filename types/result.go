package types

type (
	ResultType   = uint8
	CaseVerdict  int8
	FinalVerdict int8
	CaseResult   struct {
		CaseID        uint16
		ExecutionTime float32
		Memory        uint32
		Message       string
		Verdict       CaseVerdict
	}
	FinalResult struct {
		Points         float64
		MaxPoints      float64
		CompilerOutput string
		// For use on client-side only
		LastNonACVerdict CaseVerdict
		Verdict          FinalVerdict
	}
)

const (
	CaseVerdictAccepted CaseVerdict = iota
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
	ResultAck
)

func (verdict CaseVerdict) String() string {
	switch verdict {
	case CaseVerdictAccepted:
		return "ACCEPTED"
	case CaseVerdictWrongAnswer:
		return "WRONG_ANSWER"
	case CaseVerdictInternalError:
		return "INTERNAL_ERROR"
	case CaseVerdictTimeLimitExceeded:
		return "TIME_LIMIT_EXCEEDED"
	case CaseVerdictMemoryLimitExceeded:
		return "MEMORY_LIMIT_EXCEEDED"
	case CaseVerdictOutputLimitExceeded:
		return "OUTPUT_LIMIT_EXCEEDED"
	case CaseVerdictRuntimeError:
		return "RUNTIME_ERROR"
	}
	return "unknown"
}

func (verdict FinalVerdict) String() string {
	switch verdict {
	case FinalVerdictNormal:
		return "NORMAL"
	case FinalVerdictShortCircuit:
		return "SHORT_CIRCUIT"
	case FinalVerdictRejected:
		return "REJECTED"
	case FinalVerdictCancelled:
		return "CANCELLED"
	case FinalCompileError:
		return "COMPILE_ERROR"
	case FinalVerdictInitializationError:
		return "INIT_ERROR"
	}
	return "unknown"
}
