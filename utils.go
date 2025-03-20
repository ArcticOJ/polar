package polar

import (
	"necron.dev/pkg/ArcticOJ/db/schema"
	"necron.dev/pkg/ArcticOJ/polar/pb"
)

func resolveVerdict(v pb.CaseVerdict) schema.Verdict {
	switch v {
	case pb.CaseVerdict_ACCEPTED:
		return schema.VerdictAccepted
	case pb.CaseVerdict_WRONG_ANSWER:
		return schema.VerdictWrongAnswer
	case pb.CaseVerdict_INTERNAL_ERROR:
		return schema.VerdictInternalError
	case pb.CaseVerdict_TIME_LIMIT_EXCEEDED:
		return schema.VerdictTimeLimitExceeded
	case pb.CaseVerdict_MEMORY_LIMIT_EXCEEDED:
		return schema.VerdictMemoryLimitExceeded
	case pb.CaseVerdict_OUTPUT_LIMIT_EXCEEDED:
		return schema.VerdictOutputLimitExceeded
	case pb.CaseVerdict_RUNTIME_ERROR:
		return schema.VerdictRuntimeError
	}
	return ""
}

func getFinalVerdict(f *pb.FinalResult) (v schema.Verdict) {
	v = ""
	if f.Verdict == pb.FinalVerdict_SHORT_CIRCUIT || f.Verdict == pb.FinalVerdict_NORMAL {
		v = schema.VerdictAccepted
		if f.LastNonAcVerdict != pb.CaseVerdict_ACCEPTED {
			v = resolveVerdict(f.LastNonAcVerdict)
		}
	} else {
		switch f.Verdict {
		case pb.FinalVerdict_CANCELLED:
			v = schema.VerdictCancelled
		case pb.FinalVerdict_REJECTED:
			v = schema.VerdictRejected
		case pb.FinalVerdict_INITIALIZATION_ERROR:
			v = schema.VerdictInternalError
		case pb.FinalVerdict_COMPILATION_ERROR:
			v = schema.VerdictCompilationError
		}
	}
	return
}
