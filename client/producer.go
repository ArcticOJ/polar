package polar

import (
	"fmt"
	"github.com/ArcticOJ/igloo/v0/config"
	"github.com/ArcticOJ/polar/v0/shared"
	"github.com/ArcticOJ/polar/v0/types"
	"net"
	"time"
)

type Producer struct {
	id   uint32
	conn *shared.EncodedConn
}

func (p *Polar) NewProducer(id uint32) (prod *Producer, e error) {
	var conn net.Conn
	prod = &Producer{id: id}
	conn, e = net.DialTimeout("tcp", net.JoinHostPort(p.host, fmt.Sprint(p.port)), time.Second*5)
	if e != nil {
		return
	}
	prod.conn = shared.NewEncodedConn(conn)
	// set bound submission to given ID
	e = prod.conn.Write(types.RegisterArgs{
		Type:    types.ConnProducer,
		JudgeID: p.id,
		Secret:  config.Config.Polar.Secret,
		Data:    id,
	})
	return
}

func (p *Producer) Report(_type types.ResultType, data interface{}) error {
	return p.conn.Write(types.ReportArgs{Type: _type, Data: data})
}

func (p *Producer) Close() {
	p.conn.Close()
}
