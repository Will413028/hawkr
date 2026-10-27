package grpcclient

import "time"

// Timeout3s / Timeout5s / Timeout10s are the three timeout values already
// used across this repo's internal gRPC clients' context.WithTimeout(ctx,
// N*time.Second) calls. Values are copied verbatim from the current code —
// do not change them here; a deliberate value change is a separate,
// reviewed decision.
const (
	Timeout3s  = 3 * time.Second
	Timeout5s  = 5 * time.Second
	Timeout10s = 10 * time.Second
)
