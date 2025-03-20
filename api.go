package polar

import (
	"container/list"
	"context"
	"fmt"
	"github.com/danielgtaylor/huma/v2/sse"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/rs/zerolog/log"
	"go.elara.ws/drpc/muxserver"
	"necron.dev/pkg/ArcticOJ/bridge"
	"necron.dev/pkg/ArcticOJ/config"
	"necron.dev/pkg/ArcticOJ/db"
	"necron.dev/pkg/ArcticOJ/db/judge"
	"necron.dev/pkg/ArcticOJ/di"
	"necron.dev/pkg/ArcticOJ/polar/middlewares"
	"necron.dev/pkg/ArcticOJ/polar/pb"
	"net"
	"slices"
	"storj.io/drpc/drpcmux"
)

func Provide(conf config.Config) (bridge.API, error) {
	defs, e := loadRuntimeDefinitions()
	if e != nil {
		return nil, e
	}
	p := &Polar{
		subscribers: cmap.NewWithCustomShardingFunction[uint32, *submissionSubscribers](shardingFn),
		queued:      cmap.New[*queue](),
		submissions: cmap.NewWithCustomShardingFunction[uint32, *pb.Submission](shardingFn),
		pending:     cmap.NewWithCustomShardingFunction[uint32, []db.CaseResult](shardingFn),
		isBound:     cmap.NewWithCustomShardingFunction[uint32, struct{}](shardingFn),
		conf:        conf.Services.Bridge,
		runtimes:    defs,
		judges:      make(map[uint32]*JudgeObj),
	}
	return p, nil
}

func (p *Polar) DestroySubscribers(id uint32) {
	subscribers, ok := p.subscribers.Get(id)
	if !ok {
		return
	}
	subscribers.m.Lock()
	defer subscribers.m.Unlock()
	p.subscribers.RemoveCb(id, func(key uint32, v *submissionSubscribers, exists bool) bool {
		if !exists {
			return false
		}
		// iterate over linked list and then close & delete all subscribers
		for el := subscribers.l.Front(); el != nil; el = el.Next() {
			close(el.Value.(chan any))
		}
		subscribers.l.Init()
		subscribers = nil
		return true
	})
}

func (p *Polar) Subscribe(sub db.Submission, getReplayData func() []db.CaseResult) (chan sse.Message, func()) {
	subscribers, ok := p.subscribers.Get(sub.ID)
	if !ok {
		return nil, nil
	}
	subscribers.m.Lock()
	c := make(chan sse.Message, 1)
	c <- sse.Message{
		Data: bridge.Metadata{
			TestCount: sub.Edges.Problem.TestCount,
		},
	}
	for i, cr := range getReplayData() {
		c <- sse.Message{
			ID:   i,
			Data: cr,
		}
	}
	defer subscribers.m.Unlock()
	el := subscribers.l.PushBack(c)
	return c, func() {
		if !p.subscribers.Has(sub.ID) {
			return
		}
		subscribers.m.Lock()
		defer subscribers.m.Unlock()
		close(c)
		subscribers.l.Remove(el)
	}
}

func (p *Polar) Enqueue(sub db.Submission) error {
	p.subscribers.Set(sub.ID, &submissionSubscribers{
		l: list.New(),
	})
	return p.push(sub.Polarize(sub.Edges.Problem), false)
}

func (p *Polar) Warmup(ctx context.Context) {
	p.ctx = ctx
	p.populate(p.getPendingSubmissions())
}

func (p *Polar) updateResult(id uint32, result *db.CaseResult) bool {
	res, ok := p.pending.Get(id)
	if !ok {
		return false
	}
	res = append(res, *result)
	p.pending.Set(id, res)
	return true
}

func (p *Polar) GetResults(id uint32) []db.CaseResult {
	r, ok := p.pending.Get(id)
	if !ok {
		return nil
	}
	return r
}

func (p *Polar) IsPending(id uint32) bool {
	return p.pending.Has(id)
}

func (p *Polar) RuntimeAvailable(runtime string) bool {
	q, ok := p.queued.Get(runtime)
	if !ok {
		return false
	}
	return q.count.Load() > 0
}

func (p *Polar) Reject(j *JudgeObj, id uint32) {
	p.releaseSubmission(j, id)
}

func (p *Polar) GetJudges() map[uint32]*JudgeObj {
	p.jm.RLock()
	defer p.jm.RUnlock()
	return p.judges
}

func (p *Polar) Cancel(id uint32) bool {
	sub, ok := p.submissions.Get(id)
	if !ok {
		return false
	}
	p.submissions.Remove(id)
	// If this submission was previously marked as pending, remove it from pending submissions.
	return p.pending.RemoveCb(id, func(key uint32, v []db.CaseResult, exists bool) bool {
		if !exists {
			return false
		}
		q, ok := p.queued.Get(sub.RuntimeId)
		if !ok {
			return false
		}
		q.mutex.Lock()
		defer q.mutex.Unlock()
		q.slice = slices.DeleteFunc(q.slice, func(s *pb.Submission) bool {
			return s.Id == id
		})
		return true
	})
}

func (p *Polar) Serve() error {
	mux := drpcmux.New()
	if e := pb.DRPCRegisterPolar(mux, p); e != nil {
		return e
	}
	addr := net.JoinHostPort(p.conf.Host, fmt.Sprint(p.conf.Port))
	var lc net.ListenConfig
	l, e := lc.Listen(p.ctx, "tcp", addr)
	if e != nil {
		return e
	}
	defer l.Close()
	s := muxserver.New(wrapMiddlewares(mux,
		middlewares.PanicRecover(),
		middlewares.Logging(),
		middlewares.AuthMiddleware(func(ctx context.Context, secret string) (uint32, string, error) {
			j, e := di.C(ctx).DB().Acquire().Judge.Query().
				Where(judge.BoundSecret(secret)).
				Select(judge.FieldID, judge.FieldName).
				First(ctx)
			if e != nil {
				return 0, "", e
			}
			return j.ID, j.Name, nil
		}),
	))
	log.Info().Str("addr", addr).Msg("bridge server started")
	return s.Serve(p.ctx, l)
}
