//go:build integration

package buyer_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	. "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(module *testing.M) { testdb.Main(module, &pg) }

type queue struct {
	mu   sync.Mutex
	args []river.JobArgs
	opts []*river.InsertOpts
	fail bool
	// outsideOK 允許在 transaction 外排入（保底掃描沒有業務寫入，不在 transaction 內）。
	outsideOK bool
}

func (queue *queue) Enqueue(ctx context.Context, first river.JobArgs, opts *river.InsertOpts) error {
	if _, ok := db.TxFrom(ctx); !ok && !queue.outsideOK {
		return errors.New("enqueue outside transaction")
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if queue.fail {
		return errors.New("queue failed")
	}
	queue.args = append(queue.args, first)
	queue.opts = append(queue.opts, opts)
	return nil
}

type fixture struct {
	database *db.DB
	module   *Module
	bus      *events.Bus
	jobs     *queue
	media    *media.Fake
	now      time.Time
}

func setup(t *testing.T) *fixture {
	t.Helper()
	database := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "buyer", Dir: Migrations()}))
	fixture := &fixture{
		database: database,
		bus:      events.NewBus(),
		jobs:     &queue{},
		now:      time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC),
	}
	fixture.media = media.NewFake()
	fixture.media.Key = func(_, token string) string { return "avatar/" + token }
	fixture.module = New(Deps{
		DB:             database,
		Events:         fixture.bus,
		Jobs:           fixture.jobs,
		Media:          fixture.media,
		Now:            func() time.Time { return fixture.now },
		NicknameNumber: func() int { return 42 },
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	return fixture
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func profile(t *testing.T, fixture *fixture, id int64) Profile {
	t.Helper()
	must(t, fixture.module.OnAccountCreated(context.Background(), identity.AccountCreated{AccountID: id, Name: "測試買家"}))
	buyer, err := fixture.module.Profile(context.Background(), id)
	must(t, err)
	return buyer
}

func count(t *testing.T, fixture *fixture, query string, args ...any) int64 {
	t.Helper()
	var total int64
	must(t, fixture.database.Conn(context.Background()).QueryRow(context.Background(), query, args...).Scan(&total))
	return total
}

func addressInput(buyerID int64) AddressInput {
	return AddressInput{
		BuyerID:       buyerID,
		ReceiverName:  "測試收件人",
		ReceiverPhone: "+886912345678",
		TownCode:      "63000050",
		AddressDetail: "測試路一號",
	}
}

// 一個帳號一生只有一個買家（W3）：建帳事件建立並授予一次；重送或關閉後再建都違反 buyers_account，
// 回錯誤讓建帳整筆回滾，不重讀、不建第二個 stint。
func TestAccountCreatedCreatesTheOnlyBuyer(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	must(t, fixture.module.OnAccountCreated(ctx, identity.AccountCreated{AccountID: 101, Name: "最初暱稱"}))
	buyer, err := fixture.module.Profile(ctx, 101)
	must(t, err)
	if buyer.DisplayName != "最初暱稱" || buyer.Status != Active {
		t.Fatalf("buyer=%+v", buyer)
	}
	if buyer.BuyerID == 101 {
		t.Fatalf("buyer_id reused account_id: %+v", buyer)
	}
	if err := fixture.module.OnAccountCreated(ctx, identity.AccountCreated{AccountID: 101, Name: "不可覆寫"}); err == nil {
		t.Fatal("second buyer for the same account was accepted")
	}
	must(t, fixture.module.CloseForDeletion(ctx, buyer.BuyerID))
	if err := fixture.module.OnAccountCreated(ctx, identity.AccountCreated{AccountID: 101, Name: "重新註冊"}); err == nil {
		t.Fatal("closed account got a new buyer")
	}
	if n := count(t, fixture, "SELECT count(*) FROM buyer.buyers WHERE account_id=101"); n != 1 {
		t.Fatalf("buyers for account=%d", n)
	}
	if _, err := fixture.module.CurrentBuyerID(ctx, 101); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("current buyer after close err=%v", err)
	}
	ds, err := fixture.module.BatchDisplay(ctx, []int64{buyer.BuyerID})
	must(t, err)
	if len(ds) != 1 || ds[0].DisplayName != nil || ds[0].Avatar != nil || !ds[0].Deleted {
		t.Fatal(ds)
	}
}

func TestIdempotentEvents(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	added, removed, followed := 0, 0, 0
	seen := map[uuid.UUID]bool{}
	check := func(ctx context.Context, id uuid.UUID) error {
		if _, ok := db.TxFrom(ctx); !ok {
			return errors.New("event outside transaction")
		}
		if seen[id] {
			return errors.New("duplicate event ID")
		}
		seen[id] = true
		return nil
	}
	events.Subscribe(fixture.bus, func(ctx context.Context, event FavoriteAdded) error {
		added++
		return check(ctx, event.EventID)
	})
	events.Subscribe(fixture.bus, func(ctx context.Context, event FavoriteRemoved) error {
		removed++
		return check(ctx, event.EventID)
	})
	events.Subscribe(fixture.bus, func(ctx context.Context, event SellerFollowed) error {
		followed++
		return check(ctx, event.EventID)
	})
	for range 2 {
		must(t, fixture.module.AddFavorite(ctx, buyer.BuyerID, 11))
		must(t, fixture.module.Follow(ctx, buyer.BuyerID, 21))
	}
	for range 2 {
		must(t, fixture.module.RemoveFavorite(ctx, buyer.BuyerID, 11))
	}
	if added != 1 || removed != 1 || followed != 1 {
		t.Fatalf("added=%d removed=%d followed=%d; duplicates must not publish", added, removed, followed)
	}
}

func TestDeletionLifecycle(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 7)
	_, err := fixture.module.AddAddress(ctx, addressInput(buyer.BuyerID))
	must(t, err)
	for _, id := range []int64{11, 12} {
		must(t, fixture.module.AddFavorite(ctx, buyer.BuyerID, id))
	}
	must(t, fixture.module.Follow(ctx, buyer.BuyerID, 21))
	must(t, fixture.module.RecordVisit(ctx, buyer.BuyerID, 11))
	must(t, fixture.module.SubscribeRestock(ctx, buyer.BuyerID, 11))
	removed := map[int64]bool{}
	deleted := 0
	events.Subscribe(fixture.bus, func(_ context.Context, event FavoriteRemoved) error {
		removed[event.ProductID] = true
		return nil
	})
	events.Subscribe(fixture.bus, func(_ context.Context, _ BuyerDeleted) error {
		deleted++
		return nil
	})
	must(t, fixture.module.BeginDeletion(ctx, buyer.BuyerID))
	buyer, err = fixture.module.Profile(ctx, 7)
	must(t, err)
	if buyer.Status != PendingDeletion || deleted != 0 {
		t.Fatal("pending closed the stint")
	}
	must(t, fixture.module.BeginDeletion(ctx, buyer.BuyerID))
	for _, fn := range []func() error{func() error { return fixture.module.AddFavorite(ctx, buyer.BuyerID, 13) }, func() error { return fixture.module.Follow(ctx, buyer.BuyerID, 22) }} {
		if err = fn(); apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatal(err)
		}
	}
	must(t, fixture.module.CloseForDeletion(ctx, buyer.BuyerID))
	must(t, fixture.module.CloseForDeletion(ctx, buyer.BuyerID))
	if deleted != 1 || len(removed) != 2 || !removed[11] || !removed[12] {
		t.Fatalf("deleted=%d favorite events=%v", deleted, removed)
	}
	for _, table := range []string{"shipping_addresses", "favorites", "seller_subscriptions", "visit_history"} {
		if count(t, fixture, "SELECT count(*) FROM buyer."+table+" WHERE buyer_id=$1", buyer.BuyerID) != 0 {
			t.Fatal("not cleared", table)
		}
	}
	if count(t, fixture, "SELECT count(*) FROM buyer.restock_subscriptions WHERE buyer_id=$1", buyer.BuyerID) != 0 {
		t.Fatal("restock active after close")
	}
	if len(fixture.jobs.args) != 1 || fixture.jobs.args[0].Kind() != "buyer.media_sync" {
		t.Fatal(fixture.jobs.args)
	}
	secondBuyer := profile(t, fixture, 8)
	must(t, fixture.module.CloseForDeletion(ctx, secondBuyer.BuyerID))
	if count(t, fixture, "SELECT count(*) FROM buyer.buyers WHERE buyer_id=$1 AND status='CLOSED'", secondBuyer.BuyerID) != 1 {
		t.Fatal("no-orders did not close")
	}
}

