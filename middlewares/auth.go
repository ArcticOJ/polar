package middlewares

import (
	"context"
	"errors"
	"fmt"
	"necron.dev/pkg/ArcticOJ/polar/common"
	"storj.io/drpc"
	"storj.io/drpc/drpcmetadata"
)

type (
	authMiddleware struct {
		queryFn queryFunc
		next    drpc.Handler
	}
	queryFunc func(context.Context, string) (uint32, string, error)
)

func AuthMiddleware(queryFn queryFunc) Middleware {
	return func(next drpc.Handler) drpc.Handler {
		return authMiddleware{
			queryFn: queryFn,
			next:    next,
		}
	}
}

func (a authMiddleware) HandleRPC(stream drpc.Stream, rpc string) error {
	data, ok := drpcmetadata.Get(stream.Context())
	if !ok {
		return errors.New("invalid metadata")
	}
	secret, ok := data[common.SecretMetadataKey]
	if !ok {
		return errors.New("secret not found")
	}
	if judgeId, judgeName, e := a.queryFn(stream.Context(), secret); e == nil {
		fmt.Println(judgeId)
		return a.next.HandleRPC(streamWithValues(stream,
			"id", judgeId,
			"name", judgeName,
		), rpc)
	}
	return errors.New("invalid secret")
}
