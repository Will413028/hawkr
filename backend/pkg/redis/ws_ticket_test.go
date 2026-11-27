package redis_test

import (
	"context"
	"testing"
	"time"

	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func wsTicketTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

func TestWSTicket_ConsumeIsSingleUse(t *testing.T) {
	rdb, _ := wsTicketTestRedis(t)
	ctx := context.Background()
	const token = "unit-test-ws-ticket"
	want := &pkgredis.WSTicket{AccountID: 42, AccountType: 3, ActiveRole: 2}

	if err := pkgredis.PutWSTicket(ctx, rdb, token, want, time.Minute); err != nil {
		t.Fatalf("PutWSTicket: %v", err)
	}

	got, err := pkgredis.ConsumeWSTicket(ctx, rdb, token)
	if err != nil {
		t.Fatalf("ConsumeWSTicket: %v", err)
	}
	if got == nil || got.AccountID != 42 || got.AccountType != 3 || got.ActiveRole != 2 {
		t.Fatalf("ConsumeWSTicket = %+v, want %+v", got, want)
	}

	again, err := pkgredis.ConsumeWSTicket(ctx, rdb, token)
	if err != nil {
		t.Fatalf("second ConsumeWSTicket err: %v", err)
	}
	if again != nil {
		t.Fatalf("second ConsumeWSTicket = %+v, want nil (already consumed)", again)
	}
}

func TestConsumeWSTicket_Missing_ReturnsNilNil(t *testing.T) {
	rdb, _ := wsTicketTestRedis(t)
	got, err := pkgredis.ConsumeWSTicket(context.Background(), rdb, "definitely-absent-ticket")
	if err != nil {
		t.Fatalf("err = %v, want nil for missing key", err)
	}
	if got != nil {
		t.Fatalf("got = %+v, want nil for missing key", got)
	}
}

func TestWSTicket_Expiry(t *testing.T) {
	rdb, mr := wsTicketTestRedis(t)
	ctx := context.Background()
	const token = "unit-test-ws-ticket-expiry"
	if err := pkgredis.PutWSTicket(ctx, rdb, token, &pkgredis.WSTicket{AccountID: 1}, 30*time.Second); err != nil {
		t.Fatalf("PutWSTicket: %v", err)
	}
	mr.FastForward(31 * time.Second)
	got, err := pkgredis.ConsumeWSTicket(ctx, rdb, token)
	if err != nil {
		t.Fatalf("ConsumeWSTicket: %v", err)
	}
	if got != nil {
		t.Fatalf("got = %+v, want nil after TTL expiry", got)
	}
}
