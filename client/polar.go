package polar

import (
	"context"
	"fmt"
	"github.com/ArcticOJ/igloo/v0/config"
	"github.com/ArcticOJ/igloo/v0/logger"
	"github.com/ArcticOJ/polar/v0/pb"
	"github.com/ArcticOJ/polar/v0/shared"
	"go.elara.ws/drpc/muxconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"net"
	"storj.io/drpc/drpcmetadata"
)

type Polar struct {
	id     string
	ctx    context.Context
	stream pb.DRPCPolar_ConnectAsJudgeClient
	client pb.DRPCPolarClient
	cancel func()
}

func marshal[T proto.Message](msg T) *anypb.Any {
	if res, e := anypb.New(msg); e == nil {
		return res
	}
	return nil
}

func New(_ctx context.Context, j *pb.Judge) (p *Polar, e error) {
	ctx, cancel := context.WithCancel(_ctx)
	p = &Polar{
		ctx:    ctx,
		cancel: cancel,
	}
	addr := net.JoinHostPort(config.Config.Polar.Host, fmt.Sprint(config.Config.Polar.Port))
	var dialer net.Dialer
	rconn, e := dialer.DialContext(ctx, "tcp", addr)
	if e != nil {
		return
	}
	conn, e := muxconn.New(rconn)
	if e != nil {
		return
	}
	p.ctx = drpcmetadata.Add(ctx, shared.SecretHashMetadataKey, config.Config.Polar.SecretHash)
	p.client = pb.NewDRPCPolarClient(conn)
	p.stream, e = p.client.ConnectAsJudge(p.ctx)
	if e != nil {
		return
	}
	p.stream.Send(&pb.Request{
		Type: pb.Request_REGISTER,
		Data: marshal(j),
	})
	resp, e := p.stream.Recv()
	if e != nil {
		return
	}
	p.id = resp.GetJudgeId()
	logger.Logger.Debug().Str("judge_id", p.id).Msg("connected to polar")
	if len(p.id) == 0 {
		e = shared.ErrNoId
	}
	return
}

func (p *Polar) Close() {
	p.stream.Close()
	p.cancel()
}

func (p *Polar) Consume() *pb.Submission {
	p.stream.Send(&pb.Request{
		Type: pb.Request_CONSUME,
	})
	sub, e := p.stream.Recv()
	if e != nil {
		return nil
	}
	return sub.GetSubmission()
}

func (p *Polar) createContext(additionalData ...string) (ctx context.Context) {
	ctx = p.ctx
	for i := 0; i < len(additionalData); i += 2 {
		ctx = drpcmetadata.Add(ctx, additionalData[i], additionalData[i+1])
	}
	return
}
