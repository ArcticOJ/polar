package middlewares

import (
	"fmt"
	"storj.io/drpc"
)

type panicMiddleware struct {
	next drpc.Handler
}

func PanicRecover() Middleware {
	return func(next drpc.Handler) drpc.Handler {
		return panicMiddleware{
			next: next,
		}
	}
}

func (p panicMiddleware) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = fmt.Errorf("%v", v)
		}
	}()
	return p.next.HandleRPC(stream, rpc)
}
