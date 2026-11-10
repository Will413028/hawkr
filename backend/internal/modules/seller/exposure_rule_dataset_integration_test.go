//go:build integration

package seller_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// exposureCase 是資料集的一家店：missing 是在建立時刻（base）人工標註的 PUBLIC_LISTING 缺項；exposableAt 是各評估時刻的人工標註
// （沒有列的時刻與 base 相同）。
type exposureCase struct {
	name        string
	storeID     int64
	missing     []Requirement
	exposableAt map[time.Duration]bool
}

// 曝光規則只寫在 seller 的 SQL（seller.pub_exposable_stores(at)，條件在 store_requirement_flags 與 effective_measures）。Go 只把讀出來的
// 條件對應到 Requirement，「全部條件的 AND」在 Go（needs 表的 CapPublicListing）與 SQL 各寫一次。這個資料集鎖住四條路徑在每家店、
// 每個評估時刻都相等：SQL 函式直接查、Module.ExposableStores（exposure.Classify 讀同一個函式）、CapabilitiesOf 的 PUBLIC_LISTING（Go 的
// AND）與人工標註；缺項與人工標註相等（每個條件只缺它自己）；開通條件等於申請核准且收款帳戶的轉帳能力啟用。
// 資料集含每個生命週期、個資清理、缺 logo 與鄉鎮市區、KYC 申請的每個狀態、沒有申請、沒有收款帳戶、收款帳戶的每個狀態、PayUni 的綁定失敗與審核被拒絕（不影響閘門），
// 以及有固定期間的停權（剛好結束、早 1µs）、開放式 RISK、解除的 RISK（結束在下一刻）、警告、預告終止（剛好生效、早 1µs）。
func TestExposureRuleAgreesBetweenSQLAndGoOnADataset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	base := f.now
	us := time.Microsecond
	timedEnds := 48 * time.Hour
	notice := 720 * time.Hour
	times := []time.Duration{0, timedEnds - us, timedEnds, notice - us, notice}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := f.db.Conn(ctx).Exec(ctx, sql, args...)
		must(t, err)
	}
	always := func(v bool) map[time.Duration]bool {
		out := map[time.Duration]bool{}
		for _, d := range times {
			out[d] = v
		}
		return out
	}
	var cases []exposureCase
	add := func(name string, storeID int64, missing []Requirement, exposableAt map[time.Duration]bool) {
		if exposableAt == nil {
			exposableAt = always(len(missing) == 0)
		}
		cases = append(cases, exposureCase{name: name, storeID: storeID, missing: missing, exposableAt: exposableAt})
	}
	exposable := func() (int64, int64) {
		accountID, s := f.exposable(t)
		return accountID, s.StoreID
	}
	payuni := func(storeID int64, sql string, args ...any) {
		exec("UPDATE seller.payuni_registrations SET "+sql+" WHERE store_id = $1", append([]any{storeID}, args...)...)
	}
	kyc := func(storeID int64, sql string) {
		exec("UPDATE seller.kyc_applications SET "+sql+" WHERE store_id = $1 AND status <> 'CANCELLED'", storeID)
	}

	_, id := exposable()
	add("全部條件都滿足", id, nil, nil)

	_, id = exposable()
	must(t, f.db.InTx(ctx, func(ctx context.Context) error { return f.module.BeginDeletion(ctx, id) }))
	add("等待刪除", id, []Requirement{RequirementNotPendingDeletion}, nil)

	accountID, id := exposable()
	_, err := f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	add("已關閉", id, []Requirement{RequirementActive}, nil)

	accountID, id = exposable()
	_, err = f.module.CloseCurrent(ctx, accountID)
	must(t, err)
	exec(`UPDATE seller.stores SET retired_at = $2, pii_purged_at = $2, store_name = NULL, description = NULL, contact_phone = NULL, contact_name = NULL,
		contact_mobile = NULL, support_email = NULL, town_code = NULL, address_detail = NULL, prize = NULL, website_url = NULL,
		logo_key = NULL, logo_asset_id = NULL, cover_key = NULL, cover_asset_id = NULL WHERE store_id = $1`, id, base)
	add("已關閉且個資已清理", id, []Requirement{RequirementActive, RequirementStoreName, RequirementLogo, RequirementTownCode}, nil)

	_, id = exposable()
	exec("UPDATE seller.stores SET logo_key = NULL, logo_asset_id = NULL WHERE store_id = $1", id)
	add("沒有 logo", id, []Requirement{RequirementLogo}, nil)

	_, id = exposable()
	exec("UPDATE seller.stores SET town_code = NULL, address_detail = NULL WHERE store_id = $1", id)
	add("沒有鄉鎮市區代碼", id, []Requirement{RequirementTownCode}, nil)

	for _, st := range []struct{ name, set string }{
		{"KYC 申請改回草稿", "status = 'DRAFT', decided_at = NULL"},
		{"KYC 申請待審", "status = 'SUBMITTED', decided_at = NULL"},
		{"KYC 申請被退回", "status = 'REJECTED'"},
		{"KYC 申請已取消", "status = 'CANCELLED', decided_at = NULL"},
	} {
		_, id = exposable()
		kyc(id, st.set)
		add(st.name, id, []Requirement{RequirementOnboardingComplete}, nil)
	}

	accountID, s := f.open(t)
	f.logoed(t, accountID, s)
	add("沒有 KYC 申請", s.StoreID, []Requirement{RequirementOnboardingComplete}, nil)

	_, id = exposable()
	exec("DELETE FROM seller.payout_capabilities WHERE store_id = $1", id)
	add("KYC 已核准但沒有收款帳戶的閘門副本", id, []Requirement{RequirementOnboardingComplete}, nil)

	_, id = exposable()
	exec("UPDATE seller.payout_capabilities SET transfers_active = false WHERE store_id = $1", id)
	add("收款帳戶的轉帳能力未啟用", id, []Requirement{RequirementOnboardingComplete}, nil)

	// PayUni 不再是開通閘門的輸入：綁定失敗、審核被拒絕的店家，只要收款帳戶啟用就可以曝光。
	_, id = exposable()
	payuni(id, "bind_status = 'FAILED', bound_at = NULL, bind_error_code = 'AGENT04017', bind_failed_at = updated_at")
	add("PayUni 綁定失敗但收款帳戶已啟用", id, nil, nil)

	_, id = exposable()
	payuni(id, "outcome = 'REJECTED'")
	add("PayUni 審核被拒絕但收款帳戶已啟用", id, nil, nil)

	// 措施會記下帳號的身分快照，之後同一個身分就不能再通過 KYC：先建好所有要套措施的店，再建立措施。
	measured := make([]int64, 5)
	for i := range measured {
		_, measured[i] = exposable()
	}
	id = measured[0]
	ends := base.Add(timedEnds)
	f.violate(t, id, SeverityHigh, MeasureSuspension, &ends)
	add("有固定期間的停權", id, []Requirement{RequirementNotSuspended}, map[time.Duration]bool{
		0: false, timedEnds - us: false, timedEnds: true, notice - us: true, notice: true,
	})

	id = measured[1]
	f.riskSuspend(t, id)
	add("開放式 RISK 停權", id, []Requirement{RequirementNotSuspended}, nil)

	id = measured[2]
	// 解除 RISK 停權是把結束時間設在下一刻（base+1µs）：base 那一刻仍生效，之後不生效。
	_, err = f.lift(ctx, f.riskSuspend(t, id))
	must(t, err)
	f.now = base
	add("解除的 RISK 停權（結束在 base+1µs）", id, []Requirement{RequirementNotSuspended}, map[time.Duration]bool{
		0: false, timedEnds - us: true, timedEnds: true, notice - us: true, notice: true,
	})

	id = measured[3]
	f.violate(t, id, SeverityLow, MeasureWarning, nil)
	add("警告", id, nil, nil)

	id = measured[4]
	f.terminate(t, id, GroundNotice)
	add("預告終止", id, nil, map[time.Duration]bool{0: true, timedEnds - us: true, timedEnds: true, notice - us: true, notice: false})

	ids := make([]int64, len(cases))
	for i, c := range cases {
		ids[i] = c.storeID
	}
	q := store.New(f.db.Conn(ctx))
	for _, d := range times {
		at := base.Add(d)
		f.now = at
		rows, err := f.db.Conn(ctx).Query(ctx, "SELECT store_id FROM seller.pub_exposable_stores($1) WHERE store_id = ANY($2)", at, ids)
		must(t, err)
		var fromSQL []int64
		for rows.Next() {
			var sid int64
			must(t, rows.Scan(&sid))
			fromSQL = append(fromSQL, sid)
		}
		must(t, rows.Err())
		fromModule := f.exposed(t, ids...)
		for _, c := range cases {
			want := c.exposableAt[d]
			cur, err := f.module.Store(ctx, c.storeID)
			must(t, err)
			var listing CapabilityStatus
			for _, st := range CapabilitiesOf(cur) {
				if st.Capability == CapPublicListing {
					listing = st
				}
			}
			sql, viaModule := slices.Contains(fromSQL, c.storeID), slices.Contains(fromModule, c.storeID)
			if sql != want || viaModule != want || listing.Active != want {
				t.Errorf("%s @+%v：pub_exposable_stores=%v、ExposableStores=%v、PUBLIC_LISTING=%v（缺 %v），人工標註 %v",
					c.name, d, sql, viaModule, listing.Active, listing.Missing, want)
			}
			if d == 0 && !slices.Equal(listing.Missing, c.missing) {
				t.Errorf("%s：缺項 %v，人工標註 %v", c.name, listing.Missing, c.missing)
			}
		}
	}

	// 開通條件（SQL）等於「目前那一筆申請已核准而且收款帳戶的轉帳能力啟用」：Go 以申請狀態與閘門副本（payout_capabilities）各自重算一次。
	applications, err := q.CurrentKYCStatuses(ctx, ids)
	must(t, err)
	approvedApplication := map[int64]bool{}
	for _, a := range applications {
		approvedApplication[a.StoreID] = model.KYCStatus(a.Status) == model.KYCStatusApproved
	}
	transfers := map[int64]bool{}
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT store_id, transfers_active FROM seller.payout_capabilities WHERE store_id = ANY($1)", ids)
	must(t, err)
	for rows.Next() {
		var id int64
		var active bool
		must(t, rows.Scan(&id, &active))
		transfers[id] = active
	}
	must(t, rows.Err())
	rows.Close()
	flags, err := q.StoreRequirementFlags(ctx, ids)
	must(t, err)
	if len(flags) != len(ids) {
		t.Fatalf("條件列 %d 筆，店家 %d 家", len(flags), len(ids))
	}
	viewComplete := map[int64]bool{}
	completeRows, err := f.db.Conn(ctx).Query(ctx, "SELECT store_id, onboarding_complete FROM seller.store_requirement_flags WHERE store_id = ANY($1)", ids)
	must(t, err)
	for completeRows.Next() {
		var id int64
		var complete bool
		must(t, completeRows.Scan(&id, &complete))
		viewComplete[id] = complete
	}
	must(t, completeRows.Err())
	completeRows.Close()
	complete := 0
	for _, fl := range flags {
		wantKYC, wantTransfers := approvedApplication[fl.StoreID], transfers[fl.StoreID]
		if fl.KycApproved != wantKYC || fl.TransfersActive != wantTransfers {
			t.Errorf("店家 %d：kyc_approved=%v、transfers_active=%v，申請核准=%v、轉帳能力=%v", fl.StoreID, fl.KycApproved, fl.TransfersActive, wantKYC, wantTransfers)
		}
		want := wantKYC && wantTransfers
		if viewComplete[fl.StoreID] != want {
			t.Errorf("店家 %d：onboarding_complete=%v，申請核准=%v、轉帳能力=%v", fl.StoreID, viewComplete[fl.StoreID], wantKYC, wantTransfers)
		}
		if want {
			complete++
		}
	}
	if complete == 0 || complete == len(flags) {
		t.Fatalf("資料集要同時有完成與沒有完成開通的店家：完成 %d／%d", complete, len(flags))
	}
}

