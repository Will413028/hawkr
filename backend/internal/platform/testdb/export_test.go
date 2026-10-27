package testdb

import (
	"context"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Peer 是指向同一個 container、但沒有任何行程內狀態的 Server：模擬另一個套件行程。
func (s *Server) Peer() *Server {
	return &Server{container: s.container, shared: true, adminURL: s.adminURL, templates: map[string]*template{}}
}

// AdminURL 是 container 的管理連線字串。
func (s *Server) AdminURL() string { return s.adminURL }

// RunFunc 是 Start 起 container 的函式型別。
type RunFunc = runFunc

// StartWith 以替身起 container。
func StartWith(ctx context.Context, run RunFunc) (*Server, error) { return start(ctx, run) }

// RealRun 是真的 tcpostgres.Run。
func RealRun(ctx context.Context, img string, opts ...testcontainers.ContainerCustomizer) (*tcpostgres.PostgresContainer, error) {
	return tcpostgres.Run(ctx, img, opts...)
}
