package polar

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/ArcticOJ/blizzard/v0/config"
	"github.com/ArcticOJ/blizzard/v0/logger"
	"github.com/ArcticOJ/polar/v0/types"
	"github.com/hashicorp/yamux"
	"github.com/mitchellh/mapstructure"
	"github.com/vmihailenco/msgpack"
	"math"
	"net"
	"strconv"
	"time"
)

func (p *Polar) handleConsumer(state types.ConnState, j *JudgeObj, conn net.Conn) {
	var currentSubmission uint32 = math.MaxUint32
	ctx, cancel := context.WithCancel(p.ctx)
	defer func() {
		cancel()
		if currentSubmission != math.MaxUint32 {
			p.releaseSubmission(j, currentSubmission)
		}
	}()
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	s := bufio.NewScanner(conn)
	enc := msgpack.NewEncoder(conn)
	for s.Scan() {
		var req types.Request
		if e := msgpack.Unmarshal(s.Bytes(), &req); e != nil {
			continue
		}
		switch req.Command {
		case types.CommandConsume:
			currentSubmission = math.MaxUint32
			sub := p.Pop(ctx, state.Judge.Runtimes)
			if sub == nil {
				continue
			}
			currentSubmission = sub.ID
			// mark this submission as pending
			p.pending.Set(sub.ID, nil)
			// bind submission to this judge
			j.submissions[sub.ID] = struct{}{}
			if enc.Encode(sub) != nil {
				continue
			}
			if _, e := conn.Write([]byte("\n")); e != nil {
				continue
			}
		}
	}
}

func (p *Polar) handleProducer(j *JudgeObj, conn net.Conn) {
	defer conn.Close()
	s := bufio.NewScanner(conn)
	if !s.Scan() {
		return
	}
	// grab bound submission from the first payload
	_id, e := strconv.ParseUint(s.Text(), 10, 32)
	if e != nil {
		return
	}
	id := uint32(_id)
	if _, ok := j.submissions[id]; !ok {
		return
	}
	handler := p.messageHandler(id)
	for s.Scan() {
		buf := s.Bytes()
		var args types.ReportArgs
		if e = msgpack.Unmarshal(buf, &args); e != nil {
			continue
		}
		if handler(args.Type, args.Data) {
			// cleanup finished submission
			p.pending.Remove(id)
			p.submissions.Delete(id)
			delete(j.submissions, id)
			return
		}
	}
	handler(types.ResultNone, nil)
}

func (p *Polar) handleRegistration(conn net.Conn) (state types.ConnState, e error) {
	var args types.RegisterArgs
	s := bufio.NewScanner(conn)
	if !s.Scan() {
		e = types.ErrReqDeserialize
		return
	}
	if e = msgpack.Unmarshal(s.Bytes(), &args); e != nil {
		return
	}
	switch state.Type = args.Type; state.Type {
	case types.ConnJudge:
		if e = mapstructure.Decode(args.Data, &state.Judge); e != nil {
			return
		}
		state.JudgeID = hex.EncodeToString([]byte(fmt.Sprintf("%s-%d", state.Judge.Name, time.Now().UnixMilli())))
		_, e = conn.Write([]byte(state.JudgeID + "\n"))
	case types.ConnProducer:
		if id, ok := args.Data.(string); ok {
			state.JudgeID = id
		} else {
			e = types.ErrReqDeserialize
		}
	}
	return
}

func (p *Polar) handleConnection(conn net.Conn) {
	var (
		session *yamux.Session
		err     error
		state   types.ConnState
		j       *JudgeObj
	)
	defer func() {
		conn.Close()
	}()
	// handle registration from the first payload
	if state, err = p.handleRegistration(conn); err != nil {
		logger.Polar.Debug().Err(err).Msgf("error handling registration from '%s'", conn.RemoteAddr())
	}
	switch state.Type {
	case types.ConnJudge:
		j = &JudgeObj{
			Judge:       state.Judge,
			submissions: make(map[uint32]struct{}),
		}
		j.submissions = make(map[uint32]struct{})
		p.judges[state.JudgeID] = j
		defer p.destroy(j, state.JudgeID)
		p.RegisterRuntimes(state.Judge.Runtimes)
		break
	case types.ConnProducer:
		if _j, ok := p.judges[state.JudgeID]; ok {
			j = _j
		} else {
			return
		}
		break
	default:
		return
	}
	conf := yamux.DefaultConfig()
	conf.KeepAliveInterval = time.Second
	session, err = yamux.Server(conn, conf)
	if err != nil {
		logger.Polar.Debug().Err(err).Stringer("addr", session.RemoteAddr()).Msgf("error multiplexing connection")
		return
	}
	for {
		sconn, e := session.Accept()
		if e != nil {
			if session.IsClosed() {
				logger.Polar.Debug().Stringer("addr", session.RemoteAddr()).Msg("session closed")
				break
			}
			logger.Polar.Debug().Err(e).Stringer("addr", sconn.RemoteAddr()).Msgf("error accepting connection")
			continue
		}
		switch state.Type {
		case types.ConnJudge:
			go p.handleConsumer(state, j, sconn)
		case types.ConnProducer:
			go p.handleProducer(j, sconn)
		}
	}
}

func (p *Polar) createServer(ctx context.Context) {
	conf := net.ListenConfig{
		KeepAlive: time.Second,
	}
	l, err := conf.Listen(ctx, "tcp", net.JoinHostPort(config.Config.Host, fmt.Sprint(config.Config.Polar.Port)))
	logger.Panic(err, "failed to initialize polar")
	logger.Polar.Info().Msgf("polar listening on port %d", config.Config.Polar.Port)
	for {
		conn, e := l.Accept()
		if e != nil {
			logger.Polar.Debug().Err(err).Stringer("addr", conn.RemoteAddr()).Msg("error accepting connection")
			continue
		}
		go p.handleConnection(conn)
	}
}

func (p *Polar) releaseSubmission(j *JudgeObj, id uint32) {
	if _, ok := j.submissions[id]; ok {
		if sub, exist := p.submissions.Load(id); exist && p.IsPending(id) {
			p.pending.Remove(id)
			// requeue submission
			p.Push(sub.(types.Submission), true)
		}
		delete(j.submissions, id)
	}
}

func (p *Polar) destroy(j *JudgeObj, judgeId string) {
	j.m.Lock()
	for _, rt := range j.Runtimes {
		if q, _ok := p.queued.Get(rt.ID); _ok {
			q.count.Add(^uint32(0))
		}
	}
	j.m.Unlock()
	for id := range j.submissions {
		p.releaseSubmission(j, id)
	}
	delete(p.judges, judgeId)
}
