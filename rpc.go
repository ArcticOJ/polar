package polar

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ArcticOJ/blizzard/v0/config"
	"github.com/ArcticOJ/blizzard/v0/db/schema/contest"
	"github.com/ArcticOJ/blizzard/v0/logger"
	"github.com/ArcticOJ/polar/v0/middlewares"
	"github.com/ArcticOJ/polar/v0/pb"
	"github.com/ArcticOJ/polar/v0/shared"
	"go.elara.ws/drpc/muxserver"
	"net"
	"storj.io/drpc"
	"storj.io/drpc/drpcmetadata"
	"storj.io/drpc/drpcmux"
	"strconv"
	"time"
)

func wrapMiddlewares(base drpc.Handler, m ...middlewares.Middleware) drpc.Handler {
	for _, _m := range m {
		base = _m(base)
	}
	return base
}

func (p *Polar) serveRPC() {
	mux := drpcmux.New()
	logger.Panic(pb.DRPCRegisterPolar(mux, p), "error registering polar service")
	addr := net.JoinHostPort(config.Config.Host, fmt.Sprint(config.Config.Polar.Port))
	var lc net.ListenConfig
	l, err := lc.Listen(p.ctx, "tcp", addr)
	defer l.Close()
	logger.Panic(err, "failed to listen on %s", addr)
	s := muxserver.New(wrapMiddlewares(mux,
		middlewares.AuthMiddleware(config.Config.Polar.Secret),
		middlewares.PanicRecover(),
		middlewares.Logging(),
	))
	logger.Polar.Info().Msgf("polar listening on %s", addr)
	logger.Panic(s.Serve(p.ctx, l), "error serving polar server")
}

func (p *Polar) ConnectAsJudge(stream pb.DRPCPolar_ConnectAsJudgeStream) error {
	request, e := stream.Recv()
	if e != nil || request.Type != pb.Request_REGISTER {
		return stream.Close()
	}
	j := &pb.Judge{}
	if request.Data.UnmarshalTo(j) != nil {
		return stream.Close()
	}
	defer logger.Polar.Debug().Msg("client disconnected")
	id := hex.EncodeToString([]byte(fmt.Sprintf("%s-%d", j.Name, time.Now().UnixMilli())))
	obj := &JudgeObj{
		Judge:       j,
		submissions: make(map[uint32]struct{}),
	}
	logger.Polar.Debug().
		Str("name", j.Name).
		Str("id", id).
		Msg("judge connected")
	p.jm.Lock()
	p.judges[id] = obj
	p.jm.Unlock()
	p.RegisterRuntimes(obj.Runtimes)
	defer p.destroy(id)
	if e = stream.Send(&pb.Response{
		Data: &pb.Response_JudgeId{JudgeId: id},
	}); e != nil {
		return e
	}
	ctx := stream.Context()
	for {
		request, e = stream.Recv()
		if e != nil {
			break
		}
		switch request.Type {
		case pb.Request_CONSUME:
			sub := p.Pop(ctx, j.Runtimes)
			if sub == nil {
				break
			}
			// mark this submission as pending
			p.pending.Store(sub.Id, make([]contest.CaseResult, sub.TestCount))
			// bind submission to this judge
			obj.m.Lock()
			obj.submissions[sub.Id] = struct{}{}
			obj.m.Unlock()
			if stream.Send(&pb.Response{
				Data: &pb.Response_Submission{Submission: sub},
			}) != nil {
				// this submission cannot be consumed, so better requeue it
				p.releaseSubmission(obj, sub.Id)
				break
			}
		}
	}
	return nil
}

func parseProducerArgs(ctx context.Context) (id string, subId uint32, ok bool) {
	rawM, ok := drpcmetadata.Get(ctx)
	if !ok {
		return
	}
	id, ok = rawM[shared.JudgeIdMetadataKey]
	if !ok {
		return
	}
	_subId, ok := rawM[shared.SubmissionIdMetadataKey]
	if !ok {
		return
	}
	sid, e := strconv.ParseUint(_subId, 10, 32)
	subId, ok = uint32(sid), e == nil
	return
}

func (p *Polar) ConnectAsProducer(stream pb.DRPCPolar_ConnectAsProducerStream) error {
	judgeId, id, parseOk := parseProducerArgs(stream.Context())
	if !parseOk {
		return shared.ErrInvalidMetadata
	}
	p.jm.RLock()
	j := p.judges[judgeId]
	p.jm.RUnlock()
	if j == nil {
		return stream.Close()
	}
	logger.Polar.Debug().Str("judge", j.Name).Uint32("submission", id).Msg("producer connected")
	isAlreadyBound := false
	j.m.RLock()
	// add a safeguard to check whether current submission is already handled by another producer?
	p.isBound.SetIf(id, func(_ struct{}, present bool) (struct{}, bool) {
		isAlreadyBound = present
		return struct{}{}, !present
	})
	_, isPending := j.submissions[id]
	j.m.RUnlock()
	if isAlreadyBound || !isPending {
		return shared.ErrAlreadyJudged
	}
	var (
		e       error
		isDone  bool
		isAcked bool
	)
	handler := p.messageHandler(id)
	for {
		result, e := stream.Recv()
		if e != nil {
			break
		}
		// submission is cancelled
		if !p.IsPending(id) {
			isDone = true
			break
		}
		// in case of re-judgement, isAcked will be set to true multiple times
		if _, isAck := result.Data.(*pb.Result_None); isAck {
			if isAcked {
				if sub, ok := p.submissions.Load(id); ok {
					/*
						before re-judgement [res_1_1, res_1_2, res_1_3]
						if we don't nullify case results before proceeding, the array will end up like this: [res_2_1, res_1_2, res_1_3], resulting in inconsistency and false results specifically when enabling SHORT_CIRCUIT.
						(res_x_y denotes x-th judgement of test case y)
					*/
					p.pending.Store(sub.Id, make([]contest.CaseResult, sub.TestCount))
				}
			}
			isAcked = true
		}
		if handler(result) {
			isDone = true
			p.pending.Delete(id)
			p.submissions.Delete(id)
			break
		}
	}
	j.m.Lock()
	delete(j.submissions, id)
	j.m.Unlock()
	// if judge dies or current submission is rejected, requeue current submission
	if !isDone {
		if sub, ok := p.submissions.Load(id); ok {
			p.pending.Delete(id)
			// requeue submission
			p.Push(sub, true)
		}
	}
	return e
}
