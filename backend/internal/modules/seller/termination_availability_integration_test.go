//go:build integration

package seller_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// 終止合作的可用性由伺服器推導（StoreWithActiveMeasures.TerminationGrounds），與 CreateMeasure 的前置條件是同一份規則：每個帳號狀態、每個事由，
// 「可用清單含該事由」⇔「CreateMeasure 成功」，而且清單等於手算的期望值（測試不依賴被測的函式）。MATERIAL_BREACH 另帶一筆已存在 7 日以上的認定。
// 關聯帳號不在清單內（要來源措施，見 TestLinkedTerminationAvailabilityIsTheSameRuleAsCreateMeasure）。
// mutation：BlockFor 不擋已關閉店家的預告終止 → 「已關閉」各列失敗；BlockFor 不看 LatestOfAccount → 舊 stint 那列失敗；StoreWithActive 把 Closed 固定為 false → 已關閉各列失敗。
func TestTerminationAvailabilityIsTheSameRuleAsCreateMeasure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	var every, withoutNotice []TerminationGround
	for _, g := range TerminationGroundValues() {
		if g != GroundLinkedAccount {
			every = append(every, g)
		}
		if g != GroundLinkedAccount && g != GroundNotice {
			withoutNotice = append(withoutNotice, g)
		}
	}
	// 每個狀態先在開著的店留下一筆認定（之後 MATERIAL_BREACH 引用它），再走到要驗的狀態。
	finding := func(storeID int64) uuid.UUID {
		warn := f.violate(t, storeID, SeverityLow, MeasureWarning, nil)
		return warn.Findings[0].ID
	}
	type scene struct {
		name    string
		storeID int64
		want    []TerminationGround
		cited   uuid.UUID
	}
	var scenes []scene

	_, open := f.open(t)
	scenes = append(scenes, scene{"開著的店", open.StoreID, every, finding(open.StoreID)})

	_, suspended := f.open(t)
	cited := finding(suspended.StoreID)
	f.suspend(t, suspended.StoreID)
	scenes = append(scenes, scene{"開著而且停權中的店", suspended.StoreID, every, cited})

	ownerAccount, owner := f.open(t)
	cited = finding(owner.StoreID)
	_, err := f.module.CloseCurrent(ctx, ownerAccount)
	must(t, err)
	scenes = append(scenes, scene{"賣家自己關閉、帳號最近的店", owner.StoreID, withoutNotice, cited})

	_, closed := f.open(t)
	cited = finding(closed.StoreID)
	f.effect(t, f.terminate(t, closed.StoreID, GroundFraud))
	scenes = append(scenes, scene{"終止合作關閉、帳號最近的店", closed.StoreID, withoutNotice, cited})

	oldAccount, oldStore := f.open(t)
	cited = finding(oldStore.StoreID)
	_, err = f.module.CloseCurrent(ctx, oldAccount)
	must(t, err)
	newer, err := f.module.Apply(ctx, validApply(oldAccount))
	must(t, err)
	scenes = append(scenes, scene{"已關閉、帳號有較新的店", oldStore.StoreID, nil, cited}, scene{"帳號較新的店（開著）", newer.StoreID, every, cited})

	f.now = f.now.Add(8 * 24 * time.Hour)
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			state, err := f.module.StoreWithActiveMeasures(ctx, sc.storeID)
			must(t, err)
			if got := terminationGrounds(state); !slices.Equal(got, sc.want) {
				t.Fatalf("可建立的事由 = %v，期望 %v", got, sc.want)
			}
			for _, g := range every {
				state, err := f.module.StoreWithActiveMeasures(ctx, sc.storeID)
				must(t, err)
				grounds := terminationGrounds(state)
				offered := slices.Contains(grounds, g)
				created, createErr := f.createTermination(sc.storeID, g, func(in *CreateMeasureInput) {
					if g == GroundMaterialBreach {
						in.Termination.Violation = &ViolationBasis{CitedFindingIDs: []uuid.UUID{sc.cited}}
					}
				})
				if createErr == nil {
					i := slices.Index(grounds, g)
					if i < 0 {
						t.Fatalf("事由 %s：CreateMeasure 成功但清單沒有它：%+v", g, state.Terminations)
					}
					effective := f.now.UTC().Truncate(time.Microsecond)
					if at := state.Terminations[i].EffectiveAt; at != nil {
						effective = *at
					}
					if !created.Measure.StartsAt.Equal(effective) {
						t.Errorf("事由 %s：清單給的生效時間 %v，CreateMeasure 排的 %v", g, effective, created.Measure.StartsAt)
					}
					if (g == GroundNotice) != (state.Terminations[i].EffectiveAt != nil) {
						t.Errorf("事由 %s：只有預告終止是排程生效", g)
					}
				}
				if offered != (createErr == nil) {
					t.Errorf("事由 %s：清單說 %v，CreateMeasure 回 %v", g, offered, createErr)
				}
				if !offered && (apperr.KindOf(createErr) != apperr.KindFailedPrecondition || !strings.Contains(createErr.Error(), "店家已關閉")) {
					t.Errorf("事由 %s 不可用時要回 failed_precondition，句子說店家已關閉：%v", g, createErr)
				}
			}
		})
	}
}

