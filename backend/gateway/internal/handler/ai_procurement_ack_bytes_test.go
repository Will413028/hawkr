package handler

import (
	"encoding/json"
	"testing"

	"github.com/gin-gonic/gin"
)

// The three procurement ack endpoints used to answer with a bare gin.H literal.
// Replacing those with named structs is what puts them in the Swagger contract
// (API 設計規範 §2 forbids anonymous objects), but it also changes how the keys
// are ordered: encoding/json sorts map keys, while a struct marshals in
// declaration order.
//
// These tests pin the resulting bytes to the gin.H they replaced. They fail if
// someone reorders the struct fields into "logical" order — which would be a
// silent wire change on endpoints whose only consumer (hawkr-website)
// nobody would think to re-check for a field reordering.
//
// The gin.H literals below are copied verbatim from the pre-refactor
// implementations; keep them as-is, they are the baseline, not a fixture to
// update.
//
// The one allowed edit is an *intentional additive* key, and it must be
// announced in the literal like the candidateSetId case below. Reordering the
// struct's existing fields is never allowed — that is the silent change these
// tests exist to catch.

func TestProcurementStartCandidateSearchAckBytesUnchanged(t *testing.T) {
	legacy := gin.H{
		"requestId": "77", "runId": "501", "status": "RUNNING",
		// Intentional additive key (#2446 identity-contract expand wave, 設計書 57 §5.3):
		// the RunAccepted envelope must carry an allowlisted domainRef to the candidate
		// set, because §1.2-3 forbids runId from doubling as that reference. Additive
		// only — no existing key changed name, order, or meaning in this wave.
		"candidateSetId": "501",
	}
	current := ProcurementStartCandidateSearchResponseDTO{
		CandidateSetID: "501", RequestID: "77", RunID: "501", Status: "RUNNING",
	}
	assertSameJSONBytes(t, legacy, current)
}

func TestProcurementSelectionAckBytesUnchanged(t *testing.T) {
	legacy := gin.H{
		"requestId": "77", "candidateSetId": "501",
		"lane": "PLATFORM", "candidateId": "1", "selected": true,
	}
	current := ProcurementSelectionResponseDTO{
		CandidateID: "1", CandidateSetID: "501",
		Lane: "PLATFORM", RequestID: "77", Selected: true,
	}
	assertSameJSONBytes(t, legacy, current)
}

func assertSameJSONBytes(t *testing.T, legacy gin.H, current any) {
	t.Helper()
	want, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy gin.H: %v", err)
	}
	got, err := json.Marshal(current)
	if err != nil {
		t.Fatalf("marshal current DTO: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("ack response bytes changed — the DTO's field order must match the\n"+
			"alphabetical order encoding/json gives the gin.H it replaced.\n"+
			"  was: %s\n  now: %s", want, got)
	}
}
