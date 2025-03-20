package common

import "errors"

var (
	ErrInvalidMetadata = errors.New("could not parse metadata from request")
	ErrAlreadyJudged   = errors.New("this submission is already judged")
	ErrReqDeserialize  = errors.New("could not deserialize request")
	ErrInvalidCommand  = errors.New("invalid command")
	ErrUnhandled       = errors.New("unhandled")
	ErrNoRuntime       = errors.New("no runtimes to handle this submission")
	ErrJudgeRejected   = errors.New("judge does not support any runtimes")
)
