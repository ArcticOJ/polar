package types

type (
	ConnType uint8
	Request  struct {
		Command string
		Args    interface{}
	}
	Response struct {
		Event string
		Data  interface{}
	}
	ConnState struct {
		Type    ConnType
		JudgeID string
		Judge   Judge
	}
)

const (
	ConnJudge ConnType = iota + 1
	ConnProducer
)
