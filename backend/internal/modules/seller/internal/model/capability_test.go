package model

import (
	"errors"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// adrWhitelist 與 businessCapabilities 是能力檢查 ADR 的結論，獨立寫死，用來核對表（needs）推導出來的結果。
var (
	adrWhitelist = []Capability{
		CapAccountSelfRead, CapOrderFulfill, CapOrderContract, CapChatSend, CapChatRead,
		CapNotification, CapContentContract, CapAccountClose, CapInvoiceTaxRemedy,
	}
	businessCapabilities = []Capability{CapStorefrontWrite, CapProductManage, CapMarketing, CapInvoiceManage, CapShippingSetup}
)

// conditionsOf 模擬讀店家時由 seller.store_requirement_flags 讀出的條件（SQL 的對應由 integration 的資料集測試鎖住）：生命週期由
// status 決定，KYC 由 kyc 決定，店名、logo 與鄉鎮市區都有。
func conditionsOf(status Status, kyc bool) Conditions {
	return Conditions{
		Derived: true, Active: status != Closed, NotPendingDeletion: status != PendingDeletion, KYCApproved: kyc,
		StoreName: true, Logo: true, TownCode: true,
	}
}

// fullStore 是公開曝光條件除了生命週期與停權之外都滿足（含 KYC 核准）的店家。
func fullStore(status Status, suspended bool) Store {
	return Store{Status: status, Sanctions: sanctionsOf(suspended), Conditions: conditionsOf(status, true)}
}

// withoutKYC 是同一個店家、KYC 流程沒有核准。
func withoutKYC(s Store) Store {
	s.Conditions.KYCApproved = false
	return s
}

func statusOf(t *testing.T, statuses []CapabilityStatus, c Capability) CapabilityStatus {
	t.Helper()
	for _, st := range statuses {
		if st.Capability == c {
			return st
		}
	}
	t.Fatalf("no status for %s in %v", c, statuses)
	return CapabilityStatus{}
}

// 每個能力恰好回報一次，依 AllCapabilities 的順序；啟用 ⇔ 沒有缺項；缺項依 Requirements 的順序且不重複。
func TestCapabilitiesOfReportsEveryCapabilityConsistently(t *testing.T) {
	stores := map[string]Store{
		"active": fullStore(Active, false), "suspended": fullStore(Active, true),
		"pending": fullStore(PendingDeletion, false), "pending suspended": fullStore(PendingDeletion, true),
		"closed": fullStore(Closed, false), "bare": {Status: Active}, "no kyc": withoutKYC(fullStore(Active, false)),
	}
	order := Requirements()
	for name, s := range stores {
		got := CapabilitiesOf(s)
		if len(got) != len(AllCapabilities()) {
			t.Fatalf("%s: %d statuses", name, len(got))
		}
		for i, st := range got {
			if st.Capability != AllCapabilities()[i] {
				t.Fatalf("%s: position %d is %s", name, i, st.Capability)
			}
			if st.Active != (len(st.Missing) == 0) {
				t.Fatalf("%s: %s active=%v missing=%v", name, st.Capability, st.Active, st.Missing)
			}
			if !slices.IsSortedFunc(st.Missing, func(a, b Requirement) int { return slices.Index(order, a) - slices.Index(order, b) }) {
				t.Fatalf("%s: %s missing %v is not in requirement order", name, st.Capability, st.Missing)
			}
		}
	}
}

// 店家營運中：動作類能力都啟用，物流建店要 KYC 核准。
func TestCapabilitiesOfShippingSetupNeedsAnApprovedKYCFlow(t *testing.T) {
	for _, kyc := range []bool{true, false} {
		s := fullStore(Active, false)
		if !kyc {
			s = withoutKYC(s)
		}
		got := CapabilitiesOf(s)
		for _, c := range Capabilities() {
			st := statusOf(t, got, c)
			if c == CapShippingSetup && !kyc {
				if st.Active || !slices.Equal(st.Missing, []Requirement{RequirementKYCApproved}) {
					t.Fatalf("shipping setup without KYC = %+v", st)
				}
				continue
			}
			if !st.Active {
				t.Fatalf("kyc %v: %s = %+v", kyc, c, st)
			}
		}
	}
}

// 停權時只剩能力檢查 ADR 的白名單，其餘缺 NOT_SUSPENDED；物流建店同時缺 KYC 時兩項都列。
func TestCapabilitiesOfSuspendedKeepsOnlyTheWhitelist(t *testing.T) {
	{
		s := fullStore(Active, true)
		got := CapabilitiesOf(s)
		active := ActiveCapabilities(got)
		want := slices.Clone(adrWhitelist)
		slices.Sort(active)
		slices.Sort(want)
		if !slices.Equal(active, want) {
			t.Fatalf("active=%v want the ADR whitelist %v", active, want)
		}
		for _, c := range Capabilities() {
			if st := statusOf(t, got, c); !st.Active && !slices.Equal(st.Missing, []Requirement{RequirementNotSuspended}) {
				t.Fatalf("%s missing=%v", c, st.Missing)
			}
		}
		if st := statusOf(t, CapabilitiesOf(withoutKYC(s)), CapShippingSetup); !slices.Equal(st.Missing, []Requirement{RequirementNotSuspended, RequirementKYCApproved}) {
			t.Fatalf("unapproved shipping setup missing=%v", st.Missing)
		}
	}
}

// 等待刪除時經營類（含物流建店）與關店缺 NOT_PENDING_DELETION，履約類照常。
func TestCapabilitiesOfPendingDeletionWithholdsBusinessCapabilitiesAndClose(t *testing.T) {
	s := fullStore(PendingDeletion, false)
	got := CapabilitiesOf(s)
	for _, c := range Capabilities() {
		st := statusOf(t, got, c)
		denied := slices.Contains(businessCapabilities, c) || c == CapAccountClose
		if denied && (st.Active || !slices.Equal(st.Missing, []Requirement{RequirementNotPendingDeletion})) {
			t.Fatalf("%s = %+v", c, st)
		}
		if !denied && !st.Active {
			t.Fatalf("%s = %+v", c, st)
		}
	}
	if st := statusOf(t, CapabilitiesOf(withoutKYC(s)), CapShippingSetup); !slices.Equal(st.Missing, []Requirement{RequirementKYCApproved, RequirementNotPendingDeletion}) {
		t.Fatalf("unapproved shipping setup missing=%v", st.Missing)
	}
	both := ActiveCapabilities(CapabilitiesOf(fullStore(PendingDeletion, true)))
	if slices.Contains(both, CapAccountClose) || !slices.Contains(both, CapOrderFulfill) {
		t.Fatalf("pending and suspended=%v", both)
	}
}

// 關閉的店家沒有任何能力。
func TestCapabilitiesOfClosedStoreHasNoCapability(t *testing.T) {
	for _, st := range CapabilitiesOf(fullStore(Closed, false)) {
		if st.Active || !slices.Equal(st.Missing[:1], []Requirement{RequirementActive}) {
			t.Fatalf("closed: %+v", st)
		}
	}
}

// PUBLIC_LISTING 缺的條件就是 MissingExposure，在每一種店家下都一樣；每一項條件都只讀 Conditions 的對應欄位（或停權）。
func TestPublicListingMirrorsMissingExposure(t *testing.T) {
	variants := []Store{
		fullStore(Active, false), fullStore(PendingDeletion, false), fullStore(Closed, false), withoutKYC(fullStore(Active, false)),
		fullStore(Active, true), {Status: Active, Sanctions: NoSanctions()}, {Status: PendingDeletion, Sanctions: sanctionsOf(true)},
	}
	for _, drop := range []func(*Conditions){
		func(c *Conditions) { c.StoreName = false }, func(c *Conditions) { c.Logo = false }, func(c *Conditions) { c.TownCode = false },
	} {
		s := fullStore(Active, false)
		drop(&s.Conditions)
		variants = append(variants, s)
	}
	for i, s := range variants {
		st := statusOf(t, CapabilitiesOf(s), CapPublicListing)
		want := MissingExposure(s)
		if !slices.Equal(st.Missing, want) || st.Active != (len(want) == 0) {
			t.Fatalf("store %d: status=%+v MissingExposure=%v", i, st, want)
		}
	}
	if got := MissingExposure(variants[len(variants)-1]); !slices.Equal(got, []Requirement{RequirementTownCode}) {
		t.Fatalf("missing town code: %v", got)
	}
}

// 狀態 PUBLIC_LISTING 不是動作：不在 Capabilities()（方法不能宣告）、不進 header 的啟用清單，Authorize 也不接受它。
func TestPublicListingIsAStatusNotAnAction(t *testing.T) {
	if slices.Contains(Capabilities(), CapPublicListing) || !slices.Contains(AllCapabilities(), CapPublicListing) {
		t.Fatal("PUBLIC_LISTING must be in AllCapabilities only")
	}
	approved := CapabilitiesOf(fullStore(Active, false))
	if !statusOf(t, approved, CapPublicListing).Active || slices.Contains(ActiveCapabilities(approved), CapPublicListing) {
		t.Fatalf("active capabilities leak the status: %v", ActiveCapabilities(approved))
	}
	if err := Authorize(fullStore(Active, false), CapPublicListing); err == nil {
		t.Fatal("PUBLIC_LISTING authorized")
	}
	if err := Authorize(fullStore(Active, false), Capability("CAP_NOPE")); err == nil {
		t.Fatal("unknown capability authorized")
	}
}

// Store.Can 與 CapabilitiesOf 是同一個推導：每個動作類能力在每一種店家下都一致（含取決於 KYC 的物流建店）。
func TestStoreCanAgreesWithCapabilitiesOf(t *testing.T) {
	for _, s := range []Store{
		fullStore(Active, false), fullStore(Active, true), fullStore(PendingDeletion, false), fullStore(Closed, false),
		withoutKYC(fullStore(Active, false)), {Status: Active, Sanctions: NoSanctions()},
	} {
		got := CapabilitiesOf(s)
		for _, c := range Capabilities() {
			if want := statusOf(t, got, c).Active; s.Can(c) != want {
				t.Fatalf("%+v/%s: Can=%v want %v", s.Conditions, c, s.Can(c), want)
			}
		}
	}
}

func errorOf(t *testing.T, err error) *apperr.Error {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("not an apperr: %v", err)
	}
	return ae
}

