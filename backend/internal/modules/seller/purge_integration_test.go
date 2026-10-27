//go:build integration

package seller_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// 清理之後可以留下值的欄位（其餘一律必須是 NULL）。新增欄位時這份清單逼人決定：留下的要有理由（事實、狀態碼、時間），不是個資。
var (
	storeKept = []string{
		"store_id", "store_code", "account_id", "status", "version", "logo_key", "logo_asset_id", "cover_key", "cover_asset_id",
		"onboarding_completed", "created_at", "updated_at", "pii_purged_at", "current_closed_at", "retired_at",
	}
	kycKept = []string{
		"application_id", "store_id", "status", "version", "seller_type", "product_type",
		"submitted_at", "submission_seq", "decided_at", "created_at", "updated_at", "pii_purged_at",
	}
	registrationKept = []string{
		"store_id", "mer_id", "application_id", "bind_status", "bind_error_code", "bind_attempts", "bound_at", "intent_seq",
		"synced_seq", "sync_error_code", "resubmitted_at", "mer_status", "member_status", "credit_status", "applepay_status",
		"googlepay_status", "atm_status", "outcome", "status_observed_at", "status_checked_at", "created_at", "updated_at", "pii_purged_at",
	}
	observationKept = []string{
		"observation_id", "store_id", "source", "event_key", "mer_status", "member_status", "credit_status", "applepay_status",
		"googlepay_status", "atm_status", "executed_at", "received_at",
	}
)

// loadedStore 建一間把每個個資欄位都填滿的店：profile、KYC（個人或企業，含證件）、核准、PayUni 綁定、輪詢與 webhook 的觀測。
func (f *fixture) loadedStore(t *testing.T, company bool, idCard, bank string) (int64, Store) {
	t.Helper()
	ctx := context.Background()
	accountID, s := f.open(t)
	_, err := f.module.UpdateProfile(ctx, UpdateProfileInput{
		StoreID: s.StoreID, ExpectedVersion: s.Version, Description: ptr("自家種的蔬果"), ContactPhone: ptr("0229010572"),
		TownCode: ptr("63000050"), AddressDetail: ptr("光復路一段 1 號"), Prize: ptr("優良農民"), WebsiteURL: ptr("https://farm.example.invalid"),
	})
	must(t, err)
	draft := kycDraft(accountID, idCard, bank)
	if company {
		draft = companyDraft(accountID, idRep, taxIDOne, bank)
		draft.CompanyPhone, draft.CompanyTownCode, draft.CompanyAddressDetail = ptr("0229010572"), ptr("63000050"), ptr("中山路 2 號")
		draft.CompanyRegisterDate = ptr("2015-03-04")
		draft.CompanyDocuments = &[]KYCDocumentRef{{StagedToken: "co1"}}
	}
	d := f.savedDraft(t, draft)
	sub, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	must(t, approve(f, sub))
	f.sync(t, s.StoreID)
	f.payuni.status = pu.MerchantStatus{MerStatus: "1", MemberStatus: "5", Reason: "補件：身分證影本模糊", Credit: "1"}
	must(t, f.module.PollPayUni(ctx, s.StoreID, PayUniSourcePoll))
	n := notification(FormatPayUniMerID(s.StoreCode), "9", t1300)
	n.MerStatusNote = "備註：負責人電話有誤"
	must(t, f.deliver(t, n))
	return accountID, s
}

// row 讀一列的全部欄位（to_jsonb），讓測試不必逐欄寫死。
func (f *fixture) row(t *testing.T, table, key string, id any) map[string]any {
	t.Helper()
	var raw []byte
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(),
		fmt.Sprintf("SELECT to_jsonb(t) FROM seller.%s t WHERE %s = $1 ORDER BY 1 LIMIT 1", table, key), id).Scan(&raw))
	var out map[string]any
	must(t, json.Unmarshal(raw, &out))
	return out
}