// 關聯終止的可用性與 CreateMeasure 的前置條件同一份規則：來源是仍生效的 21.3 事由終止、帳號還沒有以它為來源的生效中關聯終止、目標店家允許。
// 每一列 can_create_linked_termination ⇔ CreateMeasure 成功；停權當來源、關聯終止當來源的列都是 false；已建立的列是 false，撤銷之後回到 true；
// 目標店家已關閉時（關聯帳號是 21.3 事由）仍可建立。
// mutation：LinkedSource.BlockFor 拿掉 NotEligible → 停權來源與關聯來源那兩段失敗；CanCreateLinkedTermination 拿掉 alreadyLinked → 已建立那段失敗；
// linkedAccounts 把 CanCreateLinkedTermination 固定為 true → 上述各段失敗。
func TestLinkedTerminationAvailabilityIsTheSameRuleAsCreateMeasure(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	accountA, sA, _ := f.submitted(t, idA, bankAcct)
	accountB, sB, _ := f.submitted(t, idA, "999888777666")

	parity := func(label string, source Measure, wantAny map[int64]bool) {
		t.Helper()
		for _, l := range f.measureOf(t, source.ID).LinkedAccounts {
			want, listed := wantAny[l.AccountID]
			if !listed {
				t.Fatalf("%s：多出帳號 %d", label, l.AccountID)
			}
			if l.CanCreateLinkedTermination != want {
				t.Errorf("%s：帳號 %d can_create_linked_termination = %v，期望 %v", label, l.AccountID, l.CanCreateLinkedTermination, want)
			}
			if (l.TerminationEffect != nil) != l.CanCreateLinkedTermination || (l.TerminationEffect != nil && l.TerminationEffect.EffectiveAt != nil) {
				t.Errorf("%s：帳號 %d 可建立時要有立即生效的效果，不可建立時沒有：%+v", label, l.AccountID, l.TerminationEffect)
			}
			src := source.ID
			_, err := f.createTermination(l.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &src })
			if l.CanCreateLinkedTermination != (err == nil) {
				t.Errorf("%s：帳號 %d 清單說 %v，CreateMeasure 回 %v", label, l.AccountID, l.CanCreateLinkedTermination, err)
			}
			if err == nil {
				// 清掉剛建立的關聯終止，讓同一個狀態可以再驗。
				created := f.measureOf(t, source.ID).LinkedAccounts
				for _, c := range created {
					if c.AccountID == l.AccountID && c.LinkedMeasureID != nil {
						f.revoke(t, f.measureOf(t, *c.LinkedMeasureID))
					}
				}
			}
		}
	}

	// 停權當來源：身分快照在，但不是 21.3 事由的終止 → 不能建立。
	suspension := f.suspend(t, sA.StoreID)
	if got := f.measureOf(t, suspension.ID).LinkedAccounts; len(got) != 1 || got[0].AccountID != accountB {
		t.Fatalf("停權來源要列出帳號 B：%+v", got)
	}
	parity("停權來源", suspension, map[int64]bool{accountB: false})
	endRisk := f.endRisk(t, suspension)
	_ = endRisk

	// 21.3 終止當來源：可以；建立之後（已有生效中的關聯終止）不行；撤銷之後回到可以。
	source := f.terminate(t, sA.StoreID, GroundFraud)
	parity("21.3 來源", source, map[int64]bool{accountB: true})
	src := source.ID
	lm, err := f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &src })
	must(t, err)
	got := f.measureOf(t, source.ID).LinkedAccounts
	if len(got) != 1 || got[0].LinkedMeasureID == nil || got[0].CanCreateLinkedTermination {
		t.Fatalf("已建立關聯終止的列要是 false：%+v", got)
	}
	if _, err := f.createTermination(sB.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &src }); apperr.KindOf(err) != apperr.KindAlreadyExists {
		t.Fatalf("同來源第二筆：%v", err)
	}

	// 關聯終止當來源：不成鏈 → 帳號 A 那一列是 false。
	parity("關聯終止當來源", f.measureOf(t, lm.Measure.ID), map[int64]bool{accountA: false})

	f.revoke(t, f.measureOf(t, lm.Measure.ID))
	got = f.measureOf(t, source.ID).LinkedAccounts
	if len(got) != 1 || got[0].LinkedMeasureID != nil || !got[0].CanCreateLinkedTermination {
		t.Fatalf("關聯終止撤銷之後回到可建立：%+v", got)
	}

	// 目標店家已關閉（賣家自己關店）：關聯帳號是 21.3 事由，仍可建立。
	_, err = f.module.CloseCurrent(ctx, accountB)
	must(t, err)
	parity("目標店家已關閉", source, map[int64]bool{accountB: true})
}

