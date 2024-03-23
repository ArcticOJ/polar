package polar

import (
	"context"
	"github.com/ArcticOJ/igloo/v0/config"
	"github.com/ArcticOJ/polar/v0/pb"
	"github.com/ArcticOJ/polar/v0/shared"
	"storj.io/drpc/drpcmetadata"
	"strconv"
)

type Producer struct {
	id     uint32
	stream pb.DRPCPolar_ConnectAsProducerClient
	ctx    context.Context
}

func (p *Polar) NewProducer(id uint32, ctx context.Context) (prod *Producer, e error) {
	prod = &Producer{
		id: id,
	}
	prod.ctx = drpcmetadata.AddPairs(
		ctx,
		map[string]string{
			shared.SecretHashMetadataKey:   config.Config.Polar.SecretHash,
			shared.JudgeIdMetadataKey:      p.id,
			shared.SubmissionIdMetadataKey: strconv.FormatUint(uint64(id), 10),
		},
	)
	prod.stream, e = p.client.ConnectAsProducer(prod.ctx)
	return
}

func (p *Producer) Report(res *pb.Result) error {
	return p.stream.Send(res)
}

func (p *Producer) Close() {
	p.stream.Close()
}

func (p *Producer) Context() context.Context {
	return p.ctx
}
