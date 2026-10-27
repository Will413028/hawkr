//go:build integration

package seller_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// queue 記下排入的 job，並確認排入發生在 transaction 內。
type queue struct {
	mu   sync.Mutex
	args []river.JobArgs
	opts []*river.InsertOpts
}

func (q *queue) Enqueue(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("enqueue outside transaction")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.args = append(q.args, args)
	q.opts = append(q.opts, opts)
	return nil
}

func (q *queue) EnqueueReport(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (bool, error) {
	return true, q.Enqueue(ctx, args, opts)
}

func (q *queue) kinds() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]string, 0, len(q.args))
	for _, a := range q.args {
		out = append(out, a.Kind())
	}
	return out
}

// images 是 media 的替身（media.Fake 的引用語意），記下轉正的 ref 類型，並確認媒體呼叫都在 transaction 外。
type images struct {
	*media.Fake
	mu        sync.Mutex
	finalized []string
	// uploaders 是每次成功轉正的上傳者（與 finalized 同序）。
	uploaders []media.Uploader
	// onFinalize 在轉正之後執行，用來製造「轉正之後、寫入之前」的交錯。
	onFinalize func()
	// rejected 是 media 拒絕轉正的 token 與回的錯誤（無效、過期或不屬於這個帳號的 token）。
	rejected map[string]error
}

func (i *images) Finalize(ctx context.Context, u media.Uploader, token, refType string) (media.Asset, error) {
	i.mu.Lock()
	rejected := i.rejected[token]
	i.mu.Unlock()
	if rejected != nil {
		return media.Asset{}, rejected
	}
	asset, err := i.Fake.Finalize(ctx, u, token, refType)
	if err != nil {
		return asset, err
	}
	i.mu.Lock()
	i.finalized = append(i.finalized, refType)
	i.uploaders = append(i.uploaders, u)
	hook := i.onFinalize
	i.onFinalize = nil
	i.mu.Unlock()
	if hook != nil {
		hook()
	}
	return asset, nil
}

func newImages() *images {
	fake := media.NewFake()
	fake.Key = func(refType, token string) string { return refType + "/" + token }
	return &images{Fake: fake}
}

// sync 執行 ref 的同步 job 一輪。
func (f *fixture) syncMedia(t *testing.T, ref media.Ref) media.SyncResult {
	t.Helper()
	res, err := f.module.SyncMedia(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (f *fixture) storeRef(refType string, id int64) media.Ref {
	return media.Ref{Type: refType, ID: strconv.FormatInt(id, 10)}
}

// storeOrders 是店家未結訂單 port 的替身：open、err 決定回答，calls 記下被問的 store_id，onCall 在查詢時執行一次（鎖外），
// 並確認查詢發生在 transaction 外（遠端呼叫不得在交易內）。
type storeOrders struct {
	mu     sync.Mutex
	open   bool
	err    error
	calls  []int64
	onCall func()
}

func (o *storeOrders) HasOpenStoreOrders(ctx context.Context, storeID int64) (bool, error) {
	if _, ok := db.TxFrom(ctx); ok {
		return false, errors.New("store orders queried inside a transaction")
	}
	o.mu.Lock()
	o.calls = append(o.calls, storeID)
	hook := o.onCall
	o.onCall = nil
	open, err := o.open, o.err
	o.mu.Unlock()
	if hook != nil {
		hook()
	}
	return open, err
}

// logBuf 收集模組的 log，測試用來確認什麼情況下會記 error。
type logBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *logBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

type fixture struct {
	logs     logBuf
	db       *db.DB
	identity *identity.Module
	module   *Module
	bus      *events.Bus
	jobs     *queue
	media    *images
	payuni   *fakePayUni
	orders   *storeOrders
	products *fakeProductStores
	registry *fakeRegistry
	now      time.Time
	codes    []int64
	nextCode int64
}

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()}, migrate.Module{Name: "seller", Dir: Migrations()},
	))
	f := &fixture{db: d, bus: events.NewBus(), jobs: &queue{}, media: newImages(), payuni: &fakePayUni{t: t}, orders: &storeOrders{}, products: &fakeProductStores{}, registry: &fakeRegistry{t: t}, now: time.Date(2026, 10, 1, 4, 0, 0, 0, time.UTC)}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	f.identity = identity.New(identity.Deps{DB: d, Events: f.bus, Jobs: f.jobs, Now: func() time.Time { return f.now }, Logger: log})
	f.module = f.newModule(t, f.payuni, f.jobs)
	return f
}

// newModule 在同一個 DB 上組一個 seller 模組：PayUni 的 client 與 job 的排入可以換（沒有設定 PayUni、真的 River）。
func (f *fixture) newModule(t *testing.T, client PayUniClient, jc jobs.ReportingEnqueuer) *Module {
	t.Helper()
	kycKeys, kycMAC := sealtest.Keys(t)
	return f.newModuleWithKeys(t, client, jc, kycKeys, kycMAC)
}

// newModuleWithKeys 同 newModule，KYC 的金鑰環由呼叫方給（測試金鑰輪替）。
func (f *fixture) newModuleWithKeys(t *testing.T, client PayUniClient, jc jobs.ReportingEnqueuer, kycKeys *seal.Keyring, kycMAC *seal.MAC) *Module {
	t.Helper()
	return f.newModuleClock(t, client, jc, kycKeys, kycMAC, func() time.Time { return f.now })
}