// ApplyStore 被擋的原因可分辨（ADR 2026-09-25-api-error-model D2：client 要在同一個 failed_precondition 底下分支）：停權中是 STORE_OPENING_SUSPENDED，
// 有生效中的終止合作是 STORE_OPENING_TERMINATED（兩者都有時是後者：解除停權也開不了），人類可讀的訊息各自正確；終止撤銷之後可以再開。
// 換身分認證被別的帳號的終止擋下（SubmitKYC）維持通用錯誤：不透露命中的帳號或欄位，沒有 reason。
// mutation：OpeningBlock 兩種都回 SUSPENDED → 終止那段失敗；OpeningBlock 先判斷停權 → 同時有停權與終止那段失敗；訊息互換 → 訊息斷言失敗。
func TestApplyStoreSeparatesSuspensionFromTermination(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	check := func(label string, err error, want apperr.Reason, text string) {
		t.Helper()
		var ae *apperr.Error
		if apperr.KindOf(err) != apperr.KindFailedPrecondition || !errors.As(err, &ae) || ae.Reason != want || ae.Message != text {
			t.Fatalf("%s：err=%v", label, err)
		}
	}
	const suspendedText = "帳號停權中，解除停權前不能開店"
	const terminatedText = "此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。"

	suspendedAccount, s := f.open(t)
	f.suspend(t, s.StoreID)
	_, err := f.module.CloseCurrent(ctx, suspendedAccount)
	must(t, err)
	_, err = f.module.Apply(ctx, validApply(suspendedAccount))
	check("停權", err, apperr.ReasonStoreOpeningSuspended, suspendedText)

	terminatedAccount, ts, m := f.terminated(t, GroundCeasedBusiness)
	_ = ts
	_, err = f.module.Apply(ctx, validApply(terminatedAccount))
	check("終止", err, apperr.ReasonStoreOpeningTerminated, terminatedText)

	// 同時有停權：仍是終止（解除停權也開不了）。
	both, bs := f.open(t)
	f.suspend(t, bs.StoreID)
	f.effect(t, f.terminate(t, bs.StoreID, GroundFraud))
	_, err = f.module.Apply(ctx, validApply(both))
	check("終止加停權", err, apperr.ReasonStoreOpeningTerminated, terminatedText)

	// 終止撤銷、店家恢復之後帳號有目前的店家：AlreadyExists（不是這兩個 reason）；撤銷發生在清理之後才是「可以再開」，由既有測試涵蓋。
	f.revoke(t, m)
	_, err = f.module.Apply(ctx, validApply(terminatedAccount))
	if apperr.KindOf(err) != apperr.KindAlreadyExists {
		t.Fatalf("恢復之後：%v", err)
	}

	// KYC 的身分比對維持通用錯誤。
	blockedAccount, blockedStore, _ := f.submitted(t, idA, bankAcct)
	f.effect(t, f.terminate(t, blockedStore.StoreID, GroundFraud))
	_ = blockedAccount
	other, _ := f.open(t)
	draft := f.savedDraft(t, kycDraft(other, idA, "999888777666"))
	_, err = f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: other, ExpectedVersion: draft.Version})
	var ae *apperr.Error
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !errors.As(err, &ae) || ae.Reason != (apperr.Reason{}) || ae.Message != "無法通過認證，請聯絡客服" {
		t.Fatalf("身分比對要維持通用錯誤：%v", err)
	}
}

