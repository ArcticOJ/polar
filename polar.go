package polar

import (
	"container/list"
	"context"
	"encoding/json"
	"entgo.io/ent/dialect/sql"
	"errors"
	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/orcaman/concurrent-map/v2"
	"necron.dev/pkg/ArcticOJ/bridge"
	"necron.dev/pkg/ArcticOJ/config"
	"necron.dev/pkg/ArcticOJ/db"
	"necron.dev/pkg/ArcticOJ/db/problem"
	"necron.dev/pkg/ArcticOJ/db/submission"
	"necron.dev/pkg/ArcticOJ/di"
	"necron.dev/pkg/ArcticOJ/logger"
	"necron.dev/pkg/ArcticOJ/polar/pb"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type (
	Polar struct {
		m           sync.RWMutex
		subscribers cmap.ConcurrentMap[uint32, *submissionSubscribers]

		conf        config.BridgeConfig
		queued      cmap.ConcurrentMap[string, *queue]
		submissions cmap.ConcurrentMap[uint32, *pb.Submission]
		pending     cmap.ConcurrentMap[uint32, []db.CaseResult]
		isBound     cmap.ConcurrentMap[uint32, struct{}]

		// Maximum allowed concurrent submissions.
		parallelism uint16
		runtimes    []*pb.RuntimeDefinition
		judges      map[uint32]*JudgeObj

		// Lock for reading/writing to judges field above.
		jm  sync.RWMutex
		ctx context.Context
	}
	queue struct {
		count    atomic.Uint32
		mutex    sync.RWMutex
		slice    []*pb.Submission
		waitChan chan string
	}
	JudgeObj struct {
		*pb.Judge

		// Internal properties below.

		submissions map[uint32]struct{}
		m           sync.RWMutex
	}
	submissionSubscribers struct {
		m sync.RWMutex
		l *list.List
	}
)

func shardingFn(k uint32) uint32 {
	return k
}

func (p *Polar) registerRuntimes(runtimes []*pb.Judge_Runtime) {
	for _, rt := range runtimes {
		// register a runtime if not present or update it
		p.queued.SetIfAbsent(rt.Id, &queue{
			waitChan: make(chan string, 1),
		})
		q, _ := p.queued.Get(rt.Id)
		q.count.Add(1)
	}
}

// TODO: remove `isPending` check, force callers to choose whether to requeue this submission or simply ignore it

func (p *Polar) releaseSubmission(j *JudgeObj, id uint32) {
	p.jm.RLock()
	_, isPending := j.submissions[id]
	p.jm.RUnlock()
	if isPending {
		p.jm.Lock()
		delete(j.submissions, id)
		p.jm.Unlock()
		if sub, exist := p.submissions.Get(id); exist && p.IsPending(id) {
			p.pending.Remove(id)
			p.push(sub, true)
		}
		return
	}
}

func (p *Polar) destroy(judgeId uint32) {
	p.jm.Lock()
	judgeObj := p.judges[judgeId]
	p.parallelism -= uint16(judgeObj.Parallelism)
	delete(p.judges, judgeId)
	p.jm.Unlock()
	judgeObj.m.Lock()
	for _, rt := range judgeObj.Runtimes {
		if q, _ok := p.queued.Get(rt.Id); _ok {
			// minus one
			q.count.Add(^uint32(0))
		}
	}
	for id := range judgeObj.submissions {
		p.releaseSubmission(judgeObj, id)
	}
	judgeObj.m.Unlock()
}

func loadRuntimeDefinitions() (defs []*pb.RuntimeDefinition, e error) {
	buf, e := os.ReadFile("runtime_definitions.json")
	if e != nil {
		return
	}
	e = json.Unmarshal(buf, &defs)
	if len(defs) == 0 {
		e = errors.New("runtime definitions are empty")
	}
	return
}

func (p *Polar) getPendingSubmissions() (r []*pb.Submission) {
	submissions := di.C(p.ctx).DB().Acquire().Submission.Query().
		WithProblem(func(query *db.ProblemQuery) {
			query.Select(problem.FieldID, problem.FieldTestCount)
		}).
		Order(submission.ByCreatedAt(sql.OrderDesc())).
		AllX(p.ctx)
	r = make([]*pb.Submission, len(submissions))
	for i, s := range submissions {
		r[i] = s.Polarize(s.Edges.Problem)
	}
	return
}

func (p *Polar) handleResult(ctx context.Context) func(*pb.Result) bool {
	lastNonAcVerdict := pb.CaseVerdict_ACCEPTED
	tx, e := di.C(ctx).DB().Acquire().Tx(ctx)
	logger.PanicIfE(e, "error starting db tx")
	id := ctx.Value("id").(uint32)
	return func(result *pb.Result) bool {
		switch res := result.Data.(type) {
		case *pb.Result_Case:
			if res.Case.Verdict != pb.CaseVerdict_ACCEPTED {
				lastNonAcVerdict = res.Case.Verdict
			}
			cr := tx.CaseResult.Create().
				SetOrder(uint16(res.Case.CaseId)).
				SetFeedback(res.Case.Feedback).
				SetVerdict(resolveVerdict(res.Case.Verdict)).
				SetMemory(res.Case.Memory).
				SetExecutionTime(res.Case.ExecutionTime).
				SetSubmissionID(id).
				SaveX(ctx)
			p.updateResult(id, cr)
			p.publish(id, sse.Message{
				Data: *cr,
			})
		case *pb.Result_Final:
			res.Final.LastNonAcVerdict = lastNonAcVerdict
			fv := getFinalVerdict(res.Final)
			defer p.DestroySubscribers(id)
			p.publish(id, sse.Message{
				Data: bridge.FinalJudgement{
					CompilerOutput: res.Final.CompilerOutput,
					Verdict:        fv,
				},
			})
			logger.PanicIfE(tx.Commit(), "error committing submission results")
			return true
		case *pb.Result_None:
			p.publish(id, sse.Message{
				Data: bridge.Ack{},
			})
		}
		return false
	}
}

func (p *Polar) onJudgeConnected(id uint32) error {
	return di.C(p.ctx).DB().Acquire().Judge.UpdateOneID(id).
		SetLastConnected(time.Now()).
		Exec(p.ctx)
}

func (p *Polar) publish(id uint32, msg sse.Message) {
	if subscribers, ok := p.subscribers.Get(id); ok {
		subscribers.m.RLock()
		for v := subscribers.l.Front(); v != nil; v = v.Next() {
			select {
			case v.Value.(chan sse.Message) <- msg:
			default:
			}
		}
		subscribers.m.RUnlock()
	}
}
