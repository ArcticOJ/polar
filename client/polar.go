package polar

import (
	"bufio"
	"context"
	"github.com/ArcticOJ/polar/v0/types"
	"github.com/hashicorp/yamux"
	"github.com/vmihailenco/msgpack"
	"io"
	"net"
	"time"
)

type Polar struct {
	consumerSession *yamux.Session
	producerSession *yamux.Session
	id              string
	CloseChan       chan types.ConnType
}

func clientConf() (conf *yamux.Config) {
	conf = yamux.DefaultConfig()
	conf.EnableKeepAlive = true
	conf.KeepAliveInterval = time.Second
	return
}

func New(ctx context.Context, host string, port uint16, j types.Judge) (p *Polar, e error) {
	var (
		consumerConn net.Conn
		producerConn net.Conn
	)
	p = &Polar{CloseChan: make(chan types.ConnType, 1)}
	if consumerConn, e = newConn(ctx, host, port); e != nil {
		return
	}
	if p.id, e = registerJudge(consumerConn, j); e != nil {
		return
	}
	conf := clientConf()
	if p.consumerSession, e = yamux.Client(consumerConn, conf); e != nil {
		return
	}
	if producerConn, e = newConn(ctx, host, port); e != nil {
		return
	}
	if e = registerProducerConn(producerConn, p.id); e != nil {
		return
	}
	p.producerSession, e = yamux.Client(producerConn, conf)
	return
}

func (p *Polar) Close() {
	p.consumerSession.Close()
	p.producerSession.Close()
}

func registerJudge(conn net.Conn, j types.Judge) (id string, e error) {
	if e = send(
		conn,
		types.RegisterArgs{
			Type: types.ConnJudge,
			Data: j,
		},
	); e != nil {
		return
	}
	s := bufio.NewScanner(conn)
	if !s.Scan() {
		e = types.ErrNoId
		return
	}
	id = s.Text()
	return
}

func registerProducerConn(conn net.Conn, id string) error {
	return send(
		conn,
		types.RegisterArgs{
			Type: types.ConnProducer,
			Data: id,
		},
	)
}

func send(w io.Writer, data interface{}) (e error) {
	var buf []byte
	buf, e = msgpack.Marshal(data)
	if e != nil {
		return
	}
	_, e = w.Write([]byte(string(buf) + "\n"))
	return
}
