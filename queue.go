package polar

import (
	"context"
	"necron.dev/pkg/ArcticOJ/polar/common"
	"necron.dev/pkg/ArcticOJ/polar/pb"
	"reflect"
)

func (q *queue) pop() *pb.Submission {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if len(q.slice) > 0 {
		toReturn := q.slice[0]
		q.slice = q.slice[1:]
		return toReturn
	}
	return nil
}

func (p *Polar) populate(s []*pb.Submission) {
	for _, _s := range s {
		p.submissions.Set(_s.Id, _s)
		p.queued.SetIfAbsent(_s.RuntimeId, &queue{
			slice:    nil,
			waitChan: make(chan string, 1),
		})
		q, _ := p.queued.Get(_s.RuntimeId)
		q.slice = append(q.slice, _s)
	}
}

func (p *Polar) push(s *pb.Submission, forced bool) error {
	q, ok := p.queued.Get(s.RuntimeId)
	// count of consumers with this runtime waiting
	cnt := q.count.Load()
	if !ok && !forced {
		return common.ErrNoRuntime
	}
	p.submissions.Set(s.Id, s)
	q.slice = append(q.slice, s)
	if cnt > 0 {
		// Notify a "random" judge waiting for a submission with this runtime ID.
		select {
		case q.waitChan <- s.RuntimeId:
		default:
		}
	}
	return nil
}

func (p *Polar) pop(ctx context.Context, runtimes []*pb.Judge_Runtime) *pb.Submission {
	cases := []reflect.SelectCase{{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}}
	for _, rt := range runtimes {
		if q, ok := p.queued.Get(rt.Id); ok {
			if sub := q.pop(); sub != nil {
				return sub
			}
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(q.waitChan),
			})
		}
	}
	for {
		chosen, val, received := reflect.Select(cases)
		// The first case is ctx.Done(), so return if it's chosen.
		if !received || chosen == 0 {
			return nil
		}
		q, ok := p.queued.Get(val.String())
		if !ok {
			return nil
		}
		if sub := q.pop(); sub != nil {
			return sub
		}
	}
}

func (p *Polar) getSubmission(id uint32) *pb.Submission {
	sub, ok := p.submissions.Get(id)
	if !ok {
		return nil
	}
	return sub
}
