//go:build integration

package seller_test

import (
	"context"
	"slices"
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"
)

// exposable 建立一間符合全部曝光條件的店家：營運中、沒有停權、KYC 核准且 PayUni 核准、店名、logo、鄉鎮市區代碼齊全。
func (f *fixture) exposable(t *testing.T) (int64, Store) {
	t.Helper()
	accountID, s := f.boundStore(t)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(context.Background(), s.StoreID, PayUniSourcePoll))
	cur, err := f.module.Store(context.Background(), s.StoreID)
	must(t, err)
	s, err = f.module.UpdateProfile(context.Background(), UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, LogoToken: "logo", UploaderID: accountID})
	must(t, err)
	return accountID, s
}

func (f *fixture) exposed(t *testing.T, ids ...int64) []int64 {
	t.Helper()
	got, err := f.module.ExposableStores(context.Background(), ids)
	must(t, err)
	out := make([]int64, 0, len(got))
	for _, e := range got {
		out = append(out, e.StoreID)
	}
	return out
}

func TestExposableStoresReturnsAStoreMeetingEveryCondition(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.exposable(t)
	got, err := f.module.ExposableStores(context.Background(), []int64{s.StoreID})
	must(t, err)
	// 公開時間是寫入路徑（公告函式）記下的第一次可曝光的時間。
	if len(got) != 1 || got[0].StoreID != s.StoreID || !got[0].PublishedAt.Equal(f.now) {
		t.Fatalf("got=%+v, want PublishedAt %v", got, f.now)
	}
}

// logoed 補上 logo（曝光的最後一個條件，與核准無關）。
func (f *fixture) logoed(t *testing.T, accountID int64, s Store) {
	t.Helper()
	cur, err := f.module.Store(context.Background(), s.StoreID)
	must(t, err)
	_, err = f.module.UpdateProfile(context.Background(), UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, LogoToken: "logo", UploaderID: accountID})
	must(t, err)
}

func (f *fixture) publishedAt(t *testing.T, storeID int64) time.Time {
	t.Helper()
	got, err := f.module.ExposableStores(context.Background(), []int64{storeID})
	must(t, err)
	if len(got) != 1 {
		t.Fatalf("store %d not exposable: %+v", storeID, got)
	}
	return got[0].PublishedAt
}

// 公開時間是第一次可曝光的時間，不是核准時間：一月核准、三月才補齊 logo 的店，從三月起算；記下之後永久不變（之後被拒絕、
// 重新核准、時間流逝都不改它）。
func TestPublishedAtStartsWhenTheStoreFirstBecomesExposable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	accountID, s := f.boundStore(t)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	approved := f.now
	f.now = approved.Add(60 * 24 * time.Hour)
	f.reconcile(t)
	if at := f.firstExposedAt(t, s.StoreID); at != nil {
		t.Fatalf("還沒有 logo 不該被記下：%v", at)
	}
	f.logoed(t, accountID, s)
	logoAt := f.now
	if got := f.publishedAt(t, s.StoreID); !got.Equal(logoAt) || !got.After(approved) {
		t.Fatalf("PublishedAt=%v, want the logo time %v (approved %v)", got, logoAt, approved)
	}
	if at := f.firstExposedAt(t, s.StoreID); at == nil || !at.Equal(logoAt) {
		t.Fatalf("first_exposed_at=%v, want the write that made the store exposable (%v)", at, logoAt)
	}
	f.now = f.now.Add(time.Hour)
	f.reconcile(t)
	f.now = f.now.Add(10 * 24 * time.Hour)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "8"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourceRefresh))
	f.now = f.now.Add(24 * time.Hour)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourceRefresh))
	if got := f.publishedAt(t, s.StoreID); !got.Equal(logoAt) {
		t.Fatalf("PublishedAt=%v moved after reject and re-approval, want %v", got, logoAt)
	}
}

// 還沒被記下時間的可曝光店（時間造成的變化，對帳 job 還沒追上）是讀取當下；記下之後固定在記下的時間，不隨時間移動。寫入路徑
// 一律當場記下，所以這裡直接清掉時間來模擬那個空窗。
func TestPublishedAtIsTheReadTimeUntilStampedThenTheStamp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, s := f.exposable(t)
	_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET first_exposed_at = NULL WHERE store_id = $1", s.StoreID)
	must(t, err)
	first := f.now
	f.now = first.Add(5 * time.Minute)
	if got := f.publishedAt(t, s.StoreID); !got.Equal(f.now) {
		t.Fatalf("unstamped PublishedAt=%v, want the read time %v", got, f.now)
	}
	f.reconcile(t)
	stamped := f.now
	f.now = f.now.Add(24 * time.Hour)
	if got := f.publishedAt(t, s.StoreID); !got.Equal(stamped) {
		t.Fatalf("stamped PublishedAt=%v, want %v", got, stamped)
	}
}

