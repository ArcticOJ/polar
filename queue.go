package polar

import (
	"context"
	"github.com/ArcticOJ/polar/v0/types"
	"reflect"
	"slices"
)

func (q *queue) pop() *types.Submission {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if len(q.slice) > 0 {
		toReturn := q.slice[0]
		q.slice = q.slice[1:]
		return &toReturn
	}
	return nil
}

func (p *Polar) Populate(s []types.Submission) {
	for _, _s := range s {
		p.submissions.Store(_s.ID, _s)
		p.queued.Upsert(_s.Runtime, &queue{
			slice:    []types.Submission{_s},
			waitChan: make(chan string, 1),
		}, func(exist bool, q *queue, newq *queue) *queue {
			if !exist {
				return newq
			}
			q.slice = append(q.slice, _s)
			return q
		})
	}
}

func (p *Polar) Push(s types.Submission, force bool) error {
	q, ok := p.queued.Get(s.Runtime)
	// count of consumers with this runtime waiting
	cnt := q.count.Load()
	if !ok || cnt == 0 && !force {
		return types.ErrNoRuntime
	}
	p.submissions.Store(s.ID, s)
	q.slice = append(q.slice, s)
	if cnt > 0 {
		// notify ONE consumer waiting on this channel
		q.waitChan <- s.Runtime
	}
	return nil
}

func (p *Polar) Pop(ctx context.Context, runtimes []types.Runtime) *types.Submission {
	var cases = []reflect.SelectCase{{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	}}
	for _, rt := range runtimes {
		if q, ok := p.queued.Get(rt.ID); ok {
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
		q, ok := p.queued.Get(val.String())
		if !ok {
			return nil
		}
		if sub := q.pop(); sub != nil {
			return sub
		}
	}
}

func (p *Polar) Cancel(id uint32, userId string) bool {
	_sub, ok := p.submissions.Load(id)
	if !ok {
		return false
	}
	sub := _sub.(types.Submission)
	if sub.AuthorID != userId {
		return false
	}
	p.submissions.Delete(id)
	// If this submission was previously marked as pending, remove it from pending, judges will automatically cancel it when failing to report result
	if p.pending.Has(id) {
		p.pending.Remove(id)
		return false
	}
	q, ok := p.queued.Get(sub.Runtime)
	if !ok {
		return false
	}
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.slice = slices.DeleteFunc(q.slice, func(s types.Submission) bool {
		return s.ID == id
	})
	return true
}