// 停權的拒絕是 PermissionDenied＋SELLER_SUSPENDED；前置條件（等待刪除、KYC 未核准）的拒絕是 FailedPrecondition＋
// SELLER_REQUIREMENT_MISSING，metadata 帶缺的條件（值名，依列舉的數字由小到大）；已關閉是 NotFound，沒有業務原因。
func TestAuthorizeDenialsCarryTheirReason(t *testing.T) {
	{
		ae := errorOf(t, Authorize(fullStore(Active, true), CapStorefrontWrite))
		if ae.Kind() != apperr.KindPermissionDenied || ae.Reason != apperr.ReasonSellerSuspended || len(ae.Metadata) != 0 {
			t.Fatalf("%+v", ae)
		}
	}
	// 停權又沒有 KYC：停權優先。
	if ae := errorOf(t, Authorize(withoutKYC(fullStore(Active, true)), CapShippingSetup)); ae.Reason != apperr.ReasonSellerSuspended {
		t.Fatalf("suspended shipping setup: %+v", ae)
	}
	cases := []struct {
		name string
		s    Store
		c    Capability
		want string
	}{
		{"pending deletion", fullStore(PendingDeletion, false), CapStorefrontWrite, "NOT_PENDING_DELETION"},
		{"pending deletion close", fullStore(PendingDeletion, false), CapAccountClose, "NOT_PENDING_DELETION"},
		{"shipping without KYC", withoutKYC(fullStore(Active, false)), CapShippingSetup, "KYC_APPROVED"},
		{"shipping, pending deletion, without KYC", withoutKYC(fullStore(PendingDeletion, false)), CapShippingSetup, "KYC_APPROVED,NOT_PENDING_DELETION"},
		{"shipping, pending deletion, with KYC", fullStore(PendingDeletion, false), CapShippingSetup, "NOT_PENDING_DELETION"},
	}
	for _, tc := range cases {
		ae := errorOf(t, Authorize(tc.s, tc.c))
		if ae.Kind() != apperr.KindFailedPrecondition || ae.Reason != apperr.ReasonSellerRequirementMissing || ae.Metadata[MissingRequirementsMetadataKey] != tc.want {
			t.Errorf("%s: kind=%v reason=%v metadata=%v want %q", tc.name, ae.Kind(), ae.Reason, ae.Metadata, tc.want)
		}
	}
	if err := Authorize(fullStore(Active, false), CapShippingSetup); err != nil {
		t.Fatalf("approved shipping setup: %v", err)
	}
	if ae := errorOf(t, Authorize(fullStore(Closed, true), CapStorefrontWrite)); ae.Kind() != apperr.KindNotFound || ae.Reason != (apperr.Reason{}) {
		t.Fatalf("closed: %+v", ae)
	}
}

