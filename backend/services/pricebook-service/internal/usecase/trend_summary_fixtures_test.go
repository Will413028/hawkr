// trend_summary_fixtures_test.go is Task 12's (issue #2249 Slice B) fixture
// regression guard: for each committed
// docs/api-fixtures/pricebook-trend-summary/*.json fixture — produced by
// literally running the real GetTrendSummary gRPC handler against a real
// Postgres testcontainer, marshaled with the SAME options
// gateway/internal/response.go's protoMarshaler actually uses in production
// (protojson.MarshalOptions{EmitUnpopulated: true} — the one-time generator
// that did this has since been deleted; see task-12-report.md) — confirm
// response.data still (a) unmarshals against the real
// pb.GetTrendSummaryResponse and (b) round-trips byte-for-byte-equivalent
// through that SAME gateway marshaler. This stays committed (unlike the
// generator) because it is cheap (no Docker, no DB, sub-second) and catches
// two distinct regressions: (a) a future proto field rename silently
// invalidating a fixture the frontend already built its adapter against,
// and (b) a fixture regenerated with the wrong marshal options (e.g. bare
// protojson.Marshal instead of EmitUnpopulated) silently dropping
// implicit-presence zero-value fields (0/""/[] on non-optional scalars and
// repeated fields) that gateway/internal/response.ProtoToRaw always emits —
// a plain protojson.Unmarshal-only round-trip CANNOT catch (b): Go's zero
// value for an absent field is indistinguishable from an explicitly-emitted
// zero value once unmarshaled, so the only way to catch a dropped
// EmitUnpopulated field is to re-marshal with the production options and
// diff against the committed bytes.
package usecase_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
)

// fixturesDir is repo-root-relative from this package
// (services/pricebook-service/internal/usecase) — mirrors this test group's
// "../../db/migrations" convention (two levels reaches
// services/pricebook-service; two more reach the repo root).
const fixturesDir = "../../../../docs/api-fixtures/pricebook-trend-summary"

// gatewayMarshaler mirrors gateway/internal/response.go's protoMarshaler
// byte for byte (protojson.MarshalOptions{EmitUnpopulated: true}, nothing
// else set — no UseProtoNames) — that is the actual marshaler
// response.OKProto/ProtoToRaw uses for every gateway response, including
// GetTrendSummary's.
var gatewayMarshaler = protojson.MarshalOptions{EmitUnpopulated: true}

func TestFixtures_ResponseDataParsesAgainstRealProto(t *testing.T) {
	names := []string{
		"ready-crop-week.json",
		"ready-crop-fresh.json",
		"partial-current-week.json",
		"market-closed-week.json",
		"insufficient-data.json",
		"missing-baseline-yoy364.json",
		"group-ready-two-markets.json",
	}
	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixturesDir, name))
			require.NoError(t, err, "fixture file must exist")

			var envelope struct {
				Request  string `json:"request"`
				Response struct {
					Code    int             `json:"code"`
					Message string          `json:"message"`
					Data    json.RawMessage `json:"data"`
				} `json:"response"`
			}
			require.NoError(t, json.Unmarshal(raw, &envelope), "envelope must be valid JSON")
			require.NotEmpty(t, envelope.Request)
			require.Equal(t, 0, envelope.Response.Code)
			require.Equal(t, "success", envelope.Response.Message)
			require.NotEmpty(t, envelope.Response.Data)

			var resp pb.GetTrendSummaryResponse
			err = protojson.Unmarshal(envelope.Response.Data, &resp)
			require.NoError(t, err, "response.data must unmarshal against pb.GetTrendSummaryResponse — a field-name typo would fail exactly here")

			// Re-marshal with the real production (gateway) options and
			// require the result to be the SAME JSON as what's committed.
			// If a fixture were ever regenerated with the wrong marshaler
			// (e.g. bare protojson.Marshal{}, which silently omits
			// implicit-presence zero values like sampleCount:0 or
			// comparisons:[]), this round-trip would ADD those fields back
			// in and fail the comparison — catching exactly the class of
			// bug a naive unmarshal-only check cannot see.
			remarshaled, err := gatewayMarshaler.Marshal(&resp)
			require.NoError(t, err)
			require.JSONEq(t, string(envelope.Response.Data), string(remarshaled),
				"response.data must be byte-for-byte equivalent to what gateway/internal/response.ProtoToRaw's real marshaler (EmitUnpopulated: true) produces — a mismatch means this fixture was generated with different marshal options than production actually uses")
		})
	}
}
