package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------

const (
	prefBuyerA int64 = 2623900520
	prefBuyerB int64 = 2688637900
)

var prefNow = time.Date(2026, 8, 30, 3, 4, 5, 0, time.UTC)

// memPrefRepo 照 buyer_invoice_preferences.sql 的**述詞形狀**實作，不是照介面語意。
//
// 🔴 每一支都逐字帶四條述詞（id／buyer_id／deleted_at IS NULL／superseded_at IS NULL）：
// 把 buyer_id 從 fake 裡拿掉，Tier A 的測試就會全部變成「驗了個寂寞」——fake 放行，
// 而真正的 SQL 才是攔截點。fake 與 SQL 的述詞不一致時，測試綠而線上破。
type memPrefRepo struct {
	domain.BuyerPreferenceRepository

	rows  map[int64]domain.BuyerPreference
	calls []string

	// lastCreate 留給「明碼與遮罩是不是照規則交到 repository」那一格。
	lastCreate domain.NewBuyerPreference
}

func newMemPrefRepo(rows ...domain.BuyerPreference) *memPrefRepo {
	r := &memPrefRepo{rows: map[int64]domain.BuyerPreference{}}
	for _, row := range rows {
		r.rows[row.ID] = row
	}
	return r
}

func aliveRow(p domain.BuyerPreference) bool {
	return p.DeletedAt == nil && p.SupersededAt == nil
}

func (r *memPrefRepo) snapshot() map[int64]domain.BuyerPreference {
	out := make(map[int64]domain.BuyerPreference, len(r.rows))
	for k, v := range r.rows {
		out[k] = v
	}
	return out
}

func (r *memPrefRepo) restore(s map[int64]domain.BuyerPreference) { r.rows = s }

func (r *memPrefRepo) GetByID(_ context.Context, id, buyerID int64) (domain.BuyerPreference, error) {
	r.calls = append(r.calls, "GetByID")
	row, ok := r.rows[id]
	if !ok || row.BuyerID != buyerID || !aliveRow(row) {
		return domain.BuyerPreference{}, profileNotFound()
	}
	return row, nil
}