func TestExposableStoresExcludesEachFailingCondition(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cases := map[string]func(t *testing.T, f *fixture) int64{
		"pending deletion": func(t *testing.T, f *fixture) int64 {
			_, s := f.exposable(t)
			must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, s.StoreID) }))
			return s.StoreID
		},
		"closed": func(t *testing.T, f *fixture) int64 {
			accountID, s := f.exposable(t)
			_, err := f.module.CloseCurrent(ctx, accountID)
			must(t, err)
			return s.StoreID
		},
		"admin suspension": func(t *testing.T, f *fixture) int64 {
			_, s := f.exposable(t)
			f.suspend(t, s.StoreID)
			return s.StoreID
		},
		"governance suspension": func(t *testing.T, f *fixture) int64 {
			_, s := f.exposable(t)
			ends := f.now.Add(30 * 24 * time.Hour)
			f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
			return s.StoreID
		},
		"kyc not approved": func(t *testing.T, f *fixture) int64 {
			accountID, s := f.open(t)
			d := f.savedDraft(t, kycDraft(accountID, idA, bankAcct))
			_, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
			must(t, err)
			cur, err := f.module.Store(ctx, s.StoreID)
			must(t, err)
			_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, LogoToken: "logo", UploaderID: accountID})
			must(t, err)
			return s.StoreID
		},
		"no kyc application": func(t *testing.T, f *fixture) int64 {
			accountID, s := f.open(t)
			_, err := f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: s.Version, LogoToken: "logo", UploaderID: accountID})
			must(t, err)
			return s.StoreID
		},
		"kyc approved but payuni not approved": func(t *testing.T, f *fixture) int64 {
			accountID, s := f.boundStore(t) // 已綁定，PayUni 還沒有審核結果。
			cur, err := f.module.Store(ctx, s.StoreID)
			must(t, err)
			_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, LogoToken: "logo", UploaderID: accountID})
			must(t, err)
			return s.StoreID
		},
		"missing logo": func(t *testing.T, f *fixture) int64 {
			_, s := f.boundStore(t)
			f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "1"}
			must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
			return s.StoreID
		},
		"missing town code": func(t *testing.T, f *fixture) int64 {
			// town_code 與 address_detail 一起清掉（address_detail 需要 town_code）。
			_, s := f.exposable(t)
			// 有 PayUni 註冊的店家不能經 UpdateProfile 清掉地址：直接清，再以別的欄位的寫入重新計算公告。
			_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET town_code = NULL, address_detail = NULL WHERE store_id = $1", s.StoreID)
			must(t, err)
			cur, err := f.module.Store(ctx, s.StoreID)
			must(t, err)
			_, err = f.module.UpdateProfile(ctx, UpdateProfileInput{StoreID: s.StoreID, ExpectedVersion: cur.Version, Prize: ptr("優良農民")})
			must(t, err)
			return s.StoreID
		},
	}
	for name, setupCase := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			_, healthy := f.exposable(t)
			failing := setupCase(t, f)
			if got := f.exposed(t, healthy.StoreID, failing); !slices.Equal(got, []int64{healthy.StoreID}) {
				t.Fatalf("exposed=%v, want only the healthy store %d (failing store %d)", got, healthy.StoreID, failing)
			}
		})
	}
}

func TestExposableAgainAfterTheSuspensionEndsOrIsLifted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, s := f.exposable(t)
	ends := f.now.Add(30 * 24 * time.Hour)
	f.violate(t, s.StoreID, SeverityHigh, MeasureSuspension, &ends)
	if got := f.exposed(t, s.StoreID); len(got) != 0 {
		t.Fatalf("suspended store exposed: %v", got)
	}
	// 到期不必等任何 job：推導以注入的 clock 為準，ends_at 當下不生效。
	f.now = ends
	if got := f.exposed(t, s.StoreID); !slices.Equal(got, []int64{s.StoreID}) {
		t.Fatalf("expired suspension still hides the store: %v", got)
	}
	f.now = f.now.Add(time.Hour)
	admin := f.suspend(t, s.StoreID)
	if got := f.exposed(t, s.StoreID); len(got) != 0 {
		t.Fatalf("admin-suspended store exposed: %v", got)
	}
	_, err := f.lift(ctx, admin)
	must(t, err)
	if got := f.exposed(t, s.StoreID); !slices.Equal(got, []int64{s.StoreID}) {
		t.Fatalf("lifted suspension still hides the store: %v", got)
	}
}

// 關閉的 stint 即使處分還生效中也不可曝光。
func TestClosedStintWithAnActiveSuspensionIsNotExposable(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, first := f.exposable(t)
	f.suspend(t, first.StoreID)
	_, err := f.module.CloseCurrent(context.Background(), accountID)
	must(t, err)
	if got := f.exposed(t, first.StoreID); len(got) != 0 {
		t.Fatalf("closed stint exposed: %v", got)
	}
}

