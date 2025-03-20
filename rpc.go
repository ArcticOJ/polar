package polar

import (
	"context"
	"github.com/rs/zerolog/log"
	"necron.dev/pkg/ArcticOJ/polar/common"
	"necron.dev/pkg/ArcticOJ/polar/middlewares"
	"necron.dev/pkg/ArcticOJ/polar/pb"
	"necron.dev/pkg/ArcticOJ/utils/numeric"
	"storj.io/drpc"
	"storj.io/drpc/drpcmetadata"
)

func wrapMiddlewares(handler drpc.Handler, m ...middlewares.Middleware) drpc.Handler {
	for _, w := range m {
		handler = w(handler)
	}
	return handler
}

func (p *Polar) ConnectAsJudge(stream pb.DRPCPolar_ConnectAsJudgeStream) error {
	// Send handshake data with a regenerated ID and runtime definitions and wait for report of available runtimes from judge.
	judgeId := stream.Context().Value("id").(uint32)
	judgeName := stream.Context().Value("name").(string)
	if e := stream.Send(&pb.Response{
		Data: &pb.Response_HandshakeData{
			HandshakeData: &pb.HandshakeData{
				Runtimes: p.runtimes,
			},
		},
	}); e != nil {
		return e
	}
	request, e := stream.Recv()
	if e != nil || request.Type != pb.Request_REGISTER {
		return common.ErrInvalidCommand
	}
	j := &pb.Judge{}
	if request.Data.UnmarshalTo(j) != nil {
		return common.ErrReqDeserialize
	}
	if len(j.Runtimes) == 0 {
		return common.ErrJudgeRejected
	}
	if e = p.onJudgeConnected(judgeId); e != nil {
		return e
	}
	obj := &JudgeObj{
		Judge:       j,
		submissions: make(map[uint32]struct{}),
	}
	log.Debug().
		Uint32("id", judgeId).
		Str("name", judgeName).
		Msg("judge connected")
	p.jm.Lock()
	p.judges[judgeId] = obj
	p.jm.Unlock()
	defer log.Debug().
		Uint32("id", judgeId).
		Str("name", judgeName).
		Msg("judge disconnected")
	p.registerRuntimes(obj.Runtimes)
	defer p.destroy(judgeId)
	// Send an OK message indicating that judge is now usable.
	if e = stream.Send(&pb.Response{
		Data: nil,
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
			sub := p.pop(ctx, j.Runtimes)
			if sub == nil {
				break
			}

			p.pending.Set(sub.Id, nil)

			obj.m.Lock()
			obj.submissions[sub.Id] = struct{}{}
			obj.m.Unlock()
			if stream.Send(&pb.Response{
				Data: &pb.Response_Submission{Submission: sub},
			}) != nil {
				// This submission cannot be consumed, so better requeue it.
				p.releaseSubmission(obj, sub.Id)
				break
			}
		}
	}
	return nil
}

func parseContext(ctx context.Context) (judgeId uint32, subId uint32) {
	judgeId = ctx.Value("id").(uint32)
	rawM, ok := drpcmetadata.Get(ctx)
	if !ok {
		return
	}
	subId = numeric.Parse[uint32](rawM[common.SubmissionIdMetadataKey])
	return
}

func (p *Polar) ConnectAsProducer(stream pb.DRPCPolar_ConnectAsProducerStream) error {
	judgeId, submissionId := parseContext(stream.Context())
	p.jm.RLock()
	j := p.judges[judgeId]
	p.jm.RUnlock()
	if j == nil {
		return common.ErrReqDeserialize
	}
	log.Debug().
		Uint32("judge", judgeId).
		Uint32("submission", submissionId).
		Msg("producer connected")
	j.m.RLock()
	// Add a safeguard to check whether current submission is already handled by another producer.
	isAlreadyBound := p.isBound.Has(submissionId)
	if !isAlreadyBound {
		p.isBound.Set(submissionId, struct{}{})
	}
	_, isPending := j.submissions[submissionId]
	j.m.RUnlock()
	if isAlreadyBound || !isPending {
		return common.ErrAlreadyJudged
	}
	var (
		e       error
		isDone  bool
		isAcked bool
	)
	handler := p.handleResult(context.WithValue(stream.Context(), "id", submissionId))
	for {
		result, e := stream.Recv()
		if e != nil {
			break
		}
		// Submission is "probably" cancelled?
		if !p.IsPending(submissionId) {
			isDone = true
			break
		}
		if _, isAck := result.Data.(*pb.Result_None); isAck {
			if isAcked {
				if sub, ok := p.submissions.Get(submissionId); ok {
					p.pending.Set(sub.Id, nil)
				}
			}
			isAcked = true
		}
		if handler(result) {
			isDone = true
			p.pending.Remove(submissionId)
			p.submissions.Remove(submissionId)
			break
		}
	}
	j.m.Lock()
	delete(j.submissions, submissionId)
	j.m.Unlock()
	// If judge crashes or current submission is rejected, requeue it outrightly.
	if !isDone {
		if sub, ok := p.submissions.Get(submissionId); ok {
			p.pending.Remove(submissionId)
			p.push(sub, true)
		}
	}
	return e
}
