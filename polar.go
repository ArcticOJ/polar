package polar

import (
	"context"
	"github.com/ArcticOJ/blizzard/v0/db/schema/contest"
	"github.com/ArcticOJ/polar/v0/pb"
	csmap "github.com/mhmtszr/concurrent-swiss-map"
	"sync"
	"sync/atomic"
)

type (
	Polar struct {
		queued *csmap.CsMap[string, *queue]
		// to resolve submission ids to submissions
		submissions *csmap.CsMap[uint32, *pb.Submission]
		pending     *csmap.CsMap[uint32, []contest.CaseResult]
		isBound     *csmap.CsMap[uint32, struct{}]
		// maximum concurrent submissions
		parallelism uint16
		judges      map[string]*JudgeObj
		// mutex for reading/writing to judges
		jm             sync.RWMutex
		ctx            context.Context
		messageHandler func(uint32) func(result *pb.Result) bool
	}
	queue struct {
		count    atomic.Uint32
		mutex    sync.RWMutex
		slice    []*pb.Submission
		waitChan chan string
	}
	JudgeObj struct {
		*pb.Judge
		// internal properties
		// current submissions
		submissions map[uint32]struct{}
		m           sync.RWMutex
	}
)

func NewPolar(ctx context.Context, messageHandler func(id uint32) func(result *pb.Result) bool) (p *Polar) {
	p = &Polar{
		queued:         csmap.Create[string, *queue](),
		pending:        csmap.Create[uint32, []contest.CaseResult](),
		submissions:    csmap.Create[uint32, *pb.Submission](),
		isBound:        csmap.Create[uint32, struct{}](),
		ctx:            ctx,
		judges:         make(map[string]*JudgeObj),
		messageHandler: messageHandler,
	}
	return
}

func (p *Polar) RegisterRuntimes(runtimes []*pb.Judge_Runtime) {
	for _, rt := range runtimes {
		// register a runtime if not present or update it
		p.queued.SetIfAbsent(rt.Id, &queue{
			waitChan: make(chan string, 1),
		})
		q, _ := p.queued.Load(rt.Id)
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

func (p *Polar) GetResults(id uint32) []contest.CaseResult {
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
	go p.serveRPC()
}

func (p *Polar) GetJudges() map[string]*JudgeObj {
	p.jm.RLock()
	defer p.jm.RUnlock()
	return p.judges
}

func (p *Polar) releaseSubmission(j *JudgeObj, id uint32) {
	p.jm.RLock()
	_, isPending := j.submissions[id]
	p.jm.RUnlock()
	if isPending {
		p.jm.Lock()
		delete(j.submissions, id)
		p.jm.Unlock()
		if sub, exist := p.submissions.Load(id); exist && p.IsPending(id) {
			p.pending.Delete(id)
			// requeue submission
			p.Push(sub, true)
		}
		return
	}
}

func (p *Polar) destroy(judgeId string) {
	p.jm.Lock()
	judgeObj := p.judges[judgeId]
	p.parallelism -= uint16(judgeObj.Parallelism)
	delete(p.judges, judgeId)
	p.jm.Unlock()
	judgeObj.m.Lock()
	for _, rt := range judgeObj.Runtimes {
		if q, _ok := p.queued.Load(rt.Id); _ok {
			q.count.Add(^uint32(0))
		}
	}
	for id := range judgeObj.submissions {
		p.releaseSubmission(judgeObj, id)
	}
	judgeObj.m.Unlock()
}
