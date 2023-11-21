package polar

import (
	"context"
	"fmt"
	"net"
	"time"
)

func newConn(ctx context.Context, host string, port uint16) (c net.Conn, e error) {
	d := net.Dialer{
		KeepAlive: time.Second,
	}
	c, e = d.DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	return
}
