//go:build integration

package agent_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// prospects 是 seller 的 ProspectContact port 的替身：email 可以隨時改，沒登記的 prospect 回 NotFound。
type prospects struct {
	mu       sync.Mutex
	emails   map[int64]*string
	inactive map[int64]seller.ProspectStatus
}

func (p *prospects) ProspectContact(_ context.Context, id int64) (seller.ProspectContact, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	email, ok := p.emails[id]
	if !ok {
		return seller.ProspectContact{}, apperr.NotFound("業者 %d 不存在", id)
	}
	status := seller.ProspectStatusActive
	if st, ok := p.inactive[id]; ok {
		status = st
	}
	return seller.ProspectContact{ProspectID: id, DisplayName: "業者", Email: email, Status: status}, nil
}

// setStatus 改 prospect 的狀態（預設進行中）。
func (p *prospects) setStatus(id int64, st seller.ProspectStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inactive[id] = st
}

func (p *prospects) set(id int64, email string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if email == "-" {
		p.emails[id] = nil
		return
	}
	p.emails[id] = &email
}

type fixture struct {
	db        *db.DB
	module    *Module
	prospects *prospects
	now       time.Time
}

const (
	prospectA = int64(101)
	prospectB = int64(102)
)

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "agent", Dir: Migrations()}))
	f := &fixture{db: d, prospects: &prospects{emails: map[int64]*string{}, inactive: map[int64]seller.ProspectStatus{}}, now: time.Date(2026, 10, 5, 4, 0, 0, 0, time.UTC)}
	f.prospects.set(prospectA, "Vendor@Example.com")
	f.prospects.set(prospectB, "other@example.com")
	f.module = New(Deps{DB: d, Prospects: f.prospects, Now: func() time.Time { return f.now }})
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantKind(t *testing.T, err error, kind apperr.Kind) {
	t.Helper()
	if err == nil || apperr.KindOf(err) != kind {
		t.Fatalf("err = %v，預期分類 %v", err, kind)
	}
}

func (f *fixture) count(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

func (f *fixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := f.db.Conn(context.Background()).Exec(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) expiry(d time.Duration) time.Time { return f.now.Add(d) }

// invite 以 Ensure 建立一張邀請（30 天後到期）。
func (f *fixture) invite(t *testing.T, prospectID, recipientID int64) IssuedInvite {
	t.Helper()
	inv, err := f.module.EnsureInvite(context.Background(), EnsureInviteInput{ProspectID: prospectID, RfqRecipientID: recipientID, ExpiresAt: f.expiry(30 * 24 * time.Hour)})
	must(t, err)
	return inv
}

// session 建立邀請並兌換出一個 session。
func (f *fixture) session(t *testing.T, prospectID, recipientID int64) (IssuedInvite, IssuedSession) {
	t.Helper()
	inv := f.invite(t, prospectID, recipientID)
	s, err := f.module.ExchangeInvite(context.Background(), inv.Secret)
	must(t, err)
	return inv, s
}

func (f *fixture) archive(t *testing.T, prospectID int64) {
	t.Helper()
	must(t, f.db.InTx(context.Background(), func(ctx context.Context) error {
		return f.module.OnProspectArchived(ctx, seller.ProspectArchived{ProspectID: prospectID})
	}))
}

// 邀請建立時存的是 SHA-256 與去空白的 email 快照；明文 secret 是 32 bytes 的 base64url（無 padding）。
func TestEnsureInviteCreatesTheInviteAndStoresOnlyTheHash(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.prospects.set(prospectA, "  Vendor@Example.com  ")
	inv := f.invite(t, prospectA, 1)
	if inv.InviteID == (uuid.UUID{}) || inv.InvitedEmail != "Vendor@Example.com" {
		t.Fatalf("inv=%+v", inv)
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(inv.Secret)
	if err != nil || len(raw) != 32 || strings.ContainsAny(inv.Secret, "=+/") {
		t.Fatalf("secret=%q err=%v", inv.Secret, err)
	}
	sum := sha256.Sum256(raw)
	var hash []byte
	var email string
	var delivered *time.Time
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(),
		"SELECT secret_hash, invited_email, secret_delivered_at FROM agent.vendor_portal_invites WHERE invite_id = $1", inv.InviteID).Scan(&hash, &email, &delivered))
	if !slices.Equal(hash, sum[:]) || email != "Vendor@Example.com" || delivered != nil {
		t.Fatalf("hash=%x email=%q delivered=%v", hash, email, delivered)
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites WHERE secret_hash = $1", []byte(inv.Secret)); n != 0 {
		t.Fatal("明文 secret 不得落地")
	}
}

// 還沒標記寄出的邀請重取時換 secret 與到期時間、重讀 email（舊 secret 失效）；邀請 ID 不變。
func TestEnsureInviteRotatesUntilDelivered(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	first := f.invite(t, prospectA, 1)
	f.prospects.set(prospectA, "new@example.com")
	second, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: 1, ExpiresAt: f.expiry(10 * 24 * time.Hour)})
	must(t, err)
	if second.InviteID != first.InviteID || second.Secret == first.Secret || second.InvitedEmail != "new@example.com" {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	_, err = f.module.ExchangeInvite(ctx, first.Secret)
	wantKind(t, err, apperr.KindNotFound)
	if _, err := f.module.ExchangeInvite(ctx, second.Secret); err != nil {
		t.Fatal(err)
	}
	prev, err := f.module.PreviewInvite(ctx, second.Secret)
	must(t, err)
	if !prev.ExpiresAt.Equal(f.expiry(10 * 24 * time.Hour).Truncate(time.Microsecond)) {
		t.Fatalf("expires=%v，重取要換成新的到期時間", prev.ExpiresAt)
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites"); n != 1 {
		t.Fatalf("邀請數 = %d", n)
	}
}

// 已標記寄出之後 Ensure 拒絕（不再換 secret、不再寄第二封）；Mark 冪等，第二次不改時間；不存在的邀請是 NotFound。
func TestMarkInviteDeliveredLocksEnsureAndIsIdempotent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv := f.invite(t, prospectA, 1)
	must(t, f.module.MarkInviteDelivered(ctx, inv.InviteID, inv.Secret))
	var first time.Time
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT secret_delivered_at FROM agent.vendor_portal_invites").Scan(&first))
	f.now = f.now.Add(time.Hour)
	must(t, f.module.MarkInviteDelivered(ctx, inv.InviteID, inv.Secret))
	var second time.Time
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT secret_delivered_at FROM agent.vendor_portal_invites").Scan(&second))
	if !first.Equal(second) {
		t.Fatalf("第二次 Mark 改了時間：%v -> %v", first, second)
	}
	_, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: 1, ExpiresAt: f.expiry(time.Hour)})
	if !errors.Is(err, ErrInviteAlreadyDelivered) {
		t.Fatalf("err = %v", err)
	}
	// 已寄出的邀請仍可兌換，secret 沒被換掉。
	if _, err := f.module.ExchangeInvite(ctx, inv.Secret); err != nil {
		t.Fatal(err)
	}
	wantKind(t, f.module.MarkInviteDelivered(ctx, uuid.NewV7(), inv.Secret), apperr.KindNotFound)
}