// 每個買家最多 MaxAddresses（20）筆地址（W4）：第 21 筆回 FailedPrecondition 且不寫入；刪掉一筆後可再新增。
// 預設頁大小就是上限，一頁即完整清單。
func TestAddressCap(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	var last Address
	for range MaxAddresses {
		address, err := fixture.module.AddAddress(ctx, addressInput(buyer.BuyerID))
		must(t, err)
		last = address
	}
	_, err := fixture.module.AddAddress(ctx, addressInput(buyer.BuyerID))
	if apperr.KindOf(err) != apperr.KindFailedPrecondition {
		t.Fatalf("address %d: err=%v, want failed precondition", MaxAddresses+1, err)
	}
	if n := count(t, fixture, "SELECT count(*) FROM buyer.shipping_addresses WHERE buyer_id=$1", buyer.BuyerID); n != MaxAddresses {
		t.Fatalf("addresses=%d", n)
	}
	page, err := fixture.module.PageAddresses(ctx, AddressListInput{BuyerID: buyer.BuyerID})
	must(t, err)
	if len(page.Items) != MaxAddresses || page.Next != nil {
		t.Fatalf("default page: items=%d next=%v", len(page.Items), page.Next)
	}
	must(t, fixture.module.DeleteAddress(ctx, buyer.BuyerID, last.AddressID, last.Version))
	_, err = fixture.module.AddAddress(ctx, addressInput(buyer.BuyerID))
	must(t, err)
}