func TestExposableStoresIgnoresUnknownAndDuplicateIDsAndKeepsInputOrder(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, a := f.exposable(t)
	_, b := f.exposable(t)
	got := f.exposed(t, b.StoreID, 999999, a.StoreID, b.StoreID)
	if !slices.Equal(got, []int64{b.StoreID, a.StoreID}) {
		t.Fatalf("exposed=%v, want [%d %d]", got, b.StoreID, a.StoreID)
	}
	if got := f.exposed(t, 999999); len(got) != 0 {
		t.Fatalf("unknown id exposed: %v", got)
	}
	if got := f.exposed(t); len(got) != 0 {
		t.Fatalf("empty input: %v", got)
	}
}

func TestExposableStoresValidatesTheBatch(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	for name, ids := range map[string][]int64{
		"zero id":     {1, 0},
		"negative id": {-5},
		"too many":    make([]int64, MaxBatchSize+1),
	} {
		if _, err := f.module.ExposableStores(ctx, ids); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	full := make([]int64, MaxBatchSize)
	for i := range full {
		full[i] = int64(i + 1)
	}
	if _, err := f.module.ExposableStores(ctx, full); err != nil {
		t.Fatalf("a full batch must be accepted: %v", err)
	}
}

// CapabilitiesOf 的 PUBLIC_LISTING 與 ExposableStores 是同一條規則：缺少的條件為空 ⇔ 店家在可曝光的清單裡，而且每個狀態都回報對的條件。
func TestPublicListingAgreesWithExposableStores(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	statuses := func(s Store) []CapabilityStatus {
		t.Helper()
		cur, err := f.module.Store(ctx, s.StoreID)
		must(t, err)
		got := CapabilitiesOf(cur)
		return got
	}
	missing := func(s Store) []Requirement {
		t.Helper()
		var listing CapabilityStatus
		for _, st := range statuses(s) {
			if st.Capability == CapPublicListing {
				listing = st
			}
		}
		if exposed := slices.Contains(f.exposed(t, s.StoreID), s.StoreID); exposed != (len(listing.Missing) == 0) || exposed != listing.Active {
			t.Fatalf("public listing=%+v but exposable=%v", listing, exposed)
		}
		return listing.Missing
	}
	// 新店：沒有 KYC 申請（NOT_STARTED）也沒有 logo，店名與鄉鎮市區代碼已經有了。
	_, fresh := f.open(t)
	if got := missing(fresh); !slices.Equal(got, []Requirement{RequirementKYCApproved, RequirementLogo}) {
		t.Fatalf("fresh store missing=%v", got)
	}
	_, s := f.exposable(t)
	if got := missing(s); len(got) != 0 {
		t.Fatalf("exposable store missing=%v", got)
	}
	f.suspend(t, s.StoreID)
	if got := missing(s); !slices.Equal(got, []Requirement{RequirementNotSuspended}) {
		t.Fatalf("suspended store missing=%v", got)
	}
}

func TestPublicListingReportsAStoreThatIsWaitingForDeletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	_, s := f.exposable(t)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, s.StoreID) }))
	cur, err := f.module.Store(ctx, s.StoreID)
	must(t, err)
	got := CapabilitiesOf(cur)
	for _, st := range got {
		switch {
		case st.Capability == CapPublicListing:
			if st.Active || !slices.Equal(st.Missing, []Requirement{RequirementNotPendingDeletion}) {
				t.Fatalf("public listing=%+v", st)
			}
		case st.Capability == CapProductManage || st.Capability == CapShippingSetup || st.Capability == CapAccountClose:
			if st.Active || !slices.Equal(st.Missing, []Requirement{RequirementNotPendingDeletion}) {
				t.Fatalf("%s=%+v", st.Capability, st)
			}
		}
	}
}

// 物流建店的前置條件是 KYC 流程核准，Authorize（經 ActingStore）從資料庫讀流程：沒有核准回 FailedPrecondition，
// reason SELLER_REQUIREMENT_MISSING，metadata 帶 KYC_APPROVED；核准的店家通過；停權仍是 PermissionDenied。
func TestActingStoreRequiresAnApprovedKYCFlowForShippingSetup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	account, _ := f.open(t)
	_, err := f.module.ActingStore(ctx, account, CapShippingSetup)
	ae := apperrOf(t, err)
	if ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonSellerRequirementMissing || ae.Metadata[MissingRequirementsMetadataKey] != "KYC_APPROVED" {
		t.Fatalf("unapproved store: %+v", ae)
	}
	if _, err := f.module.ActingStore(ctx, account, CapProductManage); err != nil {
		t.Fatalf("product manage does not depend on KYC: %v", err)
	}
	approvedAccount, approved := f.exposable(t)
	if got, err := f.module.ActingStore(ctx, approvedAccount, CapShippingSetup); err != nil || got.StoreID != approved.StoreID {
		t.Fatalf("approved store: store=%+v err=%v", got, err)
	}
	f.suspend(t, approved.StoreID)
	if _, err := f.module.ActingStore(ctx, approvedAccount, CapShippingSetup); apperr.KindOf(err) != apperr.KindPermissionDenied {
		t.Fatalf("suspended store: %v", err)
	}
}
