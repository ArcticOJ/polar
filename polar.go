package polar

import (
	"context"
	"github.com/ArcticOJ/blizzard/v0/db/models/contest"
	"github.com/ArcticOJ/polar/v0/types"
	csmap "github.com/mhmtszr/concurrent-swiss-map"
	"sync"
	"sync/atomic"
)

type (
	Polar struct {
		queued *csmap.CsMap[string, *queue]
		// to resolve submission ids to submissions
		submissions *csmap.CsMap[uint32, types.Submission]
		pending     *csmap.CsMap[uint32, []contest.CaseResult]
		// maximum concurrent submissions
		parallelism uint16
		judges      map[string]*JudgeObj
		// mutex for reading/writing to judges
		jm             sync.RWMutex
		ctx            context.Context
		messageHandler func(uint32) func(types.ResultType, interface{}) bool
	}
	queue struct {
		count    atomic.Uint32
		mutex    sync.RWMutex
		slice    []types.Submission
		waitChan chan string
	}
	JudgeObj struct {
		types.Judge
		// internal properties
		// current submissions
		submissions map[uint32]struct{}
		m           sync.RWMutex
	}
)

func NewPolar(ctx context.Context, messageHandler func(id uint32) func(t types.ResultType, data interface{}) bool) (p *Polar) {
	p = &Polar{
		queued:         csmap.Create[string, *queue](),
		pending:        csmap.Create[uint32, []contest.CaseResult](),
		submissions:    csmap.Create[uint32, types.Submission](),
		ctx:            ctx,
		judges:         make(map[string]*JudgeObj),
		messageHandler: messageHandler,
	}
	return
}

func (p *Polar) RegisterRuntimes(runtimes []types.Runtime) {
	for _, rt := range runtimes {
		// register a runtime if not present or update it
		p.queued.SetIfAbsent(rt.ID, &queue{
			waitChan: make(chan string, 1),
		})
		q, _ := p.queued.Load(rt.ID)
		q.count.Add(1)
	}
}

func (p *Polar) UpdateResult(id uint32, result contest.CaseResult) bool {
	res, ok := p.pending.Load(id)
	if !ok {
		return false
	}
	res[result.ID-1] = result
	p.pending.Store(id, res)
	return true
}

func (p *Polar) GetResult(id uint32) []contest.CaseResult {
	r, ok := p.pending.Load(id)
	if !ok {
		return nil
	}
	return r
}

func (p *Polar) IsPending(id uint32) bool {
	return p.pending.Has(id)
}

func (p *Polar) RuntimeAvailable(runtime string) bool {
	q, ok := p.queued.Load(runtime)
	if !ok {
		return false
	}
	return q.count.Load() > 0
}

func (p *Polar) Reject(j *JudgeObj, id uint32) {
	p.releaseSubmission(j, id)
}

func (p *Polar) StartServer() {
	go p.createServer()
}

func (p *Polar) GetJudges() map[string]*JudgeObj {
	p.jm.RLock()
	defer p.jm.RUnlock()
	return p.judges
}