// newModuleClock 同 newModuleWithKeys，時鐘由呼叫方給（測試多實例的時鐘差）。
func (f *fixture) newModuleClock(t *testing.T, client PayUniClient, jc jobs.ReportingEnqueuer, kycKeys *seal.Keyring, kycMAC *seal.MAC, now func() time.Time) *Module {
	t.Helper()
	log := slog.New(slog.NewTextHandler(&f.logs, nil))
	return New(Deps{
		DB: f.db, Events: f.bus, Jobs: jc, Identity: f.identity, Media: f.media, Orders: f.orders, KYCKeys: kycKeys, KYCMAC: kycMAC, PayUni: client, ProductStores: f.products, Registry: f.registry,
		StorefrontBaseURL: "https://store.example.invalid",
		Now:               now, Logger: log,
		StoreCode: func(now time.Time) (int64, error) {
			if len(f.codes) > 0 {
				code := f.codes[0]
				f.codes = f.codes[1:]
				return code, nil
			}
			f.nextCode++
			return int64(now.Year()%100)*100_000_000 + f.nextCode, nil
		},
	})
}

// exposures 是和模組同一組依賴的可曝光公告，給直接呼叫內部套件的測試用。
func (f *fixture) exposures() *exposure.Service {
	return exposure.New(exposure.Deps{Events: f.bus})
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func ptr[T any](v T) *T { return &v }

// account 建立一個帳號。先多建兩個，讓 account_id 與第一個 store_id 不同值：互換 ID 的缺陷才藏不住。
func (f *fixture) account(t *testing.T) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	for range 3 {
		must(t, f.db.Conn(ctx).QueryRow(ctx, "INSERT INTO identity.accounts (clerk_user_id) VALUES ($1) RETURNING id", uuid.NewV7().String()).Scan(&id))
	}
	return id
}

func (f *fixture) count(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

func (f *fixture) isSeller(t *testing.T, accountID int64) bool {
	t.Helper()
	a, err := f.identity.Account(context.Background(), accountID)
	must(t, err)
	return slices.Contains(a.Roles, identity.RoleSeller)
}

func validApply(accountID int64) ApplyInput {
	return ApplyInput{
		AccountID: accountID, StoreName: " 範例小農 ", ContactPhone: "+886 912-345-678",
		TownCode: "63000050", AddressDetail: "測試路一號", AcceptedAgreements: CurrentApplyAcceptances(),
	}
}

func (f *fixture) open(t *testing.T) (int64, Store) {
	t.Helper()
	accountID := f.account(t)
	s, err := f.module.Apply(context.Background(), validApply(accountID))
	must(t, err)
	f.setContact(t, s.StoreID)
	s, err = f.module.Store(context.Background(), s.StoreID)
	must(t, err)
	return accountID, s
}

// setContact 直接寫入送出 KYC 需要的店家聯絡資料（開店不帶這三欄）；不經模組，店家的版本與事件不變，測試的版本與事件計數不受影響。
func (f *fixture) setContact(t *testing.T, storeID int64) {
	t.Helper()
	_, err := f.db.Conn(context.Background()).Exec(context.Background(),
		"UPDATE seller.stores SET contact_name = '王小明', contact_mobile = '0912345678', support_email = 'owner@example.com' WHERE store_id = $1", storeID)
	must(t, err)
}

// NewReader 只要資料庫就讀得到店家（運維子命令的 delegation 依賴），讀到的與 Module.Store 相同，包含已關閉的店與不存在（NotFound）。
func TestReaderReadsTheSameStoreAsTheModuleWithOnlyTheDatabase(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	r := NewReader(f.db, func() time.Time { return f.now })
	got, err := r.Store(context.Background(), s.StoreID)
	must(t, err)
	want, err := f.module.Store(context.Background(), s.StoreID)
	must(t, err)
	if got.StoreID != want.StoreID || got.AccountID != want.AccountID || got.Status != want.Status || got.Suspended() != want.Suspended() {
		t.Fatalf("Reader.Store = %+v，want %+v", got, want)
	}
	if _, err := r.Store(context.Background(), s.StoreID+1000); err == nil {
		t.Fatal("不存在的店家應回錯誤")
	}
}

func TestApplyCreatesStoreGrantsSellerAndPublishes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	var opened []StoreOpened
	events.Subscribe(f.bus, func(_ context.Context, e StoreOpened) error {
		opened = append(opened, e)
		return nil
	})
	accountID, s := f.open(t)
	if s.StoreName != "範例小農" || *s.ContactPhone != "0912345678" || *s.TownCode != "63000050" ||
		*s.AddressDetail != "測試路一號" || s.Version != 1 || s.Status != Active || s.Suspended() {
		t.Fatalf("store=%+v", s)
	}
	if s.StoreID == accountID {
		t.Fatal("fixture must keep store_id and account_id apart")
	}
	if !f.isSeller(t, accountID) {
		t.Fatal("SELLER not granted")
	}
	if len(opened) != 1 || opened[0].StoreID != s.StoreID || opened[0].AccountID != accountID || opened[0].OccurredAt != f.now {
		t.Fatalf("events=%+v", opened)
	}
	if !slices.Contains(f.jobs.kinds(), revalidate.Args{}.Kind()) {
		t.Fatalf("jobs=%v", f.jobs.kinds())
	}
	current, err := f.module.CurrentStore(context.Background(), accountID)
	must(t, err)
	if current.StoreID != s.StoreID {
		t.Fatalf("current=%+v", current)
	}
}