func nonNull(row map[string]any, kept []string) []string {
	var out []string
	for column, v := range row {
		if v != nil && !slices.Contains(kept, column) {
			out = append(out, column)
		}
	}
	slices.Sort(out)
	return out
}

func (f *fixture) applicationID(t *testing.T, storeID int64) string {
	t.Helper()
	var id string
	must(t, f.db.Conn(context.Background()).QueryRow(context.Background(), "SELECT application_id::text FROM seller.kyc_applications WHERE store_id=$1", storeID).Scan(&id))
	return id
}

func (f *fixture) purged(t *testing.T, storeID int64) bool {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM seller.stores WHERE store_id=$1 AND pii_purged_at IS NOT NULL", storeID) == 1
}

// retired 回報店家已退場（第一段清理：公開內容與圖片清掉、發了 StoreRetired）。
func (f *fixture) retired(t *testing.T, storeID int64) bool {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM seller.stores WHERE store_id=$1 AND retired_at IS NOT NULL", storeID) == 1
}

// 清理涵蓋每一個個資欄位：店家、KYC（個人與企業各一）、PayUni 註冊與觀測、證件列。清理之前先確認這些欄位真的有值，免得
// 測試空轉。拿掉任何一個欄位的清理，對應的 NULL 斷言與資料庫的 *_purged_check 都會失敗。
func TestPurgeClearsEveryPersonalColumn(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	type stint struct {
		account int64
		store   Store
		app     string
	}
	var stints []stint
	for _, company := range []bool{false, true} {
		bank := bankAcct
		if company {
			bank = "999888777666"
		}
		account, s := f.loadedStore(t, company, idA, bank)
		stints = append(stints, stint{account, s, f.applicationID(t, s.StoreID)})
	}
	for _, st := range stints {
		store := f.row(t, "stores", "store_id", st.store.StoreID)
		for _, c := range []string{"store_name", "description", "contact_phone", "contact_name", "contact_mobile", "support_email", "town_code", "address_detail", "prize", "website_url"} {
			if store[c] == nil {
				t.Fatalf("測試沒有填 stores.%s", c)
			}
		}
		app := f.row(t, "kyc_applications", "application_id", st.app)
		if app["real_name"] == nil || app["bank_account_enc"] == nil || app["id_card_hash"] == nil && app["rep_id_card_hash"] == nil {
			t.Fatalf("KYC 沒有填滿：%v", nonNull(app, kycKept))
		}
		reg := f.row(t, "payuni_registrations", "store_id", st.store.StoreID)
		if reg["reason"] == nil {
			t.Fatalf("PayUni 註冊沒有填滿：%v", nonNull(reg, registrationKept))
		}
		if f.count(t, "SELECT count(*) FROM seller.payuni_observations WHERE store_id=$1 AND reason IS NOT NULL", st.store.StoreID) == 0 {
			t.Fatal("沒有帶 reason 的觀測")
		}
		if f.count(t, "SELECT count(*) FROM seller.kyc_documents WHERE application_id=$1::uuid", st.app) < 3 {
			t.Fatal("沒有證件")
		}
		certRef := media.Ref{Type: media.RefTypeCertification, ID: st.app}
		f.syncMedia(t, certRef)
		if len(f.media.Referenced(certRef)) < 3 {
			t.Fatalf("清理之前證件的引用應該已登記：%v", f.media.Referenced(certRef))
		}
		_, err := f.module.CloseCurrent(ctx, st.account)
		must(t, err)
	}
	f.now = f.now.Add(StoreRetention + time.Second)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 2 {
		t.Fatalf("purged=%d", n)
	}
	for _, st := range stints {
		if cols := nonNull(f.row(t, "stores", "store_id", st.store.StoreID), storeKept); len(cols) != 0 {
			t.Errorf("stores 沒清：%v", cols)
		}
		if cols := nonNull(f.row(t, "kyc_applications", "application_id", st.app), kycKept); len(cols) != 0 {
			t.Errorf("kyc_applications 沒清：%v", cols)
		}
		if cols := nonNull(f.row(t, "payuni_registrations", "store_id", st.store.StoreID), registrationKept); len(cols) != 0 {
			t.Errorf("payuni_registrations 沒清：%v", cols)
		}
		if f.count(t, "SELECT count(*) FROM seller.payuni_observations WHERE store_id=$1", st.store.StoreID) == 0 {
			t.Error("觀測紀錄不該被刪（狀態碼與時間是事實）")
		}
		for _, o := range f.observations(t, st.store.StoreID) {
			if cols := nonNull(o, observationKept); len(cols) != 0 {
				t.Errorf("payuni_observations 沒清：%v", cols)
			}
		}
		if f.count(t, "SELECT count(*) FROM seller.kyc_documents WHERE application_id=$1::uuid", st.app) != 0 {
			t.Error("證件列沒刪")
		}
		// 保留的事實：狀態碼與流程狀態、審核紀錄。
		reg := f.row(t, "payuni_registrations", "store_id", st.store.StoreID)
		if reg["mer_status"] == nil || reg["outcome"] == "NONE" || reg["bind_status"] != "BOUND" {
			t.Errorf("PayUni 的狀態碼被清掉：%v", reg)
		}
		if f.count(t, "SELECT count(*) FROM seller.kyc_reviews WHERE application_id=$1::uuid", st.app) < 2 {
			t.Error("審核紀錄被清掉")
		}
	}
	// 證件圖片的引用不在清理的 transaction 內解除：刪證件列的同時記成退場並排入同步 job，執行之後一張都不留。
	for _, st := range stints {
		if !slices.Contains(f.jobs.kinds(), "seller.media_sync") {
			t.Fatal("沒有排入證件引用的同步")
		}
		certRef := media.Ref{Type: media.RefTypeCertification, ID: st.app}
		f.syncMedia(t, certRef)
		if got := f.media.Referenced(certRef); len(got) != 0 {
			t.Fatalf("證件引用沒有全部解除：%v", got)
		}
	}
	// 呈現層看到的 Store：已關閉、店名是空字串（墓碑由呈現層依 Status 決定）。
	got, err := f.module.Store(ctx, stints[0].store.StoreID)
	must(t, err)
	if got.Status != Closed || got.StoreName != "" || got.Description != nil || got.ContactPhone != nil || got.AddressDetail != nil {
		t.Fatalf("store=%+v", got)
	}
}