// 條件沒有推導過（沒有經 stint 讀出）的店家：Authorize 拒絕（不是 apperr，是程式錯誤），CapabilitiesOf 與 Can 一項都不啟用。
func TestStoreWithoutDerivedConditionsHasNoCapability(t *testing.T) {
	s := Store{Status: Active, Sanctions: NoSanctions()}
	err := Authorize(s, CapOrderFulfill)
	var ae *apperr.Error
	if err == nil || errors.As(err, &ae) {
		t.Fatalf("Authorize without conditions = %v", err)
	}
	for _, st := range CapabilitiesOf(s) {
		want := slices.DeleteFunc(Needs(st.Capability), func(r Requirement) bool { return r == RequirementNotSuspended })
		if st.Active || !slices.Equal(st.Missing, want) {
			t.Fatalf("%s = %+v", st.Capability, st)
		}
	}
	if s.Can(CapOrderFulfill) {
		t.Fatal("Can without conditions")
	}
}

// 停權白名單由 needs 表推導，結果等於 ADR 的結論；需要 KYC 的只有物流建店與公開曝光；公開曝光需要全部條件。
func TestNeedsTableDerivesTheWhitelistAndKYCDependency(t *testing.T) {
	got, want := GrantedWhenSuspended(), slices.Clone(adrWhitelist)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("whitelist=%v want %v", got, want)
	}
	for _, c := range AllCapabilities() {
		if want, got := c == CapShippingSetup || c == CapPublicListing, slices.Contains(Needs(c), RequirementKYCApproved); got != want {
			t.Errorf("%s needs KYC_APPROVED=%v", c, got)
		}
		if len(Needs(c)) == 0 || Needs(c)[0] != RequirementActive {
			t.Errorf("%s needs %v: every capability needs STORE_ACTIVE", c, Needs(c))
		}
	}
	if !slices.Equal(Needs(CapPublicListing), Requirements()) {
		t.Fatalf("PUBLIC_LISTING needs %v", Needs(CapPublicListing))
	}
	if len(needs) != len(AllCapabilities()) {
		t.Fatalf("needs has %d entries for %d capabilities", len(needs), len(AllCapabilities()))
	}
}

