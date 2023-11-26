package polar

import (
	"fmt"
	"github.com/ArcticOJ/polar/v0/types"
	"github.com/vmihailenco/msgpack"
	"net"
)

type Producer struct {
	id   uint32
	conn net.Conn
	enc  *msgpack.Encoder
}

func (p *Polar) NewProducer(id uint32) (prod *Producer, e error) {
	prod = &Producer{id: id}
	prod.conn, e = p.producerSession.Open()
	if e != nil {
		return
	}
	// set bound submission to given ID
	_, e = prod.conn.Write([]byte(fmt.Sprintf("%d\n", id)))
	if e != nil {
		return
	}
	prod.enc = msgpack.NewEncoder(prod.conn)
	return
}

func (p *Producer) Report(_type types.ResultType, data interface{}) error {
	return p.send(types.ReportArgs{Type: _type, Data: data})
}

func (p *Producer) send(data interface{}) (e error) {
	if e = p.enc.Encode(data); e != nil {
		return
	}
	// terminate message with newline
	_, e = p.conn.Write([]byte("\n"))
	return
}

func (p *Producer) Close() {
	p.conn.Close()
}
