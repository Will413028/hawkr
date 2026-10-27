// Package grpcclient centralizes the internal gRPC client boilerplate shared
// by every services/*/internal/client/*/client.go dial site: insecure
// transport credentials and client-side keepalive, applied first, followed
// by whatever DialOptions (auth interceptor, OTel stats handler, ...) the
// caller already uses today. Dial never changes, adds, or removes a
// caller's interceptor or stats-handler choice, and a caller-supplied
// grpc.WithKeepaliveParams in opts still wins (grpc.NewClient uses the last
// DialOption of a given kind) — it only removes the duplicated
// grpc.WithTransportCredentials(insecure.NewCredentials()) line that was
// hand-written identically at every internal gRPC client constructor in
// this repo, and gives every caller a real liveness signal it didn't have
// before (see the keepalive block in Dial for why).
package grpcclient

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Dial connects to target with insecure transport credentials and this
// package's client keepalive parameters applied first, followed by opts in
// the order given. opts is exactly what each call site already passes to
// grpc.NewClient today (its own interceptor, stats handler, or nothing) —
// Dial does not inspect, reorder, or filter it, and appending opts last
// means a caller that supplies its own grpc.WithKeepaliveParams still
// overrides the default below.
func Dial(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	allOpts := make([]grpc.DialOption, 0, len(opts)+2)
	allOpts = append(allOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	allOpts = append(allOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		// Time/Timeout give every internal gRPC client a real liveness signal
		// on an idle-but-open stream — replacing the application-layer
		// per-frame idle guess gateway/internal/proxy/ai_assistant_stream.go
		// used to stand in for this, before that guess was removed in favor
		// of this transport-level check. PermitWithoutStream stays false:
		// this repo's clients only need liveness checks while a stream is
		// actually open, and true would additionally ping fully-idle
		// connections, generating traffic this fix has no reason to add.
		//
		// Time (20s) MUST stay <= the server's KeepaliveEnforcementPolicy
		// MinTime (pkg/server/grpc.go BuildServerOptions, currently 10s) or
		// the server sends GOAWAY("too_many_pings") and tears the connection
		// down — the exact opposite of what a liveness check is for. If you
		// change either value, check the other file first.
		Time:                20 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: false,
	}))
	allOpts = append(allOpts, opts...)
	return grpc.NewClient(target, allOpts...)
}
