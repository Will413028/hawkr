//go:build integration

package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

func TestWatchRunEvents_LiveWakeDeliversNewEvent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementGRPCTestDB(ctx, t)
	defer cleanup()
	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())

	const buyerID = identity.BuyerID(9001)
	req, _, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)
	run, isNew, err := repo.ClaimRun(ctx, repository.ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "cr-live-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn, RequestID: req.ID,
		ControllerKind: domain.ControllerKindGraph,
	})
	require.NoError(t, err)
	require.True(t, isNew)

	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, DurableRunDeps{}, repo, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})

	watchCtx, cancelWatch := context.WithCancel(buyerActorContext(itoa64(int64(buyerID))))
	defer cancelWatch()
	stream := &fakeRunEventStream{ctx: watchCtx}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.WatchRunEvents(&aipb.WatchRunEventsRequest{RunId: itoa64(run.ID)}, stream)
	}()

	require.Eventually(t, func() bool {
		return len(stream.Sent()) >= 1
	}, 5*time.Second, 20*time.Millisecond, "initial replay event was not sent")

	initial := stream.Sent()
	require.Len(t, initial, 1)
	require.Equal(t, "1", initial[0].GetSeq())
	require.Equal(t, "RUN_ACCEPTED", initial[0].GetType())
	require.Equal(t, itoa64(run.ID), initial[0].GetRunId())

	_, err = repo.AppendRunEvent(ctx, repository.AppendRunEventInput{
		RunID: run.ID, BuyerID: buyerID, EventType: "TURN_APPLIED",
		SchemaVersion: "v1", PublicPayloadJSON: []byte(`{"foo":"bar"}`),
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(stream.Sent()) >= 2
	}, 5*time.Second, 20*time.Millisecond, "live-wake event was not delivered within timeout")

	got := stream.Sent()
	require.Len(t, got, 2)
	require.Equal(t, "2", got[1].GetSeq())
	require.Equal(t, "TURN_APPLIED", got[1].GetType())
	require.Equal(t, itoa64(run.ID), got[1].GetRunId())
	require.JSONEq(t, `{"foo":"bar"}`, string(got[1].GetDataJson()))

	cancelWatch()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("WatchRunEvents did not return after context cancel")
	}
}
