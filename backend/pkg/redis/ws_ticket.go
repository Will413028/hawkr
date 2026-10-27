package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const wsTicketPrefix = "ws_ticket:"

func wsTicketKey(token string) string { return wsTicketPrefix + token }

// WSTicket is a short-lived, single-use credential minted by an authed POST and
// redeemed once on the WebSocket handshake. The browser cannot set WS headers
// and the httpOnly session cookie does not reach the cross-site WS in dev, so
// this opaque ticket carries the resolved identity into the wss URL instead.
// ActiveRole is the buyer/seller view (1=buyer, 2=seller, 0=none) resolved at
// mint time; AccountType is the role-set bitmask the account holds.
type WSTicket struct {
	AccountID   int64 `json:"accountId"`
	AccountType int32 `json:"accountType"`
	ActiveRole  int32 `json:"activeRole"`
}

// PutWSTicket stores t under token with the given (short) TTL. Reuse
// GenerateSessionToken to mint the opaque token — it is a 256-bit URL-safe
// random secret, exactly what a ticket needs.
func PutWSTicket(ctx context.Context, rdb *redis.Client, token string, t *WSTicket, ttl time.Duration) error {
	payload, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, wsTicketKey(token), payload, ttl).Err()
}

// ConsumeWSTicket atomically reads-and-deletes the ticket (GETDEL), making it
// single-use. Returns (nil, nil) if the ticket is absent, expired, or already
// consumed.
func ConsumeWSTicket(ctx context.Context, rdb *redis.Client, token string) (*WSTicket, error) {
	val, err := rdb.GetDel(ctx, wsTicketKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var t WSTicket
	if err := json.Unmarshal(val, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