func TestAddressDefaultsAndOwnership(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	other := profile(t, fixture, 2)
	first, err := fixture.module.AddAddress(ctx, addressInput(buyer.BuyerID))
	must(t, err)
	if !first.IsDefault || first.ReceiverPhone != "0912345678" {
		t.Fatal(first)
	}
	fixture.now = fixture.now.Add(time.Minute)
	second, err := fixture.module.AddAddress(ctx, addressInput(buyer.BuyerID))
	must(t, err)
	fixture.now = fixture.now.Add(time.Minute)
	constraint, err := fixture.module.AddAddress(ctx, addressInput(buyer.BuyerID))
	must(t, err)
	input := addressInput(buyer.BuyerID)
	input.AddressID = first.AddressID
	input.ExpectedVersion = first.Version
	input.IsDefault = false
	first, err = fixture.module.UpdateAddress(ctx, input)
	must(t, err)
	if !first.IsDefault {
		t.Fatal("unset sole default")
	}
	_, err = fixture.module.GetAddress(ctx, other.BuyerID, first.AddressID)
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatal(err)
	}
	input.BuyerID = other.BuyerID
	input.ExpectedVersion = first.Version
	_, err = fixture.module.UpdateAddress(ctx, input)
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatal(err)
	}
	if err = fixture.module.DeleteAddress(ctx, other.BuyerID, first.AddressID, 0); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatal(err)
	}
	_, err = fixture.database.Conn(ctx).Exec(ctx, "UPDATE buyer.shipping_addresses SET is_default=true WHERE address_id=$1", second.AddressID)
	if err == nil {
		t.Fatal("DB accepted two default addresses")
	}
	must(t, fixture.module.DeleteAddress(ctx, buyer.BuyerID, first.AddressID, 0))
	constraint, err = fixture.module.GetAddress(ctx, buyer.BuyerID, constraint.AddressID)
	must(t, err)
	if !constraint.IsDefault {
		t.Fatal("newest remaining not promoted")
	}
	must(t, fixture.module.DeleteAddress(ctx, buyer.BuyerID, constraint.AddressID, 0))
	second, err = fixture.module.GetAddress(ctx, buyer.BuyerID, second.AddressID)
	must(t, err)
	if !second.IsDefault {
		t.Fatal("last remaining not promoted")
	}
}