// 回歸（盤點 §4、§6-1）：舊流程先建店、授予 SELLER，之後才驗證縣市。任何一欄不合法都不能留下店家或角色。
func TestApplyValidatesEverythingBeforeWriting(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID := f.account(t)
	for name, mutate := range map[string]func(*ApplyInput){
		"bad town code":             func(in *ApplyInput) { in.TownCode = "99999999" },
		"county code as town":       func(in *ApplyInput) { in.TownCode = "63000" },
		"address without town":      func(in *ApplyInput) { in.TownCode = "" },
		"bad phone":                 func(in *ApplyInput) { in.ContactPhone = "12345" },
		"blank store name":          func(in *ApplyInput) { in.StoreName = "  " },
		"store name over 100 runes": func(in *ApplyInput) { in.StoreName = string(make([]rune, 101)) },
	} {
		in := validApply(accountID)
		mutate(&in)
		_, err := f.module.Apply(context.Background(), in)
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	if n := f.count(t, "SELECT count(*) FROM seller.stores"); n != 0 {
		t.Fatalf("stores written: %d", n)
	}
	if f.isSeller(t, accountID) {
		t.Fatal("SELLER granted although the input was invalid")
	}
	if len(f.jobs.kinds()) != 0 {
		t.Fatalf("jobs=%v", f.jobs.kinds())
	}
}

// 回歸（盤點 §4）：舊流程在介紹、縣市、得獎紀錄都沒填時，靜默丟掉行政區與地址。
func TestApplyKeepsDistrictAndAddressWithoutOtherFields(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID := f.account(t)
	s, err := f.module.Apply(context.Background(), ApplyInput{AcceptedAgreements: CurrentApplyAcceptances(), AccountID: accountID, StoreName: "小店", TownCode: "64000120", AddressDetail: "光復路"})
	must(t, err)
	if s.TownCode == nil || *s.TownCode != "64000120" || s.AddressDetail == nil || *s.AddressDetail != "光復路" || s.ContactPhone != nil || s.Description != nil {
		t.Fatalf("store=%+v", s)
	}
}

func TestApplyEventFailureRollsBackStoreAndRole(t *testing.T) {
	t.Parallel()
	f := setup(t)
	events.Subscribe(f.bus, func(context.Context, StoreOpened) error { return errors.New("reject") })
	accountID := f.account(t)
	if _, err := f.module.Apply(context.Background(), validApply(accountID)); err == nil {
		t.Fatal("event failure lost")
	}
	if f.count(t, "SELECT count(*) FROM seller.stores") != 0 || f.isSeller(t, accountID) {
		t.Fatal("store or role survived the rollback")
	}
}

func TestApplyRejectsSecondStoreAndReonboardsWithNewStint(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, first := f.open(t)
	if _, err := f.module.Apply(ctx, validApply(accountID)); apperr.KindOf(err) != apperr.KindAlreadyExists {
		t.Fatalf("second apply: %v", err)
	}
	closed, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if closed.Status != Closed || closed.Closure == nil || f.isSeller(t, accountID) {
		t.Fatalf("closed=%+v seller=%v", closed, f.isSeller(t, accountID))
	}
	second, err := f.module.Apply(ctx, validApply(accountID))
	must(t, err)
	if second.StoreID == first.StoreID || second.StoreCode == first.StoreCode || !f.isSeller(t, accountID) {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	old, err := f.module.Store(ctx, first.StoreID)
	must(t, err)
	if old.Status != Closed {
		t.Fatalf("old stint=%+v", old)
	}
}

func TestApplyRetriesStoreCodeCollision(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.codes = []int64{2612345678}
	_, first := f.open(t)
	f.codes = []int64{2612345678, 2612345679}
	_, second := f.open(t)
	if first.StoreCode != 2612345678 || second.StoreCode != 2612345679 {
		t.Fatalf("codes=%d %d", first.StoreCode, second.StoreCode)
	}
}

// 回歸（盤點 §4）：舊的 UpdateProfile 是整筆覆寫，沒帶的電話會被清成空字串。
func TestUpdateProfileKeepsOmittedFieldsAndClearsExplicitEmpty(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	updated, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, Description: ptr("新鮮蔬果")})
	must(t, err)
	if updated.ContactPhone == nil || *updated.ContactPhone != "0912345678" || *updated.TownCode != "63000050" || *updated.Description != "新鮮蔬果" || updated.Version != s.Version+1 {
		t.Fatalf("updated=%+v", updated)
	}
	updated, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: updated.Version, ContactPhone: ptr("")})
	must(t, err)
	if updated.ContactPhone != nil {
		t.Fatalf("explicit empty phone must clear: %+v", updated)
	}
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: updated.Version, TownCode: ptr("")})
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("address left without a town: %v", err)
	}
	updated, err = f.module.UpdateProfile(ctx, UpdateProfileInput{
		StoreID: s.StoreID, ExpectedVersion: updated.Version, TownCode: ptr("67000320"), AddressDetail: ptr(""),
		WebsiteURL: ptr("HTTPS://Example.COM/Shop"),
	})
	must(t, err)
	if *updated.WebsiteURL != "https://example.com/Shop" {
		t.Fatalf("website url not normalized: %s", *updated.WebsiteURL)
	}
	if *updated.TownCode != "67000320" || updated.AddressDetail != nil {
		t.Fatalf("updated=%+v", updated)
	}
	for name, in := range map[string]UpdateProfileInput{
		"website scheme": {WebsiteURL: ptr("ftp://example.com")},
	} {
		in.StoreID, in.ExpectedVersion = s.StoreID, updated.Version
		if _, err := f.module.UpdateProfile(ctx, in); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// 文字驗證失敗的換圖請求不消耗 staged token：沒有轉正，用同一個 token 修正後重送成功。
func TestRejectedProfileUpdateDoesNotConsumeTheStagedToken(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	_, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, ContactPhone: ptr("not-a-phone"), LogoToken: "logo1", UploaderID: accountID})
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err=%v", err)
	}
	if len(f.media.finalized) != 0 {
		t.Fatalf("token finalized by a rejected request: %v", f.media.finalized)
	}
	updated, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, ContactPhone: ptr("0912345678"), LogoToken: "logo1", UploaderID: accountID})
	must(t, err)
	if len(f.media.finalized) != 1 || updated.LogoKey == nil {
		t.Fatalf("retry: finalized=%v store=%+v", f.media.finalized, updated)
	}
}