func (r *memPrefRepo) ListByBuyer(_ context.Context, buyerID int64, limit int32) ([]domain.BuyerPreference, error) {
	r.calls = append(r.calls, "ListByBuyer")
	out := []domain.BuyerPreference{}
	for _, row := range r.rows {
		if row.BuyerID == buyerID && aliveRow(row) && int32(len(out)) < limit {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *memPrefRepo) Create(_ context.Context, in domain.NewBuyerPreference, now time.Time) (domain.BuyerPreference, bool, error) {
	r.calls = append(r.calls, "Create")
	r.lastCreate = in
	// ON CONFLICT DO NOTHING：uq_bip_carrier_dedup（買家×載具型別×值）與
	// uq_bip_taxid_dedup（買家×統編），兩者的述詞都只涵蓋存活列。
	for _, row := range r.rows {
		if row.BuyerID != in.BuyerID || !aliveRow(row) {
			continue
		}
		if in.CarrierValue != "" && derefString(row.CarrierValueMask) == in.CarrierMask &&
			derefString(row.CarrierType) == in.CarrierType {
			return domain.BuyerPreference{}, false, nil
		}
		if in.TaxID != "" && derefString(row.TaxID) == in.TaxID {
			return domain.BuyerPreference{}, false, nil
		}
	}
	row := domain.BuyerPreference{
		ID:             in.ID,
		BuyerID:        in.BuyerID,
		PreferenceKind: in.PreferenceKind,
		IsDefault:      in.IsDefault,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if in.CarrierType != "" {
		row.CarrierType = strPtr(in.CarrierType)
	}
	if in.CarrierMask != "" {
		row.CarrierValueMask = strPtr(in.CarrierMask)
	}
	if in.TaxID != "" {
		row.TaxID = strPtr(in.TaxID)
	}
	if in.TaxTitle != "" {
		row.TaxTitle = strPtr(in.TaxTitle)
	}
	if in.DonationCode != "" {
		row.DonationCode = strPtr(in.DonationCode)
	}
	if in.Label != "" {
		row.Label = strPtr(in.Label)
	}
	r.rows[row.ID] = row
	return row, true, nil
}

func (r *memPrefRepo) ClearDefault(_ context.Context, buyerID int64, now time.Time) (int64, error) {
	r.calls = append(r.calls, "ClearDefault")
	var n int64
	for id, row := range r.rows {
		if row.BuyerID == buyerID && aliveRow(row) && row.IsDefault {
			row.IsDefault = false
			row.UpdatedAt = now
			r.rows[id] = row
			n++
		}
	}
	return n, nil
}

func (r *memPrefRepo) SetDefault(_ context.Context, id, buyerID int64, now time.Time) (bool, error) {
	r.calls = append(r.calls, "SetDefault")
	row, ok := r.rows[id]
	if !ok || row.BuyerID != buyerID || !aliveRow(row) {
		return false, nil
	}
	row.IsDefault = true
	row.UpdatedAt = now
	r.rows[id] = row
	return true, nil
}

func (r *memPrefRepo) Supersede(_ context.Context, id, buyerID, by int64, now time.Time) (bool, error) {
	r.calls = append(r.calls, "Supersede")
	row, ok := r.rows[id]
	if !ok || row.BuyerID != buyerID || !aliveRow(row) {
		return false, nil
	}
	row.SupersededAt = &now
	row.SupersededByID = &by
	row.IsDefault = false
	row.UpdatedAt = now
	r.rows[id] = row
	return true, nil
}

func (r *memPrefRepo) SoftDelete(_ context.Context, id, buyerID int64, now time.Time) (bool, error) {
	r.calls = append(r.calls, "SoftDelete")
	row, ok := r.rows[id]
	if !ok || row.BuyerID != buyerID || !aliveRow(row) {
		return false, nil
	}
	row.DeletedAt = &now
	row.IsDefault = false
	row.UpdatedAt = now
	r.rows[id] = row
	return true, nil
}

// rollbackTx 是會**真的 rollback** 的 TxManager：callback 回錯時把 repo 還原成進入
// 交易前的狀態。
//
// 🔴 directTx 在這裡不夠用：本檔要驗的不變式之一是「SetDefault 打一個不屬於你的 id
// 時，你原本的預設不能被清掉」，而那件事只有在 rollback 真的發生時才成立。用
// directTx 測會得到一個**綠燈的假保證**——清掉了，但測試看不到。
type rollbackTx struct{ repo *memPrefRepo }

func (t rollbackTx) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	before := t.repo.snapshot()
	if err := fn(ctx); err != nil {
		t.repo.restore(before)
		return err
	}
	return nil
}

func newPreferenceHarness(t *testing.T, rows ...domain.BuyerPreference) (*Preference, *memPrefRepo) {
	t.Helper()
	repo := newMemPrefRepo(rows...)
	var seq int64 = 9000000000
	uc := NewPreference(rollbackTx{repo: repo}, repo, nil, func() int64 {
		seq++
		return seq
	}, nil).WithClock(func() time.Time { return prefNow })
	return uc, repo
}

// stubBarcodeVerifier 是 #3256 入口層驗證的替身。
//
// **記下被驗的值不只記次數**：只記次數的替身對「驗錯了東西」（例如把 label 或空字串
// 送去驗）零鑑別力。
type stubBarcodeVerifier struct {
	calls []string
	err   error
}

func (s *stubBarcodeVerifier) VerifyMobileBarcode(_ context.Context, barcode string) error {
	s.calls = append(s.calls, barcode)
	return s.err
}

func prefAmegoErr(code string) error {
	return &amego.APIError{Endpoint: "/json/barcode", Code: code, Msg: "x", Class: amego.ClassPermanent}
}

// 手機條碼在**存進 DB 之前**要先驗存在性（#3256 入口層）。
//
// 🔴 這一層之前不存在：dev 實測時我送了一個明顯無效的 /ZZZZZZZ，profile 照樣建立成功
// （displayValue 都遮罩好了），直到付款後開票才被 AMEGO 擋下。買家要等到那時才知道
// 自己打錯了，而那時只剩「轉人工」一條路。
func TestPreferenceCreate_RejectsNonExistentBarcode(t *testing.T) {
	for _, tc := range []struct{ name, code string }{
		{"不存在", "9000113"},
		{"格式錯誤", "9000112"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc, repo := newPreferenceHarness(t)
			v := &stubBarcodeVerifier{err: prefAmegoErr(tc.code)}
			uc = uc.WithBarcodeVerifier(v)

			_, err := uc.Create(context.Background(), CreateProfileInput{
				BuyerID: prefBuyerA, CarrierType: codemeta.InvoiceCarrierTypeMobileBarcode.Value,
				Value: "/ZZZZZZZ", Label: "測試",
			})
			assertBizCode(t, err, bizcode.InvoiceCarrierInvalid)
			if len(v.calls) != 1 || v.calls[0] != "/ZZZZZZZ" {
				t.Fatalf("應驗 value 本身一次，實得 %v", v.calls)
			}
			// 🔴 擋下來就**不得落庫**：只回錯誤但留下一列，等於驗了個寂寞。
			if n := len(repo.rows); n != 0 {
				t.Fatalf("驗證未過不得建立 profile，實得 %d 列", n)
			}
		})
	}
}

// 🔴 fail-open：驗證服務本身出問題時不得擋住買家存載具。
//
// 這支是上面那支的鑑別力來源：少了它，一個「只要 VerifyMobileBarcode 回錯就擋」的
// 實作也會通過——而那會讓一次 AMEGO 逾時把所有買家的載具設定都擋掉。
func TestPreferenceCreate_BarcodeVerifyFailureIsFailOpen(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"未知碼", prefAmegoErr("9999999")},
		{"網路錯誤", errors.New("dial tcp: i/o timeout")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc, repo := newPreferenceHarness(t)
			uc = uc.WithBarcodeVerifier(&stubBarcodeVerifier{err: tc.err})

			if _, err := uc.Create(context.Background(), CreateProfileInput{
				BuyerID: prefBuyerA, CarrierType: codemeta.InvoiceCarrierTypeMobileBarcode.Value,
				Value: "/ABC123+", Label: "測試",
			}); err != nil {
				t.Fatalf("驗證本身失敗時應照常建立（fail-open）：%v", err)
			}
			if len(repo.rows) != 1 {
				t.Fatalf("應建立 1 列，實得 %d", len(repo.rows))
			}
		})
	}
}