func TestPIICleanupStintsAndBacklog(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	old := profile(t, fixture, 1)
	phone, city, job := "0912345678", "臺北市", "測試職業"
	gender := Male
	birth := fixture.now.AddDate(-20, 0, 0)
	_, err := fixture.module.UpdateProfile(ctx, UpdateProfileInput{
		BuyerID:         old.BuyerID,
		ExpectedVersion: old.Version,
		DisplayName:     ptr("待清理個資"),
		Phone:           &phone,
		Gender:          &gender,
		BirthDate:       &birth,
		City:            &city,
		Job:             &job,
	})
	must(t, err)
	must(t, fixture.module.CloseForDeletion(ctx, old.BuyerID))
	fresh := profile(t, fixture, 200)
	for index := int64(2); index <= 102; index++ {
		closed := profile(t, fixture, index)
		must(t, fixture.module.CloseForDeletion(ctx, closed.BuyerID))
	}
	fixture.now = fixture.now.AddDate(0, 0, 31)
	must(t, fixture.module.CleanupPII(ctx))
	if count(t, fixture, "SELECT count(*) FROM buyer.buyers WHERE status='CLOSED' AND pii_cleared_at IS NOT NULL") != 102 {
		t.Fatal("cleanup left backlog beyond first batch")
	}
	if count(t, fixture, "SELECT count(*) FROM buyer.buyers WHERE buyer_id=$1 AND display_name IS NULL AND pii_cleared_at IS NOT NULL", old.BuyerID) != 1 {
		t.Fatal("old stint not cleared")
	}
	if count(t, fixture, "SELECT count(*) FROM buyer.buyers WHERE buyer_id=$1 AND phone IS NULL AND gender IS NULL AND avatar IS NULL AND birth_date IS NULL AND city IS NULL AND job IS NULL", old.BuyerID) != 1 {
		t.Fatal("closed stint PII remains")
	}
	buyer, err := fixture.module.Profile(ctx, 200)
	must(t, err)
	if buyer.BuyerID != fresh.BuyerID || buyer.DisplayName != "測試買家" || buyer.PIIClearedAt != nil {
		t.Fatalf("cleanup crossed stint boundary: %+v", buyer)
	}
	total := len(fixture.jobs.args)
	must(t, fixture.module.CleanupPII(ctx))
	if len(fixture.jobs.args) != total {
		t.Fatal("cleanup repeated completed work")
	}
}

func TestRestockFanOutNotifiesOnce(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	first := profile(t, fixture, 1)
	second := profile(t, fixture, 2)
	targeted := map[int64]int{}
	events.Subscribe(fixture.bus, func(_ context.Context, event RestockTargeted) error {
		targeted[event.BuyerID]++
		return nil
	})
	for _, id := range []int64{first.BuyerID, first.BuyerID, second.BuyerID} {
		must(t, fixture.module.SubscribeRestock(ctx, id, 11))
	}
	{
		_, err := fixture.module.FanOut(ctx, FanOutInput{ProductID: 11, Limit: 100, OccurredAt: fixture.now})
		must(t, err)
	}
	{
		_, err := fixture.module.FanOut(ctx, FanOutInput{ProductID: 11, Limit: 100, OccurredAt: fixture.now})
		must(t, err)
	}
	if targeted[first.BuyerID] != 1 || targeted[second.BuyerID] != 1 {
		t.Fatal(targeted)
	}
	checks, err := fixture.module.BatchCheckRestock(ctx, first.BuyerID, []int64{11})
	must(t, err)
	if checks[11] {
		t.Fatal("notified still active")
	}
}

func TestMembershipIdempotency(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	other := profile(t, fixture, 2)
	var err error
	input := AssignMembershipInput{BuyerID: buyer.BuyerID, PlanCode: "basic", RequestID: "request-1"}
	first, err := fixture.module.AssignMembership(ctx, input)
	must(t, err)
	second, err := fixture.module.AssignMembership(ctx, input)
	must(t, err)
	if first != second {
		t.Fatal(first, second)
	}
	input.PlanCode = "premium"
	if _, err = fixture.module.AssignMembership(ctx, input); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatal("same key different plan accepted", err)
	}
	input.PlanCode = "basic"
	input.BuyerID = other.BuyerID
	if _, err = fixture.module.AssignMembership(ctx, input); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatal("same key different buyer accepted", err)
	}
	if count(t, fixture, "SELECT count(*) FROM buyer.memberships") != 1 {
		t.Fatal("idempotency added rows")
	}
	fixture.now = fixture.now.AddDate(0, 0, 31)
	ent, err := fixture.module.Entitlement(ctx, buyer.BuyerID)
	must(t, err)
	if ent.PlanCode != "free" || ent.PeriodStart.Day() != 28 {
		t.Fatal(ent)
	}
}

