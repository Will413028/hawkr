package delegation

import (
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"google.golang.org/grpc/codes"
)

// ParseExpectedVersion converts the wire form of expectedVersion (a numeric
// string, §4) into the optional CAS argument the repositories take.
//
// "" means the caller sent none — the seller's own path — and yields nil, i.e.
// no compare-and-set. Anything non-numeric or negative is rejected with 9260
// rather than degraded into nil: silently dropping the predicate would turn a
// delegated write into last-writer-wins with no error anywhere, which is exactly
// the outcome the version column exists to prevent.
//
// Shared by the seller module and social-service; both feed it into `WHERE ... AND
// (expected < 0 OR version = expected)` queries whose sentinel for "no CAS" is a
// negative number, so a negative input must never reach the query.
func ParseExpectedVersion(raw string) (*int64, error) {
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		return nil, bizStatus(codes.InvalidArgument, bizcode.DelegationInvalidCommand,
			"expected_version must be a non-negative integer string")
	}
	return &v, nil
}
