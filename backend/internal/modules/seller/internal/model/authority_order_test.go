package model

import (
	"slices"
	"testing"
	"time"
	"uuid"
)

var authorityEpoch = time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)

func openOrder() AuthorityOrder {
	return AuthorityOrder{
		ID: uuid.NewV7(), Kind: AuthorityDataRequest, ReceivedAt: authorityEpoch, DueAt: authorityEpoch.Add(72 * time.Hour),
		HoldUntil: DefaultHoldUntil(authorityEpoch), RegisteredAt: authorityEpoch.Add(time.Minute), Version: 1,
	}
}

func TestAuthorityHoldIsOneHundredEightyFourDays(t *testing.T) {
	if AuthorityHold != 4416*time.Hour {
		t.Fatalf("AuthorityHold=%v", AuthorityHold)
	}
	if got := DefaultHoldUntil(authorityEpoch); !got.Equal(authorityEpoch.Add(4416 * time.Hour)) {
		t.Fatalf("DefaultHoldUntil=%v", got)
	}
}

func TestDefaultAuthorityDueByKind(t *testing.T) {
	want := map[AuthorityOrderKind]time.Duration{
		AuthorityRestrictProduct: 48 * time.Hour, AuthoritySuspendAccount: 48 * time.Hour, AuthorityDataRequest: 72 * time.Hour, AuthorityOther: 72 * time.Hour,
	}
	for _, k := range AuthorityOrderKindValues() {
		if got := DefaultAuthorityDue(k); got != want[k] {
			t.Errorf("%s：%v 預期 %v", k, got, want[k])
		}
	}
	if len(want) != len(AuthorityOrderKindValues()) {
		t.Fatal("每個種類都要有預設期限")
	}
}

// 只有暫停帳號與限制或移除商品會產生措施，措施的理由書才是告知。
func TestOnlySuspendAndRestrictCanBeNotifiedByMeasure(t *testing.T) {
	for _, k := range AuthorityOrderKindValues() {
		want := k == AuthoritySuspendAccount || k == AuthorityRestrictProduct
		if k.MeasureCanNotify() != want {
			t.Errorf("%s：%v", k, k.MeasureCanNotify())
		}
	}
}

// Q5 (b)：保存期限取目前與「提供之後 184 天」較晚的；只有晚於目前才延長。
func TestHoldAfterProvisionTakesTheLater(t *testing.T) {
	current := DefaultHoldUntil(authorityEpoch)
	for name, tt := range map[string]struct {
		provide time.Time
		want    time.Time
		extend  bool
	}{
		"提供時間就是收文": {authorityEpoch, current, false},
		"提供比收文晚":   {authorityEpoch.Add(3 * 24 * time.Hour), authorityEpoch.Add(3*24*time.Hour + AuthorityHold), true},
		"晚一微秒":     {authorityEpoch.Add(time.Microsecond), authorityEpoch.Add(time.Microsecond + AuthorityHold), true},
	} {
		got, extended := HoldAfterProvision(current, tt.provide)
		if !got.Equal(tt.want) || extended != tt.extend {
			t.Errorf("%s：%v %v，預期 %v %v", name, got, extended, tt.want, tt.extend)
		}
	}
	far := current.Add(500 * 24 * time.Hour)
	if got, extended := HoldAfterProvision(far, authorityEpoch.Add(time.Hour)); !got.Equal(far) || extended {
		t.Fatalf("已延得更遠的不縮短：%v %v", got, extended)
	}
}

func TestAuthorityOrderStateAndAvailability(t *testing.T) {
	o := openOrder()
	if !o.Open() || o.State() != AuthorityStateOpen || len(o.Availability().Complete) != 0 || len(o.Availability().ExtendHold) != 0 {
		t.Fatalf("處理中：%+v", o.Availability())
	}
	o.Completed = &AuthorityCompletion{At: authorityEpoch.Add(time.Hour), AdminID: 1, Summary: "x"}
	a := o.Availability()
	if o.Open() || o.State() != AuthorityStateCompleted || len(a.Complete) != 1 || a.Complete[0] != AuthorityBlockCompleted {
		t.Fatalf("已完成：%+v", a)
	}
	if len(a.ExtendHold) != 0 {
		t.Fatal("已完成也能延長保存")
	}
}