// 已撤銷的邀請 Ensure 回 ErrInviteRevoked（不是「已寄出」：舊實作把撤銷說成已寄出，邀請無聲地不寄）；撤銷與已寄出並存時也是撤銷。
func TestEnsureInviteOnRevokedInviteIsDistinctFromDelivered(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.invite(t, prospectA, 1)
	f.archive(t, prospectA)
	_, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: 1, ExpiresAt: f.expiry(time.Hour)})
	if !errors.Is(err, ErrInviteRevoked) || errors.Is(err, ErrInviteAlreadyDelivered) {
		t.Fatalf("err = %v", err)
	}
	inv := f.invite(t, prospectB, 2)
	must(t, f.module.MarkInviteDelivered(ctx, inv.InviteID, inv.Secret))
	f.archive(t, prospectB)
	_, err = f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectB, RfqRecipientID: 2, ExpiresAt: f.expiry(time.Hour)})
	if !errors.Is(err, ErrInviteRevoked) {
		t.Fatalf("err = %v", err)
	}
}

func TestEnsureInviteRequiresADeliverableEmail(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	in := func(prospect, recipient int64) EnsureInviteInput {
		return EnsureInviteInput{ProspectID: prospect, RfqRecipientID: recipient, ExpiresAt: f.expiry(time.Hour)}
	}
	f.prospects.set(prospectA, "-")
	f.prospects.set(prospectB, "待確認")
	for _, p := range []int64{prospectA, prospectB} {
		_, err := f.module.EnsureInvite(ctx, in(p, p))
		if !errors.Is(err, ErrNoDeliverableEmail) {
			t.Fatalf("prospect %d: err = %v", p, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites"); n != 0 {
		t.Fatalf("沒有可寄的 email 不得留下邀請，n=%d", n)
	}
	// 已建立之後 email 變成收不到信：重取回 ErrNoDeliverableEmail，邀請原樣。
	f.prospects.set(prospectA, "ok@example.com")
	inv := f.invite(t, prospectA, 5)
	f.prospects.set(prospectA, "N/A")
	_, err := f.module.EnsureInvite(ctx, in(prospectA, 5))
	if !errors.Is(err, ErrNoDeliverableEmail) {
		t.Fatalf("err = %v", err)
	}
	if _, err := f.module.ExchangeInvite(ctx, inv.Secret); err != nil {
		t.Fatalf("失敗的重取不得換掉原本的 secret：%v", err)
	}
	// prospect 不存在：seller 的 NotFound 原樣回來。
	_, err = f.module.EnsureInvite(ctx, in(999, 9))
	wantKind(t, err, apperr.KindNotFound)
}

// expires_at 必須在未來、不超過 180 天（舊實作什麼都收：過去的時間產生立刻失效卻照寄的邀請，過遠的時間產生長效 bearer）。
func TestEnsureInviteValidatesExpiry(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for name, at := range map[string]time.Time{
		"過去": f.now.Add(-time.Hour), "現在": f.now, "超過上限": f.now.Add(180*24*time.Hour + time.Second), "零值": {},
	} {
		_, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: 1, ExpiresAt: at})
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s：err = %v", name, err)
		}
	}
	for _, bad := range []EnsureInviteInput{{ProspectID: 0, RfqRecipientID: 1}, {ProspectID: 1, RfqRecipientID: -1}} {
		bad.ExpiresAt = f.expiry(time.Hour)
		_, err := f.module.EnsureInvite(ctx, bad)
		wantKind(t, err, apperr.KindInvalidInput)
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites"); n != 0 {
		t.Fatalf("n=%d", n)
	}
	_, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: 1, ExpiresAt: f.expiry(180 * 24 * time.Hour)})
	must(t, err)
}

// 併發的 Ensure 對同一個 RFQ 收件人不會撞出 unique violation（舊實作：兩個都走「不存在」，第二個 INSERT 失敗成 Internal）：
// 全部成功、只有一列，最後一次換的 secret 是有效的那一個。
func TestEnsureInviteConcurrentCallsNeverSurfaceAUniqueViolation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	const n = 16
	var wg sync.WaitGroup
	secrets := make([]string, n)
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inv, err := f.module.EnsureInvite(context.Background(), EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: 7, ExpiresAt: f.expiry(time.Hour)})
			secrets[i], errs[i] = inv.Secret, err
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if c := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites"); c != 1 {
		t.Fatalf("邀請數 = %d", c)
	}
	valid := 0
	for _, s := range secrets {
		if _, err := f.module.ExchangeInvite(context.Background(), s); err == nil {
			valid++
		}
	}
	if valid != 1 {
		t.Fatalf("有效的 secret 有 %d 個，預期只有最後一次換的那一個", valid)
	}
}