func (f *fixture) observations(t *testing.T, storeID int64) []map[string]any {
	t.Helper()
	rows, err := f.db.Conn(context.Background()).Query(context.Background(), "SELECT to_jsonb(o) FROM seller.payuni_observations o WHERE store_id=$1", storeID)
	must(t, err)
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw []byte
		must(t, rows.Scan(&raw))
		var m map[string]any
		must(t, json.Unmarshal(raw, &m))
		out = append(out, m)
	}
	must(t, rows.Err())
	return out
}

// 只清已關閉、關閉嚴格超過保留期（StoreRetention）的 stint：營業中的、還沒滿保留期的、剛好滿保留期的都不動；差一秒不動、晚一秒才清。
// 拿掉候選查詢的期限條件，這個測試的「保留期前一秒」與「剛好保留期」會被清掉；資料庫的 stores_pii_purged_check 另外擋下直接寫入。
func TestPurgeOnlyTouchesStintsClosedForMoreThanTheStoreRetention(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	_, openStore := f.loadedStore(t, false, idA, bankAcct)
	oldAccount, oldStore := f.loadedStore(t, false, idB, "111222333444")
	_, err := f.module.CloseCurrent(ctx, oldAccount)
	must(t, err)
	closedAt := f.now
	midAccount, midStore := f.loadedStore(t, false, idC, "555666777888")
	f.now = closedAt.Add(11 * 24 * time.Hour)
	_, err = f.module.CloseCurrent(ctx, midAccount)
	must(t, err)
	recentAccount, recentStore := f.loadedStore(t, false, idRep, "123123123123")
	f.now = closedAt.Add(StoreRetention - 24*time.Hour)
	_, err = f.module.CloseCurrent(ctx, recentAccount)
	must(t, err)

	// oldStore 關閉後的保留期前一秒、剛好保留期：都不清。
	f.now = closedAt.Add(StoreRetention - time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 0 {
		t.Fatalf("保留期前一秒不該清：n=%d err=%v", n, err)
	}
	f.now = closedAt.Add(StoreRetention)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 0 {
		t.Fatalf("剛好保留期不該清：n=%d err=%v", n, err)
	}
	f.now = f.now.Add(time.Second)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 1 || !f.purged(t, oldStore.StoreID) {
		t.Fatalf("n=%d", n)
	}
	for name, id := range map[string]int64{"營業中": openStore.StoreID, "關閉 11 天": midStore.StoreID, "關閉 1 天": recentStore.StoreID} {
		if f.purged(t, id) {
			t.Errorf("%s的店被清了", name)
		}
		if f.row(t, "stores", "store_id", id)["store_name"] == nil || f.row(t, "kyc_applications", "store_id", id)["real_name"] == nil ||
			f.row(t, "stores", "store_id", id)["contact_name"] == nil || f.row(t, "stores", "store_id", id)["support_email"] == nil {
			t.Errorf("%s的店個資被動到", name)
		}
	}
	// 資料庫的保底：直接寫入也過不了。
	for name, q := range map[string]string{
		"營業中":     fmt.Sprintf("UPDATE seller.stores SET pii_purged_at=$1, store_name=NULL, description=NULL, contact_phone=NULL, contact_name=NULL, contact_mobile=NULL, support_email=NULL, town_code=NULL, address_detail=NULL, prize=NULL, website_url=NULL WHERE store_id=%d", openStore.StoreID),
		"沒有清聯絡資料": fmt.Sprintf("UPDATE seller.stores SET status='CLOSED', pii_purged_at=$1, store_name=NULL, description=NULL, contact_phone=NULL, town_code=NULL, address_detail=NULL, prize=NULL, website_url=NULL WHERE store_id=%d", recentStore.StoreID),
		"沒有清欄位":   fmt.Sprintf("UPDATE seller.stores SET pii_purged_at=$1, store_name=NULL WHERE store_id=%d", recentStore.StoreID),
	} {
		if _, err := f.db.Conn(ctx).Exec(ctx, q, f.now); err == nil {
			t.Errorf("資料庫接受了不合規的清理：%s", name)
		}
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.kyc_applications SET pii_purged_at=$1 WHERE store_id=$2", f.now, openStore.StoreID); err == nil {
		t.Error("KYC 標記清理卻沒有清欄位")
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE seller.payuni_registrations SET pii_purged_at=$1 WHERE store_id=$2", f.now, openStore.StoreID); err == nil {
		t.Error("PayUni 標記清理卻沒有清欄位")
	}
}

// 措施的身分快照屬於措施：來源的 KYC 被清理之後它還在，同一個身分仍然不能在別的帳號通過認證；處分結束才刪。
func TestSanctionSnapshotSurvivesThePurgeOfItsSourceKYC(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	account, s := f.loadedStore(t, false, idA, bankAcct)
	sanction := f.suspend(t, s.StoreID)
	before := f.count(t, "SELECT count(*) FROM seller.measure_identities")
	if before == 0 {
		t.Fatal("沒有快照")
	}
	_, err := f.module.CloseCurrent(ctx, account)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if f.row(t, "kyc_applications", "store_id", s.StoreID)["id_card_hash"] != nil {
		t.Fatal("KYC 的 hash 沒清")
	}
	if n := f.count(t, "SELECT count(*) FROM seller.kyc_identity_hashes"); n != 0 {
		t.Fatalf("申請的比對 hash 要隨個資清理刪除：%d", n)
	}
	if after := f.count(t, "SELECT count(*) FROM seller.measure_identities"); after != before {
		t.Fatalf("快照被清理動到：%d → %d", before, after)
	}
	other, _ := f.open(t)
	d := f.savedDraft(t, kycDraft(other, idA, "444555666777"))
	if _, err := f.module.SubmitKYC(ctx, SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: other, ExpectedVersion: d.Version}); err == nil {
		t.Fatal("清理之後同一個身分證號不該通過認證")
	}
	// 措施結束才刪快照。
	_, err = f.lift(ctx, sanction)
	must(t, err)
	if n := f.count(t, "SELECT count(*) FROM seller.measure_identities"); n != 0 {
		t.Fatalf("解除之後快照還在：%d", n)
	}
}

