package shared

import (
	"github.com/hashicorp/yamux"
)

func MuxConfig() *yamux.Config {
	conf := yamux.DefaultConfig()
	conf.MaxStreamWindowSize = 2 << 20
	return conf
}
