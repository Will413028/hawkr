package server

import (
	"os"

	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
)

func init() {
	if err := os.Setenv("INTERNAL_AUTH_MODE", interceptor.InternalAuthModeOff); err != nil {
		panic(err)
	}
}