// AG1：Ensure 只替進行中的 prospect 建立或換 secret。封存、被合併、已轉成店家一律 ErrProspectNotActive，且不留下邀請、不換
// 掉原本的 secret；撤銷的邀請仍先回 ErrInviteRevoked。
func TestEnsureInviteRequiresAnActiveProspect(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	in := func(prospect, recipient int64) EnsureInviteInput {
		return EnsureInviteInput{ProspectID: prospect, RfqRecipientID: recipient, ExpiresAt: f.expiry(time.Hour)}
	}
	for _, st := range []seller.ProspectStatus{seller.ProspectStatusArchived, seller.ProspectStatusConverted} {
		f.prospects.setStatus(prospectA, st)
		_, err := f.module.EnsureInvite(ctx, in(prospectA, 1))
		if !errors.Is(err, ErrProspectNotActive) || errors.Is(err, ErrInviteRevoked) || errors.Is(err, ErrNoDeliverableEmail) {
			t.Fatalf("%s：err = %v", st, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites"); n != 0 {
		t.Fatalf("非進行中的 prospect 不得留下邀請，n=%d", n)
	}
	// 已存在、還沒寄出的邀請：prospect 離開進行中之後不再換 secret，原本的 secret 照舊（要不要撤銷是封存事件的事）。
	inv := f.invite(t, prospectB, 2)
	f.prospects.setStatus(prospectB, seller.ProspectStatusConverted)
	_, err := f.module.EnsureInvite(ctx, in(prospectB, 2))
	if !errors.Is(err, ErrProspectNotActive) {
		t.Fatalf("err = %v", err)
	}
	if _, err := f.module.ExchangeInvite(ctx, inv.Secret); err != nil {
		t.Fatalf("失敗的重取不得換掉原本的 secret：%v", err)
	}
	// 撤銷優先：封存的 prospect 的已撤銷邀請回 ErrInviteRevoked（既有行為），不是 NotActive。
	f.prospects.setStatus(prospectB, seller.ProspectStatusActive)
	f.archive(t, prospectB)
	f.prospects.setStatus(prospectB, seller.ProspectStatusArchived)
	_, err = f.module.EnsureInvite(ctx, in(prospectB, 2))
	if !errors.Is(err, ErrInviteRevoked) {
		t.Fatalf("err = %v", err)
	}
}

// AG2：既有邀請掛的 prospect 與輸入不同時拒絕（InvalidInput），邀請原樣（不換 secret、不改 email 快照、不改 prospect）。
func TestEnsureInviteRejectsAnotherProspectsRecipient(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv := f.invite(t, prospectA, 1)
	_, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectB, RfqRecipientID: 1, ExpiresAt: f.expiry(time.Hour)})
	wantKind(t, err, apperr.KindInvalidInput)
	if _, err := f.module.ExchangeInvite(ctx, inv.Secret); err != nil {
		t.Fatalf("被拒絕的重取不得換掉 secret：%v", err)
	}
	var email string
	var prospect int64
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT prospect_id, invited_email FROM agent.vendor_portal_invites WHERE rfq_recipient_id = 1").Scan(&prospect, &email))
	if prospect != prospectA || email != "Vendor@Example.com" {
		t.Fatalf("prospect=%d email=%q", prospect, email)
	}
	// 已寄出的邀請也先判斷歸屬，不把別人的呼叫說成「已寄出」。
	must(t, f.module.MarkInviteDelivered(ctx, inv.InviteID, inv.Secret))
	_, err = f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectB, RfqRecipientID: 1, ExpiresAt: f.expiry(time.Hour)})
	wantKind(t, err, apperr.KindInvalidInput)
}

// AG4：Mark 帶寄出的那一把 secret，只有它仍是目前的 secret 才標記。Ensure 在寄信途中換了 secret：過時的 Mark 不生效
// （ErrInviteSecretSuperseded），邀請維持沒寄出，之後 Ensure 還能換；用目前的 secret Mark 才算寄出。
func TestMarkInviteDeliveredIgnoresASupersededSecret(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	first := f.invite(t, prospectA, 1)
	second, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: 1, ExpiresAt: f.expiry(time.Hour)})
	must(t, err)
	err = f.module.MarkInviteDelivered(ctx, first.InviteID, first.Secret)
	if !errors.Is(err, ErrInviteSecretSuperseded) {
		t.Fatalf("err = %v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites WHERE secret_delivered_at IS NOT NULL"); n != 0 {
		t.Fatal("過時的 Mark 不得標記寄出")
	}
	if _, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: 1, ExpiresAt: f.expiry(time.Hour)}); err != nil {
		t.Fatalf("沒寄出的邀請還要能換：%v", err)
	}
	third, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: 1, ExpiresAt: f.expiry(time.Hour)})
	must(t, err)
	if errors.Is(f.module.MarkInviteDelivered(ctx, second.InviteID, second.Secret), ErrInviteSecretSuperseded) == false {
		t.Fatal("第二把也已過時")
	}
	must(t, f.module.MarkInviteDelivered(ctx, third.InviteID, third.Secret))
	must(t, f.module.MarkInviteDelivered(ctx, third.InviteID, third.Secret)) // 同一把重送：冪等
	// 已寄出之後，別把舊的 secret 當成成功。
	if err := f.module.MarkInviteDelivered(ctx, third.InviteID, first.Secret); !errors.Is(err, ErrInviteSecretSuperseded) {
		t.Fatalf("err = %v", err)
	}
	// 格式不對是 InvalidInput；不存在的邀請仍是 NotFound。
	wantKind(t, f.module.MarkInviteDelivered(ctx, third.InviteID, "x"), apperr.KindInvalidInput)
	wantKind(t, f.module.MarkInviteDelivered(ctx, uuid.NewV7(), third.Secret), apperr.KindNotFound)
}

// AG4 的競爭：Mark(s1) 與 Ensure（換 secret）同時發生，結果只有兩種，而且永遠一致：Mark 先到，邀請寄出、Ensure 之後拿到
// ErrInviteAlreadyDelivered、s1 有效；Ensure 先到，Mark 回 Superseded、s2 有效、s1 失效。不會出現「標記寄出但寄出的連結已失效」。
func TestMarkInviteDeliveredRacesWithRotation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for round := range int64(30) {
		recipient := 100 + round
		s1 := f.invite(t, prospectA, recipient)
		var markErr, ensureErr error
		var s2 IssuedInvite
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); markErr = f.module.MarkInviteDelivered(ctx, s1.InviteID, s1.Secret) }()
		go func() {
			defer wg.Done()
			s2, ensureErr = f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: recipient, ExpiresAt: f.expiry(time.Hour)})
		}()
		wg.Wait()
		_, s1Err := f.module.PreviewInvite(ctx, s1.Secret)
		switch {
		case markErr == nil:
			if !errors.Is(ensureErr, ErrInviteAlreadyDelivered) || s1Err != nil {
				t.Fatalf("round %d：Mark 先到卻 ensure=%v s1=%v", round, ensureErr, s1Err)
			}
		case errors.Is(markErr, ErrInviteSecretSuperseded):
			if ensureErr != nil || s1Err == nil {
				t.Fatalf("round %d：Ensure 先到卻 ensure=%v s1=%v", round, ensureErr, s1Err)
			}
			if _, err := f.module.PreviewInvite(ctx, s2.Secret); err != nil {
				t.Fatalf("round %d：s2 要有效：%v", round, err)
			}
			if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites WHERE rfq_recipient_id = $1 AND secret_delivered_at IS NOT NULL", recipient); n != 0 {
				t.Fatalf("round %d：過時的 Mark 標記了寄出", round)
			}
		default:
			t.Fatalf("round %d：mark=%v", round, markErr)
		}
	}
}