// 恢復營業由帳號上全部的終止合作推導（不看是哪一筆關的店）：帳號有兩筆生效中的終止，撤銷其中一筆店家仍關閉、不發 StoreReopened；撤銷最後一筆才恢復，
// 關店歷史記著當初關店的那一筆（closed_by_measure_id 是稽核事實，不因別筆撤銷而變）。兩個撤銷順序都驗：先撤銷關店的那一筆、先撤銷後來建立的那一筆。
// mutation：ReopenPlan 改回只看 closed_by_measure_id 是被撤銷的那一筆 → 「先撤銷關店那筆」（店重開又被對帳關掉）與「最後撤銷後建立的那筆」（不重開）兩段失敗；
// ReopenPlan 不排除 revoking → 最後一筆撤銷時不恢復。
func TestTwoTerminationsKeepTheStoreClosedUntilTheLastOneIsRevoked(t *testing.T) {
	t.Parallel()
	for _, order := range []string{"closing-first", "later-first"} {
		t.Run(order, func(t *testing.T) {
			f := setup(t)
			reopened := record[StoreReopened](f)
			_, s, closing := f.terminated(t, GroundFraud)
			f.now = f.now.Add(10 * 24 * time.Hour)
			second := f.terminate(t, s.StoreID, GroundCeasedBusiness)
			first, last := closing, second
			if order == "later-first" {
				first, last = second, closing
			}
			f.now = f.now.Add(10 * 24 * time.Hour)
			f.revoke(t, first)
			if got := f.storeRow(t, s.StoreID); got.Status != Closed || got.Closure == nil || len(reopened.all()) != 0 || f.storeClosures(t, s.StoreID) != 0 {
				t.Fatalf("另一筆終止仍生效，店家要維持關閉：%+v reopened=%d", got, len(reopened.all()))
			}
			if !f.isSuspended(t, s.StoreID) {
				t.Fatal("另一筆終止仍生效，帳號仍停權")
			}
			f.revoke(t, last)
			back := f.storeRow(t, s.StoreID)
			if back.Status != Active || back.Closure != nil || len(reopened.all()) != 1 || f.storeClosures(t, s.StoreID) != 1 {
				t.Fatalf("最後一筆撤銷要恢復：%+v reopened=%d", back, len(reopened.all()))
			}
			var closedMeasure uuid.UUID
			must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT closed_by_measure_id FROM seller.store_closures WHERE store_id=$1 AND reopened_at IS NOT NULL", s.StoreID).Scan(&closedMeasure))
			if closedMeasure != closing.ID {
				t.Fatalf("關店歷史要記當初關店的那一筆：%s，期望 %s", closedMeasure, closing.ID)
			}
			if f.isSuspended(t, s.StoreID) {
				t.Fatal("全部撤銷之後帳號恢復")
			}
		})
	}
}

// 來源撤銷連帶撤銷關聯終止時，同一個 transaction 內「撤銷之後」才算還剩幾筆生效中的終止：關聯帳號自己另有一筆獨立的終止時，店家仍關閉；來源帳號的店家恢復。
// mutation：ReopenPlan 的 revoking 只放被直接撤銷的那一筆（不含關聯終止）→ 關聯帳號仍有「別筆」之外的自己這筆被當成生效而不恢復的判斷反向：
// 把 plan.linked 排除出 revokingIDs → 沒有獨立終止的關聯帳號那段（TestSourceRevocationCascades…）與本測試的第二段失敗。
func TestCascadeRevocationReopensOnlyAccountsWithNoOtherEffectiveTermination(t *testing.T) {
	t.Parallel()
	f := setup(t)
	reopened := record[StoreReopened](f)
	_, sA, _ := f.submitted(t, idA, bankAcct)
	_, sB, _ := f.submitted(t, idA, "999888777666")
	_, sC, _ := f.submitted(t, idA, "555444333222")
	source := f.terminate(t, sA.StoreID, GroundFraud)
	f.effect(t, source)
	src := source.ID
	for _, target := range []Store{sB, sC} {
		lm, err := f.createTermination(target.StoreID, GroundLinkedAccount, func(in *CreateMeasureInput) { in.Termination.SourceMeasureID = &src })
		must(t, err)
		f.effect(t, lm.Measure)
	}
	independent := f.terminate(t, sC.StoreID, GroundCeasedBusiness)
	for _, s := range []Store{sA, sB, sC} {
		if f.storeRow(t, s.StoreID).Status != Closed {
			t.Fatalf("店 %d 應該已關閉", s.StoreID)
		}
	}
	f.now = f.now.Add(5 * 24 * time.Hour)
	f.revoke(t, f.measureOf(t, source.ID))
	for _, tc := range []struct {
		store Store
		want  Status
	}{{sA, Active}, {sB, Active}, {sC, Closed}} {
		if got := f.storeRow(t, tc.store.StoreID).Status; got != tc.want {
			t.Fatalf("店 %d 狀態 %s，期望 %s", tc.store.StoreID, got, tc.want)
		}
	}
	if len(reopened.all()) != 2 {
		t.Fatalf("reopened=%d", len(reopened.all()))
	}
	f.revoke(t, f.measureOf(t, independent.ID))
	if f.storeRow(t, sC.StoreID).Status != Active || len(reopened.all()) != 3 {
		t.Fatalf("帳號 C 的獨立終止撤銷後恢復：%s reopened=%d", f.storeRow(t, sC.StoreID).Status, len(reopened.all()))
	}
}