// 其他的前置拒絕（版本、能力）同樣發生在轉正之前。
func TestStaleVersionAndSuspensionRejectBeforeFinalize(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version + 5, LogoToken: "logo1", UploaderID: accountID}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("err=%v", err)
	}
	f.suspend(t, s.StoreID)
	if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, LogoToken: "logo1", UploaderID: accountID}); err == nil {
		t.Fatal("suspended store accepted a logo")
	}
	if len(f.media.finalized) != 0 {
		t.Fatalf("finalized=%v", f.media.finalized)
	}
}

// 完成新手引導只設旗標：已存的值不再通過現行驗證（這裡是不在內政部表裡的鄉鎮市區代碼）也能完成；重送冪等，版本要相符。
func TestCompleteOnboardingDoesNotRevalidateStoredFields(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET town_code = '99999999', version = version + 1 WHERE store_id = $1", s.StoreID)
	must(t, err)
	cur, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, Prize: ptr("金牌")}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("precondition: a stored retired town must fail validation, err=%v", err)
	}
	if _, err := f.module.CompleteOnboarding(ctx, s.StoreID, cur.Version-1); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("stale version err=%v", err)
	}
	done, err := f.module.CompleteOnboarding(ctx, s.StoreID, cur.Version)
	must(t, err)
	if !done.OnboardingCompleted || done.Version != cur.Version+1 || *done.TownCode != "99999999" || done.StoreName != cur.StoreName {
		t.Fatalf("done=%+v", done)
	}
	again, err := f.module.CompleteOnboarding(ctx, s.StoreID, done.Version)
	must(t, err)
	if again.Version != done.Version {
		t.Fatalf("idempotent call bumped the version: %d -> %d", done.Version, again.Version)
	}
}

func TestUpdateProfileVersionCAS(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var changed int
	events.Subscribe(f.bus, func(context.Context, StoreProfileChanged) error {
		changed++
		return nil
	})
	_, s := f.open(t)
	if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, Prize: ptr("金牌")}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("missing version: %v", err)
	}
	_, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, Prize: ptr("金牌")})
	must(t, err)
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, Prize: ptr("銀牌")})
	if apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("stale version: %v", err)
	}
	current, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if *current.Prize != "金牌" || current.Version != s.Version+1 || changed != 1 {
		t.Fatalf("store=%+v changed=%d", current, changed)
	}
}

func TestImagesLinkByStoreIDAndUseVersion(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	withLogo, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, LogoToken: "logo1", UploaderID: accountID})
	must(t, err)
	if withLogo.LogoKey == nil || *withLogo.LogoKey != media.RefTypeSellerLogo+"/logo1" || withLogo.Version != s.Version+1 {
		t.Fatalf("store=%+v", withLogo)
	}
	logoRef := f.storeRef(media.RefTypeSellerLogo, s.StoreID)
	if f.media.finalized[0] != media.RefTypeSellerLogo || len(f.media.Referenced(logoRef)) != 0 ||
		!slices.Contains(f.jobs.kinds(), "seller.media_sync") {
		t.Fatalf("finalized=%+v references=%v jobs=%v", f.media.finalized, f.media.Referenced(logoRef), f.jobs.kinds())
	}
	f.syncMedia(t, logoRef)
	if got := f.media.Referenced(logoRef); fmt.Sprint(got) != "[asset-logo1]" {
		t.Fatalf("references=%v", got)
	}
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, CoverToken: "cover1", UploaderID: accountID})
	if apperr.KindOf(err) != apperr.KindVersionConflict || len(f.media.finalized) != 1 {
		t.Fatalf("stale cover: %v finalized=%d", err, len(f.media.finalized))
	}
	withCover, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: withLogo.Version, CoverToken: "cover1", UploaderID: accountID})
	must(t, err)
	if withCover.CoverKey == nil || *withCover.LogoKey != *withLogo.LogoKey {
		t.Fatalf("store=%+v", withCover)
	}
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		_, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: withCover.Version, LogoToken: "x", UploaderID: accountID})
		if apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("image change inside a transaction: %v", err)
		}
		return nil
	}))
}

// 文字欄位、logo 與封面在同一個版本條件的 transaction 一起寫入：衝突時三者都沒有寫入，轉正的引用靠收斂釋放。
func TestProfileAndImagesAreWrittenInOneVersionedTransaction(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	updated, err := f.module.UpdateProfile(ctx, UpdateProfileInput{
		StoreID: s.StoreID, ExpectedVersion: s.Version, Prize: ptr("金牌"), LogoToken: "logo1", CoverToken: "cover1", UploaderID: accountID,
	})
	must(t, err)
	if *updated.Prize != "金牌" || updated.LogoKey == nil || updated.CoverKey == nil || updated.Version <= s.Version {
		t.Fatalf("store=%+v", updated)
	}
	if fmt.Sprint(f.media.finalized) != fmt.Sprint([]string{media.RefTypeSellerLogo, media.RefTypeSellerCover}) {
		t.Fatalf("finalized=%+v", f.media.finalized)
	}
	// 沒有文字欄位改變時，換圖照樣寫入。
	only, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: updated.Version, LogoToken: "logo2", UploaderID: accountID})
	must(t, err)
	if *only.LogoKey != media.RefTypeSellerLogo+"/logo2" || *only.CoverKey != *updated.CoverKey {
		t.Fatalf("store=%+v", only)
	}

	f.media.onFinalize = func() {
		_, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: only.Version, Prize: ptr("銅牌")})
		must(t, err)
	}
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: only.Version, Prize: ptr("銀牌"), LogoToken: "logo3", UploaderID: accountID})
	if apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("err=%v", err)
	}
	current, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if *current.Prize != "銅牌" || *current.LogoKey != *only.LogoKey {
		t.Fatalf("conflicting update left a partial write: %+v", current)
	}
}

