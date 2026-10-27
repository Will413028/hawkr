package usecase

import (
	"context"
	"math"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func boolPtr(b bool) *bool { return &b }

// TestParseSortOrder covers parseSortOrder's two distinct failure modes
// directly, since Create/Update only exercise it indirectly through the DB
// layer:
//   - ANY parse failure (empty string, non-numeric garbage, even a number too
//     big to fit in int64) must fall back to 0 and return no error — this is
//     the regression case: internal/grpc/handler.go:392 passes
//     req.SortOrder (a proto3 string field with no has-flag) straight into
//     dto.CreateStyleRequest with no validation, so an empty string is a
//     normal, reachable "client didn't set sort_order" input, not a garbage
//     input that should fail the request.
//   - A value that parses successfully but doesn't fit in int32 must error —
//     this is the actual overflow gap the original finding was about: before
//     this migration such a value reached Postgres's INTEGER column and the
//     DB's own range check rejected it; the int32(...) conversion done in Go
//     after the migration doesn't error, it silently wraps.
func TestParseSortOrder(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		want      int32
		wantErr   bool
		errSubstr string
	}{
		{name: "empty string defaults to 0", in: "", want: 0},
		{name: "non-numeric garbage defaults to 0", in: "not-a-number", want: 0},
		{name: "ordinary value parses through", in: "5", want: 5},
		{name: "zero parses through", in: "0", want: 0},
		{name: "negative value parses through", in: "-3", want: -3},
		{name: "max int32 parses through", in: "2147483647", want: math.MaxInt32},
		{name: "min int32 parses through", in: "-2147483648", want: math.MinInt32},
		{
			name: "value too big to fit int64 defaults to 0 (Atoi itself fails to parse)",
			in:   "99999999999999999999999999999999",
			want: 0,
		},
		{
			name:      "value beyond int32 range errors instead of wrapping",
			in:        "2147483648", // math.MaxInt32 + 1
			wantErr:   true,
			errSubstr: "sort order out of range",
		},
		{
			name:      "large negative value beyond int32 range errors",
			in:        "-2147483649", // math.MinInt32 - 1
			wantErr:   true,
			errSubstr: "sort order out of range",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseSortOrder(c.in)
			if c.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), c.errSubstr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, c.want, got)
		})
	}
}

// execRecorderDBTX is a minimal gen.DBTX stub (see
// internal/repository/postgres/gen/db.go) that records whether Exec was
// invoked. It exists so the tests below can prove Update() actually reached
// the DB layer via a positive assertion on the recorded call, instead of
// inferring that from a panic when calling a method on a nil pgxpool.Pool —
// the previous technique, which would also "pass" if something panicked
// earlier for an unrelated reason. Query/QueryRow are never exercised by the
// code paths under test here and are left as harmless stubs.
type execRecorderDBTX struct {
	execCalled bool
}

func (s *execRecorderDBTX) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	s.execCalled = true
	return pgconn.CommandTag{}, nil
}

func (s *execRecorderDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (s *execRecorderDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return nil
}

// TestStyle_Update_IsDefaultFalseOnly_IsNoOp guards the early-return at the top
// of Update(): a request whose only set field is IsDefault=false (every other
// field nil) must be treated identically to an all-nil request — a silent
// no-op, same as the pre-migration gorm version's `if len(updates) == 0 {
// return nil }`. There, updates["is_default"] was only ever populated when
// IsDefault was true; nil AND explicit false both left the map empty.
//
// uc is built directly with &StyleUseCase{q: gen.New(stub)} (same pattern as
// internal/usecase/quota_style_store_scope_test.go's &StyleUseCase{q:
// gen.New(pool)}), backed by execRecorderDBTX so a regression that lets this
// request fall through to uc.q.UpdateAiStyle is caught by a positive
// assertion (stub.execCalled) rather than a panic.
func TestStyle_Update_IsDefaultFalseOnly_IsNoOp(t *testing.T) {
	stub := &execRecorderDBTX{}
	uc := &StyleUseCase{q: gen.New(stub)}

	err := uc.Update(context.Background(), 1, dto.UpdateStyleRequest{IsDefault: boolPtr(false)})
	require.NoError(t, err)
	require.False(t, stub.execCalled, "IsDefault=false-only must short-circuit before touching the DB layer")
}

// TestStyle_Update_AllNil_IsNoOp is the baseline this behavior must match:
// an UpdateStyleRequest with every field nil (unchanged from before this
// task) must also short-circuit without touching uc.q.
func TestStyle_Update_AllNil_IsNoOp(t *testing.T) {
	stub := &execRecorderDBTX{}
	uc := &StyleUseCase{q: gen.New(stub)}

	err := uc.Update(context.Background(), 1, dto.UpdateStyleRequest{})
	require.NoError(t, err)
	require.False(t, stub.execCalled, "an all-nil request must short-circuit before touching the DB layer")
}

// spyTxManager records whether WithTx was invoked, and actually runs fn (no
// real BEGIN/COMMIT — just a passthrough) so the closure's DB calls execute
// for real. A spy that didn't call fn would make any assertion on Update()'s
// returned error vacuously true: `affected` would stay at its zero value no
// matter what the closure contains, so a real regression inside the closure
// could never fail this test.
//
// Since the setsIsDefaultTrue branch builds its *gen.Queries from uc.pool
// inside the transaction (gen.New(database.GetDBTX(txCtx, uc.pool)), matching
// TransformUseCase.q(ctx)), a lightweight DBTX stub can no longer stand in
// for it the way execRecorderDBTX does above — there is no seam to inject a
// fake DBTX once the pool is real. See the test below for how it gets a real
// (but unreachable) pool without needing Docker/testcontainers.
type spyTxManager struct {
	called bool
}

func (s *spyTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	s.called = true
	return fn(ctx)
}

// TestStyle_Update_IsDefaultTrue_DoesNotShortCircuit is the contrast case:
// IsDefault=true is the one value that must NOT be treated as "no fields set"
// — it has to fall through past the early-return and reach the transactional
// branch (uc.txManager.WithTx, wrapping ClearCategoryDefaultStyle,
// MarkAiStyleDefault, then UpdateAiStyle), and that branch must actually
// attempt a real write.
//
// uc.pool is a real (non-nil) *pgxpool.Pool pointed at a closed local port.
// pgxpool.New never dials eagerly (verified: constructing it here takes low
// single-digit milliseconds), so this stays a fast, hermetic unit test — but
// the spy's fn now runs for real, so the first Exec inside it does attempt an
// actual connection and fails immediately and deterministically with
// "connection refused". That failure is the proof the closure actually ran
// DB-touching code; a nil *pgxpool.Pool would panic here instead of
// returning an error (verified), which is why this can't just be
// `&StyleUseCase{txManager: spy}` with pool left at its zero value.
func TestStyle_Update_IsDefaultTrue_DoesNotShortCircuit(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://user:pass@127.0.0.1:1/db")
	require.NoError(t, err, "pgxpool.New must not dial eagerly")
	defer pool.Close()

	spy := &spyTxManager{}
	uc := &StyleUseCase{txManager: spy, pool: pool}

	err = uc.Update(context.Background(), 1, dto.UpdateStyleRequest{IsDefault: boolPtr(true)})
	require.Error(t, err, "the real write attempt must fail (connection refused on a closed local port)")
	require.NotEqual(t, "style not found", err.Error(), "this failure must come from actually attempting the write, not from a swallowed no-op")
	require.True(t, spy.called, "IsDefault=true must fall through past the early-return and reach uc.txManager.WithTx")
}