// 邀請到期或撤銷之前可以重複兌換，每次都發新的 session（舊實作單次兌換：換裝置、換瀏覽器就進不來）。
func TestExchangeIsReusableAndEverySessionIsNew(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv := f.invite(t, prospectA, 11)
	a, err := f.module.ExchangeInvite(ctx, inv.Secret)
	must(t, err)
	f.now = f.now.Add(time.Hour)
	b, err := f.module.ExchangeInvite(ctx, inv.Secret)
	must(t, err)
	if a.Token == b.Token || a.InviteID != inv.InviteID || b.RfqRecipientID != 11 {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
	for _, tok := range []string{a.Token, b.Token} {
		s, err := f.module.ValidateSession(ctx, tok)
		must(t, err)
		if s.InviteID != inv.InviteID || s.RfqRecipientID != 11 || s.Verification != nil {
			t.Fatalf("s=%+v", s)
		}
	}
	if !a.IdleExpiresAt.Equal(a.IdleExpiresAt.Truncate(time.Microsecond)) || a.IdleExpiresAt.Sub(f.now.Add(-time.Hour)) != 24*time.Hour ||
		a.AbsoluteExpiresAt.Sub(f.now.Add(-time.Hour)) != 7*24*time.Hour {
		t.Fatalf("idle=%v absolute=%v", a.IdleExpiresAt, a.AbsoluteExpiresAt)
	}
}

// 兌換不可用的邀請一律是同一個 NotFound，猜 secret 的人分不出成因（未知、格式錯、過期、撤銷）。
func TestExchangeAndPreviewShareOneOpaqueNotFound(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	expired := f.invite(t, prospectA, 1)
	f.now = f.now.Add(31 * 24 * time.Hour) // 過了 30 天的到期
	revoked := func() IssuedInvite {
		inv, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectB, RfqRecipientID: 2, ExpiresAt: f.expiry(time.Hour)})
		must(t, err)
		f.archive(t, prospectB)
		return inv
	}()
	unknown := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	var msgs []string
	for name, secret := range map[string]string{"過期": expired.Secret, "撤銷": revoked.Secret, "未知": unknown, "格式錯": "not base64!", "空": "", "太短": "AAAA"} {
		_, err := f.module.ExchangeInvite(ctx, secret)
		if apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("Exchange %s：err = %v", name, err)
		}
		msgs = append(msgs, err.Error())
		_, err = f.module.PreviewInvite(ctx, secret)
		if apperr.KindOf(err) != apperr.KindNotFound {
			t.Fatalf("Preview %s：err = %v", name, err)
		}
		msgs = append(msgs, err.Error())
	}
	for _, m := range msgs {
		if m != msgs[0] {
			t.Fatalf("錯誤訊息不一致：%q vs %q", m, msgs[0])
		}
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_sessions"); n != 0 {
		t.Fatalf("不可用的邀請不得產生 session，n=%d", n)
	}
}

// session 不會比邀請活得久：絕對期限被邀請的到期截斷，idle 不超過絕對期限。
func TestSessionNeverOutlivesTheInvite(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: 1, ExpiresAt: f.expiry(10 * time.Hour)})
	must(t, err)
	s, err := f.module.ExchangeInvite(ctx, inv.Secret)
	must(t, err)
	want := f.expiry(10 * time.Hour)
	if !s.AbsoluteExpiresAt.Equal(want) || !s.IdleExpiresAt.Equal(want) {
		t.Fatalf("idle=%v absolute=%v，預期都被截斷到 %v", s.IdleExpiresAt, s.AbsoluteExpiresAt, want)
	}
	f.now = want.Add(time.Second)
	_, err = f.module.ValidateSession(ctx, s.Token)
	wantKind(t, err, apperr.KindUnauthenticated)
}

