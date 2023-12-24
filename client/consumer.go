package polar

import (
	"github.com/ArcticOJ/polar/v0/shared"
	"github.com/ArcticOJ/polar/v0/types"
	"io"
	"net"
)

type Consumer struct {
	runtime     string
	conn        *shared.EncodedConn
	MessageChan chan types.Submission
}

func (p *Polar) NewConsumer() (c *Consumer, e error) {
	var conn net.Conn
	c = &Consumer{
		MessageChan: make(chan types.Submission, 1),
	}
	if e != nil {
		return
	}
	conn, e = p.consumerSession.Open()
	if e != nil {
		return
	}
	c.conn = shared.NewEncodedConn(conn)
	return
}

func (c *Consumer) Consume() error {
	defer c.Close()
	for {
		if e := c.conn.Write(types.Request{
			Command: types.CommandConsume,
		}); e != nil {
			return e
		}
		if !c.conn.Scan() {
			return io.EOF
		}
		var sub types.Submission
		if c.conn.Read(&sub) != nil {
			continue
		}
		c.MessageChan <- sub
	}
}

func (c *Consumer) Close() {
	close(c.MessageChan)
	c.conn.Close()
}