// 非手機條碼不呼叫驗證：/json/barcode 只驗手機條碼，拿捐贈碼去問是白費一次配額。
func TestPreferenceCreate_NonBarcodeSkipsVerification(t *testing.T) {
	uc, _ := newPreferenceHarness(t)
	v := &stubBarcodeVerifier{}
	uc = uc.WithBarcodeVerifier(v)

	if _, err := uc.Create(context.Background(), CreateProfileInput{
		BuyerID: prefBuyerA, CarrierType: codemeta.InvoiceCarrierTypeDonation.Value,
		Value: "12345", Label: "捐贈",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(v.calls) != 0 {
		t.Fatalf("非手機條碼不得呼叫載具驗證，實得 %v", v.calls)
	}
}

// carrierRow 造一列存活的手機條碼常用發票資料。
func carrierRow(id, buyerID int64, mask string, isDefault bool) domain.BuyerPreference {
	return domain.BuyerPreference{
		ID:               id,
		BuyerID:          buyerID,
		PreferenceKind:   codemeta.InvoiceCarrierTypeMobileBarcode.Value,
		CarrierType:      strPtr("3J0002"),
		CarrierValueMask: strPtr(mask),
		IsDefault:        isDefault,
		CreatedAt:        prefNow,
		UpdatedAt:        prefNow,
	}
}

// countAliveDefaults 是「恰好一個預設」不變式的量測（uq_bip_default 的 Go 端影子）。
func countAliveDefaults(repo *memPrefRepo, buyerID int64) int {
	n := 0
	for _, row := range repo.rows {
		if row.BuyerID == buyerID && aliveRow(row) && row.IsDefault {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// 端點 2：Create
// ---------------------------------------------------------------------------

func TestPreference_Create_MobileBarcode(t *testing.T) {
	uc, repo := newPreferenceHarness(t)

	got, err := uc.Create(context.Background(), CreateProfileInput{
		BuyerID:     prefBuyerA,
		CarrierType: codemeta.InvoiceCarrierTypeMobileBarcode.Value,
		Value:       "/T7R3U+P",
		Label:       "我的手機條碼",
	})
	if err != nil {
		t.Fatalf("建立應成功，實際 err = %v", err)
	}
	if got.ProfileID == 0 || got.CarrierType != codemeta.InvoiceCarrierTypeMobileBarcode.Value {
		t.Fatalf("回應的 profileId／carrierType 不對：%+v", got)
	}

	in := repo.lastCreate
	// chk_bip_shape 要求 MOBILE_BARCODE 的 carrier_type 恆為 '3J0002'。
	if in.CarrierType != "3J0002" {
		t.Fatalf("carrier_type 必須是 AMEGO 原值 3J0002，實際 %q", in.CarrierType)
	}
	// 明碼要原樣交到 repository（加密在那一層），遮罩由本層算。
	if in.CarrierValue != "/T7R3U+P" {
		t.Fatalf("明碼必須原樣交給 repository（加密收斂在該層），實際 %q", in.CarrierValue)
	}
	if in.CarrierMask != maskCarrierValue("/T7R3U+P") || in.CarrierMask == "/T7R3U+P" {
		t.Fatalf("carrier_value_mask 必須是遮罩後的值，實際 %q", in.CarrierMask)
	}
	// 🔴 回應恆為遮罩：明碼一個字都不得出服務邊界（§6.1.1 遮罩段）。
	if got.DisplayValue == "/T7R3U+P" {
		t.Fatalf("displayValue 吐出了明碼：%q", got.DisplayValue)
	}
}

func TestPreference_Create_SetDefaultClearsPrevious(t *testing.T) {
	uc, repo := newPreferenceHarness(t, carrierRow(101, prefBuyerA, "/AAA***1", true))

	if _, err := uc.Create(context.Background(), CreateProfileInput{
		BuyerID:     prefBuyerA,
		CarrierType: codemeta.InvoiceCarrierTypeMobileBarcode.Value,
		Value:       "/T7R3U+P",
		SetDefault:  true,
	}); err != nil {
		t.Fatalf("建立應成功，實際 err = %v", err)
	}

	if n := countAliveDefaults(repo, prefBuyerA); n != 1 {
		t.Fatalf("存活列的預設必須恰好一個（uq_bip_default），實際 %d", n)
	}
	if !repo.rows[101].IsDefault == false {
		t.Fatalf("舊的預設沒有被清掉")
	}
	// 先清後設：順序反過來會撞 uq_bip_default。
	if idx := callIndex(repo.calls, "ClearDefault"); idx < 0 || idx > callIndex(repo.calls, "Create") {
		t.Fatalf("ClearDefault 必須在 Create 之前，實際呼叫序 %v", repo.calls)
	}
}

func TestPreference_Create_Rejects(t *testing.T) {
	cases := []struct {
		name string
		in   CreateProfileInput
		code int32
	}{
		{"值域外的 carrierType", CreateProfileInput{BuyerID: prefBuyerA, CarrierType: "MEMBER", Value: "x"}, bizcode.InvoiceCarrierInvalid},
		{"手機條碼格式錯", CreateProfileInput{BuyerID: prefBuyerA, CarrierType: "MOBILE_BARCODE", Value: "T7R3U+P"}, bizcode.InvoiceCarrierInvalid},
		{"自然人憑證格式錯", CreateProfileInput{BuyerID: prefBuyerA, CarrierType: "CITIZEN_CERT", Value: "AB1234"}, bizcode.InvoiceCarrierInvalid},
		{"捐贈碼格式錯", CreateProfileInput{BuyerID: prefBuyerA, CarrierType: "DONATION", Value: "12"}, bizcode.InvoiceCarrierInvalid},
		{"統編型帶了 value", CreateProfileInput{BuyerID: prefBuyerA, CarrierType: "TAX_ID", Value: "/T7R3U+P", BuyerTaxID: "22223223", BuyerName: "範例生鮮"}, bizcode.InvoiceCarrierInvalid},
		{"統編型缺抬頭", CreateProfileInput{BuyerID: prefBuyerA, CarrierType: "TAX_ID", BuyerTaxID: "22223223"}, bizcode.InvoiceCarrierInvalid},
		{"載具＋統編（三選一違反）", CreateProfileInput{BuyerID: prefBuyerA, CarrierType: "MOBILE_BARCODE", Value: "/T7R3U+P", BuyerTaxID: "22223223"}, bizcode.InvoiceTargetConflict},
		{"捐贈＋統編（三選一違反）", CreateProfileInput{BuyerID: prefBuyerA, CarrierType: "DONATION", Value: "5380", BuyerTaxID: "22223223"}, bizcode.InvoiceTargetConflict},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uc, repo := newPreferenceHarness(t)
			_, err := uc.Create(context.Background(), c.in)
			if !isBizCode(err, c.code) {
				t.Fatalf("預期業務碼 %d，實際 err = %v", c.code, err)
			}
			if len(repo.rows) != 0 {
				t.Fatalf("驗證失敗時不得留下任何列，實際 %d 列", len(repo.rows))
			}
		})
	}
}

func TestPreference_Create_LimitExceeded(t *testing.T) {
	rows := make([]domain.BuyerPreference, 0, buyerProfileCap)
	for i := 0; i < buyerProfileCap; i++ {
		rows = append(rows, carrierRow(int64(200+i), prefBuyerA, "/A"+string(rune('a'+i))+"***1", false))
	}
	uc, _ := newPreferenceHarness(t, rows...)

	_, err := uc.Create(context.Background(), CreateProfileInput{
		BuyerID:     prefBuyerA,
		CarrierType: codemeta.InvoiceCarrierTypeMobileBarcode.Value,
		Value:       "/T7R3U+P",
	})
	if !isBizCode(err, bizcode.InvoiceProfileLimitExceeded) {
		t.Fatalf("第 %d 筆必須回 9303，實際 err = %v", buyerProfileCap+1, err)
	}
}

// TestPreference_Create_LimitIsPerBuyer 釘住「上限述詞帶 buyer_id」：
// 別人的 20 筆不該讓我建不了第 1 筆。
func TestPreference_Create_LimitIsPerBuyer(t *testing.T) {
	rows := make([]domain.BuyerPreference, 0, buyerProfileCap)
	for i := 0; i < buyerProfileCap; i++ {
		rows = append(rows, carrierRow(int64(300+i), prefBuyerB, "/B"+string(rune('a'+i))+"***1", false))
	}
	uc, _ := newPreferenceHarness(t, rows...)

	if _, err := uc.Create(context.Background(), CreateProfileInput{
		BuyerID:     prefBuyerA,
		CarrierType: codemeta.InvoiceCarrierTypeMobileBarcode.Value,
		Value:       "/T7R3U+P",
	}); err != nil {
		t.Fatalf("另一位買家的筆數不該影響我，實際 err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// 端點 3：Update（supersede 語意）
// ---------------------------------------------------------------------------

func TestPreference_Update_SupersedesAndInheritsDefault(t *testing.T) {
	uc, repo := newPreferenceHarness(t, carrierRow(401, prefBuyerA, "/AAA***1", true))

	got, err := uc.Update(context.Background(), UpdateProfileInput{
		BuyerID:   prefBuyerA,
		ProfileID: 401,
		Value:     "/T7R3U+P",
		Label:     "改個名字",
	})
	if err != nil {
		t.Fatalf("編輯應成功，實際 err = %v", err)
	}
	if got.ProfileID == 401 {
		t.Fatalf("編輯必須產生新列（append-only，I-14），實際仍是舊 id")
	}

	old := repo.rows[401]
	if old.SupersededAt == nil || old.SupersededByID == nil || *old.SupersededByID != got.ProfileID {
		t.Fatalf("舊列必須指向新列，實際 %+v", old)
	}
	if old.IsDefault {
		t.Fatalf("被取代的列不得仍是預設（chk_bip_default_alive）")
	}
	// 🔴 預設必須被繼承：編輯自己的預設載具之後預設消失，下一次結帳會靜默退成
	// 無載具發票，而買家什麼都沒改。
	if !repo.rows[got.ProfileID].IsDefault {
		t.Fatalf("新列必須繼承 is_default")
	}
	if n := countAliveDefaults(repo, prefBuyerA); n != 1 {
		t.Fatalf("存活列的預設必須恰好一個，實際 %d", n)
	}
	// 先建新列、再 supersede：fk_bip_superseded 不是 DEFERRABLE，反過來會 23503。
	if callIndex(repo.calls, "Create") > callIndex(repo.calls, "Supersede") {
		t.Fatalf("Create 必須在 Supersede 之前（fk_bip_superseded），實際呼叫序 %v", repo.calls)
	}
}

func TestPreference_Update_ValidatesAgainstStoredKind(t *testing.T) {
	uc, _ := newPreferenceHarness(t, carrierRow(402, prefBuyerA, "/AAA***1", false))

	// 舊列是手機條碼 ⇒ 新值必須通過手機條碼的正則，不是「隨便一個字串都行」。
	_, err := uc.Update(context.Background(), UpdateProfileInput{
		BuyerID:   prefBuyerA,
		ProfileID: 402,
		Value:     "5380",
	})
	if !isBizCode(err, bizcode.InvoiceCarrierInvalid) {
		t.Fatalf("預期 9305，實際 err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// 端點 4：Delete
// ---------------------------------------------------------------------------

func TestPreference_Delete(t *testing.T) {
	uc, repo := newPreferenceHarness(t, carrierRow(501, prefBuyerA, "/AAA***1", false))

	if err := uc.Delete(context.Background(), prefBuyerA, 501); err != nil {
		t.Fatalf("刪除應成功，實際 err = %v", err)
	}
	if repo.rows[501].DeletedAt == nil {
		t.Fatalf("列沒有被軟刪")
	}
}

// TestPreference_Delete_DefaultIsBlocked 釘住 9304：刪除預設一律擋、不自動改指。
//
// 自動改指會讓下一次結帳靜默換成另一種載具，而發票開錯載具只能作廢重開。
func TestPreference_Delete_DefaultIsBlocked(t *testing.T) {
	uc, repo := newPreferenceHarness(t, carrierRow(502, prefBuyerA, "/AAA***1", true))

	err := uc.Delete(context.Background(), prefBuyerA, 502)
	if !isBizCode(err, bizcode.InvoiceProfileDefaultRequired) {
		t.Fatalf("預期 9304，實際 err = %v", err)
	}
	if repo.rows[502].DeletedAt != nil {
		t.Fatalf("被擋下時不得留下任何刪除痕跡")
	}
}

// ---------------------------------------------------------------------------
// 端點 5：SetDefault / ClearDefault
// ---------------------------------------------------------------------------

func TestPreference_SetDefault_ExactlyOneDefault(t *testing.T) {
	uc, repo := newPreferenceHarness(t,
		carrierRow(601, prefBuyerA, "/AAA***1", true),
		carrierRow(602, prefBuyerA, "/BBB***2", false),
	)

	got, err := uc.SetDefault(context.Background(), prefBuyerA, 602)
	if err != nil {
		t.Fatalf("設預設應成功，實際 err = %v", err)
	}
	if !got.IsDefault || got.ProfileID != 602 {
		t.Fatalf("回應必須是轉移後的那一列，實際 %+v", got)
	}
	if n := countAliveDefaults(repo, prefBuyerA); n != 1 {
		t.Fatalf("存活列的預設必須恰好一個，實際 %d", n)
	}
	if repo.rows[601].IsDefault {
		t.Fatalf("舊的預設沒有被清掉")
	}
}

// TestPreference_SetDefault_ForeignIDKeepsMyDefault 是本檔最重要的一格。
//
// ClearDefault 與 SetDefault 若不同交易（或失敗不 rollback），打一個不屬於你的 id
// 會把你原本的預設清掉：前端收到 404，而你的預設**真的沒了**，下一次結帳靜默開成
// 無載具發票。
func TestPreference_SetDefault_ForeignIDKeepsMyDefault(t *testing.T) {
	uc, repo := newPreferenceHarness(t,
		carrierRow(701, prefBuyerA, "/AAA***1", true),
		carrierRow(702, prefBuyerB, "/BBB***2", false),
	)

	_, err := uc.SetDefault(context.Background(), prefBuyerA, 702) // 別人的列
	if !isBizCode(err, bizcode.InvoiceProfileNotFound) {
		t.Fatalf("越權必須回 9302（與查無不可分辨），實際 err = %v", err)
	}
	if !repo.rows[701].IsDefault {
		t.Fatalf("失敗的設定不得清掉我原本的預設 —— ClearDefault 與 SetDefault 必須同交易")
	}
	if repo.rows[702].IsDefault {
		t.Fatalf("別人的列被設成了預設：buyer_id 沒有進述詞")
	}
}

func TestPreference_ClearDefault(t *testing.T) {
	uc, repo := newPreferenceHarness(t,
		carrierRow(801, prefBuyerA, "/AAA***1", true),
		carrierRow(802, prefBuyerB, "/BBB***2", true),
	)

	if err := uc.ClearDefault(context.Background(), prefBuyerA); err != nil {
		t.Fatalf("清除預設應成功，實際 err = %v", err)
	}
	if repo.rows[801].IsDefault {
		t.Fatalf("我的預設沒有被清掉")
	}
	if !repo.rows[802].IsDefault {
		t.Fatalf("清除預設打到了別人的列：buyer_id 沒有進述詞")
	}
}

// ---------------------------------------------------------------------------
// Tier A：越權與查無不可分辨（usecase 層的形狀，逐位元比對在 grpc 層）
// ---------------------------------------------------------------------------

func TestPreference_TierA_ForeignAndMissingAreIdentical(t *testing.T) {
	uc, _ := newPreferenceHarness(t, carrierRow(901, prefBuyerB, "/BBB***2", false))

	foreign := uc.Delete(context.Background(), prefBuyerA, 901)   // 存在但不是我的
	missing := uc.Delete(context.Background(), prefBuyerA, 90199) // 不存在

	if !isBizCode(foreign, bizcode.InvoiceProfileNotFound) || !isBizCode(missing, bizcode.InvoiceProfileNotFound) {
		t.Fatalf("兩者都必須是 9302，實際 foreign=%v missing=%v", foreign, missing)
	}
	if foreign.Error() != missing.Error() {
		t.Fatalf("越權與查無的訊息必須逐字相同：\n foreign=%q\n missing=%q", foreign.Error(), missing.Error())
	}
}

func callIndex(ss []string, want string) int {
	for i, s := range ss {
		if s == want {
			return i
		}
	}
	return -1
}