func TestEveryAuthorityOrderBlockHasADistinctSentence(t *testing.T) {
	seen := map[string]AuthorityOrderBlock{}
	for _, b := range AuthorityOrderBlockValues() {
		msg := b.Text().Text
		if msg == "" {
			t.Errorf("AuthorityOrderBlock %s 沒有句子", b)
		}
		if prev, dup := seen[msg]; dup {
			t.Errorf("%s 與 %s 是同一句：%q", b, prev, msg)
		}
		seen[msg] = b
	}
	if AuthorityOrderBlock("UNKNOWN").Text().Text != "" {
		t.Error("不認得的原因不得有句子")
	}
}

// 保存的涵蓋：登記時已清理（快照）是「來不及」，與時間無關；其餘已清理的是保存期限已過之後清的，不論現在保存是否有效；沒清的看保存期限。
func TestStoreCoverageOf(t *testing.T) {
	o := openOrder()
	earlier, later := o.RegisteredAt.Add(-time.Second), o.RegisteredAt.Add(time.Hour)
	for name, tt := range map[string]struct {
		purged     *time.Time
		atRegister bool
		now        time.Time
		want       StoreCoverage
	}{
		"沒清、保存中": {nil, false, o.HoldUntil.Add(-time.Microsecond), CoverageHeld},
		"沒清、剛到期": {nil, false, o.HoldUntil, CoverageHoldEnded},
		"登記時已清理": {&earlier, true, o.RegisteredAt, CoveragePurgedBeforeRegistration},
		"登記時已清理、清理的時鐘比登記快":      {&later, true, o.RegisteredAt, CoveragePurgedBeforeRegistration},
		"登記時已清理、之後延長保存":         {&earlier, true, o.HoldUntil.Add(-time.Hour), CoveragePurgedBeforeRegistration},
		"登記之後清理（保存已過）":          {&later, false, o.HoldUntil.Add(time.Hour), CoveragePurgedAfterHoldEnded},
		"登記之後清理、之後延長使保存再度有效":    {&later, false, o.HoldUntil.Add(-time.Hour), CoveragePurgedAfterHoldEnded},
		"沒有快照、清理時間早於登記（時間不得決定）": {&earlier, false, o.RegisteredAt, CoveragePurgedAfterHoldEnded},
	} {
		if got := o.StoreCoverageOf(tt.purged, tt.atRegister, tt.now); got != tt.want {
			t.Errorf("%s：%s 預期 %s", name, got, tt.want)
		}
	}
}

// 撤銷登記之後保存解除：hold_until 可能還沒到，不能說「保存期限已過」（HoldEnded、PurgedAfterHoldEnded）；登記時已清理仍是登記當下的事實。
// mutation：拿掉 Revoked 的分支 → 保存期限未到的撤銷來函變成 HoldEnded 與 HoldActive 不一致，斷言失敗。
func TestStoreCoverageOfRevokedOrder(t *testing.T) {
	o := openOrder()
	at := o.RegisteredAt.Add(time.Minute)
	o.Revoked = &AuthorityRevocation{At: at, AdminID: 1, Reason: "x"}
	purged := o.RegisteredAt.Add(time.Hour)
	for name, tt := range map[string]struct {
		purged     *time.Time
		atRegister bool
		now        time.Time
		want       StoreCoverage
	}{
		"保存期限未到、沒清":   {nil, false, o.HoldUntil.Add(-time.Hour), CoverageHoldReleasedByRevocation},
		"保存期限已過、沒清":   {nil, false, o.HoldUntil.Add(time.Hour), CoverageHoldReleasedByRevocation},
		"撤銷後清理（期限未到）": {&purged, false, o.HoldUntil.Add(-time.Hour), CoverageHoldReleasedByRevocation},
		"登記時已清理":      {&purged, true, o.HoldUntil.Add(-time.Hour), CoveragePurgedBeforeRegistration},
	} {
		if got := o.StoreCoverageOf(tt.purged, tt.atRegister, tt.now); got != tt.want {
			t.Errorf("%s：%s 預期 %s", name, got, tt.want)
		}
		if o.HoldActive(tt.now) {
			t.Errorf("%s：撤銷的來函不得保存中", name)
		}
	}
}

