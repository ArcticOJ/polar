package polar

import (
	"github.com/ArcticOJ/polar/v0/pb"
	"github.com/ArcticOJ/polar/v0/shared"
	"strconv"
)

type Producer struct {
	id     uint32
	stream pb.DRPCPolar_ConnectAsProducerClient
}

func (p *Polar) NewProducer(id uint32) (prod *Producer, e error) {
	prod = &Producer{
		id: id,
	}
	prod.stream, e = p.client.ConnectAsProducer(p.createContext(
		shared.JudgeIdMetadataKey, p.id,
		shared.SubmissionIdMetadataKey, strconv.FormatUint(uint64(id), 10),
	))
	return
}

func (p *Producer) Report(res *pb.Result) error {
	return p.stream.Send(res)
}

func (p *Producer) Close() {
	p.stream.Close()
}