// Store.Can 不接受非動作類能力：panic，不回一個沒根據的答案。
func TestStoreCanRejectsCapabilitiesItCannotAnswer(t *testing.T) {
	for _, c := range []Capability{CapPublicListing, Capability("CAP_NOPE")} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Can(%s) did not panic", c)
				}
			}()
			fullStore(Active, false).Can(c)
		}()
	}
}

// 沒有經過停權推導的 Store（Sanctions 是 nil）視為停權：不授予任何需要 NOT_SUSPENDED 的能力、不可曝光。
func TestStoreWithoutDerivedSuspensionIsTreatedAsSuspended(t *testing.T) {
	s := fullStore(Active, false)
	if s.Suspended() {
		t.Fatal("推導為沒有措施的店家不是停權")
	}
	s.Sanctions = nil
	if !s.Suspended() {
		t.Fatal("零值要視為停權")
	}
	got := CapabilitiesOf(s)
	active := ActiveCapabilities(got)
	want := slices.Clone(adrWhitelist)
	slices.Sort(active)
	slices.Sort(want)
	if !slices.Equal(active, want) {
		t.Fatalf("零值只剩白名單：active=%v want=%v", active, want)
	}
	if len(MissingExposure(s)) == 0 {
		t.Fatal("零值不可曝光")
	}
}

// sanctionsOf 是測試用的推導結果：suspended 時帶一筆生效中的措施。
func sanctionsOf(suspended bool) *Sanctions {
	if !suspended {
		return NoSanctions()
	}
	return SanctionedBy(EffectiveMeasure{Basis: BasisViolation})
}