// 重跑是 no-op：已清理的列不再是候選，清理時間不變；關店之後才到的 PayUni 通知不會把 reason 寫回去。
func TestPurgeIsIdempotentAndLateNotificationsStayClean(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	account, s := f.loadedStore(t, false, idA, bankAcct)
	_, err := f.module.CloseCurrent(ctx, account)
	must(t, err)
	f.now = f.now.Add(StoreRetention + time.Second)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	stamp := f.row(t, "stores", "store_id", s.StoreID)["pii_purged_at"]
	f.now = f.now.Add(time.Hour)
	if n, err := f.module.PurgePII(ctx); err != nil || n != 0 {
		t.Fatalf("重跑：n=%d err=%v", n, err)
	}
	if f.row(t, "stores", "store_id", s.StoreID)["pii_purged_at"] != stamp {
		t.Fatal("重跑改了清理時間")
	}
	late := notification(FormatPayUniMerID(s.StoreCode), "1", t1500)
	late.MerStatusNote = "關店之後的備註"
	must(t, f.deliver(t, late))
	if f.row(t, "payuni_registrations", "store_id", s.StoreID)["reason"] != nil {
		t.Fatal("關店之後的通知把 reason 寫回註冊列")
	}
	for _, o := range f.observations(t, s.StoreID) {
		if o["reason"] != nil {
			t.Fatalf("關店之後的觀測帶著 reason：%v", o)
		}
	}
}

