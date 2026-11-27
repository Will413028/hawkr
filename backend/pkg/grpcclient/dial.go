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
		Time:                20 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: false,
	}))
	allOpts = append(allOpts, opts...)
	return grpc.NewClient(target, allOpts...)
}
