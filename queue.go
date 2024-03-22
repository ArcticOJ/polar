package polar

import (
	"context"
	"github.com/ArcticOJ/polar/v0/pb"
	"github.com/ArcticOJ/polar/v0/shared"
	"reflect"
	"slices"
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

func (p *Polar) Populate(s []*pb.Submission) {
	for _, _s := range s {
		p.submissions.Store(_s.Id, _s)
		p.queued.SetIfAbsent(_s.Runtime, &queue{
			slice:    nil,
			waitChan: make(chan string, 1),
		})
		q, _ := p.queued.Load(_s.Runtime)
		q.slice = append(q.slice, _s)
	}
}

func (p *Polar) Push(s *pb.Submission, forced bool) error {
	q, ok := p.queued.Load(s.Runtime)
	// count of consumers with this runtime waiting
	cnt := q.count.Load()
	if !ok && !forced {
		return shared.ErrNoRuntime
	}
	p.submissions.Store(s.Id, s)
	q.slice = append(q.slice, s)
	if cnt > 0 {
		// notify ONE consumer waiting on this channel
		select {
		case q.waitChan <- s.Runtime:
		default:
		}
	}
	return nil
}

func (p *Polar) Pop(ctx context.Context, runtimes []*pb.Judge_Runtime) *pb.Submission {
	cases := []reflect.SelectCase{{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}}
	for _, rt := range runtimes {
		if q, ok := p.queued.Load(rt.Id); ok {
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
		// the first case is ctx.Done(), so if it's chosen, return
		if !received || chosen == 0 {
			return nil
		}
		q, ok := p.queued.Load(val.String())
		if !ok {
			return nil
		}
		if sub := q.pop(); sub != nil {
			return sub
		}
	}
}

func (p *Polar) GetSubmission(id uint32) *pb.Submission {
	sub, ok := p.submissions.Load(id)
	if !ok {
		return nil
	}
	return sub
}

func (p *Polar) Cancel(id uint32, userId string) bool {
	sub, ok := p.submissions.Load(id)
	if !ok {
		return false
	}
	if sub.AuthorId != userId {
		return false
	}
	p.submissions.Delete(id)
	// If this submission was previously marked as pending, remove it from pending, judges will automatically cancel it when failing to report result
	if p.pending.Delete(id) {
		return false
	}
	q, ok := p.queued.Load(sub.Runtime)
	if !ok {
		return false
	}
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.slice = slices.DeleteFunc(q.slice, func(s *pb.Submission) bool {
		return s.Id == id
	})
	return true
}
