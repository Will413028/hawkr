package order

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

const (
	testBuyerID  int64 = 2623900520
	otherBuyerID int64 = 2688637900
	testPrefID   int64 = 2685914682123456
)

// fakePrefs 是完整能力的 reader（GetByID／GetDefault／OpenCarrierValue 都真的實作）。
//
// 🔴 GetByID 的歸屬述詞在這裡是**模擬 SQL 的 WHERE**：真實 repository 把
// `buyer_id = ?` 寫進 SQL，查不到與不屬於你不可分辨。本假物件必須複製同一個語意，
// 否則「歸屬錯誤」那支測試驗的就不是它要驗的東西。
type fakePrefs struct {
	rows map[int64]domain.BuyerPreference
	// defaultOf[buyerID] = preference id
	defaultOf map[int64]int64
	plaintext map[int64]string

	getByIDCalls int
	gotBuyerID   int64

	getByIDErr error
	defaultErr error
	openErr    error
}

func (f *fakePrefs) GetByID(_ context.Context, id, buyerID int64) (domain.BuyerPreference, error) {
	f.getByIDCalls++
	f.gotBuyerID = buyerID
	if f.getByIDErr != nil {
		return domain.BuyerPreference{}, f.getByIDErr
	}
	row, ok := f.rows[id]
	if !ok || row.BuyerID != buyerID || row.DeletedAt != nil || row.SupersededAt != nil {
		return domain.BuyerPreference{}, pkgerrors.NewBusiness(bizcode.InvoiceProfileNotFound, "not found")
	}
	return row, nil
}

func (f *fakePrefs) GetDefault(_ context.Context, buyerID int64) (domain.BuyerPreference, error) {
	if f.defaultErr != nil {
		return domain.BuyerPreference{}, f.defaultErr
	}
	id, ok := f.defaultOf[buyerID]
	if !ok {
		return domain.BuyerPreference{}, ErrNoDefaultPreference
	}
	return f.rows[id], nil
}

func (f *fakePrefs) OpenCarrierValue(_ context.Context, p domain.BuyerPreference) (string, error) {
	if f.openErr != nil {
		return "", f.openErr
	}
	if p.CarrierValueCiphertext == nil {
		return "", nil
	}
	return f.plaintext[p.ID], nil
}

func strp(s string) *string { return &s }

func mobilePref(id, buyerID int64) domain.BuyerPreference {
	return domain.BuyerPreference{
		ID: id, BuyerID: buyerID,
		PreferenceKind:         codemeta.InvoiceCarrierTypeMobileBarcode.Value,
		CarrierType:            strp(domain.AmegoCarrierMobileBarcode),
		CarrierValueCiphertext: strp("ciphertext"),
		CarrierValueHash:       strp("hash"),
	}
}

func newPrefs(rows ...domain.BuyerPreference) *fakePrefs {
	f := &fakePrefs{
		rows:      map[int64]domain.BuyerPreference{},
		defaultOf: map[int64]int64{},
		plaintext: map[int64]string{},
	}
	for _, r := range rows {
		f.rows[r.ID] = r
	}
	return f
}

// ---------------------------------------------------------------------------
// 四條路徑
// ---------------------------------------------------------------------------

