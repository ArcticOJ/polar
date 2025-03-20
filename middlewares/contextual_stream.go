package middlewares

import (
	"context"
	"storj.io/drpc"
)

type contextualStream struct {
	ctx context.Context
	drpc.Stream
}

func streamWithValues(next drpc.Stream, valPairs ...any) *contextualStream {
	ctx := next.Context()
	for i := 0; i < len(valPairs); i += 2 {
		ctx = context.WithValue(ctx, valPairs[i], valPairs[i+1])
	}
	return &contextualStream{
		ctx:    ctx,
		Stream: next,
	}
}

func (s contextualStream) Context() context.Context {
	return s.ctx
}
