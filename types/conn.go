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
)

const (
	ConnJudge ConnType = iota + 1
	ConnProducer
)

func (t ConnType) String() string {
	if t == ConnJudge {
		return "JUDGE"
	}
	return "PRODUCER"
}
