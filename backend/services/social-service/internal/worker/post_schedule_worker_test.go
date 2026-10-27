package worker

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"go.uber.org/zap"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/social-service/internal/usecase"
)

// ── 假件 ──────────────────────────────────────────────────────────────────────

type fakeScheduleRepo struct {
	convergeDeleted   int64
	convergeHidden    int64
	convergeExpired   int64
	convergeErr       error
	due               []int64
	dueErr            error
	claimRow          *gen.CmsPost
	claimErr          error
	publishOK         bool
	publishErr        error
	releaseCalls      []time.Time
	exhaustCalls      int
	revocationCalls   int
	publishCalls      int
	claimCalls        []claimCall
	listDueWindow     int
	listDueLimit      int
	convergeExpiredHr int
	// leaseHeldByMe 是終寫 CAS 落空時租約歸屬探針的回答；leaseProbeErr 讓測試模擬探針
	// 本身失敗（那時必須退回「不寫終態」的保守側）。
	leaseHeldByMe   bool
	leaseProbeErr   error
	leaseProbeCalls int
	// exhaustOK 為 nil 時 ExhaustRetries 回 true（既有預設）；指定 false 用來模擬「租約已
	// 被接手或該列已被收斂」，那時終態不是我寫的。
	exhaustOK *bool
}

type claimCall struct {
	id    int64
	owner string
	now   time.Time
	until time.Time
}

func (f *fakeScheduleRepo) ConvergeDeleted(_ context.Context, _ time.Time) (int64, error) {
	return f.convergeDeleted, f.convergeErr
}

func (f *fakeScheduleRepo) ConvergeHidden(_ context.Context, _ time.Time) (int64, error) {
	return f.convergeHidden, f.convergeErr
}

func (f *fakeScheduleRepo) ConvergeExpired(_ context.Context, _ time.Time, windowHours int) (int64, error) {
	f.convergeExpiredHr = windowHours
	return f.convergeExpired, f.convergeErr
}

func (f *fakeScheduleRepo) ListDue(_ context.Context, _ time.Time, windowHours, limit int) ([]int64, error) {
	f.listDueWindow, f.listDueLimit = windowHours, limit
	return f.due, f.dueErr
}

func (f *fakeScheduleRepo) Claim(_ context.Context, id int64, owner string, now, until time.Time, _ int) (*gen.CmsPost, error) {
	f.claimCalls = append(f.claimCalls, claimCall{id: id, owner: owner, now: now, until: until})
	return f.claimRow, f.claimErr
}

