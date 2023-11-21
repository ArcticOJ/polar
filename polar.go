package polar

import (
	"context"
	"github.com/ArcticOJ/blizzard/v0/db/models/contest"
	"github.com/ArcticOJ/polar/v0/types"
	cmap "github.com/orcaman/concurrent-map/v2"
	"sync"
	"sync/atomic"
)

func defShardingFn(u uint32) uint32 {
	return u
}

type (
	Polar struct {
		queued cmap.ConcurrentMap[string, *queue]
		// to resolve submission ids to submissions
		submissions    sync.Map
		pending        cmap.ConcurrentMap[uint32, []contest.CaseResult]
		judges         map[string]*types.JudgeObj
		ctx            context.Context
		messageHandler func(uint32) func(types.ResultType, interface{}) bool
	}
	queue struct {
		count    atomic.Uint32
		mutex    sync.RWMutex
		slice    []types.Submission
		waitChan chan string
	}
)

func NewPolar(ctx context.Context, messageHandler func(id uint32) func(t types.ResultType, data interface{}) bool) (p *Polar) {
	p = &Polar{
		queued:         cmap.New[*queue](),
		pending:        cmap.NewWithCustomShardingFunction[uint32, []contest.CaseResult](defShardingFn),
		ctx:            ctx,
		judges:         make(map[string]*types.JudgeObj),
		messageHandler: messageHandler,
	}
	return
}

func (p *Polar) RegisterRuntimes(runtimes []types.Runtime) {
	for _, rt := range runtimes {
		// register a runtime if not present or update it
		p.queued.Upsert(rt.ID, &queue{
			waitChan: make(chan string, 1),
		}, func(exist bool, q *queue, newq *queue) *queue {
			if !exist {
				newq.count.Add(1)
				return newq
			}
			q.count.Add(1)
			return q
		})
	}
}

func (p *Polar) UpdateResult(id uint32, result contest.CaseResult) bool {
	res, ok := p.pending.Get(id)
	if !ok {
		return false
	}
	p.pending.Set(id, append(res, result))
	return true
}

func (p *Polar) GetResult(id uint32) []contest.CaseResult {
	r, ok := p.pending.Get(id)
	if !ok {
		return nil
	}
	return r
}

func (p *Polar) IsPending(id uint32) bool {
	return p.pending.Has(id)
}

func (p *Polar) RuntimeSupported(runtime string) bool {
	q, ok := p.queued.Get(runtime)
	if !ok {
		return false
	}
	return q.count.Load() > 0
}

func (p *Polar) Reject(j *types.JudgeObj, id uint32) {
	p.releaseSubmission(j, id)
}

func (p *Polar) StartServer() {
	go p.createServer(p.ctx)
}

func (p *Polar) GetJudges() map[string]*types.JudgeObj {
	return p.judges
}
