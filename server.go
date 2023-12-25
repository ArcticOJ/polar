package polar

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ArcticOJ/blizzard/v0/config"
	"github.com/ArcticOJ/blizzard/v0/db/models/contest"
	"github.com/ArcticOJ/blizzard/v0/logger"
	"github.com/ArcticOJ/polar/v0/shared"
	"github.com/ArcticOJ/polar/v0/types"
	"github.com/hashicorp/yamux"
	"github.com/mitchellh/mapstructure"
	"math"
	"net"
	"strings"
	"time"
)

func (p *Polar) handleConsumer(ctx context.Context, j *JudgeObj, conn *shared.EncodedConn) {
	var currentSubmission uint32 = math.MaxUint32
	defer func() {
		if currentSubmission != math.MaxUint32 {
			p.releaseSubmission(j, currentSubmission)
		}
	}()
	for conn.Scan() {
		var req types.Request
		if e := conn.Read(&req); e != nil {
			continue
		}
		switch req.Command {
		case types.CommandConsume:
			currentSubmission = math.MaxUint32
			sub := p.Pop(ctx, j.Runtimes)
			if sub == nil {
				continue
			}
			currentSubmission = sub.ID
			// mark this submission as pending
			p.pending.Store(sub.ID, make([]contest.CaseResult, sub.TestCount))
			// bind submission to this judge
			j.m.Lock()
			j.submissions[sub.ID] = struct{}{}
			j.m.Unlock()
			// this submission cannot be consumed, so better requeue it
			if conn.Write(sub) != nil {
				p.releaseSubmission(j, sub.ID)
			}
		}
	}
}

func (p *Polar) handleProducer(j *JudgeObj, conn *shared.EncodedConn, id uint32) {
	logger.Polar.Debug().Str("judge", j.Name).Stringer("addr", conn.Conn().RemoteAddr()).Uint32("submission", id).Msg("producer connected")
	j.m.RLock()
	_, isPending := j.submissions[id]
	j.m.RUnlock()
	if !isPending {
		return
	}
	isDone := false
	handler := p.messageHandler(id)
	for conn.Scan() {
		// submission is cancelled
		if !p.IsPending(id) {
			isDone = true
			break
		}
		var args types.ReportArgs
		if conn.Read(&args) != nil {
			continue
		}
		if handler(args.Type, args.Data) {
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
}

func (p *Polar) handleJudge(j types.Judge, conn *shared.EncodedConn) {
	id := hex.EncodeToString([]byte(fmt.Sprintf("%s-%d", j.Name, time.Now().UnixMilli())))
	obj := &JudgeObj{
		Judge:       j,
		submissions: make(map[uint32]struct{}),
	}
	logger.Polar.Debug().
		Str("name", j.Name).
		Str("id", id).
		Stringer("addr", conn.Conn().RemoteAddr()).
		Msg("judge connected")
	p.jm.Lock()
	p.judges[id] = obj
	p.jm.Unlock()
	p.RegisterRuntimes(obj.Runtimes)
	defer p.destroy(id)
	if conn.Write(id) != nil {
		return
	}
	s, e := yamux.Server(conn.Conn(), shared.MuxConfig())
	if e != nil {
		logger.Polar.Debug().Err(e).Stringer("addr", conn.Conn().RemoteAddr()).Msg("error multiplexing connection")
		return
	}
	ctx, cancel := context.WithCancel(p.ctx)
	go func() {
		<-s.CloseChan()
		cancel()
	}()
	for {
		c, e := s.Accept()
		if e != nil {
			return
		}
		go p.handleConsumer(ctx, obj, shared.NewEncodedConn(c))
	}
}

func (p *Polar) handleConn(conn net.Conn) {
	defer conn.Close()
	c := shared.NewEncodedConn(conn)
	if !c.Scan() {
		return
	}
	var args types.RegisterArgs
	// parse the first payload as args
	if c.Read(&args) != nil {
		return
	}
	if strings.TrimSpace(args.Secret) != strings.TrimSpace(config.Config.Polar.Secret) {
		logger.Polar.Warn().Stringer("type", args.Type).Stringer("addr", conn.RemoteAddr()).Msg("client tried to connect with invalid secret")
		return
	}
	switch args.Type {
	case types.ConnJudge:
		var judge types.Judge
		if mapstructure.Decode(args.Data, &judge) != nil {
			return
		}
		p.handleJudge(judge, c)
	case types.ConnProducer:
		p.jm.RLock()
		j := p.judges[args.JudgeID]
		p.jm.RUnlock()
		id, ok := args.Data.(uint32)
		if !ok {
			return
		}
		p.handleProducer(j, c, id)
	}
}

func (p *Polar) createServer() {
	conf := net.ListenConfig{}
	l, err := conf.Listen(p.ctx, "tcp", net.JoinHostPort(config.Config.Host, fmt.Sprint(config.Config.Polar.Port)))
	logger.Panic(err, "failed to initialize polar")
	logger.Polar.Info().Msgf("polar listening on port %d", config.Config.Polar.Port)
	for {
		conn, e := l.Accept()
		if e != nil {
			logger.Polar.Debug().Err(err).Stringer("addr", conn.RemoteAddr()).Msg("error accepting connection")
			continue
		}
		go p.handleConn(conn)
	}
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
	p.parallelism -= judgeObj.Parallelism
	delete(p.judges, judgeId)
	p.jm.Unlock()
	judgeObj.m.Lock()
	for _, rt := range judgeObj.Runtimes {
		if q, _ok := p.queued.Load(rt.ID); _ok {
			q.count.Add(^uint32(0))
		}
	}
	for id := range judgeObj.submissions {
		p.releaseSubmission(judgeObj, id)
	}
	judgeObj.m.Unlock()
}
