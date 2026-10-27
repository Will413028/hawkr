//go:build integration

package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
)

func TestMessageRepository_CreateWithLock_InsertsMessage(t *testing.T) {
	pool := getTestPool(t)
	repo := NewMessageRepository(pool, database.NewTxManager(pool))
	ctx := context.Background()

	msg, err := repo.CreateWithLock(ctx, gen.CreateMessageParams{
		ID:          1001,
		RoomID:      "room-1",
		SenderID:    42,
		ReceiverID:  100,
		Type:        1,
		Content:     "hello",
		PreviewText: stringPointer("hello"),
		CreatedAt:   timePointer(time.Now()),
		IsRead:      boolPointer(false),
		SeqRoom:     "room-1",
	})
	if err != nil {
		t.Fatalf("CreateWithLock: %v", err)
	}
	if msg.ID != 1001 {
		t.Errorf("msg.ID = %d, want 1001", msg.ID)
	}
	if msg.SeqID != 1 {
		t.Errorf("msg.SeqID = %d, want 1 (first message in a fresh room)", msg.SeqID)
	}

	got, err := repo.GetByID(ctx, 1001)
	if err != nil {
		t.Fatalf("GetByID after CreateWithLock: %v", err)
	}
	if got.RoomID != "room-1" || got.Content != "hello" {
		t.Errorf("GetByID = %+v, want room_id=room-1 content=hello", got)
	}
}

// TestMessageRepository_CreateWithLock_ConcurrentSameRoom_AssignsSequentialSeqIDs
// is the regression test for the exact race CreateWithLock's advisory lock
// exists to prevent: seq_id is computed as COALESCE(MAX(seq_id),0)+1 scoped to
// room_id (db/queries/query.sql AcquireRoomLock/CreateMessage), so without the
// lock serializing concurrent transactions on the same room, two goroutines
// can both read the same MAX and insert the same seq_id — this is what
// idx_messages_room_seq's UNIQUE (room_id, seq_id) index would then reject for
// one of them. This test proves the txManager.WithTx-based rewrite still
// serializes correctly, not just that it compiles.
func TestMessageRepository_CreateWithLock_ConcurrentSameRoom_AssignsSequentialSeqIDs(t *testing.T) {
	pool := getTestPool(t)
	repo := NewMessageRepository(pool, database.NewTxManager(pool))
	ctx := context.Background()

	const n = 10
	const roomID = "room-concurrent"
	var wg sync.WaitGroup
	seqIDs := make([]int64, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg, err := repo.CreateWithLock(ctx, gen.CreateMessageParams{
				ID:          int64(2000 + idx),
				RoomID:      roomID,
				SenderID:    1,
				ReceiverID:  2,
				Type:        1,
				Content:     "concurrent",
				PreviewText: stringPointer("concurrent"),
				CreatedAt:   timePointer(time.Now()),
				IsRead:      boolPointer(false),
				SeqRoom:     roomID,
			})
			errs[idx] = err
			seqIDs[idx] = msg.SeqID
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: CreateWithLock: %v", i, err)
		}
	}

	seen := make(map[int64]bool, n)
	for _, id := range seqIDs {
		if seen[id] {
			t.Fatalf("seq_id %d assigned more than once — advisory lock did not serialize concurrent CreateWithLock calls for the same room, got %v", id, seqIDs)
		}
		seen[id] = true
	}
	for want := int64(1); want <= n; want++ {
		if !seen[want] {
			t.Errorf("seq_id %d never assigned, got %v", want, seqIDs)
		}
	}
}

// TestMessageRepository_CreateWithLock_FailureRollsBackNoPartialRow proves the
// rewrite still rolls back cleanly on failure the way the original hand-rolled
// `defer tx.Rollback(ctx)` did: forces a messages_pkey collision on the second
// CreateMessage inside the transaction (after AcquireRoomLock has already
// succeeded) and asserts no second row, and that the room's advisory lock
// (transaction-scoped: pg_advisory_xact_lock) released on rollback rather than
// wedging every later call for the same room.
func TestMessageRepository_CreateWithLock_FailureRollsBackNoPartialRow(t *testing.T) {
	pool := getTestPool(t)
	repo := NewMessageRepository(pool, database.NewTxManager(pool))
	ctx := context.Background()

	const roomID = "room-rollback"
	first, err := repo.CreateWithLock(ctx, gen.CreateMessageParams{
		ID:          3000,
		RoomID:      roomID,
		SenderID:    1,
		ReceiverID:  2,
		Type:        1,
		Content:     "first",
		PreviewText: stringPointer("first"),
		CreatedAt:   timePointer(time.Now()),
		IsRead:      boolPointer(false),
		SeqRoom:     roomID,
	})
	if err != nil {
		t.Fatalf("seed CreateWithLock: %v", err)
	}

	_, err = repo.CreateWithLock(ctx, gen.CreateMessageParams{
		ID:          first.ID, // reuse the same PK to force a duplicate-key failure
		RoomID:      roomID,
		SenderID:    1,
		ReceiverID:  2,
		Type:        1,
		Content:     "duplicate id",
		PreviewText: stringPointer("duplicate id"),
		CreatedAt:   timePointer(time.Now()),
		IsRead:      boolPointer(false),
		SeqRoom:     roomID,
	})
	if err == nil {
		t.Fatal("expected a duplicate primary key error, got nil")
	}

	var count int64
	if scanErr := pool.QueryRow(ctx,
		`SELECT count(*) FROM messages WHERE room_id = $1`, roomID,
	).Scan(&count); scanErr != nil {
		t.Fatalf("count messages: %v", scanErr)
	}
	if count != 1 {
		t.Errorf("messages in %s = %d, want 1 (the failed CreateMessage must not leave a second row — WithTx must have rolled back)", roomID, count)
	}

	third, err := repo.CreateWithLock(ctx, gen.CreateMessageParams{
		ID:          3001,
		RoomID:      roomID,
		SenderID:    1,
		ReceiverID:  2,
		Type:        1,
		Content:     "third",
		PreviewText: stringPointer("third"),
		CreatedAt:   timePointer(time.Now()),
		IsRead:      boolPointer(false),
		SeqRoom:     roomID,
	})
	if err != nil {
		t.Fatalf("CreateWithLock after rollback: %v (advisory lock may not have released)", err)
	}
	if third.SeqID != 2 {
		t.Errorf("third.SeqID = %d, want 2 (the failed insert must not have consumed a seq_id)", third.SeqID)
	}
}