// 曝光條件 has_store_name 只看 NOT NULL，靠 stores_store_name_check 保證有值就不是空白，而且「空白」與寫入端的 strings.TrimSpace 同一個集合
// （unicode.IsSpace）。直接以 SQL 寫入（繞過寫入端）：每個 unicode.IsSpace 的碼位（逐一列舉全部碼位，不是手抄的清單），單獨、在開頭、在結尾
// 都被這一條擋下；夾在中間、以及 TrimSpace 不清的鄰近碼位（零寬空白、蒙古文母音分隔符、BOM、word joiner、控制字元）可以寫入，而且
// 與 Go 的判斷相同。mutation：CHECK 改回一參數的 btrim → tab、NBSP、U+3000 等 24 個碼位寫入成功，失敗。
func TestStoreNameCheckRejectsEveryUnicodeSpaceAtEitherEnd(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	insert := `INSERT INTO seller.stores (store_code, account_id, store_name, created_at, updated_at) VALUES ($1, $1, $2, $3, $3)`
	code := int64(3100000000)
	var spaces []rune
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.IsSpace(r) {
			spaces = append(spaces, r)
		}
	}
	if len(spaces) < 25 {
		t.Fatalf("unicode.IsSpace 只有 %d 個碼位", len(spaces))
	}
	for _, r := range spaces {
		for _, name := range []string{string(r), string(r) + "店", "店" + string(r)} {
			if strings.TrimSpace(name) == name {
				t.Fatalf("%q：Go 的 TrimSpace 不清它，測試前提不成立", name)
			}
			code++
			f.wantViolates(t, fmt.Sprintf("店名 %+q", name), "stores_store_name_check", insert, code, name, f.now)
		}
	}
	accepted := []string{}
	for _, r := range spaces {
		accepted = append(accepted, "店"+string(r)+"名")
	}
	for _, r := range []rune{0x200B, 0x180E, 0xFEFF, 0x2060, 0x0008, 0x000E, 0x001F, 0x00A1, 0x3001} {
		accepted = append(accepted, string(r))
	}
	for _, name := range accepted {
		if strings.TrimSpace(name) != name || name == "" {
			t.Fatalf("%q：Go 的 TrimSpace 會清它，測試前提不成立", name)
		}
		code++
		if _, err := f.db.Conn(ctx).Exec(ctx, insert, code, name, f.now); err != nil {
			t.Errorf("店名 %+q：寫入端會接受，DB 也要接受：%v", name, err)
		}
	}
}