func TestAvatarOrder(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 98)
	token := "staged"
	updated, err := fixture.module.UpdateProfile(ctx, UpdateProfileInput{
		BuyerID:           buyer.BuyerID,
		ExpectedVersion:   buyer.Version,
		AvatarStagedToken: &token,
	})
	must(t, err)
	if updated.Avatar == nil || *updated.Avatar != "avatar/staged" {
		t.Fatal(updated)
	}
	if len(fixture.jobs.args) != 1 || fixture.jobs.args[0].Kind() != "buyer.media_sync" {
		t.Fatalf("reference registration must be a sync job: %v", fixture.jobs.args)
	}
	ref := media.Ref{Type: media.RefTypeBuyerAvatar, ID: fmt.Sprint(buyer.BuyerID)}
	if got := fixture.media.Referenced(ref); len(got) != 0 {
		t.Fatalf("a write must not create references: %v", got)
	}
	if _, err := fixture.module.SyncMedia(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if got := fixture.media.Referenced(ref); fmt.Sprint(got) != "[asset-staged]" {
		t.Fatalf("references after sync = %v", got)
	}
	must(t, fixture.database.InTx(ctx, func(ctx context.Context) error {
		_, err := fixture.module.UpdateProfile(ctx, UpdateProfileInput{
			BuyerID:           buyer.BuyerID,
			ExpectedVersion:   updated.Version,
			AvatarStagedToken: &token,
		})
		if apperr.KindOf(err) != apperr.KindFailedPrecondition {
			t.Fatalf("nested avatar=%v", err)
		}
		return nil
	}))
}

func TestEventAndJobFailureRollBack(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	events.Subscribe(fixture.bus, func(context.Context, FavoriteAdded) error { return errors.New("reject event") })
	if err := fixture.module.AddFavorite(ctx, buyer.BuyerID, 1); err == nil {
		t.Fatal("event rejection lost")
	}
	if count(t, fixture, "SELECT count(*) FROM buyer.favorites") != 0 {
		t.Fatal("favorite survived event failure")
	}
	fixture.jobs.fail = true
	if err := fixture.module.CloseForDeletion(ctx, buyer.BuyerID); err == nil {
		t.Fatal("job rejection lost")
	}
	buyer, err := fixture.module.Profile(ctx, 1)
	must(t, err)
	if buyer.Status != Active {
		t.Fatal("close survived job failure")
	}
}

func TestSchemaConstraints(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	_, err := fixture.module.AddAddress(ctx, addressInput(buyer.BuyerID))
	must(t, err)
	for _, sql := range []string{"UPDATE buyer.buyers SET gender='invalid' WHERE buyer_id=$1", "UPDATE buyer.buyers SET status='invalid' WHERE buyer_id=$1", "UPDATE buyer.buyers SET display_name='' WHERE buyer_id=$1", "UPDATE buyer.buyers SET display_name=repeat('名',101) WHERE buyer_id=$1", "UPDATE buyer.buyers SET display_name=NULL WHERE buyer_id=$1", "UPDATE buyer.buyers SET pii_cleared_at=now() WHERE buyer_id=$1"} {
		if _, err = fixture.database.Conn(ctx).Exec(ctx, sql, buyer.BuyerID); err == nil {
			t.Fatal("DB accepted invalid enum/invariant", sql)
		}
	}
	must(t, fixture.module.SubscribeRestock(ctx, buyer.BuyerID, 11))
	if _, err = fixture.database.Conn(ctx).Exec(ctx, "UPDATE buyer.restock_subscriptions SET status='bad' WHERE buyer_id=$1", buyer.BuyerID); err == nil {
		t.Fatal("invalid restock status")
	}
}

func TestKeysetPaginationAndVisits(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	views := 0
	events.Subscribe(fixture.bus, func(context.Context, ProductViewed) error {
		views++
		return nil
	})
	for _, id := range []int64{1, 2, 3} {
		must(t, fixture.module.AddFavorite(ctx, buyer.BuyerID, id))
		must(t, fixture.module.RecordVisit(ctx, buyer.BuyerID, id))
	}
	must(t, fixture.module.RecordVisit(ctx, buyer.BuyerID, 1))
	if views != 4 {
		t.Fatal(views)
	}
	page, err := fixture.module.ListFavorites(ctx, ListInput{BuyerID: buyer.BuyerID, PageSize: 2})
	must(t, err)
	if len(page.Items) != 2 || page.Items[0].ID != 3 || page.Next == nil {
		t.Fatal(page)
	}
	page, err = fixture.module.ListFavorites(ctx, ListInput{BuyerID: buyer.BuyerID, PageSize: 2, After: page.Next})
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].ID != 1 || page.Next != nil {
		t.Fatal(page)
	}
	must(t, fixture.module.ClearVisits(ctx, buyer.BuyerID))
	visits, err := fixture.module.ListVisits(ctx, ListInput{BuyerID: buyer.BuyerID})
	must(t, err)
	if len(visits.Items) != 0 {
		t.Fatal(visits)
	}
}