// 保留期（RetireDue）看帳號上全部未撤銷的終止中最晚的申覆期限：關店之後才發現的第二筆 21.3 事由，申覆期限比較晚，清理等到它之後；
// 第二筆上有待處理的申覆時，更晚的清理也要暫停（legal hold 看帳號），裁定之後下一輪清理。已撤銷的終止不延長保留（只剩關店那筆時回到它的期限）。
// mutation：RetireDueStores 的 retire_after 拿掉帳號上最晚終止的那一項（只看關店那筆）→ 第一段（關店那筆期限之後仍保留）失敗；
// 最晚終止的子查詢拿掉 status='ENFORCED' → 最後一段（撤銷第二筆後回到第一筆期限）失敗。
func TestRetentionFollowsTheLatestAppealDeadlineAmongAllTerminations(t *testing.T) {
	t.Parallel()
	f := setup(t)
	purge := func() int {
		n, err := f.module.PurgePII(context.Background())
		must(t, err)
		return n
	}
	accountID, s, closing := f.terminated(t, GroundFraud)
	firstDeadline := closing.StartsAt.Add(4416 * time.Hour)
	f.now = closing.StartsAt.Add(50 * 24 * time.Hour)
	second := f.terminate(t, s.StoreID, GroundCeasedBusiness)
	secondDeadline := second.StartsAt.Add(4416 * time.Hour)
	if !secondDeadline.After(firstDeadline) {
		t.Fatalf("第二筆的申覆期限要比較晚：%s %s", secondDeadline, firstDeadline)
	}

	f.now = firstDeadline.Add(time.Microsecond)
	if purge() != 0 || f.purged(t, s.StoreID) {
		t.Fatal("關店那筆的申覆期限已過，但帳號上還有申覆期限更晚的終止：不清")
	}
	f.now = secondDeadline
	if purge() != 0 {
		t.Fatal("剛好是最晚的申覆期限不清（要嚴格超過）")
	}

	// 最晚的那一筆上有待處理的申覆：期限過了也不清；裁定之後下一輪清。
	f.now = secondDeadline.Add(-time.Hour)
	f.appeal(t, accountID, f.measureOf(t, second.ID), "我不同意")
	f.now = secondDeadline.Add(time.Hour)
	if purge() != 0 || f.purged(t, s.StoreID) {
		t.Fatal("非關店那筆的申訴待處理：legal hold 暫停清理")
	}
	cur := f.measureOf(t, second.ID)
	_, err := f.module.DecideAppeal(context.Background(), DecideAppealInput{MeasureID: second.ID, AdminID: 9, ExpectedVersion: cur.Version, Outcome: RequestRejected, Reason: "維持"})
	must(t, err)
	if purge() != 1 || !f.purged(t, s.StoreID) {
		t.Fatal("裁定之後、最晚的申覆期限已過：清理")
	}

	// 已撤銷的終止不延長保留：撤銷第二筆之後回到關店那筆的期限。
	g := setup(t)
	purgeG := func() int {
		n, err := g.module.PurgePII(context.Background())
		must(t, err)
		return n
	}
	_, s2, closing2 := g.terminated(t, GroundFraud)
	g.now = closing2.StartsAt.Add(50 * 24 * time.Hour)
	extra := g.terminate(t, s2.StoreID, GroundCeasedBusiness)
	g.revoke(t, extra)
	g.now = closing2.StartsAt.Add(4416 * time.Hour).Add(time.Microsecond)
	if purgeG() != 1 || !g.purged(t, s2.StoreID) {
		t.Fatal("第二筆已撤銷：保留期回到關店那筆的申覆期限")
	}
}