// 空字串沿用現在的圖；只有空白的 token 是 InvalidInput，指向對應的模組欄位，而且不轉正任何圖。
func TestWhitespaceStagedTokenIsInvalidInput(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	for field, in := range map[string]UpdateProfileInput{
		"LogoToken":  {LogoToken: "  ", CoverToken: "cover1"},
		"CoverToken": {CoverToken: "\t"},
	} {
		in.StoreID, in.ExpectedVersion, in.UploaderID = s.StoreID, s.Version, accountID
		_, err := f.module.UpdateProfile(ctx, in)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != field || len(f.media.finalized) != 0 {
			t.Fatalf("%s: err=%v finalized=%d", field, err, len(f.media.finalized))
		}
	}
}

// 關店可逆：logo 與封面連同 media 的引用留到個資清理才釋放（記成退場並排入同步），關店本身不排入引用的同步。
func TestCloseKeepsTheImagesUntilThePurgeReleasesThem(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	withLogo, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, LogoToken: "logo1", UploaderID: accountID})
	must(t, err)
	_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: withLogo.Version, CoverToken: "cover1", UploaderID: accountID})
	must(t, err)
	for _, ref := range []media.Ref{f.storeRef(media.RefTypeSellerLogo, s.StoreID), f.storeRef(media.RefTypeSellerCover, s.StoreID)} {
		f.syncMedia(t, ref)
	}
	f.jobs.args = nil
	closed, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if closed.LogoKey == nil || closed.CoverKey == nil || slices.Contains(f.jobs.kinds(), "seller.media_sync") {
		t.Fatalf("closed=%+v jobs=%v", closed, f.jobs.kinds())
	}
	for _, ref := range []media.Ref{f.storeRef(media.RefTypeSellerLogo, s.StoreID), f.storeRef(media.RefTypeSellerCover, s.StoreID)} {
		f.syncMedia(t, ref)
		if got := f.media.Referenced(ref); len(got) != 1 {
			t.Fatalf("%s 的引用在關店之後仍在：%v", ref.Type, got)
		}
	}
	f.now = f.now.Add(StoreRetention + time.Second)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 1 || !slices.Contains(f.jobs.kinds(), "seller.media_sync") {
		t.Fatalf("n=%d jobs=%v", n, f.jobs.kinds())
	}
	purged := f.storeRow(t, s.StoreID)
	if purged.LogoKey != nil || purged.CoverKey != nil {
		t.Fatalf("清理釋放 logo 與封面：%+v", purged)
	}
	for _, ref := range []media.Ref{f.storeRef(media.RefTypeSellerLogo, s.StoreID), f.storeRef(media.RefTypeSellerCover, s.StoreID)} {
		f.syncMedia(t, ref)
		if got := f.media.Referenced(ref); len(got) != 0 {
			t.Fatalf("%s 的引用清理之後要全部解除：%v", ref.Type, got)
		}
	}
}

// RISK 停權與結束：事件、同一段停權期間、結束後的冪等。
func TestRiskSuspensionAndEnd(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	suspended := record[StoreSuspended](f)
	unsuspended := record[StoreUnsuspended](f)
	accountID, s := f.open(t)
	review := f.now.Add(7 * 24 * time.Hour)
	if _, err := f.module.CreateMeasure(ctx, CreateMeasureInput{AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "x", Note: "n", ReviewBy: &review, Risk: &RiskBasis{Ground: "FOOD"}}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("unknown ground: %v", err)
	}
	first := f.riskSuspend(t, s.StoreID)
	if got := suspended.all(); len(got) != 1 || got[0].PeriodID <= 0 || got[0].AccountID != accountID {
		t.Fatalf("suspended events=%+v", got)
	}
	current, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if !current.Suspended() || current.Version != s.Version {
		t.Fatalf("store=%+v", current)
	}
	_, other := f.open(t)
	_ = other
	f.afterCreation(first)
	ended, err := f.module.EndMeasure(ctx, EndMeasureInput{MeasureID: first.ID, AdminID: 9, ExpectedVersion: first.Version})
	must(t, err)
	if ended.State != StateEnded || ended.CurrentRevision != first.CurrentRevision+1 || ended.EndsAt == nil || !ended.EndsAt.Equal(f.now) {
		t.Fatalf("ended=%+v", ended)
	}
	if _, err := f.module.EndMeasure(ctx, EndMeasureInput{MeasureID: first.ID, AdminID: 9, ExpectedVersion: first.Version}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("end with the old version: %v", err)
	}
	if _, err := f.module.EndMeasure(ctx, EndMeasureInput{MeasureID: first.ID, AdminID: 9, ExpectedVersion: ended.Version}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("end twice: %v", err)
	}
	if got := unsuspended.all(); len(got) != 1 || got[0].PeriodID != suspended.all()[0].PeriodID || got[0].StoreID != s.StoreID {
		t.Fatalf("one suspension period must carry one id: suspended=%+v unsuspended=%+v", suspended.all(), got)
	}
	if f.isSuspended(t, s.StoreID) {
		t.Fatal("ended measure still suspends")
	}
}