// 同一張邀請有效的 session 最多 5 個，超過時撤銷最舊的；別張邀請（同一個業者的另一張 RFQ）的 session 不受影響
// （舊實作按 identity 撤銷：點第二張邀請會踢掉第一張已驗證的 session）。
func TestExchangeCapsSessionsPerInviteAndLeavesOtherInvitesAlone(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	other, otherSession := f.session(t, prospectA, 2)
	_, err := f.module.VerifySession(ctx, otherSession.Token, VerificationGoogle, "vendor@example.com")
	must(t, err)
	inv := f.invite(t, prospectA, 1)
	var tokens []string
	for range 7 {
		f.now = f.now.Add(time.Minute)
		s, err := f.module.ExchangeInvite(ctx, inv.Secret)
		must(t, err)
		tokens = append(tokens, s.Token)
	}
	for i, tok := range tokens {
		_, err := f.module.ValidateSession(ctx, tok)
		if i < 2 {
			wantKind(t, err, apperr.KindUnauthenticated)
		} else if err != nil {
			t.Fatalf("session %d 應該有效：%v", i, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_sessions WHERE invite_id = $1 AND revoked_at IS NULL", inv.InviteID); n != 5 {
		t.Fatalf("有效 session = %d", n)
	}
	s, err := f.module.ValidateSession(ctx, otherSession.Token)
	must(t, err)
	if s.InviteID != other.InviteID || s.Verification == nil {
		t.Fatalf("另一張邀請的 session 被影響：%+v", s)
	}
}

// 上限滿了、全部都已驗證：新發的 session 不在撤銷之列，撤銷最舊的已驗證 session。
func TestExchangeOverCapKeepsTheNewSessionAndRevokesTheOldestVerifiedWhenAllAreVerified(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv := f.invite(t, prospectA, 1)
	var tokens []string
	for range 5 {
		f.now = f.now.Add(time.Minute)
		s, err := f.module.ExchangeInvite(ctx, inv.Secret)
		must(t, err)
		_, err = f.module.VerifySession(ctx, s.Token, VerificationGoogle, "vendor@example.com")
		must(t, err)
		tokens = append(tokens, s.Token)
	}
	f.now = f.now.Add(time.Minute)
	fresh, err := f.module.ExchangeInvite(ctx, inv.Secret)
	must(t, err)
	info, err := f.module.ValidateSession(ctx, fresh.Token)
	must(t, err)
	if info.Verification != nil {
		t.Fatalf("新發的 session 不該已驗證：%+v", info)
	}
	_, err = f.module.ValidateSession(ctx, tokens[0])
	wantKind(t, err, apperr.KindUnauthenticated)
	for i, tok := range tokens[1:] {
		got, err := f.module.ValidateSession(ctx, tok)
		if err != nil || got.Verification == nil {
			t.Fatalf("已驗證的 session %d 應該保留：%+v %v", i+1, got, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_sessions WHERE invite_id = $1 AND revoked_at IS NULL", inv.InviteID); n != 5 {
		t.Fatalf("有效 session = %d", n)
	}
}

// 上限滿了、有未驗證的 session：先撤銷最舊的未驗證，已驗證的（即使更舊）保留。
func TestExchangeOverCapRevokesTheOldestUnverifiedFirst(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv := f.invite(t, prospectA, 1)
	var tokens []string
	for range 5 {
		f.now = f.now.Add(time.Minute)
		s, err := f.module.ExchangeInvite(ctx, inv.Secret)
		must(t, err)
		tokens = append(tokens, s.Token)
	}
	for _, i := range []int{0, 2} {
		_, err := f.module.VerifySession(ctx, tokens[i], VerificationGoogle, "vendor@example.com")
		must(t, err)
	}
	f.now = f.now.Add(time.Minute)
	fresh, err := f.module.ExchangeInvite(ctx, inv.Secret)
	must(t, err)
	for i, wantAlive := range []bool{true, false, true, true, true} {
		_, err := f.module.ValidateSession(ctx, tokens[i])
		if wantAlive && err != nil {
			t.Fatalf("session %d 應該有效：%v", i, err)
		}
		if !wantAlive {
			wantKind(t, err, apperr.KindUnauthenticated)
		}
	}
	_, err = f.module.ValidateSession(ctx, fresh.Token)
	must(t, err)
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_sessions WHERE invite_id = $1 AND revoked_at IS NULL", inv.InviteID); n != 5 {
		t.Fatalf("有效 session = %d", n)
	}
}

// 併發兌換同一張邀請：session 上限仍成立（邀請列在 transaction 內加鎖，計數不會交錯）。
func TestConcurrentExchangeKeepsTheCap(t *testing.T) {
	t.Parallel()
	f := setup(t)
	inv := f.invite(t, prospectA, 1)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.module.ExchangeInvite(context.Background(), inv.Secret); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_sessions WHERE revoked_at IS NULL"); n != 5 {
		t.Fatalf("有效 session = %d", n)
	}
}

func TestPreviewMasksTheEmailAndHasNoSideEffects(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv := f.invite(t, prospectA, 1)
	for range 3 {
		p, err := f.module.PreviewInvite(ctx, inv.Secret)
		must(t, err)
		if p.MaskedEmail != "V***@Example.com" || !p.ExpiresAt.Equal(f.expiry(30*24*time.Hour)) {
			t.Fatalf("preview=%+v", p)
		}
		if p.InviteID != inv.InviteID || p.RfqRecipientID != 1 {
			t.Fatalf("preview 的 ID：%+v，要 invite=%v rfq=1", p, inv.InviteID)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_sessions"); n != 0 {
		t.Fatalf("preview 不得建立 session，n=%d", n)
	}
	if _, err := f.module.ExchangeInvite(ctx, inv.Secret); err != nil {
		t.Fatalf("preview 不得消費邀請：%v", err)
	}
}

// idle 期限滑動，但距上次看到不到 5 分鐘不寫入（節流）；滑動不超過絕對期限；寫入在同一個 statement 內，不吞錯誤。
func TestValidateSessionSlidesIdleWithAThrottle(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.session(t, prospectA, 1)
	row := func() (idle, seen time.Time) {
		must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT idle_expires_at, last_seen_at FROM agent.vendor_portal_sessions").Scan(&idle, &seen))
		return
	}
	start := f.now
	f.now = start.Add(4 * time.Minute)
	_, err := f.module.ValidateSession(ctx, s.Token)
	must(t, err)
	if idle, seen := row(); !seen.Equal(start) || !idle.Equal(start.Add(24*time.Hour)) {
		t.Fatalf("節流內不得寫入：idle=%v seen=%v", idle, seen)
	}
	f.now = start.Add(5 * time.Minute)
	_, err = f.module.ValidateSession(ctx, s.Token)
	must(t, err)
	if idle, seen := row(); !seen.Equal(f.now) || !idle.Equal(f.now.Add(24*time.Hour)) {
		t.Fatalf("超過節流要滑動：idle=%v seen=%v", idle, seen)
	}
	// 每天使用就一直有效，直到絕對期限。
	for day := 1; day <= 6; day++ {
		f.now = start.Add(time.Duration(day) * 23 * time.Hour)
		if _, err := f.module.ValidateSession(ctx, s.Token); err != nil {
			t.Fatalf("day %d: %v", day, err)
		}
	}
	f.now = start.Add(7*24*time.Hour + time.Second)
	_, err = f.module.ValidateSession(ctx, s.Token)
	wantKind(t, err, apperr.KindUnauthenticated)
	if idle, _ := row(); idle.After(start.Add(7 * 24 * time.Hour)) {
		t.Fatalf("idle %v 超過絕對期限", idle)
	}
}

func TestValidateSessionRejectsEveryUnusableStateWithOneError(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, idle := f.session(t, prospectA, 1)
	_, revokedSession := f.session(t, prospectB, 2)
	f.exec(t, "UPDATE agent.vendor_portal_sessions SET revoked_at = $1 WHERE token_hash = $2", f.now, sha(t, revokedSession.Token))
	f.prospects.set(103, "x@example.com")
	_, s3 := f.session(t, 103, 3)
	f.exec(t, "UPDATE agent.vendor_portal_invites SET revoked_at = $1, revoke_reason = 'PROSPECT_ARCHIVED' WHERE rfq_recipient_id = 3", f.now)
	var msgs []string
	for name, tok := range map[string]string{"邀請撤銷": s3.Token, "session 撤銷": revokedSession.Token, "未知": base64.RawURLEncoding.EncodeToString(make([]byte, 32)), "格式錯": "???", "空": ""} {
		_, err := f.module.ValidateSession(ctx, tok)
		if apperr.KindOf(err) != apperr.KindUnauthenticated {
			t.Fatalf("%s：err = %v", name, err)
		}
		msgs = append(msgs, err.Error())
	}
	f.now = f.now.Add(25 * time.Hour) // idle 24 小時過了
	_, err := f.module.ValidateSession(ctx, idle.Token)
	wantKind(t, err, apperr.KindUnauthenticated)
	msgs = append(msgs, err.Error())
	for _, m := range msgs {
		if m != msgs[0] {
			t.Fatalf("錯誤訊息不一致：%q vs %q", m, msgs[0])
		}
	}
}

func TestValidateSessionFailsWhenTheInviteExpires(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv, err := f.module.EnsureInvite(ctx, EnsureInviteInput{ProspectID: prospectA, RfqRecipientID: 1, ExpiresAt: f.expiry(time.Hour)})
	must(t, err)
	s, err := f.module.ExchangeInvite(ctx, inv.Secret)
	must(t, err)
	f.now = f.now.Add(2 * time.Hour)
	_, err = f.module.ValidateSession(ctx, s.Token)
	wantKind(t, err, apperr.KindUnauthenticated)
	// 到期的邀請也不能再兌換。
	_, err = f.module.ExchangeInvite(ctx, inv.Secret)
	wantKind(t, err, apperr.KindNotFound)
}

func sha(t *testing.T, secret string) []byte {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(secret)
	must(t, err)
	sum := sha256.Sum256(raw)
	return sum[:]
}

// 驗證的 email 與邀請的快照去前後空白、不分大小寫相等才成立（比對在 SQL 的 CAS 內）；不符的 session 維持未驗證。
func TestVerifySessionComparesTheNormalizedEmail(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.session(t, prospectA, 1) // 快照是 Vendor@Example.com
	for _, wrong := range []string{"other@example.com", "vendor@example.com.evil.com", "", "   ", "xvendor@example.com"} {
		_, err := f.module.VerifySession(ctx, s.Token, VerificationGoogle, wrong)
		wantKind(t, err, apperr.KindPermissionDenied)
	}
	if info, err := f.module.ValidateSession(ctx, s.Token); err != nil || info.Verification != nil {
		t.Fatalf("info=%+v err=%v，不符的驗證不得留下紀錄", info, err)
	}
	f.now = f.now.Add(time.Minute)
	info, err := f.module.VerifySession(ctx, s.Token, VerificationGoogle, "  vENDOR@example.COM ")
	must(t, err)
	if v := info.Verification; v == nil || v.Method != VerificationGoogle || v.Email != "vENDOR@example.COM" || !v.At.Equal(f.now) {
		t.Fatalf("verification=%+v", v)
	}
	again, err := f.module.ValidateSession(ctx, s.Token)
	must(t, err)
	if again.Verification == nil || again.Verification.Method != VerificationGoogle {
		t.Fatalf("verification=%+v", again.Verification)
	}
}

// 已經以相同 email 驗證過的 session 重送是冪等的成功（時間不變）；驗證只屬於這個 session：新的兌換從未驗證開始。
func TestVerifySessionIsIdempotentAndPerSession(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv, s := f.session(t, prospectA, 1)
	first, err := f.module.VerifySession(ctx, s.Token, VerificationGoogle, "vendor@example.com")
	must(t, err)
	f.now = f.now.Add(time.Hour)
	second, err := f.module.VerifySession(ctx, s.Token, VerificationGoogle, " VENDOR@example.com")
	must(t, err)
	if !first.Verification.At.Equal(second.Verification.At) || first.Verification.Email != second.Verification.Email {
		t.Fatalf("重送改了驗證紀錄：%+v -> %+v", first.Verification, second.Verification)
	}
	next, err := f.module.ExchangeInvite(ctx, inv.Secret)
	must(t, err)
	info, err := f.module.ValidateSession(ctx, next.Token)
	must(t, err)
	if info.Verification != nil {
		t.Fatal("新的 session 不得繼承驗證")
	}
	if _, err := f.module.ValidateSession(ctx, s.Token); err != nil {
		t.Fatalf("新的兌換不得讓已驗證的 session 失效：%v", err)
	}
}

// 舊實作的缺口（StepUp 的 SQL 沒有 repository 層的測試）：過期、撤銷的 session 不能被驗證，邀請撤銷或到期後也不能；
// 不認得的驗證方式是 InvalidInput。
func TestVerifySessionRefusesUnusableSessions(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	email := "vendor@example.com"
	_, expired := f.session(t, prospectA, 1)
	f.exec(t, "UPDATE agent.vendor_portal_sessions SET idle_expires_at = $1 WHERE token_hash = $2", f.now.Add(-time.Second), sha(t, expired.Token))
	_, revoked := f.session(t, prospectA, 2)
	f.exec(t, "UPDATE agent.vendor_portal_sessions SET revoked_at = $1 WHERE token_hash = $2", f.now, sha(t, revoked.Token))
	_, ofRevokedInvite := f.session(t, prospectA, 3)
	f.exec(t, "UPDATE agent.vendor_portal_invites SET revoked_at = $1, revoke_reason = 'PROSPECT_ARCHIVED' WHERE rfq_recipient_id = 3", f.now)
	for name, tok := range map[string]string{"idle 過期": expired.Token, "session 撤銷": revoked.Token, "邀請撤銷": ofRevokedInvite.Token, "未知": base64.RawURLEncoding.EncodeToString(make([]byte, 32)), "格式錯": "x"} {
		_, err := f.module.VerifySession(ctx, tok, VerificationGoogle, email)
		if apperr.KindOf(err) != apperr.KindPermissionDenied {
			t.Fatalf("%s：err = %v", name, err)
		}
	}
	_, live := f.session(t, prospectA, 4)
	_, err := f.module.VerifySession(ctx, live.Token, VerificationMethod("FACEBOOK"), email)
	wantKind(t, err, apperr.KindInvalidInput)
	// 已經用 email 驗證的 session，換一個 email 重送不能覆蓋。
	_, err = f.module.VerifySession(ctx, live.Token, VerificationGoogle, email)
	must(t, err)
	_, err = f.module.VerifySession(ctx, live.Token, VerificationGoogle, "other@example.com")
	wantKind(t, err, apperr.KindPermissionDenied)
}

func TestVerifySessionConcurrentCallsAllSucceedWithOneRecord(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.session(t, prospectA, 1)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.module.VerifySession(context.Background(), s.Token, VerificationGoogle, "vendor@example.com"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_sessions WHERE verified_at IS NOT NULL"); n != 1 {
		t.Fatalf("n=%d", n)
	}
}

// 驗證時的快照是寄出時的 email：prospect 之後改了 email，已寄出的邀請仍比對寄到的那一個（邀請寄到哪就驗哪）。
func TestVerificationUsesTheSnapshotNotTheCurrentProspectEmail(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv := f.invite(t, prospectA, 1)
	must(t, f.module.MarkInviteDelivered(ctx, inv.InviteID, inv.Secret))
	f.prospects.set(prospectA, "changed@example.com")
	s, err := f.module.ExchangeInvite(ctx, inv.Secret)
	must(t, err)
	_, err = f.module.VerifySession(ctx, s.Token, VerificationGoogle, "changed@example.com")
	wantKind(t, err, apperr.KindPermissionDenied)
	if _, err := f.module.VerifySession(ctx, s.Token, VerificationGoogle, "vendor@example.com"); err != nil {
		t.Fatal(err)
	}
}

// prospect 封存：該 prospect 的全部邀請（reason 記下）與底下的 session 在同一個 transaction 內失效；別的 prospect 不受影響；
// 重送是冪等的（不改原本的撤銷時間與原因）。舊實作沒有任何寫入者撤銷邀請。
func TestProspectArchivedRevokesItsInvitesAndSessions(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv1, s1 := f.session(t, prospectA, 1)
	_, s2 := f.session(t, prospectA, 2)
	_, keep := f.session(t, prospectB, 3)
	f.archive(t, prospectA)
	for _, tok := range []string{s1.Token, s2.Token} {
		_, err := f.module.ValidateSession(ctx, tok)
		wantKind(t, err, apperr.KindUnauthenticated)
	}
	_, err := f.module.ExchangeInvite(ctx, inv1.Secret)
	wantKind(t, err, apperr.KindNotFound)
	if _, err := f.module.ValidateSession(ctx, keep.Token); err != nil {
		t.Fatalf("別的 prospect 的 session 不得受影響：%v", err)
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites WHERE prospect_id = $1 AND revoke_reason = 'PROSPECT_ARCHIVED' AND revoked_at = $2", prospectA, f.now); n != 2 {
		t.Fatalf("撤銷的邀請 = %d", n)
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_sessions s JOIN agent.vendor_portal_invites i USING (invite_id) WHERE i.prospect_id = $1 AND s.revoked_at IS NULL", prospectA); n != 0 {
		t.Fatalf("還有 %d 個有效 session", n)
	}
	revokedAt := f.now
	f.now = f.now.Add(time.Hour)
	f.archive(t, prospectA)
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites WHERE prospect_id = $1 AND revoked_at = $2", prospectA, revokedAt); n != 2 {
		t.Fatal("重送改了撤銷時間")
	}
	f.archive(t, 4242) // 沒有邀請的 prospect：no-op
}

// 合併不撤銷：輸家（source）的邀請改掛贏家（target），email 快照、secret、session 與驗證都不變（ADR「合併時的邀請」）；
// 贏家自己的邀請不動；之後贏家封存才撤銷（含改掛過來的）。重送冪等；沒有邀請是 no-op。
func TestProspectMergedRepointsTheSourceInvitesToTheWinner(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	loserInv, loser := f.session(t, prospectA, 1)
	_, verified := f.session(t, prospectA, 3)
	_, err := f.module.VerifySession(ctx, verified.Token, VerificationGoogle, "vendor@example.com")
	must(t, err)
	_, winner := f.session(t, prospectB, 2)
	merge := func() {
		must(t, f.db.InTx(ctx, func(ctx context.Context) error {
			return f.module.OnProspectMerged(ctx, seller.ProspectMerged{SourceProspectID: prospectA, TargetProspectID: prospectB})
		}))
	}
	merge()
	merge()
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites WHERE prospect_id = $1", prospectB); n != 3 {
		t.Fatalf("贏家名下的邀請 = %d，預期 3", n)
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites WHERE prospect_id = $1 OR revoked_at IS NOT NULL", prospectA); n != 0 {
		t.Fatalf("輸家名下還有邀請或有邀請被撤銷，n=%d", n)
	}
	// secret 與 session 照常可用，email 快照維持輸家當時的。
	for _, tok := range []string{loser.Token, verified.Token, winner.Token} {
		if _, err := f.module.ValidateSession(ctx, tok); err != nil {
			t.Fatalf("合併後 session 要維持有效：%v", err)
		}
	}
	if _, err := f.module.ExchangeInvite(ctx, loserInv.Secret); err != nil {
		t.Fatalf("合併後邀請連結要維持有效：%v", err)
	}
	if info, err := f.module.ValidateSession(ctx, verified.Token); err != nil || info.Verification == nil || info.Verification.Email != "vendor@example.com" {
		t.Fatalf("驗證紀錄要保留：%+v %v", info, err)
	}
	var email string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT invited_email FROM agent.vendor_portal_invites WHERE rfq_recipient_id = 1").Scan(&email))
	if email != "Vendor@Example.com" {
		t.Fatalf("email 快照 = %q", email)
	}
	// 之後贏家封存：改掛過來的邀請一併撤銷。
	f.archive(t, prospectB)
	for _, tok := range []string{loser.Token, verified.Token, winner.Token} {
		_, err := f.module.ValidateSession(ctx, tok)
		wantKind(t, err, apperr.KindUnauthenticated)
	}
	f.archive(t, prospectA) // 輸家已經沒有邀請：no-op
}

// handler 要在發布方的 transaction 內執行：在外面呼叫會讓邀請與 prospect 的狀態不一致，所以拒絕；handler 失敗時整個 transaction rollback。
func TestRevocationHandlersRequireATransactionAndRollBackWithIt(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.session(t, prospectA, 1)
	if err := f.module.OnProspectArchived(ctx, seller.ProspectArchived{ProspectID: prospectA}); err == nil {
		t.Fatal("transaction 外必須拒絕")
	}
	if err := f.module.OnProspectMerged(ctx, seller.ProspectMerged{SourceProspectID: prospectA}); err == nil {
		t.Fatal("transaction 外必須拒絕")
	}
	boom := errors.New("boom")
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		if err := f.module.OnProspectArchived(ctx, seller.ProspectArchived{ProspectID: prospectA}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if _, err := f.module.ValidateSession(ctx, s.Token); err != nil {
		t.Fatalf("發布方 rollback 時撤銷也要 rollback：%v", err)
	}
	err = f.db.InTx(ctx, func(ctx context.Context) error {
		return f.module.OnProspectArchived(ctx, seller.ProspectArchived{ProspectID: 0})
	})
	wantKind(t, err, apperr.KindInvalidInput)
}

// 清理：絕對期限過了超過 7 天的 session；到期或撤銷超過 90 天的邀請連同 email 與底下的 session；邊界內的都留著；重跑是 no-op。
func TestPurgeDeletesOnlyWhatIsPastRetention(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.session(t, prospectA, 1) // 邀請 30 天到期、session 7 天到期
	f.session(t, prospectB, 2)
	f.now = f.now.Add(30*24*time.Hour + 90*24*time.Hour) // oldInv 剛好過 90 天（到期 + 90 天 = 現在）：還不算「超過」
	res, err := f.module.PurgeExpired(ctx)
	must(t, err)
	if res.Invites != 0 {
		t.Fatalf("剛好 90 天不刪：%+v", res)
	}
	if res.Sessions != 2 {
		t.Fatalf("兩個 session 都早過了絕對期限 + 7 天：%+v", res)
	}
	f.now = f.now.Add(time.Second)
	res, err = f.module.PurgeExpired(ctx)
	must(t, err)
	if res.Invites != 2 || res.Sessions != 0 {
		t.Fatalf("res=%+v", res)
	}
	if f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites")+f.count(t, "SELECT count(*) FROM agent.vendor_portal_sessions") != 0 {
		t.Fatal("清理後不得留下邀請或 session（email 隨邀請刪除）")
	}
	res, err = f.module.PurgeExpired(ctx)
	must(t, err)
	if res != (PurgeResult{}) {
		t.Fatalf("重跑應是 no-op：%+v", res)
	}
}

func TestPurgeTreatsRevokedAndLiveInvitesSeparately(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.invite(t, prospectA, 1) // 30 天後到期，之後被撤銷
	f.archive(t, prospectA)
	live := f.invite(t, prospectB, 2)
	f.now = f.now.Add(91 * 24 * time.Hour) // 撤銷已超過 90 天；活的那張邀請 30 天前就到期但還沒滿 90 天
	res, err := f.module.PurgeExpired(ctx)
	must(t, err)
	if res.Invites != 1 {
		t.Fatalf("res=%+v，只有撤銷超過 90 天的那張該刪", res)
	}
	if n := f.count(t, "SELECT count(*) FROM agent.vendor_portal_invites WHERE invite_id = $1", live.InviteID); n != 1 {
		t.Fatal("到期不滿 90 天的邀請被刪了")
	}
}

func TestPeriodicJobsAndWorkersAreRegistered(t *testing.T) {
	t.Parallel()
	f := setup(t)
	if jobs := f.module.PeriodicJobs(); len(jobs) != 1 {
		t.Fatalf("jobs=%d", len(jobs))
	}
}

// 儲存的代碼是 UPPER_SNAKE，DB 的 CHECK 與 Go 的值集合相同（CodeMeta 退場 ADR D3）。
func TestChecksMatchGo(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	strs := func(n int, at func(int) string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = at(i)
		}
		return out
	}
	methods, reasons := VerificationMethodValues(), RevokeReasonValues()
	for _, tt := range []struct {
		table, name string
		values      []string
	}{
		{"vendor_portal_sessions", "vendor_portal_sessions_method_check", strs(len(methods), func(i int) string { return string(methods[i]) })},
		{"vendor_portal_invites", "vendor_portal_invites_revoke_reason_check", strs(len(reasons), func(i int) string { return string(reasons[i]) })},
	} {
		for _, v := range tt.values {
			if !regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*$`).MatchString(v) {
				t.Fatalf("%s: %q is not UPPER_SNAKE", tt.name, v)
			}
		}
		var def string
		must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid=$1::regclass AND conname=$2", "agent."+tt.table, tt.name).Scan(&def))
		var actual []string
		for _, m := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(def, -1) {
			actual = append(actual, m[1])
		}
		slices.Sort(actual)
		values := slices.Clone(tt.values)
		slices.Sort(values)
		if !slices.Equal(actual, values) {
			t.Fatalf("%s: DB=%v Go=%v", tt.name, actual, values)
		}
	}
}

// 第二道防線：資料庫拒絕不一致的列（寫入端出錯時不會留下壞資料）。
func TestDatabaseRejectsInconsistentRows(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	inv := f.invite(t, prospectA, 1)
	_, s := f.session(t, prospectB, 2)
	hash := sha(t, s.Token)
	for name, stmt := range map[string]string{
		"撤銷原因沒有時間":    fmt.Sprintf("UPDATE agent.vendor_portal_invites SET revoke_reason = 'PROSPECT_ARCHIVED' WHERE invite_id = '%s'", inv.InviteID),
		"撤銷時間沒有原因":    fmt.Sprintf("UPDATE agent.vendor_portal_invites SET revoked_at = now() WHERE invite_id = '%s'", inv.InviteID),
		"未知的撤銷原因":     fmt.Sprintf("UPDATE agent.vendor_portal_invites SET revoked_at = now(), revoke_reason = 'X' WHERE invite_id = '%s'", inv.InviteID),
		"hash 長度不對":   fmt.Sprintf("UPDATE agent.vendor_portal_invites SET secret_hash = '\\x00' WHERE invite_id = '%s'", inv.InviteID),
		"email 有前後空白": fmt.Sprintf("UPDATE agent.vendor_portal_invites SET invited_email = ' a@b.co' WHERE invite_id = '%s'", inv.InviteID),
		"驗證紀錄只有一部分":   fmt.Sprintf("UPDATE agent.vendor_portal_sessions SET verification_method = 'GOOGLE' WHERE token_hash = '\\x%x'", hash),
		"未知的驗證方式":     fmt.Sprintf("UPDATE agent.vendor_portal_sessions SET verification_method = 'SMS', verified_email = 'a@b.co', verified_at = now() WHERE token_hash = '\\x%x'", hash),
		"idle 超過絕對期限": fmt.Sprintf("UPDATE agent.vendor_portal_sessions SET idle_expires_at = absolute_expires_at + interval '1 second' WHERE token_hash = '\\x%x'", hash),
		"第二張邀請同一個收件人": fmt.Sprintf("INSERT INTO agent.vendor_portal_invites (invite_id, rfq_recipient_id, prospect_id, invited_email, secret_hash, expires_at, created_at, updated_at) SELECT '%s', 1, 1, 'a@b.co', sha256('x'), now(), now(), now()", uuid.NewV7()),
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, stmt); err == nil {
			t.Errorf("%s：資料庫應該拒絕", name)
		}
	}
}

// AG7：session 上的驗證紀錄只用於授權，證據由步驟 8 的入口帶進 ai 的報價版本（ADR「驗證的證據」），所以 ValidateSession 與
// VerifySession 都要交出 SessionID、方式、驗證的 email 與時間，而且兩者給的是同一份。
func TestSessionInfoCarriesTheVerificationEvidence(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.session(t, prospectA, 1)
	var id uuid.UUID
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT session_id FROM agent.vendor_portal_sessions WHERE token_hash = $1", sha(t, s.Token)).Scan(&id))
	verified, err := f.module.VerifySession(ctx, s.Token, VerificationGoogle, "vendor@example.com")
	must(t, err)
	validated, err := f.module.ValidateSession(ctx, s.Token)
	must(t, err)
	for name, info := range map[string]SessionInfo{"verify": verified, "validate": validated} {
		v := info.Verification
		if info.SessionID != id || v == nil || v.Method != VerificationGoogle || v.Email != "vendor@example.com" || !v.At.Equal(f.now) {
			t.Fatalf("%s：%+v verification=%+v", name, info, v)
		}
	}
}