// 預定生效的預告終止不算「仍生效中的終止」：帳號的店因另一筆終止合作關店時，排著的預告隨關店結束（ENDED，不再是預定生效），
// 所以撤銷關店那一筆之後店家恢復，預告不會讓店家維持關閉。預定生效而且帳號的店已被終止關閉的組合，狀態機本身到不了（關店即結束預告）；
// 「預定生效不算生效中」另由 EffectiveMeasures 的 starts_at <= now 條件與 TestEffectiveMeasuresAndStateAgreeAtTheBoundaries 鎖住。
// mutation：endNotices 不結束預告 → 預告狀態斷言失敗；ReopenPlan 把任何未撤銷的終止（不分是否已開始）都算進去 → 預告若仍未結束時店家不恢復而失敗。
func TestScheduledNoticeDoesNotKeepTheStoreClosedAfterTheOtherTerminationIsRevoked(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.open(t)
	notice := f.terminate(t, s.StoreID, GroundNotice)
	if notice.State != StateScheduled {
		t.Fatalf("預告建立後是預定生效：%s", notice.State)
	}
	f.now = f.now.Add(time.Hour)
	fraud := f.terminate(t, s.StoreID, GroundFraud)
	f.effect(t, fraud)
	if f.storeRow(t, s.StoreID).Status != Closed {
		t.Fatal("終止關店")
	}
	if got := f.measureOf(t, notice.ID).State; got != StateEnded {
		t.Fatalf("關店時排著的預告隨之結束：%s", got)
	}
	f.revoke(t, fraud)
	if got := f.storeRow(t, s.StoreID); got.Status != Active || got.Closure != nil {
		t.Fatalf("撤銷關店那一筆之後恢復：%+v", got)
	}
}

// 恢復營業的條件之一是「造成關店的那一筆已撤銷」（closed_by_measure_id 是關店當時的事實）：預告終止先關店，之後在同一個已關閉的店建立 21.3 事由，
// 撤銷後建立的那一筆，店家仍因預告終止而關閉（條款第 21.4 條）；撤銷預告（期限內）而 21.3 還在時也不恢復，兩筆都撤銷才恢復，不論撤銷順序。
// mutation：ReopenPlan 拿掉「造成關店的那一筆已撤銷」的判斷 → later-first 的第一次撤銷後店家被恢復而失敗；
// 把判斷改成只看 revoking（不看之前已撤銷）→ closing-first 的最後一次撤銷不恢復而失敗。
func TestRevokingATerminationThatDidNotCloseTheStoreNeverReopensIt(t *testing.T) {
	t.Parallel()
	for _, order := range []string{"later-first", "closing-first"} {
		t.Run(order, func(t *testing.T) {
			f := setup(t)
			reopened := record[StoreReopened](f)
			_, s := f.open(t)
			notice := f.terminate(t, s.StoreID, GroundNotice)
			f.now = notice.StartsAt.Add(time.Hour)
			f.effect(t, notice)
			closed := f.storeRow(t, s.StoreID)
			if closed.Status != Closed || closed.Closure == nil || closed.Closure.MeasureID == nil || *closed.Closure.MeasureID != notice.ID {
				t.Fatalf("預告終止生效後關店，關店的措施是預告：%+v", closed.Closure)
			}
			f.now = f.now.Add(24 * time.Hour)
			later := f.terminate(t, s.StoreID, GroundCeasedBusiness)
			first, last := later, notice
			if order == "closing-first" {
				first, last = notice, later
			}
			f.now = f.now.Add(24 * time.Hour)
			f.revoke(t, first)
			if got := f.storeRow(t, s.StoreID); got.Status != Closed || got.Closure == nil || len(reopened.all()) != 0 {
				t.Fatalf("第一筆撤銷後店家仍因預告終止而關閉：%+v reopened=%d", got, len(reopened.all()))
			}
			f.revoke(t, last)
			if got := f.storeRow(t, s.StoreID); got.Status != Active || got.Closure != nil || len(reopened.all()) != 1 {
				t.Fatalf("兩筆都撤銷後恢復：%+v reopened=%d", got, len(reopened.all()))
			}
			if f.storeClosures(t, s.StoreID) != 1 {
				t.Fatalf("關店歷史有一筆")
			}
		})
	}
}

