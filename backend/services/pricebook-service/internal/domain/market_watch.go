package domain

import (
	"context"
	"errors"
)

const MarketWatchTargetLimit = 20

var (
	ErrInvalidWatchTarget = errors.New("invalid or inactive market watch target")
	ErrWatchLimit         = errors.New("market watch target limit reached")
)

type MarketWatchTargetResult struct {
	WatchTargetID int64
	Created       bool
}

type MarketWatchRepository interface {
	Create(ctx context.Context, accountID, newID int64, cropCode, marketCode string) (MarketWatchTargetResult, error)
	Delete(ctx context.Context, accountID, watchID int64) error
}