func (f *fakeScheduleRepo) Publish(ctx context.Context, _ int64, _ string, _ int16, _ time.Time,
	sameTx func(context.Context, gen.DBTX) error) (bool, error) {
	f.publishCalls++
	if f.publishErr != nil {
		return false, f.publishErr
	}
	if !f.publishOK {
		return false, nil
	}
	if sameTx != nil {
		if err := sameTx(ctx, nil); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (f *fakeScheduleRepo) ReleaseLease(_ context.Context, _ int64, _ string, _, next time.Time) (bool, error) {
	f.releaseCalls = append(f.releaseCalls, next)
	return true, nil
}

func (f *fakeScheduleRepo) ExhaustRetries(_ context.Context, _ int64, _ string, _ time.Time) (bool, error) {
	f.exhaustCalls++
	if f.exhaustOK != nil {
		return *f.exhaustOK, nil
	}
	return true, nil
}

func (f *fakeScheduleRepo) IsLeaseHeldBy(_ context.Context, _ int64, _ string) (bool, error) {
	f.leaseProbeCalls++
	return f.leaseHeldByMe, f.leaseProbeErr
}

func (f *fakeScheduleRepo) CancelByRevocation(_ context.Context, _ int64, _ string, _ time.Time) (bool, error) {
	f.revocationCalls++
	return true, nil
}

type fakeAuditClient struct {
	intentErr     error
	intents       []usecase.DelegationAuditIntent
	confirms      []usecase.DelegationAuditConfirm
	confirmErrs   []error // 逐次回傳，用完之後一律 nil
	confirmCalls  int
	intentLogID   string
	intentReplayd bool
}

func (f *fakeAuditClient) RecordAuditIntent(_ context.Context, in usecase.DelegationAuditIntent) (*usecase.DelegationAuditIntentResult, error) {
	f.intents = append(f.intents, in)
	if f.intentErr != nil {
		return nil, f.intentErr
	}
	id := f.intentLogID
	if id == "" {
		id = "8800000000000001"
	}
	return &usecase.DelegationAuditIntentResult{AuditLogID: id, Replayed: f.intentReplayd, Result: "PENDING"}, nil
}

func (f *fakeAuditClient) ConfirmAuditResult(_ context.Context, in usecase.DelegationAuditConfirm) error {
	f.confirms = append(f.confirms, in)
	f.confirmCalls++
	if f.confirmCalls <= len(f.confirmErrs) {
		return f.confirmErrs[f.confirmCalls-1]
	}
	return nil
}

// fakeHashtagRepo 只實作 AdjustCountersForVisibilityChange 會走到的兩支，其餘 panic：
// 這支測試若哪天開始碰到別的方法，是行為變了而不是測試該放寬。
type fakeHashtagRepo struct {
	ids     []int64
	deltas  []int32
	listErr error
}

func (f *fakeHashtagRepo) ListHashtagIDsByPostID(_ context.Context, _ gen.DBTX, _ int64) ([]int64, error) {
	return f.ids, f.listErr
}

func (f *fakeHashtagRepo) AdjustPostCount(_ context.Context, _ gen.DBTX, _ int64, delta int32) error {
	f.deltas = append(f.deltas, delta)
	return nil
}

func (f *fakeHashtagRepo) UpsertByName(context.Context, gen.DBTX, int64, string) (int64, error) {
	panic("unexpected")
}
func (f *fakeHashtagRepo) InsertBinding(context.Context, gen.DBTX, int64, int64, string, int16) error {
	panic("unexpected")
}
func (f *fakeHashtagRepo) UpdateBinding(context.Context, gen.DBTX, int64, int64, string, int16) error {
	panic("unexpected")
}
func (f *fakeHashtagRepo) DeleteBindings(context.Context, gen.DBTX, int64, []int64) error {
	panic("unexpected")
}
func (f *fakeHashtagRepo) DeleteAllBindings(context.Context, gen.DBTX, int64) error {
	panic("unexpected")
}
func (f *fakeHashtagRepo) ListByPostID(context.Context, gen.DBTX, int64) ([]gen.CmsPostHashtag, error) {
	panic("unexpected")
}
func (f *fakeHashtagRepo) ListByPostIDs(context.Context, []int64) (map[int64][]gen.CmsPostHashtag, error) {
	panic("unexpected")
}
func (f *fakeHashtagRepo) ListHashtagsByIDs(context.Context, []int64) (map[int64]gen.CmsHashtag, error) {
	panic("unexpected")
}
func (f *fakeHashtagRepo) SearchByPrefix(context.Context, string, int32) ([]gen.SearchHashtagsByPrefixRow, error) {
	panic("unexpected")
}
func (f *fakeHashtagRepo) SearchByContain(context.Context, string, int32) ([]gen.SearchHashtagsByContainRow, error) {
	panic("unexpected")
}
func (f *fakeHashtagRepo) PopularInRange(context.Context, time.Time, time.Time, int32) ([]gen.SelectPopularHashtagsInRangeRow, error) {
	panic("unexpected")
}

var _ domain.HashtagRepository = (*fakeHashtagRepo)(nil)
var _ domain.ScheduledPostRepository = (*fakeScheduleRepo)(nil)
var _ usecase.DelegationAuditClient = (*fakeAuditClient)(nil)

// ── 測試骨架 ──────────────────────────────────────────────────────────────────

func testConfig() config.ScheduledPostPublishConfig {
	return config.ScheduledPostPublishConfig{
		PollIntervalSeconds:       30,
		BatchSize:                 50,
		LeaseSeconds:              120,
		MaxRetry:                  5,
		BackoffBaseSeconds:        60,
		BackoffMaxSeconds:         1800,
		AuditIntentDeadlineMs:     800,
		AuditConfirmDeadlineMs:    400,
		AuditConfirmMaxAttempts:   3,
		AuditConfirmTotalBudgetMs: 2000,
		TxBudgetMs:                5000,
		WindowHours:               24,
	}
}

var fixedNow = time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)

func newTestPublisher(repo *fakeScheduleRepo, audit *fakeAuditClient, tags *fakeHashtagRepo) *postSchedulePublisher {
	if tags == nil {
		tags = &fakeHashtagRepo{}
	}
	p := newPostSchedulePublisher(testConfig(), repo, usecase.NewHashtagUseCase(tags, func() int64 { return 1 }),
		audit, nil, zap.NewNop())
	p.owner = "test-pod/1"
	p.now = func() time.Time { return fixedNow }
	return p
}

func pendingRow() *gen.CmsPost {
	return &gen.CmsPost{
		ID:                   2699000000000007,
		SellerID:             2685914682,
		Version:              3,
		PublishStatus:        domain.PostPublishStatusDraft,
		ScheduledPublishAt:   workerTimePointer(fixedNow.Add(-time.Minute)),
		ScheduleState:        workerStringPointer(domain.ScheduleStatePending),
		ScheduledBySessionID: workerInt64Pointer(7700000000000001),
		ScheduleRetryCount:   0,
	}
}

// ── 測試 ──────────────────────────────────────────────────────────────────────

// 上線初期沒有任何 pending 列是預期狀態，空表跑一輪 tick 不得報錯也不得動任何後續步驟。
func TestTick_EmptyTableIsQuiet(t *testing.T) {
	repo := &fakeScheduleRepo{}
	audit := &fakeAuditClient{}
	newTestPublisher(repo, audit, nil).tick(context.Background())

	if len(repo.claimCalls) != 0 || repo.publishCalls != 0 || len(audit.intents) != 0 {
		t.Fatalf("empty candidate set must do nothing: claims=%d publishes=%d intents=%d",
			len(repo.claimCalls), repo.publishCalls, len(audit.intents))
	}
	if repo.convergeExpiredHr != 24 || repo.listDueWindow != 24 || repo.listDueLimit != 50 {
		t.Errorf("window/limit must come from config: expiredHr=%d dueWindow=%d dueLimit=%d",
			repo.convergeExpiredHr, repo.listDueWindow, repo.listDueLimit)
	}
}

func TestPublishOne_HappyPath(t *testing.T) {
	row := pendingRow()
	repo := &fakeScheduleRepo{due: []int64{row.ID}, claimRow: row, publishOK: true}
	audit := &fakeAuditClient{}
	tags := &fakeHashtagRepo{ids: []int64{100, 200}}

	newTestPublisher(repo, audit, tags).tick(context.Background())

	if len(audit.intents) != 1 {
		t.Fatalf("intents = %d, want 1", len(audit.intents))
	}
	in := audit.intents[0]
	// 冪等鍵的第二段必須是被 claim 那一列的 scheduled_publish_at，不是 now()：用別的
	// 東西當第二段，重試會各自算出不同的鍵，一則貼文重試三次留下三列假 PENDING。
	wantKey := "schedule:2699000000000007:" + strconv.FormatInt(row.ScheduledPublishAt.Unix(), 10)
	if in.IdempotencyKey != wantKey {
		t.Errorf("idempotency key = %q, want %q", in.IdempotencyKey, wantKey)
	}
	if in.Origin != domain.DelegationOriginScheduled {
		t.Errorf("origin = %q, want scheduled — interactive 會讓已到期的工作階段被拒", in.Origin)
	}
	if in.Action != "POST_PUBLISH" || in.Capability != "posts.publish" {
		t.Errorf("action/capability = %q/%q", in.Action, in.Capability)
	}
	if in.SessionID != "7700000000000001" {
		t.Errorf("session id must come from scheduled_by_session_id, got %q", in.SessionID)
	}
	if !in.NotifySeller {
		t.Error("notifySeller 必須顯式為 true，否則 outbox 不建、賣家永遠不知道貼文發了")
	}

	if repo.publishCalls != 1 {
		t.Fatalf("publish calls = %d, want 1", repo.publishCalls)
	}
	// hashtag 計數在發布的同一個交易內 +1；草稿期間從未計數過，這裡是唯一的加點。
	if len(tags.deltas) != 2 || tags.deltas[0] != 1 {
		t.Errorf("hashtag deltas = %v, want two +1", tags.deltas)
	}

	if len(audit.confirms) != 1 {
		t.Fatalf("confirms = %d, want 1", len(audit.confirms))
	}
	c := audit.confirms[0]
	if c.Result != "SUCCEEDED" {
		t.Errorf("confirm result = %q", c.Result)
	}
	// 排程不改內容也不 bump version ⇒ 前後版本相等，記的是「發布時處於哪個版本」。
	if c.ResourceVersionAfter != in.ResourceVersionBefore {
		t.Errorf("version before/after must match: %q vs %q", in.ResourceVersionBefore, c.ResourceVersionAfter)
	}
	if len(repo.releaseCalls) != 0 || repo.exhaustCalls != 0 || repo.revocationCalls != 0 {
		t.Error("成功路徑不得走任何釋放／終態轉換")
	}
}

// 終寫 CAS rows = 0：放棄本次發布，且三件事都不做（不 confirm、不記指標、不動租約）。
// 終寫 CAS 落空且**租約已被接手**：連 FAILED 都不能寫。
//
// 這一側是原本「一律不寫」的正確理由，保留原斷言。
func TestPublishOne_TerminalCASLost_LeaseTakenOver(t *testing.T) {
	row := pendingRow()
	repo := &fakeScheduleRepo{due: []int64{row.ID}, claimRow: row, publishOK: false, leaseHeldByMe: false}
	audit := &fakeAuditClient{}

	newTestPublisher(repo, audit, nil).tick(context.Background())

	if repo.publishCalls != 1 {
		t.Fatalf("publish calls = %d, want 1", repo.publishCalls)
	}
	if repo.leaseProbeCalls != 1 {
		t.Fatalf("rows = 0 必須問一次租約歸屬才能決定寫不寫；實際問了 %d 次", repo.leaseProbeCalls)
	}
	if len(audit.confirms) != 0 {
		t.Error("租約已被接手時連 FAILED 都不能寫：接手的 pod 拿到同一列 auditLogId，" +
			"confirm 先到先贏，寫 FAILED 會把它隨後那次成功發布的 SUCCEEDED 擋掉")
	}
	if len(repo.releaseCalls) != 0 {
		t.Error("rows = 0 不得清租約：租約若已被別人持有，動它就是清別人的")
	}
	if repo.exhaustCalls != 0 || repo.revocationCalls != 0 {
		t.Error("rows = 0 不得寫任何終態值：那由收斂語句或接手的 pod 負責")
	}
}

// 終寫 CAS 落空而**租約仍在本 pod 手上**：必須寫 FAILED。
//
// 這一側沒有接手者（否則 owner 早就被 Claim 改掉了），rows = 0 只可能是另外三個述詞
// 之一不成立 —— 已刪除／已下架／已被收斂轉態。那些列不會再回到候選集合，也就是說
// **這裡是最後一個知道這列稽核該關的地方**。不寫就永久 PENDING，而永久 PENDING 會讓
// delegation_audit_pending_total 響滿整個 24 小時窗口再假性 resolve，把真正需要人看的
// PENDING 一起淹掉。
func TestPublishOne_TerminalCASLost_StillHoldingLease(t *testing.T) {
	row := pendingRow()
	repo := &fakeScheduleRepo{due: []int64{row.ID}, claimRow: row, publishOK: false, leaseHeldByMe: true}
	audit := &fakeAuditClient{}

	newTestPublisher(repo, audit, nil).tick(context.Background())

	if repo.leaseProbeCalls != 1 {
		t.Fatalf("必須問一次租約歸屬；實際問了 %d 次", repo.leaseProbeCalls)
	}
	if len(audit.confirms) != 1 {
		t.Fatalf("租約仍在我手上 ⇒ 沒有接手者 ⇒ 必須關掉這列稽核；實際 confirm %d 次", len(audit.confirms))
	}
	got := audit.confirms[0]
	if got.Result != "FAILED" {
		t.Errorf("述詞不成立代表發布確實沒發生，結論是 FAILED；實際 %q", got.Result)
	}
	if got.BizCode != bizcode.DelegationScheduledPostNotPublishable {
		t.Errorf("biz_code 要說得出「貼文已不符合發布條件」（9283），才分得出與重試耗盡的差別；實際 %d", got.BizCode)
	}
	if got.AfterSnapshotJSON != "" || len(got.ChangedFields) != 0 {
		t.Error("寫入沒有發生，after 快照與 changed_fields 必須留空 —— 記上去就是稽核在說謊")
	}
	if len(repo.releaseCalls) != 0 || repo.exhaustCalls != 0 || repo.revocationCalls != 0 {
		t.Error("關稽核不代表可以動狀態欄：終態仍由收斂語句負責")
	}
}

// 租約歸屬探針自己失敗時，退回不寫。
//
// 兩種誤判的代價不對等：誤寫 FAILED 會擋掉接手者的真實結論（稽核記成失敗、outbox 一列
// 不建、賣家收不到通知，而貼文其實發出去了）；漏寫只是留一列 PENDING 給告警。所以不
// 確定時選後者。
func TestPublishOne_TerminalCASLost_ProbeFailureStaysSilent(t *testing.T) {
	row := pendingRow()
	repo := &fakeScheduleRepo{
		due: []int64{row.ID}, claimRow: row, publishOK: false,
		leaseHeldByMe: true, leaseProbeErr: errors.New("connection reset"),
	}
	audit := &fakeAuditClient{}

	newTestPublisher(repo, audit, nil).tick(context.Background())

	if len(audit.confirms) != 0 {
		t.Errorf("探針失敗時不得猜：誤寫 FAILED 會擋掉接手者的結論；實際 confirm %d 次", len(audit.confirms))
	}
}

// 9271 是唯一走「取消排程」的分支；其餘拒絕一律退避重試。
func TestPublishOne_SessionRevoked(t *testing.T) {
	row := pendingRow()
	repo := &fakeScheduleRepo{due: []int64{row.ID}, claimRow: row, publishOK: true}
	audit := &fakeAuditClient{
		intentErr: pkgerrors.NewBusiness(bizcode.DelegationSessionRevoked, "代行工作階段已被撤銷"),
	}

	newTestPublisher(repo, audit, nil).tick(context.Background())

	if repo.revocationCalls != 1 {
		t.Fatalf("cancel-by-revocation calls = %d, want 1", repo.revocationCalls)
	}
	if repo.publishCalls != 0 {
		t.Error("被撤銷的工作階段不得發布")
	}
	if len(repo.releaseCalls) != 0 {
		t.Error("撤銷是終態，不是重試")
	}
}

func TestPublishOne_IntentFailureRetriesWithBackoff(t *testing.T) {
	row := pendingRow()
	row.ScheduleRetryCount = 1 // 第二次失敗 ⇒ 退避 60×2 = 120s
	repo := &fakeScheduleRepo{due: []int64{row.ID}, claimRow: row, publishOK: true}
	audit := &fakeAuditClient{intentErr: errors.New("upstream unavailable")}

	newTestPublisher(repo, audit, nil).tick(context.Background())

	if len(repo.releaseCalls) != 1 {
		t.Fatalf("release calls = %d, want 1", len(repo.releaseCalls))
	}
	if got, want := repo.releaseCalls[0], fixedNow.Add(120*time.Second); !got.Equal(want) {
		t.Errorf("next attempt = %v, want %v", got, want)
	}
	if repo.publishCalls != 0 || repo.revocationCalls != 0 {
		t.Error("intent 失敗後不得繼續發布，也不得當成撤銷")
	}
}

// MAX_RETRY 用盡：轉 expired（OBS8 的 retry_exhausted），不再排下一次。
func TestPublishOne_RetriesExhausted(t *testing.T) {
	row := pendingRow()
	row.ScheduleRetryCount = 4 // +1 = 5 = MaxRetry
	repo := &fakeScheduleRepo{due: []int64{row.ID}, claimRow: row}
	audit := &fakeAuditClient{intentErr: errors.New("upstream unavailable")}

	newTestPublisher(repo, audit, nil).tick(context.Background())

	if repo.exhaustCalls != 1 {
		t.Fatalf("exhaust calls = %d, want 1", repo.exhaustCalls)
	}
	if len(repo.releaseCalls) != 0 {
		t.Error("用盡之後不得再排下一次嘗試")
	}
}

// 同 tx 的 hashtag 計數失敗 ⇒ 整個交易回滾 ⇒ 不算已發布，走重試。
func TestPublishOne_SameTxFailureRetries(t *testing.T) {
	row := pendingRow()
	repo := &fakeScheduleRepo{due: []int64{row.ID}, claimRow: row, publishOK: true}
	audit := &fakeAuditClient{}
	tags := &fakeHashtagRepo{listErr: errors.New("counter read failed")}

	newTestPublisher(repo, audit, tags).tick(context.Background())

	if len(audit.confirms) != 0 {
		t.Error("交易回滾了就沒有發布這回事，不得 confirm SUCCEEDED")
	}
	if len(repo.releaseCalls) != 1 {
		t.Fatalf("release calls = %d, want 1", len(repo.releaseCalls))
	}
}

// 重試耗盡：那是最後一次嘗試，稽核必須在這裡關。
//
// 未耗盡時不 confirm 是對的（下一次重試會用同一個冪等鍵拿回同一列，由它去關）；耗盡就
// 沒有下一次了。原本兩條路徑都不寫，於是每一則「重試到放棄」的貼文都留下一列永久 PENDING
// —— 而它同時已經被記進 delegation_scheduled_post_expired_total{reason="retry_exhausted"}，
// 兩個訊號說著矛盾的話：指標說結案了、稽核說還不知道。
func TestPublishOne_RetryExhaustedClosesAuditRow(t *testing.T) {
	row := pendingRow()
	row.ScheduleRetryCount = 4 // MaxRetry = 5，這一次失敗就是第 5 次
	repo := &fakeScheduleRepo{due: []int64{row.ID}, claimRow: row, publishOK: true}
	audit := &fakeAuditClient{}
	tags := &fakeHashtagRepo{listErr: errors.New("counter read failed")}

	newTestPublisher(repo, audit, tags).tick(context.Background())

	if repo.exhaustCalls != 1 {
		t.Fatalf("第 5 次失敗必須轉 expired；實際 exhaust %d 次", repo.exhaustCalls)
	}
	if len(audit.confirms) != 1 {
		t.Fatalf("耗盡之後沒有下一次重試了，稽核必須在這裡關；實際 confirm %d 次", len(audit.confirms))
	}
	got := audit.confirms[0]
	if got.Result != "FAILED" {
		t.Errorf("貼文沒發出去，結論是 FAILED；實際 %q", got.Result)
	}
	if got.BizCode != bizcode.DelegationScheduledPostRetryExhausted {
		t.Errorf("biz_code 要說得出「重試已用盡」（9284）—— 這件事要查我們自己，"+
			"與 9283「貼文自己不能發了」（要問賣家）是兩回事；實際 %d", got.BizCode)
	}
	if got.AfterSnapshotJSON != "" || len(got.ChangedFields) != 0 {
		t.Error("發布沒有發生，after 快照與 changed_fields 必須留空")
	}
}

// 重試耗盡但 ExhaustRetries 落空（租約已被接手／該列已被收斂）：稽核歸別人寫。
func TestPublishOne_RetryExhaustedButLeaseLostStaysSilent(t *testing.T) {
	row := pendingRow()
	row.ScheduleRetryCount = 4
	repo := &fakeScheduleRepo{
		due: []int64{row.ID}, claimRow: row, publishOK: true, exhaustOK: boolPtr(false),
	}
	audit := &fakeAuditClient{}
	tags := &fakeHashtagRepo{listErr: errors.New("counter read failed")}

	newTestPublisher(repo, audit, tags).tick(context.Background())

	if len(audit.confirms) != 0 {
		t.Errorf("ExhaustRetries 落空 ⇒ 終態不是我寫的 ⇒ 稽核也不歸我關；實際 confirm %d 次",
			len(audit.confirms))
	}
}

func boolPtr(b bool) *bool { return &b }

// 租約基準逐筆取：兩列各自拿到自己的 now 與到期時間（不變式 4）。
func TestTick_LeaseBaselineIsPerRow(t *testing.T) {
	repo := &fakeScheduleRepo{due: []int64{1, 2}, claimRow: nil}
	audit := &fakeAuditClient{}
	p := newTestPublisher(repo, audit, nil)

	// 每次呼叫 now() 往前走一秒，逐筆取才看得出差別。
	tick := fixedNow
	p.now = func() time.Time {
		tick = tick.Add(time.Second)
		return tick
	}
	p.tick(context.Background())

	if len(repo.claimCalls) != 2 {
		t.Fatalf("claim calls = %d, want 2", len(repo.claimCalls))
	}
	if repo.claimCalls[0].now.Equal(repo.claimCalls[1].now) {
		t.Error("兩列共用同一個租約基準 ⇒ 不變式從「單筆最壞耗時 < 租約」退化成「批次總耗時 < 租約」")
	}
	for i, c := range repo.claimCalls {
		if want := c.now.Add(120 * time.Second); !c.until.Equal(want) {
			t.Errorf("[%d] lease until = %v, want %v", i, c.until, want)
		}
	}
}

// 退避序列必須與 config.backoffSeriesTotalSeconds 用的同一條，否則啟動時斷言的是一條
// 不會被執行的序列。base=60 cap=1800 ⇒ 60,120,240,480,960,1800,1800…
func TestBackoffSeries(t *testing.T) {
	p := newTestPublisher(&fakeScheduleRepo{}, &fakeAuditClient{}, nil)
	want := []int{60, 120, 240, 480, 960, 1800, 1800}
	for i, w := range want {
		if got := p.backoff(int32(i)); got != time.Duration(w)*time.Second {
			t.Errorf("backoff(%d) = %v, want %ds", i, got, w)
		}
	}
}

// pending 卻沒有排程歸屬 ⇒ 稽核無從建立 ⇒ 不發布，走失敗釋放讓它最後落進逾期告警。
func TestPublishOne_MissingScheduleOwnerDoesNotPublish(t *testing.T) {
	row := pendingRow()
	row.ScheduledBySessionID = nil
	repo := &fakeScheduleRepo{due: []int64{row.ID}, claimRow: row, publishOK: true}
	audit := &fakeAuditClient{}

	newTestPublisher(repo, audit, nil).tick(context.Background())

	if len(audit.intents) != 0 || repo.publishCalls != 0 {
		t.Fatalf("intents=%d publishes=%d, both must be 0", len(audit.intents), repo.publishCalls)
	}
	if len(repo.releaseCalls) != 1 {
		t.Errorf("release calls = %d, want 1", len(repo.releaseCalls))
	}
}

func workerStringPointer(value string) *string { return &value }

func workerInt64Pointer(value int64) *int64 { return &value }

func workerTimePointer(value time.Time) *time.Time { return &value }

// confirm 在總預算內重試；耗盡後貼文仍是已發布，稽核列留 PENDING（視窗 A）。
func TestConfirm_RetriesWithinBudget(t *testing.T) {
	row := pendingRow()
	repo := &fakeScheduleRepo{due: []int64{row.ID}, claimRow: row, publishOK: true}
	audit := &fakeAuditClient{confirmErrs: []error{errors.New("timeout"), errors.New("timeout")}}

	newTestPublisher(repo, audit, nil).tick(context.Background())

	if audit.confirmCalls != 3 {
		t.Fatalf("confirm calls = %d, want 3 (兩次失敗後第三次成功)", audit.confirmCalls)
	}
	if len(repo.releaseCalls) != 0 {
		t.Error("貼文已經發出去了，不得再把租約放回候選集合重發")
	}
}