// 保留期只看帳號上未撤銷的終止：關店之後才生效的另一筆終止把保留期延到它自己的申覆期限；它撤銷之後不再延長，回到關店後的保留期
// （造成關店的終止在關店時生效，它的申覆期限與關店後的保留期同一刻到期，所以延長只可能來自關店之後才生效的終止）。
// 對照：撤銷前同一時間點不清（另一筆未撤銷，取最晚的）。
// mutation：RetireDueStores 的申覆期限不篩 status = 'ENFORCED'（已撤銷的也算）→ 撤銷後的清理斷言失敗；拿掉申覆期限那一項 → 撤銷前的斷言失敗。
func TestARevokedLaterTerminationDoesNotExtendTheRetention(t *testing.T) {
	t.Parallel()
	f := setup(t)
	purge := func() int {
		n, err := f.module.PurgePII(context.Background())
		must(t, err)
		return n
	}
	_, s := f.open(t)
	closing := f.terminate(t, s.StoreID, GroundCeasedBusiness)
	f.effect(t, closing)
	if got := f.storeRow(t, s.StoreID); got.Status != Closed || got.Closure == nil || *got.Closure.MeasureID != closing.ID {
		t.Fatalf("關店的是第一筆：%+v", got.Closure)
	}
	f.now = f.now.Add(10 * 24 * time.Hour)
	later := f.terminate(t, s.StoreID, GroundFraud)
	closingDeadline := closing.StartsAt.Add(4416 * time.Hour)
	if !later.StartsAt.Add(4416 * time.Hour).After(closingDeadline) {
		t.Fatal("關店之後才生效的那一筆，申覆期限要比較晚")
	}
	f.now = closingDeadline.Add(time.Microsecond)
	if purge() != 0 || f.purged(t, s.StoreID) {
		t.Fatal("另一筆終止未撤銷：保留到最晚的申覆期限")
	}
	f.now = closingDeadline.Add(-time.Hour)
	f.revoke(t, later)
	if got := f.storeRow(t, s.StoreID); got.Status != Closed {
		t.Fatalf("造成關店的終止仍生效，店家仍關閉：%s", got.Status)
	}
	f.now = closingDeadline.Add(time.Microsecond)
	if purge() != 1 || !f.purged(t, s.StoreID) {
		t.Fatal("另一筆已撤銷：保留期只看未撤銷的終止，這個時間點可以清理")
	}
}

// 帳號等待刪除時，終止生效的 job 什麼都不做：不重算停權（不發 StoreSuspended、不開停權期間），店家維持等待刪除，由刪帳以 ACCOUNT_DELETION 關店。
// mutation：effect 把 Recompute 放回判斷等待刪除之前 → 預告生效時發出 StoreSuspended，斷言失敗。
func TestTerminationEffectDoesNothingWhileTheAccountIsBeingDeleted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	suspended := record[StoreSuspended](f)
	closed := record[StoreClosed](f)
	_, s := f.open(t)
	notice := f.terminate(t, s.StoreID, GroundNotice)
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, s.StoreID) }))
	f.now = notice.StartsAt.Add(time.Hour)
	f.effect(t, notice)
	if len(suspended.all()) != 0 || len(closed.all()) != 0 || f.count(t, "SELECT count(*) FROM seller.suspension_periods WHERE store_id=$1", s.StoreID) != 0 {
		t.Fatalf("等待刪除的帳號不重算停權：suspended=%d closed=%d", len(suspended.all()), len(closed.all()))
	}
	if got := f.storeRow(t, s.StoreID); got.Status != PendingDeletion {
		t.Fatalf("店家維持等待刪除：%s", got.Status)
	}
}

