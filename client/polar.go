package polar

import (
	"context"
	"fmt"
	"github.com/ArcticOJ/igloo/v0/config"
	"github.com/ArcticOJ/polar/v0/shared"
	"github.com/ArcticOJ/polar/v0/types"
	"github.com/hashicorp/yamux"
	"io"
	"net"
	"time"
)

type Polar struct {
	host string
	port uint16
	// a single connection is probably enough to handle submissions
	consumerConn    *shared.EncodedConn
	consumerSession *yamux.Session
	id              string
	CloseChan       chan types.ConnType
	ctx             context.Context
	cancel          func()
}

func New(_ctx context.Context, j types.Judge) (p *Polar, e error) {
	var conn net.Conn
	ctx, cancel := context.WithCancel(_ctx)
	p = &Polar{
		CloseChan: make(chan types.ConnType, 1),
		ctx:       ctx,
		cancel:    cancel,
		host:      config.Config.Polar.Host,
		port:      config.Config.Polar.Port,
	}
	conn, e = net.DialTimeout("tcp", net.JoinHostPort(p.host, fmt.Sprint(p.port)), time.Second)
	if e != nil {
		return
	}
	p.consumerConn = shared.NewEncodedConn(conn)
	p.id, e = p.registerJudge(j)
	if e != nil {
		return
	}
	if p.id == "" {
		e = types.ErrNoId
		return
	}
	p.consumerSession, e = yamux.Client(conn, shared.MuxConfig())
	return
}

func (p *Polar) Close() {
	p.consumerSession.Close()
	p.cancel()
}

func (p *Polar) registerJudge(j types.Judge) (id string, e error) {
	e = p.consumerConn.Write(types.RegisterArgs{
		Type:   types.ConnJudge,
		Secret: config.Config.Polar.Secret,
		Data:   j,
	})
	if e != nil {
		return
	}
	if !p.consumerConn.Scan() {
		e = io.EOF
		return
	}
	e = p.consumerConn.Read(&id)
	return
}