func TestResolveCarrier_None(t *testing.T) {
	// B2B 一律走 NONE：統編與抬頭由結帳快照直接帶，沒有載具要解析。
	prefs := newPrefs()
	got, err := resolveCarrier(context.Background(), prefs,
		codemeta.InvoiceCarrierSelectionModeNone.Value, 0, testBuyerID,
		"12345678", "範例生鮮股份有限公司", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.BuyerTaxID != "12345678" || got.BuyerTitle != "範例生鮮股份有限公司" {
		t.Fatalf("統編／抬頭沒有透傳：%+v", got)
	}
	if got.CarrierType.Code != "" || got.CarrierID1 != "" || got.DonationCode != "" {
		t.Fatalf("NONE 不該解析出任何載具：%+v", got)
	}
	if got.OwnershipMismatch {
		t.Fatalf("NONE 不該觸發歸屬錯誤")
	}
	if prefs.getByIDCalls != 0 {
		t.Fatalf("NONE 不該去讀 buyer_invoice_preferences（讀了 %d 次）", prefs.getByIDCalls)
	}
}

func TestResolveCarrier_Donation(t *testing.T) {
	got, err := resolveCarrier(context.Background(), newPrefs(),
		codemeta.InvoiceCarrierSelectionModeDonation.Value, 0, testBuyerID, "", "", "9527")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CarrierType.Code != codemeta.InvoiceCarrierTypeDonation.Code {
		t.Fatalf("carrier type = %q", got.CarrierType.Code)
	}
	if got.DonationCode != "9527" {
		t.Fatalf("donation_code = %q", got.DonationCode)
	}
	if got.BuyerTaxID != "" || got.CarrierID1 != "" {
		t.Fatalf("I-04 互斥：DONATION 不該同時帶統編或載具：%+v", got)
	}
}

func TestResolveCarrier_Donation_EmptyCodeFailsLoud(t *testing.T) {
	// chk_ois_donation_pair 是雙向等式 ⇒ 走到這裡代表快照壞了。
	// 靜默開成無載具會把買家的捐贈意願吞掉，而發票不可回收。
	if _, err := resolveCarrier(context.Background(), newPrefs(),
		codemeta.InvoiceCarrierSelectionModeDonation.Value, 0, testBuyerID, "", "", ""); err == nil {
		t.Fatalf("DONATION 而 donation_code 為空必須 fail-loud")
	}
}

func TestResolveCarrier_ExplicitPreference_OwnershipOK(t *testing.T) {
	prefs := newPrefs(mobilePref(testPrefID, testBuyerID))
	prefs.plaintext[testPrefID] = "/T7R3U+P"

	got, err := resolveCarrier(context.Background(), prefs,
		codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value, testPrefID, testBuyerID, "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prefs.gotBuyerID != testBuyerID {
		t.Fatalf("歸屬述詞收到的 buyer_id = %d，want %d（必須取自訂單）", prefs.gotBuyerID, testBuyerID)
	}
	if got.OwnershipMismatch {
		t.Fatalf("歸屬正確時不該標 mismatch")
	}
	if got.CarrierType.Code != codemeta.InvoiceCarrierTypeMobileBarcode.Code {
		t.Fatalf("carrier type = %q", got.CarrierType.Code)
	}
	if got.CarrierID1 != "/T7R3U+P" {
		t.Fatalf("載具明文 = %q", got.CarrierID1)
	}
}

// TestResolveCarrier_ExplicitPreference_OwnershipMismatch 是本檔最重要的一支（I-47）。
//
// 🔴 陰性斷言在最後兩行：歸屬錯誤時**不得**回一個「看起來解析成功的無載具結果」。
// 那正是「fallback 成無載具」的形狀，而它會把 IDOR 靜默吞掉。
func TestResolveCarrier_ExplicitPreference_OwnershipMismatch(t *testing.T) {
	// 這一列真的存在，但屬於**別的**買家。
	prefs := newPrefs(mobilePref(testPrefID, otherBuyerID))
	prefs.plaintext[testPrefID] = "/T7R3U+P"

	got, err := resolveCarrier(context.Background(), prefs,
		codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value, testPrefID, testBuyerID, "", "", "")
	if err != nil {
		t.Fatalf("歸屬錯誤不是傳輸錯誤，應回結果 ＋ nil error：%v", err)
	}
	if !got.OwnershipMismatch {
		t.Fatalf("歸屬錯誤必須標成 OwnershipMismatch（否則會 fallback 成無載具，把 IDOR 靜默吞掉）")
	}
	if got.CarrierType.Code != "" || got.CarrierID1 != "" || got.DonationCode != "" ||
		got.BuyerTaxID != "" || got.BuyerTitle != "" {
		t.Fatalf("歸屬錯誤時不得交出任何買受人資料：%+v", got)
	}
}

func TestResolveCarrier_ExplicitPreference_TransportErrorPropagates(t *testing.T) {
	// 「連線斷了」與「不屬於你」的正確處置相反（重試 vs 轉人工）⇒ 不得吞成 mismatch。
	prefs := newPrefs()
	prefs.getByIDErr = errors.New("connection reset by peer")

	if _, err := resolveCarrier(context.Background(), prefs,
		codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value, testPrefID, testBuyerID, "", "", ""); err == nil {
		t.Fatalf("非業務碼的錯誤必須往上拋（fail-loud → DLQ），不得吞成歸屬錯誤")
	}
}

func TestResolveCarrier_ExplicitPreference_RequiresBuyerID(t *testing.T) {
	prefs := newPrefs(mobilePref(testPrefID, testBuyerID))
	if _, err := resolveCarrier(context.Background(), prefs,
		codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value, testPrefID, 0, "", "", ""); err == nil {
		t.Fatalf("沒有 buyer_id 就驗不了歸屬，必須 fail-loud 而不是『不驗』")
	}
}

func TestResolveCarrier_ExplicitPreference_MissingIDFailsLoud(t *testing.T) {
	if _, err := resolveCarrier(context.Background(), newPrefs(),
		codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value, 0, testBuyerID, "", "", ""); err == nil {
		t.Fatalf("EXPLICIT_PREFERENCE 而 preference_id 為空必須 fail-loud（chk_ois_pref_pair）")
	}
}

// TestResolveCarrier_UseBuyerDefault 驗 §6.1.2(b) 的**兩步**鏈。
//
// ⚠️ 舊版（§5.4.3 散文與 CodeMeta Description 的殘留）寫的是四步，中間兩步是
// 「拿買家手機／email 去開光貿 amego 載具」——那個設計已被 D6／D7／ADR-22 整體移除，
// `InvoiceCarrierType` 也只有 4 值、沒有承載它的第 5 個值。本測試釘住現行的兩步。
func TestResolveCarrier_UseBuyerDefault(t *testing.T) {
	t.Run("① 有存活的預設 profile → 用它", func(t *testing.T) {
		prefs := newPrefs(mobilePref(testPrefID, testBuyerID))
		prefs.defaultOf[testBuyerID] = testPrefID
		prefs.plaintext[testPrefID] = "/T7R3U+P"

		got, err := resolveCarrier(context.Background(), prefs,
			codemeta.InvoiceCarrierSelectionModeUseBuyerDefault.Value, 0, testBuyerID, "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.CarrierType.Code != codemeta.InvoiceCarrierTypeMobileBarcode.Code || got.CarrierID1 != "/T7R3U+P" {
			t.Fatalf("沒有用上預設 profile：%+v", got)
		}
	})

	t.Run("② 沒有預設 → 無載具發票（正常分支，不是錯誤）", func(t *testing.T) {
		prefs := newPrefs()
		got, err := resolveCarrier(context.Background(), prefs,
			codemeta.InvoiceCarrierSelectionModeUseBuyerDefault.Value, 0, testBuyerID, "", "", "")
		if err != nil {
			t.Fatalf("沒有預設是正常值不是錯誤：%v", err)
		}
		if got.CarrierType.Code != "" || got.CarrierID1 != "" || got.OwnershipMismatch {
			t.Fatalf("應為無載具發票：%+v", got)
		}
	})

	t.Run("預設是統編型 → 帶統編與抬頭", func(t *testing.T) {
		prefs := newPrefs(domain.BuyerPreference{
			ID: testPrefID, BuyerID: testBuyerID,
			PreferenceKind: codemeta.InvoiceCarrierTypeTaxId.Value,
			TaxID:          strp("12345678"), TaxTitle: strp("範例生鮮股份有限公司"),
		})
		prefs.defaultOf[testBuyerID] = testPrefID

		got, err := resolveCarrier(context.Background(), prefs,
			codemeta.InvoiceCarrierSelectionModeUseBuyerDefault.Value, 0, testBuyerID, "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.BuyerTaxID != "12345678" || got.BuyerTitle != "範例生鮮股份有限公司" {
			t.Fatalf("統編型預設沒有攤開：%+v", got)
		}
		if got.CarrierID1 != "" {
			t.Fatalf("I-04 互斥：統編型不得同時帶載具值：%+v", got)
		}
	})

	t.Run("預設是捐贈型 → 帶捐贈碼", func(t *testing.T) {
		prefs := newPrefs(domain.BuyerPreference{
			ID: testPrefID, BuyerID: testBuyerID,
			PreferenceKind: codemeta.InvoiceCarrierTypeDonation.Value,
			DonationCode:   strp("9527"),
		})
		prefs.defaultOf[testBuyerID] = testPrefID

		got, err := resolveCarrier(context.Background(), prefs,
			codemeta.InvoiceCarrierSelectionModeUseBuyerDefault.Value, 0, testBuyerID, "", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.DonationCode != "9527" {
			t.Fatalf("捐贈型預設沒有攤開：%+v", got)
		}
	})

	t.Run("查詢真的失敗 → 往上拋，不得當成『沒有預設』", func(t *testing.T) {
		prefs := newPrefs()
		prefs.defaultErr = errors.New("connection reset by peer")
		if _, err := resolveCarrier(context.Background(), prefs,
			codemeta.InvoiceCarrierSelectionModeUseBuyerDefault.Value, 0, testBuyerID, "", "", ""); err == nil {
			t.Fatalf("查詢失敗必須 fail-loud，否則買家設了載具卻被開成無載具")
		}
	})
}

func TestResolveCarrier_UnknownModeFailsLoud(t *testing.T) {
	if _, err := resolveCarrier(context.Background(), newPrefs(),
		"USE_BUYER_PHONE", 0, testBuyerID, "", "", ""); err == nil {
		t.Fatalf("值域外的 mode 必須 fail-loud（猜一個＝在憑證上寫錯買受人身分）")
	}
}

func TestFromPreference_EmptyDecryptedValueFailsLoud(t *testing.T) {
	prefs := newPrefs(mobilePref(testPrefID, testBuyerID))
	prefs.plaintext[testPrefID] = "" // 解密路徑壞了
	if _, err := resolveCarrier(context.Background(), prefs,
		codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value, testPrefID, testBuyerID, "", "", ""); err == nil {
		t.Fatalf("載具明文為空必須 fail-loud（AMEGO 不擋，買家會歸不到戶）")
	}
}

// fakePrefRepo 是 domain.BuyerPreferenceRepository 的最小假物件。
//
// 嵌入介面而不是逐支實作：本檔只在乎讀取面（GetDefault／WithDecryptedCarrierValue），
// 寫入面（B08 端點 2–5）沒被覆寫 ⇒ 呼叫即 panic 並指名方法——「這條路徑本來就不該
// 被走到」與「回了零值」因此分得開。
type fakePrefRepo struct {
	domain.BuyerPreferenceRepository

	defaultPref  domain.BuyerPreference
	defaultFound bool
	defaultErr   error
	plain        string
	openErr      error
}

func (f *fakePrefRepo) GetByID(context.Context, int64, int64) (domain.BuyerPreference, error) {
	return domain.BuyerPreference{}, nil
}

// ListByBuyer 是 B08 列表用的；本檔測的是結帳缺省鏈，一律回空。
// 🔴 不要把它改成回 defaultPref：那會讓「用 List 代替 GetDefault」看起來可行，
// 而 List 帶 LIMIT，預設那一列排在 limit 之外時會靜默回「沒有預設」。
func (f *fakePrefRepo) ListByBuyer(context.Context, int64, int32) ([]domain.BuyerPreference, error) {
	return nil, nil
}

func (f *fakePrefRepo) GetDefault(context.Context, int64) (domain.BuyerPreference, bool, error) {
	return f.defaultPref, f.defaultFound, f.defaultErr
}

func (f *fakePrefRepo) WithDecryptedCarrierValue(
	_ context.Context, _, _ int64, fn func(string, string) error,
) error {
	if f.openErr != nil {
		return f.openErr
	}
	return fn(codemeta.InvoiceCarrierTypeMobileBarcode.Value, f.plain)
}

// TestRepoPreferenceReader_NoDefaultIsNotAnError 釘住這一整個檔案裡最容易寫錯的一格。
//
// repository 的 `found == false` 是**正常分支**——多數買家沒設過預設常用發票資料，
// 他們的發票就該開成無載具（§6.1.2(b) 的第 2 步）。把它翻成一般 error 的後果是：
// **每一個沒設過預設的買家，他的 order.paid 都會進 DLQ**，而那是絕大多數買家。
// 服務看起來「有在動」（設過預設的少數人開得出票），DLQ 卻在無聲堆積。
func TestRepoPreferenceReader_NoDefaultIsNotAnError(t *testing.T) {
	r := RepoPreferenceReader{Repo: &fakePrefRepo{defaultFound: false}}

	_, err := r.GetDefault(context.Background(), testBuyerID)
	if !errors.Is(err, ErrNoDefaultPreference) {
		t.Fatalf("沒有預設必須回 ErrNoDefaultPreference（讓呼叫端走無載具那一步），實際 err = %v", err)
	}

	// 陰性：真的 DB 錯誤**不得**被翻成 ErrNoDefaultPreference，否則會把故障靜默成
	// 「這個買家沒設預設」，開出一張本來該有載具的無載具發票。
	boom := errors.New("connection reset")
	r2 := RepoPreferenceReader{Repo: &fakePrefRepo{defaultErr: boom}}
	_, err2 := r2.GetDefault(context.Background(), testBuyerID)
	if errors.Is(err2, ErrNoDefaultPreference) {
		t.Fatal("DB 錯誤被翻成「沒有預設」—— 故障會靜默變成無載具發票")
	}
	if !errors.Is(err2, boom) {
		t.Fatalf("DB 錯誤必須原樣上傳，實際 err = %v", err2)
	}
}

// TestRepoPreferenceReader_OpenCarrierValue 釘住明文出口的兩條路。
func TestRepoPreferenceReader_OpenCarrierValue(t *testing.T) {
	r := RepoPreferenceReader{Repo: &fakePrefRepo{plain: "/ABC+-9"}}

	// 有密文 ⇒ 走 repository 的明文出口。
	got, err := r.OpenCarrierValue(context.Background(),
		domain.BuyerPreference{ID: testPrefID, BuyerID: testBuyerID, CarrierValueCiphertext: strp("cipher")})
	if err != nil || got != "/ABC+-9" {
		t.Fatalf("有密文時應取得明文，實際 (%q, %v)", got, err)
	}

	// 沒有密文（TAX_ID／DONATION）⇒ 回空字串 ＋ nil，且**不得**去打 repository。
	if v, err := r.OpenCarrierValue(context.Background(), domain.BuyerPreference{}); err != nil || v != "" {
		t.Fatalf("沒有載具值的列應回 (\"\", nil)，實際 (%q, %v)", v, err)
	}
}

// TestRepoPreferenceReader_NilRepoFailsLoud：接線漏掉時必須 fail-loud，不得回零值。
func TestRepoPreferenceReader_NilRepoFailsLoud(t *testing.T) {
	r := RepoPreferenceReader{}
	if _, err := r.GetDefault(context.Background(), testBuyerID); !errors.Is(err, ErrPreferenceCapabilityUnavailable) {
		t.Fatalf("Repo 為 nil 時 GetDefault 必須 fail-loud，err = %v", err)
	}
	_, err := r.OpenCarrierValue(context.Background(), domain.BuyerPreference{CarrierValueCiphertext: strp("x")})
	if !errors.Is(err, ErrPreferenceCapabilityUnavailable) {
		t.Fatalf("Repo 為 nil 時 OpenCarrierValue 必須 fail-loud，err = %v", err)
	}
}