// 開店入口的可用性（StoreOpening）與 ApplyStore 的前置條件是同一份規則：每個帳號狀態，「can_apply 為真」⇔「Apply 成功」，原因清單等於手算的期望值
// （終止合作、停權、帳號正在刪除、已有店家，依優先序；測試不依賴被測的函式），Apply 被擋下時的錯誤對應清單的第一個原因；停權到期後兩邊同時放行。
// mutation：OpeningBlockers 少列一種原因（例如不看終止）→ 該列的清單與 Apply 一致性斷言失敗；StoreOpening 自己另算（例如只看停權）→ 終止那列失敗；
// OpeningBlock 不用清單的第一個而另寫判斷 → 錯誤對應斷言失敗。
func TestStoreOpeningAvailabilityIsTheSameRuleAsApplyStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	scenes := []struct {
		name    string
		account func() int64
		want    []OpeningBlocker
		reason  apperr.Reason
		kind    apperr.Kind
	}{
		{"乾淨的帳號", func() int64 { return f.account(t) }, nil, apperr.Reason{}, 0},
		{"已有店家", func() int64 { id, _ := f.open(t); return id }, []OpeningBlocker{OpeningStoreExists}, apperr.Reason{}, apperr.KindAlreadyExists},
		{"帳號正在刪除", func() int64 {
			id := f.account(t)
			must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.identity.BeginDeletion(ctx, id) }))
			return id
		}, []OpeningBlocker{OpeningAccountDeleting}, apperr.Reason{}, apperr.KindFailedPrecondition},
		{"停權中、店已關閉", func() int64 {
			id, s := f.open(t)
			f.suspend(t, s.StoreID)
			_, err := f.module.CloseCurrent(ctx, id)
			must(t, err)
			return id
		}, []OpeningBlocker{OpeningSuspended}, apperr.ReasonStoreOpeningSuspended, apperr.KindFailedPrecondition},
		{"終止合作、店已關閉", func() int64 { id, _, _ := f.terminated(t, GroundFraud); return id }, []OpeningBlocker{OpeningTerminated}, apperr.ReasonStoreOpeningTerminated, apperr.KindFailedPrecondition},
		{"終止合作加停權", func() int64 {
			id, s := f.open(t)
			f.suspend(t, s.StoreID)
			f.effect(t, f.terminate(t, s.StoreID, GroundFraud))
			if f.storeRow(t, s.StoreID).Status != Closed {
				t.Fatal("終止之後關店")
			}
			return id
		}, []OpeningBlocker{OpeningTerminated, OpeningSuspended}, apperr.ReasonStoreOpeningTerminated, apperr.KindFailedPrecondition},
		{"預告終止、賣家自己關店後預告隨之結束", func() int64 {
			id, s := f.open(t)
			f.terminate(t, s.StoreID, GroundNotice)
			f.now = f.now.Add(time.Hour)
			_, err := f.module.CloseCurrent(ctx, id)
			must(t, err)
			return id
		}, nil, apperr.Reason{}, 0},
	}
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			accountID := sc.account()
			got, err := f.module.StoreOpening(ctx, accountID)
			must(t, err)
			if got.CanApply != (len(sc.want) == 0) || !slices.Equal(openingBlockers(got), sc.want) {
				t.Fatalf("StoreOpening = %+v，期望原因 %v", got, sc.want)
			}
			for _, u := range got.Unavailable {
				if u.Message.Text == "" || u.Message != u.Blocker.Text(uitext.ZhTW) {
					t.Fatalf("原因 %s 的句子 %q 不是該原因的文字", u.Blocker, u.Message)
				}
			}
			_, applyErr := f.module.Apply(ctx, validApply(accountID))
			if got.CanApply != (applyErr == nil) {
				t.Fatalf("can_apply = %v 但 Apply 的結果是 %v", got.CanApply, applyErr)
			}
			if applyErr == nil {
				return
			}
			var ae *apperr.Error
			if !errors.As(applyErr, &ae) || apperr.KindOf(applyErr) != sc.kind || ae.Reason != sc.reason {
				t.Fatalf("Apply 的錯誤要對應清單的第一個原因：%v", applyErr)
			}
			if ae.Message != got.Unavailable[0].Message.Text {
				t.Fatalf("Apply 的錯誤文字 %q 要等於可用性清單第一筆的文字 %q", ae.Message, got.Unavailable[0].Message)
			}
		})
	}
	// 停權到期之後兩邊同時放行。
	accountID, s := f.open(t)
	ends := f.now.Add(time.Hour)
	f.violate(t, s.StoreID, SeverityLow, MeasureSuspension, &ends)
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	if got, err := f.module.StoreOpening(ctx, accountID); err != nil || got.CanApply || !slices.Equal(openingBlockers(got), []OpeningBlocker{OpeningSuspended}) {
		t.Fatalf("停權期間：%+v %v", got, err)
	}
	f.now = ends.Add(time.Second)
	if got, err := f.module.StoreOpening(ctx, accountID); err != nil || !got.CanApply || len(got.Unavailable) != 0 {
		t.Fatalf("停權到期：%+v %v", got, err)
	}
	if _, err := f.module.Apply(ctx, validApply(accountID)); err != nil {
		t.Fatalf("停權到期後 Apply 成功：%v", err)
	}
}

// openingBlockers 取出可用性清單的原因（依序）。
func openingBlockers(o StoreOpening) []OpeningBlocker {
	var out []OpeningBlocker
	for _, u := range o.Unavailable {
		out = append(out, u.Blocker)
	}
	return out
}

// terminationGrounds 是 state.Terminations 的事由（依清單順序）。
func terminationGrounds(state StoreMeasures) []TerminationGround {
	out := make([]TerminationGround, len(state.Terminations))
	for i, o := range state.Terminations {
		out[i] = o.Ground
	}
	return out
}