func TestAuthorityTargetNames(t *testing.T) {
	store := int64(5)
	whole := AuthorityOrderTarget{AccountID: 1}
	one := AuthorityOrderTarget{AccountID: 1, StoreID: &store}
	if !whole.Names(1, 5) || !whole.Names(1, 6) || whole.Names(2, 5) {
		t.Fatal("不指定店家：帳號的任何店家都算")
	}
	if !one.Names(1, 5) || one.Names(1, 6) || one.Names(2, 5) {
		t.Fatal("指定店家：只算那一家")
	}
}

// 可用性逐格：處理中可以完成、延長、撤銷；已完成不能完成也不能撤銷、可以延長；已撤銷登記全部不行（原因是 REVOKED，先於 COMPLETED）；
// 記下告知只對暫不告知的對象，撤銷登記之後一律不行。保存只在沒有撤銷登記而且期限未到時有效；撤銷登記的來函不能建立措施。
func TestAuthorityOrderAvailabilityCells(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	recheck := now
	withheld := &AuthorityNotice{Kind: AuthorityNoticeWithheld, RecheckOn: &recheck}
	notified := &AuthorityNotice{Kind: AuthorityNoticeNotified}
	base := AuthorityOrder{HoldUntil: now.Add(time.Hour), Targets: []AuthorityOrderTarget{{Notice: withheld}, {Notice: notified}, {}}}
	completed := base
	completed.Completed = &AuthorityCompletion{At: now}
	revoked := base
	revoked.Revoked = &AuthorityRevocation{At: now, AdminID: 1, Reason: "x"}
	none := []AuthorityOrderBlock(nil)
	for name, c := range map[string]struct {
		o                        AuthorityOrder
		complete, extend, revoke []AuthorityOrderBlock
		notice                   [][]AuthorityOrderBlock
		hold                     bool
	}{
		"處理中": {base, none, none, none, [][]AuthorityOrderBlock{nil, {AuthorityBlockNoticeNotWithheld}, {AuthorityBlockNoticeNotWithheld}}, true},
		"已完成": {completed, []AuthorityOrderBlock{AuthorityBlockCompleted}, none, []AuthorityOrderBlock{AuthorityBlockCompleted},
			[][]AuthorityOrderBlock{nil, {AuthorityBlockNoticeNotWithheld}, {AuthorityBlockNoticeNotWithheld}}, true},
		"已撤銷登記": {revoked, []AuthorityOrderBlock{AuthorityBlockRevoked}, []AuthorityOrderBlock{AuthorityBlockRevoked}, []AuthorityOrderBlock{AuthorityBlockRevoked},
			[][]AuthorityOrderBlock{{AuthorityBlockRevoked}, {AuthorityBlockRevoked}, {AuthorityBlockRevoked}}, false},
	} {
		a := c.o.Availability()
		if !slices.Equal(a.Complete, c.complete) || !slices.Equal(a.ExtendHold, c.extend) || !slices.Equal(a.Revoke, c.revoke) {
			t.Errorf("%s：complete=%v extend=%v revoke=%v", name, a.Complete, a.ExtendHold, a.Revoke)
		}
		for i := range c.notice {
			if !slices.Equal(a.RecordNotice[i], c.notice[i]) {
				t.Errorf("%s：對象 %d 的記下告知 %v，期望 %v", name, i, a.RecordNotice[i], c.notice[i])
			}
		}
		if c.o.HoldActive(now) != c.hold {
			t.Errorf("%s：hold=%v", name, c.o.HoldActive(now))
		}
		for _, b := range append(append(a.Complete, a.Revoke...), a.RecordNotice[1]...) {
			if b.Text().Text == "" {
				t.Errorf("%s：原因 %s 沒有句子", name, b)
			}
		}
	}
	st := AuthorityTargetStore{Target: TerminationTarget{Closed: true, LatestOfAccount: true}}
	if base.CanCreateMeasureOn(st) || !base.CanCreateTerminationOn(st) || revoked.CanCreateTerminationOn(st) {
		t.Fatal("關閉而且是最近的 stint：不能建立一般措施、可以建立終止；撤銷登記的來函都不行")
	}
	if open := (AuthorityTargetStore{}); !base.CanCreateMeasureOn(open) || revoked.CanCreateMeasureOn(open) {
		t.Fatal("開著的店可以建立措施；撤銷登記的來函不行")
	}
	if base.State() != AuthorityStateOpen || completed.State() != AuthorityStateCompleted || revoked.State() != AuthorityStateRevoked || revoked.Open() {
		t.Fatal("狀態")
	}
}
