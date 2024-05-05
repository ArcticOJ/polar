package middlewares

import "storj.io/drpc"

type Middleware = func(next drpc.Handler) drpc.Handler