// 停權檢查每次讀 DB：停權回 PermissionDenied；讀取失敗時回錯誤，不可當成沒有停權（取代舊 gateway 的 fail-open）。
func TestActingStoreFailsClosed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	acting, err := f.module.ActingStore(ctx, accountID, CapProductManage)
	must(t, err)
	if acting.StoreID != s.StoreID {
		t.Fatalf("acting=%+v", acting)
	}
	f.suspend(t, s.StoreID)
	if _, err := f.module.ActingStore(ctx, accountID, CapProductManage); apperr.KindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("suspended: %v", err)
	}
	if _, err := f.module.ActingStore(ctx, f.account(t), CapProductManage); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("no store: %v", err)
	}
	broken, cancel := context.WithCancel(ctx)
	cancel()
	for _, id := range []int64{accountID, f.account(t)} {
		if acting, err := f.module.ActingStore(broken, id, CapAccountSelfRead); err == nil {
			t.Fatalf("database failure must deny: acting=%+v", acting)
		}
	}
	other := f.account(t)
	_, err = f.module.Apply(ctx, validApply(other))
	must(t, err)
	if _, err := f.module.ActingStore(broken, other, CapAccountSelfRead); err == nil {
		t.Fatal("database failure must deny an unsuspended store too")
	}
}

// 停權的賣家仍可以自己關店（能力檢查 ADR 的 CAP_ACCOUNT_CLOSE；舊流程回誤導的 9204）。
func TestSuspendedSellerCanCloseStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var closedEvents []StoreClosed
	events.Subscribe(f.bus, func(_ context.Context, e StoreClosed) error {
		closedEvents = append(closedEvents, e)
		return nil
	})
	accountID, s := f.open(t)
	f.suspend(t, s.StoreID)
	closed, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if closed.Status != Closed || f.isSeller(t, accountID) || len(closedEvents) != 1 || closedEvents[0].StoreID != s.StoreID {
		t.Fatalf("closed=%+v events=%+v", closed, closedEvents)
	}
	if _, err := f.module.CloseCurrent(ctx, accountID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("close again: %v", err)
	}
	review := f.now.Add(7 * 24 * time.Hour)
	if _, err := f.module.CreateMeasure(ctx, CreateMeasureInput{AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "x", Note: "n", ReviewBy: &review, Risk: &RiskBasis{Ground: RiskFraud}}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("suspend closed store: %v", err)
	}
}

func TestDeletionParticipantIdempotency(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var pendingEvents []StorePendingDeletion
	events.Subscribe(f.bus, func(_ context.Context, e StorePendingDeletion) error {
		pendingEvents = append(pendingEvents, e)
		return nil
	})
	accountID, s := f.open(t)
	f.jobs.args = nil
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		if err := f.identity.LockAccount(ctx, accountID); err != nil {
			return err
		}
		if err := f.module.BeginDeletion(ctx, s.StoreID); err != nil {
			return err
		}
		return f.module.BeginDeletion(ctx, s.StoreID)
	}))
	pending, err := f.module.CurrentStore(ctx, accountID)
	must(t, err)
	if pending.Status != PendingDeletion || pending.Version != s.Version {
		t.Fatalf("pending=%+v", pending)
	}
	if len(pendingEvents) != 1 || pendingEvents[0].StoreID != s.StoreID || pendingEvents[0].AccountID != accountID ||
		!slices.Contains(f.jobs.kinds(), "revalidate") {
		t.Fatalf("pending transition must publish once and invalidate: events=%+v jobs=%v", pendingEvents, f.jobs.kinds())
	}
	for range 2 {
		must(t, f.db.InTx(ctx, func(ctx context.Context) error {
			if err := f.identity.LockAccount(ctx, accountID); err != nil {
				return err
			}
			return f.module.CloseForDeletion(ctx, s.StoreID)
		}))
	}
	if _, err := f.module.CurrentStore(ctx, accountID); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("current after close: %v", err)
	}
	if f.isSeller(t, accountID) || f.count(t, "SELECT count(*) FROM seller.stores WHERE status='CLOSED'") != 1 {
		t.Fatal("close not applied once")
	}
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, s.StoreID) }))
}

func TestSchemaConstraints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, s := f.open(t)
	for _, sql := range []string{
		"UPDATE seller.stores SET town_code='6300005' WHERE store_id=$1",
		"UPDATE seller.stores SET town_code=NULL WHERE store_id=$1",
		"UPDATE seller.stores SET logo_key='k' WHERE store_id=$1",
		"UPDATE seller.stores SET store_name=' 空白 ' WHERE store_id=$1",
		"UPDATE seller.stores SET contact_phone='' WHERE store_id=$1",
		"UPDATE seller.stores SET website_url='javascript:alert(1)' WHERE store_id=$1",
		"INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) SELECT 2699999999, account_id, 'x', now(), now() FROM seller.stores WHERE store_id=$1",
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, sql, s.StoreID); err == nil {
			t.Fatalf("DB accepted: %s", sql)
		}
	}
}

func stringsOf[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}

