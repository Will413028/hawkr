//go:build integration

package procurementrelay

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var epoch = time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)

type env struct {
	t      *testing.T
	d      *db.DB
	bus    *events.Bus
	module *procurement.Module
	pub    uuid.UUID
	buyer  int64
}

func newEnv(t *testing.T) *env {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(migrate.Module{Name: "procurement", Dir: procurement.Migrations()}))
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	events.Subscribe(bus, OnRfqPublished(queue))
	m := procurement.New(procurement.Deps{DB: d, Now: func() time.Time { return epoch }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Events: bus})
	e := &env{t: t, d: d, bus: bus, module: m, buyer: 1}
	e.seedPublication()
	return e
}

func (e *env) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.d.Pool().Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

// seedPublication 寫入一份發布所需的最小父列（需求、現行版本、候選集、核准、發布），邀請由各測試加。
func (e *env) seedPublication() {
	request, revision, set, approval := uuid.NewV7(), uuid.NewV7(), uuid.NewV7(), uuid.NewV7()
	e.pub = uuid.NewV7()
	e.exec(`INSERT INTO procurement.requests (id, buyer_id, status, created_at, updated_at) VALUES ($1, $2, 'MATCHING', $3, $3)`, request, e.buyer, epoch)
	e.exec(`INSERT INTO procurement.request_revisions (id, request_id, revision_no, item, quantity, unit, delivery_city, created_at) VALUES ($1, $2, 1, '小白菜', 3, 'KG', '台北市', $3)`, revision, request, epoch)
	e.exec(`INSERT INTO procurement.candidate_sets (id, request_id, request_revision_id, buyer_id, created_at) VALUES ($1, $2, $3, $4, $5)`, set, request, revision, e.buyer, epoch)
	e.exec(`INSERT INTO procurement.rfq_publication_approvals (id, buyer_id, request_id, request_revision_id, range_lower_amount, range_upper_amount, candidate_set_id, state, schema_version, client_request_id, generated_at, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 100, 200, $5, 'SUCCEEDED', 'v1', $6, $7, $7, $7, $7)`, approval, e.buyer, request, revision, set, uuid.NewV7(), epoch)
	e.exec(`INSERT INTO procurement.rfq_publications (id, buyer_id, request_id, request_revision_id, approval_id, approval_version, schema_version, client_request_id, published_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 1, 'v1', $6, $7, $7, $7)`, e.pub, e.buyer, request, revision, approval, uuid.NewV7(), epoch)
}

func (e *env) invite(position int, storeID int64) uuid.UUID {
	id := uuid.NewV7()
	e.exec(`INSERT INTO procurement.rfq_publication_invites (id, publication_id, position, store_id, delivery_state_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $5, $5)`, id, e.pub, position, storeID, epoch)
	return id
}

func (e *env) state(invite uuid.UUID) string {
	e.t.Helper()
	var s string
	if err := e.d.Pool().QueryRow(context.Background(), `SELECT delivery_state FROM procurement.rfq_publication_invites WHERE id = $1`, invite).Scan(&s); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *env) publish(ctx context.Context, invites ...procurement.RfqInvite) error {
	return events.Publish(ctx, e.bus, procurement.RfqPublished{PublicationID: e.pub, BuyerID: e.buyer, PublishedAt: epoch, Invites: invites})
}

type queuedJob struct {
	args        InviteCreatedArgs
	maxAttempts int
}

func (e *env) queued() []queuedJob {
	e.t.Helper()
	rows, err := e.d.Pool().Query(context.Background(), `SELECT args, max_attempts FROM river.river_job WHERE kind = $1 ORDER BY id`, InviteCreatedArgs{}.Kind())
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	var out []queuedJob
	for rows.Next() {
		var raw []byte
		var q queuedJob
		if err := rows.Scan(&raw, &q.maxAttempts); err != nil {
			e.t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &q.args); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

func TestPublishEnqueuesJobsThatSendTheEventAndMarkTheInvitesSent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a, b := e.invite(0, 11), e.invite(1, 22)
	if err := e.d.InTx(t.Context(), func(ctx context.Context) error {
		return e.publish(ctx, procurement.RfqInvite{InviteID: a, StoreID: 11}, procurement.RfqInvite{InviteID: b, StoreID: 22})
	}); err != nil {
		t.Fatal(err)
	}
	queued := e.queued()
	if len(queued) != 2 || queued[0].maxAttempts != maxAttempts {
		t.Fatalf("jobs = %+v，預期每個邀請一個、max_attempts %d", queued, maxAttempts)
	}
	out := &fakeSender{}
	w := NewWorker(e.module, out)
	for _, q := range queued {
		if err := w.inviteCreated(t.Context(), q.args, false); err != nil {
			t.Fatal(err)
		}
	}
	if len(out.sent) != 2 {
		t.Fatalf("sent = %+v", out.sent)
	}
	var first map[string]string
	if err := json.Unmarshal(out.sent[0].Body, &first); err != nil {
		t.Fatal(err)
	}
	if first["inviteId"] != a.String() || first["publicationId"] != e.pub.String() || first["storeId"] != "11" {
		t.Fatalf("first payload = %v", first)
	}
	if e.state(a) != "SENT" || e.state(b) != "SENT" {
		t.Fatalf("states = %s, %s", e.state(a), e.state(b))
	}
}

func TestExhaustedRetriesMarkTheInviteFailed(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a := e.invite(0, 11)
	args := InviteCreatedArgs{InviteID: a, PublicationID: e.pub, StoreID: 11}
	w := NewWorker(e.module, &fakeSender{err: errors.New("broker down")})
	if err := w.inviteCreated(t.Context(), args, false); err == nil {
		t.Fatal("送出失敗要回錯誤")
	}
	if got := e.state(a); got != "QUEUED" {
		t.Fatalf("未用盡時 state = %s，預期 QUEUED", got)
	}
	if err := w.inviteCreated(t.Context(), args, true); err == nil {
		t.Fatal("用盡仍要回錯誤")
	}
	if got := e.state(a); got != "FAILED" {
		t.Fatalf("用盡後 state = %s，預期 FAILED", got)
	}
}

func TestRetryAfterSentDoesNotRegressTheState(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a := e.invite(0, 11)
	args := InviteCreatedArgs{InviteID: a, PublicationID: e.pub, StoreID: 11}
	if err := NewWorker(e.module, &fakeSender{}).inviteCreated(t.Context(), args, false); err != nil {
		t.Fatal(err)
	}
	_ = NewWorker(e.module, &fakeSender{err: errors.New("broker down")}).inviteCreated(t.Context(), args, true)
	if got := e.state(a); got != "SENT" {
		t.Fatalf("state = %s，已 SENT 的邀請不得被改成 FAILED", got)
	}
}

func TestRolledBackPublishEnqueuesNothing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a := e.invite(0, 11)
	boom := errors.New("rollback")
	err := e.d.InTx(t.Context(), func(ctx context.Context) error {
		if err := e.publish(ctx, procurement.RfqInvite{InviteID: a, StoreID: 11}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if q := e.queued(); len(q) != 0 {
		t.Fatalf("回滾後排了 %d 個 job", len(q))
	}
}