func stringsOf[T ~string](values []T) []string {
	out := make([]string, len(values))
	for index, value := range values {
		out[index] = string(value)
	}
	return out
}

func TestEnumChecksMatchGo(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	for _, tt := range []struct {
		table, name string
		values      []string
	}{
		{"buyers", "buyers_status_check", stringsOf(StatusValues())},
		{"buyers", "buyers_gender_check", stringsOf(GenderValues())},
		{"memberships", "memberships_source_check", stringsOf(MembershipSourceValues())},
		{"memberships", "memberships_status_check", stringsOf(MembershipStatusValues())},
	} {
		for _, value := range tt.values {
			if !regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*$`).MatchString(value) {
				t.Fatalf("%s: stored code %q is not UPPER_SNAKE", tt.name, value)
			}
		}
		var def string
		must(t, fixture.database.Conn(ctx).QueryRow(ctx, "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid=$1::regclass AND conname=$2", "buyer."+tt.table, tt.name).Scan(&def))
		matches := regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(def, -1)
		actual := make([]string, 0, len(matches))
		for _, module := range matches {
			actual = append(actual, module[1])
		}
		slices.Sort(actual)
		slices.Sort(tt.values)
		if !slices.Equal(actual, tt.values) {
			t.Fatalf("%s: DB=%v Go=%v", tt.name, actual, tt.values)
		}
	}
}

func TestMembershipWriteAndDBConstraints(t *testing.T) {
	t.Parallel()
	fixture := setup(t)
	ctx := context.Background()
	buyer := profile(t, fixture, 1)
	var err error
	zero := int32(0)
	negative := int64(-1)
	for _, input := range []UpdatePlanInput{{PlanCode: "free", MonthlyAnswerLimit: &zero}, {PlanCode: "free", MonthlyPriceTWD: &negative}, {PlanCode: "free"}} {
		if _, err = fixture.module.UpdatePlan(ctx, input); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatal("invalid plan accepted", err)
		}
	}
	if _, err := fixture.module.Plan(ctx, "missing"); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("missing plan=%v", err)
	}
	largePrice := int64(1 << 31)
	updated, err := fixture.module.UpdatePlan(ctx, UpdatePlanInput{PlanCode: "basic", MonthlyPriceTWD: &largePrice})
	if err != nil || updated.MonthlyPriceTWD != largePrice {
		t.Fatalf("price=%+v err=%v", updated, err)
	}
	read, err := fixture.module.Plan(ctx, "basic")
	if err != nil || read.MonthlyPriceTWD != largePrice {
		t.Fatalf("plan=%+v err=%v", read, err)
	}
	_, err = fixture.module.AssignMembership(ctx, AssignMembershipInput{BuyerID: buyer.BuyerID, PlanCode: "free", RequestID: "one"})
	must(t, err)
	for _, stmt := range []string{"UPDATE buyer.memberships SET source='invalid' WHERE buyer_id=$1", "UPDATE buyer.memberships SET source='SYSTEM_DEFAULT' WHERE buyer_id=$1", "UPDATE buyer.memberships SET request_id='' WHERE buyer_id=$1", "UPDATE buyer.memberships SET status='invalid' WHERE buyer_id=$1", "UPDATE buyer.memberships SET period_end=period_start WHERE buyer_id=$1"} {
		if _, err = fixture.database.Conn(ctx).Exec(ctx, stmt, buyer.BuyerID); err == nil {
			t.Fatal("DB accepted", stmt)
		}
	}
	_, err = fixture.database.Conn(ctx).Exec(ctx, "INSERT INTO buyer.memberships(id,buyer_id,plan_code,period_start,period_end,source,status,request_id,created_at) VALUES ($1,$2,'free',$3,$4,'ADMIN','ACTIVE','two',$3)", uuid.NewV7(), buyer.BuyerID, fixture.now, fixture.now.AddDate(0, 1, 0))
	if err == nil {
		t.Fatal("DB accepted two active memberships")
	}
	_, err = fixture.module.AssignMembership(ctx, AssignMembershipInput{BuyerID: buyer.BuyerID, PlanCode: "free", RequestID: "two"})
	must(t, err)
	if count(t, fixture, "SELECT count(*) FROM buyer.memberships WHERE buyer_id=$1 AND status='ACTIVE'", buyer.BuyerID) != 1 {
		t.Fatal("assignment did not supersede")
	}
}

func ptr[T any](value T) *T { return &value }