// 儲存的代碼是 UPPER_SNAKE，DB 的 CHECK 與 Go 的值集合相同（CodeMeta 退場 ADR D3）；縣市的 CHECK 與驗證用的表相同。
func TestChecksMatchGo(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for _, tt := range []struct {
		table, name string
		values      []string
		upperSnake  bool
	}{
		{"stores", "stores_status_check", stringsOf(StatusValues()), true},
		{"measures", "measures_kind_check", stringsOf(MeasureKindValues()), true},
		{"measures", "measures_basis_value_check", stringsOf(BasisValues()), true},
		{"measures", "measures_risk_ground_check", stringsOf(RiskGroundValues()), true},
		{"measures", "measures_status_check", stringsOf(model.MeasureStatusValues()), true},
		{"measures", "measures_revoked_reason_check", stringsOf(RevokeReasonValues()), true},
		{"measures", "measures_termination_ground_check", stringsOf(TerminationGroundValues()), true},
		{"measure_revisions", "measure_revisions_system_cause_check", stringsOf(RevisionSystemCauseValues()), true},
		{"store_closures", "store_closures_closed_by_check", stringsOf(ClosedByValues()), true},
		{"measure_requests", "measure_requests_kind_check", stringsOf(model.RequestKindValues()), true},
		{"measure_requests", "measure_requests_state_check", stringsOf(RequestStateValues()), true},
		{"measure_requests", "measure_requests_close_cause_check", stringsOf(RequestCloseCauseValues()), true},
		{"reports", "reports_category_check", stringsOf(policy.ReporterSelectable()), true},
		{"reports", "reports_outcome_check", stringsOf(OutcomeValues()), true},
		{"report_supplements", "report_supplements_matched_by_check", stringsOf(SupplementMatchValues()), true},
		{"cases", "cases_category_check", stringsOf(policy.Assessable()), true},
		{"cases", "cases_severity_check", stringsOf(SeverityValues()), true},
		{"cases", "cases_close_check", append(stringsOf(CaseCloseReasonValues()), stringsOf(DecisionValues())...), true},
		{"case_inquiries", "case_inquiries_kind_check", stringsOf(InquiryKindValues()), true},
		{"case_inquiries", "case_inquiries_reply_check", stringsOf(InquiryReplyChannelValues()), true},
		{"case_inquiries", "case_inquiries_withdraw_check", stringsOf(InquiryWithdrawCauseValues()), true},
		{"authority_orders", "authority_orders_kind_check", stringsOf(AuthorityOrderKindValues()), true},
		{"authority_order_targets", "authority_order_targets_notice_check", stringsOf(AuthorityNoticeKindValues()), true},
		{"authority_order_hold_extensions", "authority_order_hold_extensions_cause_check", stringsOf(HoldExtensionCauseValues()), true},
		{"violations", "violations_category_check", stringsOf(policy.Assessable()), true},
		{"violations", "violations_severity_check", stringsOf(SeverityValues()), true},
		{"violations", "violations_status_check", stringsOf(ViolationStatusValues()), true},
		{"violations", "violations_revoked_reason_check", slices.DeleteFunc(stringsOf(RevokeReasonValues()), func(v string) bool { return v == "FINDING_REVOKED" || v == "SOURCE_REVOKED" }), true},
		{"prospects", "prospects_status_check", stringsOf(ProspectStatusValues()), true},
		{"prospects", "prospects_vendor_type_check", stringsOf(VendorTypeValues()), true},
		{"prospects", "prospects_outreach_check", stringsOf(OutreachStatusValues()), true},
		{"prospect_contacts", "prospect_contacts_kind_check", stringsOf(ContactKindValues()), true},
		{"kyc_registration_checks", "kyc_registration_checks_outcome_check", stringsOf(RegistrationOutcomeValues()), true},
	} {
		for _, v := range tt.values {
			if tt.upperSnake && !regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*$`).MatchString(v) {
				t.Fatalf("%s: %q is not UPPER_SNAKE", tt.name, v)
			}
		}
		var def string
		must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid=$1::regclass AND conname=$2", "seller."+tt.table, tt.name).Scan(&def))
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
	// 數字的界限：DB 的 CHECK 與 Go 的常數是同一個值。通報佐證的鍵是 (通報, 第幾次送出, 次內位置)：次內位置 0 到 MaxEvidence-1（每次送出最多 5 張）、
	// 第幾次送出 0（原通報）到 MaxSupplements，補充的 seq 是 1 到 MaxSupplements。
	for _, tt := range []struct {
		table, name, column string
		lower, upper        int // 欄位 >= lower AND 欄位 <= upper
	}{
		{"report_evidence", "report_evidence_position_check", "position", 0, model.MaxEvidence - 1},
		{"report_evidence", "report_evidence_submission_check", "submission_seq", 0, model.MaxSupplements},
		{"report_supplements", "report_supplements_seq_check", "seq", 1, model.MaxSupplements},
		{"case_inquiry_reply_evidence", "case_inquiry_reply_evidence_position_check", "position", 0, model.MaxEvidence - 1},
	} {
		var def string
		must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid=$1::regclass AND conname=$2", "seller."+tt.table, tt.name).Scan(&def))
		bound := func(op string) int {
			m := regexp.MustCompile(`"?` + tt.column + `"? ` + op + ` (\d+)\b`).FindStringSubmatch(def)
			if m == nil {
				t.Fatalf("%s：%s 沒有 %s %s 的界限：%s", tt.table, tt.name, tt.column, op, def)
			}
			n, err := strconv.Atoi(m[1])
			must(t, err)
			return n
		}
		if lower, upper := bound(">="), bound("<="); lower != tt.lower || upper != tt.upper {
			t.Errorf("%s：DB 的界限是 [%d, %d]，Go 是 [%d, %d]：%s", tt.name, lower, upper, tt.lower, tt.upper, def)
		}
	}
}

// 登記資料比對的欄位：DB 的 kyc_registration_checks_fields_check 對 mismatched_fields 與 unverifiable_fields 各列一份允許的
// 欄位，兩份都要與 Go 的 RegistrationComparedFields 相同。
func TestRegistrationFieldsCheckMatchesGo(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var def string
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid=$1::regclass AND conname=$2", "seller.kyc_registration_checks", "kyc_registration_checks_fields_check").Scan(&def))
	want := slices.Clone(model.RegistrationComparedFields())
	slices.Sort(want)
	arrays := regexp.MustCompile(`ARRAY\[([^\]]*)\]`).FindAllStringSubmatch(def, -1)
	if len(arrays) != 2 {
		t.Fatalf("預期 mismatched_fields 與 unverifiable_fields 各一份 ARRAY，實際 %d 份：%s", len(arrays), def)
	}
	for _, a := range arrays {
		var got []string
		for _, m := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(a[1], -1) {
			got = append(got, m[1])
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Fatalf("DB=%v Go=%v：%s", got, want, def)
		}
	}
}

// 停權的效果以帳號為單位（W-A）：關店不解除停權，先前 stint 上未解除的停權讓開店失敗；已關閉的 stint 上的停權可以
// 解除，結束的停權期間與開始時同一個 PeriodID，之後才能重新開店。
func TestSuspensionFollowsTheAccountAcrossStints(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var suspended []StoreSuspended
	var unsuspended []StoreUnsuspended
	events.Subscribe(f.bus, func(_ context.Context, e StoreSuspended) error {
		suspended = append(suspended, e)
		return nil
	})
	events.Subscribe(f.bus, func(_ context.Context, e StoreUnsuspended) error {
		unsuspended = append(unsuspended, e)
		return nil
	})
	accountID, s := f.open(t)
	result := f.suspend(t, s.StoreID)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if _, err := f.module.Apply(ctx, validApply(accountID)); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("apply with an unlifted suspension on a closed stint: %v", err)
	}
	if f.isSeller(t, accountID) || f.count(t, "SELECT count(*) FROM seller.stores WHERE account_id=$1", accountID) != 1 {
		t.Fatal("apply wrote although the account is suspended")
	}
	_, err = f.lift(ctx, result)
	must(t, err)
	if len(suspended) != 1 || len(unsuspended) != 1 || suspended[0].PeriodID != unsuspended[0].PeriodID || unsuspended[0].StoreID != s.StoreID {
		t.Fatalf("suspended=%+v unsuspended=%+v", suspended, unsuspended)
	}
	second, err := f.module.Apply(ctx, validApply(accountID))
	must(t, err)
	if second.Suspended() {
		t.Fatalf("second=%+v", second)
	}
}

// 能力由生命週期與停權推導（W-B）：停權時不能改店面（PermissionDenied），等待刪除時不能做經營類的事、也不能自己關店
// （FailedPrecondition），履約類的能力照樣有。
func TestCapabilitiesGateStoreWrites(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	f.suspend(t, s.StoreID)
	if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, Prize: ptr("金牌")}); apperr.KindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("suspended profile write: %v", err)
	}
	if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, LogoToken: "x", UploaderID: accountID}); apperr.KindOf(err) != apperr.KindPermissionDenied || len(f.media.finalized) != 0 {
		t.Fatalf("suspended logo write: %v finalized=%d", err, len(f.media.finalized))
	}
	if _, err := f.module.ActingStore(ctx, accountID, CapOrderFulfill); err != nil {
		t.Fatalf("suspended seller must still fulfil: %v", err)
	}

	pendingAccount, pending := f.open(t)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error {
		if err := f.identity.LockAccount(ctx, pendingAccount); err != nil {
			return err
		}
		return f.module.BeginDeletion(ctx, pending.StoreID)
	}))
	if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: pending.StoreID, ExpectedVersion: pending.Version, Prize: ptr("金牌")}); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("pending profile write: %v", err)
	}
	for _, c := range []Capability{CapProductManage, CapStorefrontWrite, CapMarketing, CapInvoiceManage, CapAccountClose} {
		if _, err := f.module.ActingStore(ctx, pendingAccount, c); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("pending store got %s: %v", c, err)
		}
	}
	for _, c := range []Capability{CapOrderFulfill, CapOrderContract, CapAccountSelfRead, CapChatSend} {
		if _, err := f.module.ActingStore(ctx, pendingAccount, c); err != nil {
			t.Fatalf("pending store lost %s: %v", c, err)
		}
	}
	if _, err := f.module.CloseCurrent(ctx, pendingAccount); apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("owner closes a pending store: %v", err)
	}
}

// 合併後沒有任何改變的更新不寫入：版本不變、不發事件；過期的版本照樣衝突（審查 CR-F4）。
func TestUpdateProfileWithoutChangesIsANoOp(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var changed int
	events.Subscribe(f.bus, func(context.Context, StoreProfileChanged) error {
		changed++
		return nil
	})
	_, s := f.open(t)
	for _, in := range []UpdateProfileInput{{}, {StoreName: ptr(s.StoreName), TownCode: ptr("63000050")}} {
		in.StoreID, in.ExpectedVersion = s.StoreID, s.Version
		same, err := f.module.UpdateProfile(ctx, in)
		must(t, err)
		if same.Version != s.Version {
			t.Fatalf("no-op bumped the version: %+v", same)
		}
	}
	if changed != 0 {
		t.Fatalf("no-op emitted %d events", changed)
	}
	if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version + 1}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("stale version: %v", err)
	}
}

// 轉正之後寫入失敗（這裡是版本衝突）：轉正沒有建立引用，所以沒有補償路徑，也沒有任何引用或排入的同步 job。
func TestImageFailureAfterFinalizeLeavesNoReference(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountID, s := f.open(t)
	f.media.onFinalize = func() {
		if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, Prize: ptr("金牌")}); err != nil {
			t.Error(err)
		}
	}
	f.jobs.args = nil
	if _, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, LogoToken: "logo1", UploaderID: accountID}); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("err=%v", err)
	}
	if kinds := f.jobs.kinds(); slices.Contains(kinds, "seller.media_sync") {
		t.Fatalf("失敗的寫入不該排入同步：%v", kinds)
	}
	if got := f.media.Referenced(f.storeRef(media.RefTypeSellerLogo, s.StoreID)); len(got) != 0 {
		t.Fatalf("引用 = %v", got)
	}
	current, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	if current.LogoKey != nil {
		t.Fatalf("logo written: %+v", current)
	}
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// kycApprovedStore 開店並讓 KYC 核准。
func (f *fixture) kycApprovedStore(t *testing.T, idCard, bank string) Store {
	t.Helper()
	_, s := f.approvedWith(t, idCard, bank)
	return s
}

func (f *fixture) kycApprovedStoreWithAccount(t *testing.T, idCard, bank string) (int64, Store) {
	t.Helper()
	return f.approvedWith(t, idCard, bank)
}