// 沒有每日上限（舊實作每天只清 100 筆、不迴圈）：250 個符合條件的 stint 一次清完，也不會碰沒資格的。
func TestPurgeIsNotCappedAtOneBatch(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	closedAt := f.now.Add(-StoreRetention - 24*time.Hour)
	_, err := f.db.Conn(ctx).Exec(ctx, `
		WITH s AS (
			INSERT INTO seller.stores (store_code, account_id, status, store_name, created_at, updated_at, current_closed_at)
			SELECT 1000 + g, 100000 + g, 'CLOSED', '店' || g, $1::timestamptz, $1::timestamptz, $1::timestamptz FROM generate_series(1, 250) g
			UNION ALL
			SELECT 5000 + g, 200000 + g, 'CLOSED', '新' || g, $2::timestamptz, $2::timestamptz, $2::timestamptz FROM generate_series(1, 3) g
			RETURNING store_id, created_at)
		INSERT INTO seller.store_closures (store_id, closed_at, closed_by) SELECT store_id, created_at, 'ACCOUNT_OWNER' FROM s`,
		closedAt, f.now.Add(-24*time.Hour))
	must(t, err)
	n, err := f.module.PurgePII(ctx)
	must(t, err)
	if n != 250 {
		t.Fatalf("purged=%d", n)
	}
	if left := f.count(t, "SELECT count(*) FROM seller.stores WHERE status='CLOSED' AND pii_purged_at IS NULL"); left != 3 {
		t.Fatalf("沒清的=%d（應該只剩關閉 10 天的 3 間）", left)
	}
}
