package types

import "errors"

var (
	ErrReqDeserialize = errors.New("could not deserialize request")
	ErrInvalidCommand = errors.New("invalid command")
	ErrUnhandled      = errors.New("unhandled")
	ErrNoRuntime      = errors.New("no runtimes to handle this submission")
	ErrNoId           = errors.New("eof: could not retrieve id for this judge")
)
