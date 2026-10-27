//go:build integration

// services/ai-service/internal/procurement/usecase/graph_activity_ledger_integration_test.go
//
// Permanent regression guard for the append-only two-phase Activity ledger
// design (T12 sub-project 3c): proves — against a REAL Postgres container,
// not a fake store — that a single node execution's entry (empty text) and
// completion (real text) snapshots land as two separate, real
// procurement_agent_run_events rows sharing one rowId (embedded in
// public_payload_json — there is no row_id column, the ledger stays
// append-only) with seq strictly increasing. TestAppendRunEvent_SeqMonotonic
// already proves seq increments across two unrelated AppendRunEvent calls;
// this test is additive — it proves the SAME rowId appears in both rows AND
// seq is still monotonic, which that test's two unrelated-EventType calls
// never exercised.
package usecase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func TestActivityLedger_Integration_RealPostgres_EntryAndCompletionShareRowIDWithMonotonicSeq(t *testing.T) {
	ctx := context.Background()

	db, drop := pg.Clone(ctx, t)
	defer drop()

	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(90200)
	run, _, err := repo.ClaimRun(ctx, repository.ClaimRunInput{
		BuyerID: buyerID, ClientRequestID: "activity-ledger-integration-1", CommandKind: "SUBMIT_TURN",
		PayloadHash: "hash-1", RunType: domain.RunTypeTurn,
		ControllerKind: domain.ControllerKindGraph,
	})
	require.NoError(t, err)

	node := NewValidateSlotsNode(seqIDs(9000))
	tctx := newTestAgentContext(t, map[string]any{
		"current_slots": domain.Slots{},
		"now":           time.Now().UTC(),
	})

	var payloads []ActivityEventPayload
	for ev, err := range node.Run(tctx, domain.SlotPatch{}) {
		require.NoError(t, err)
		if ev == nil || ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.FunctionCall == nil || p.FunctionCall.Name != emitActivityEventFuncName {
				continue
			}
			payload, ok := p.FunctionCall.Args["payload"].(ActivityEventPayload)
			require.True(t, ok)
			payloads = append(payloads, payload)
		}
	}
	require.Len(t, payloads, 2, "validate_slots must emit an entry snapshot then a completion snapshot")
	entry, completion := payloads[0], payloads[1]
	require.Equal(t, entry.RowID, completion.RowID, "entry and completion must share one rowId")
	require.Empty(t, entry.Text)
	require.NotEmpty(t, completion.Text)

	eventType, entryJSON := ledgerEventTypeForActivity(entry)
	entryEvent, err := repo.AppendRunEvent(ctx, repository.AppendRunEventInput{
		RunID: run.ID, BuyerID: buyerID, EventType: eventType,
		SchemaVersion: "v1", PublicPayloadJSON: entryJSON,
	})
	require.NoError(t, err)

	_, completionJSON := ledgerEventTypeForActivity(completion)
	completionEvent, err := repo.AppendRunEvent(ctx, repository.AppendRunEventInput{
		RunID: run.ID, BuyerID: buyerID, EventType: eventType,
		SchemaVersion: "v1", PublicPayloadJSON: completionJSON,
	})
	require.NoError(t, err)
	require.Greater(t, completionEvent.Seq, entryEvent.Seq)

	rows, err := pool.Query(ctx,
		`SELECT seq, public_payload_json FROM procurement_agent_run_events WHERE run_id = $1 AND event_type = $2 ORDER BY seq`,
		run.ID, eventType)
	require.NoError(t, err)
	defer rows.Close()

	var rowIDs []string
	var seqs []int64
	for rows.Next() {
		var seq int64
		var raw []byte
		require.NoError(t, rows.Scan(&seq, &raw))
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(raw, &decoded))
		seqs = append(seqs, seq)
		rowIDs = append(rowIDs, decoded["rowId"].(string))
	}
	require.NoError(t, rows.Err())
	require.Len(t, seqs, 2, "exactly two AGENT_ACTIVITY_SNAPSHOT rows for this run")
	require.Less(t, seqs[0], seqs[1], "seq strictly increasing across the two real INSERTs")
	require.Equal(t, rowIDs[0], rowIDs[1], "both real DB rows carry the same rowId in their JSON payload")
}
