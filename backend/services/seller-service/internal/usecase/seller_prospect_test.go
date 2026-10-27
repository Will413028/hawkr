package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

type fakeProspectRepo struct {
	byTaxID     map[string]domain.SellerProspect
	byKeys      []domain.SellerProspect
	taxIDErr    error // 非 nil 時 FindByTaxID 優先回這個，蓋過 map 查找
	keysErr     error // 非 nil 時 FindByMatchKeys 回這個
	taxIDCalls  int
	keysCalls   int
	createCalls int
	lastKeysArg domain.ProspectMatchKeys

	updateCalls  int
	lastUpdateIn domain.SellerProspect // Update 實際收到的引數，供斷言 UpdateForAdmin 有沒有換掉呼叫端帶來的 MatchKeys

	setArchivedCalls int
	setArchivedRows  int64 // SetArchived 的回傳列數，每個測試自行指定；零值(0)代表「未放行」而非隨機值，不能被誤讀成「忘了設」
	setArchivedErr   error

	updateMediaCalls int
	lastMediaUpdate  domain.ProspectMediaUpdate // UpdateMedia 實際收到的引數，供斷言 objectKey／kind／version 有沒有正確傳下去
	updateMediaOut   domain.SellerProspect
	updateMediaErr   error
}

func (f *fakeProspectRepo) Create(context.Context, domain.SellerProspect) (domain.SellerProspect, error) {
	f.createCalls++
	return domain.SellerProspect{}, nil
}
func (f *fakeProspectRepo) Search(context.Context, domain.ProspectSearchParams) ([]domain.ProspectSearchHit, error) {
	return nil, nil
}
func (f *fakeProspectRepo) FindByTaxID(_ context.Context, taxID string) (domain.SellerProspect, error) {
	f.taxIDCalls++
	if f.taxIDErr != nil {
		return domain.SellerProspect{}, f.taxIDErr
	}
	if p, ok := f.byTaxID[taxID]; ok {
		return p, nil
	}
	return domain.SellerProspect{}, pgx.ErrNoRows
}
func (f *fakeProspectRepo) FindByMatchKeys(_ context.Context, keys domain.ProspectMatchKeys) ([]domain.SellerProspect, error) {
	f.keysCalls++
	f.lastKeysArg = keys
	if f.keysErr != nil {
		return nil, f.keysErr
	}
	return f.byKeys, nil
}
func (f *fakeProspectRepo) MarkConverted(context.Context, int64, int64, *int64) (int64, error) {
	return 1, nil
}

// FindUsableInviteByToken／ConsumeInvite：inert stubs. 這個 fake 服務的是 Match
// 的比對邏輯測試，不涉及邀請歸因（那條路徑見 prospect_conversion_test.go 的
// stubProspectAttribution）——這裡只是讓 domain.SellerProspectRepository 編譯得過。
func (f *fakeProspectRepo) FindUsableInviteByToken(context.Context, string) (domain.ProspectInvite, error) {
	return domain.ProspectInvite{}, pgx.ErrNoRows
}
func (f *fakeProspectRepo) ConsumeInvite(context.Context, int64) error { return nil }

// CreateInvite／RevokeInvite：跟上面同一組 inert stubs，理由相同——這個
// fake 服務的是 Match 的比對邏輯測試，不涉及邀請建立與撤銷（那條路徑見
// TestCreateInviteTokenIsUnpredictableAndCapped 的 stubProspectRepo），這裡
// 只是讓 domain.SellerProspectRepository 編譯得過。
func (f *fakeProspectRepo) CreateInvite(context.Context, int64, string, time.Time) (domain.ProspectInvite, error) {
	return domain.ProspectInvite{}, nil
}
func (f *fakeProspectRepo) RevokeInvite(context.Context, int64, int64) (int64, error) { return 0, nil }

// ListForAdmin／GetByID／ListInvites：跟上面同一組 inert stubs，理由相同——
// 這個 fake 服務的是 Match 的比對邏輯測試，不涉及 admin 列表／詳情頁
// （那條路徑見 seller_prospect_admin_integration_test.go），這裡只是讓
// domain.SellerProspectRepository 編譯得過。
func (f *fakeProspectRepo) ListForAdmin(context.Context, domain.ProspectAdminListParams) ([]domain.SellerProspect, int64, error) {
	return nil, 0, nil
}
func (f *fakeProspectRepo) GetByID(context.Context, int64) (domain.SellerProspect, error) {
	return domain.SellerProspect{}, pgx.ErrNoRows
}
func (f *fakeProspectRepo) ListInvites(context.Context, int64) ([]domain.ProspectInvite, error) {
	return nil, nil
}

// ListDuplicatePairs：inert stub，同一組理由——這個 fake 服務的是 Match 的
// 比對邏輯測試，不涉及 T2 重複清單掃描（那條路徑見
// TestListDuplicatesReportsMatchedKeyAndTruncation 的 stubProspectRepo），
// 這裡只是讓 domain.SellerProspectRepository 編譯得過。
func (f *fakeProspectRepo) ListDuplicatePairs(context.Context) ([]domain.ProspectDuplicatePair, error) {
	return nil, nil
}

// Update 記錄呼叫端實際傳入的 domain.SellerProspect（含 MatchKeys）並原樣
// 回傳，供 TestUpdateForAdmin_* 斷言 UpdateForAdmin 是否真的丟掉呼叫端帶來的
// 舊 MatchKeys、改用 BuildProspectMatchKeys 從最新聯絡欄位重算。
func (f *fakeProspectRepo) Update(_ context.Context, in domain.SellerProspect) (domain.SellerProspect, error) {
	f.updateCalls++
	f.lastUpdateIn = in
	return in, nil
}

// SetArchived 回傳值由每個測試透過 setArchivedRows／setArchivedErr 明確指定，
// 不給預設「成功」（1, nil）——那種預設會讓下一個沒注意到的測試看起來通過，
// 實際上完全沒驗證到 rows == 0 的分支。
func (f *fakeProspectRepo) SetArchived(context.Context, int64, bool, *int64) (int64, error) {
	f.setArchivedCalls++
	return f.setArchivedRows, f.setArchivedErr
}

// UpdateMedia 同樣不給預設成功：換圖的衝突分支是這條路徑最重要的行為，
// 預設回 nil error 會讓忘了設期望值的測試假性通過。
func (f *fakeProspectRepo) UpdateMedia(_ context.Context, in domain.ProspectMediaUpdate) (domain.SellerProspect, error) {
	f.updateMediaCalls++
	f.lastMediaUpdate = in
	return f.updateMediaOut, f.updateMediaErr
}

// Merge：inert stub，同一組理由——這個 fake 服務的是 Match 的比對邏輯測試，
// 不涉及合併交易（那條路徑見 seller_prospect_merge_integration_test.go），
// 這裡只是讓 domain.SellerProspectRepository 編譯得過。
func (f *fakeProspectRepo) Merge(context.Context, int64, int64, *int64) (domain.SellerProspect, error) {
	return domain.SellerProspect{}, nil
}

// CurrentStoreExists／ListTaxIDMatches：inert stubs，同一組理由——這個 fake
// 服務的是 Match 的比對邏輯測試，不涉及 admin 歸因綁定或 tax_id 對帳（那條
// 路徑見 prospect_conversion_test.go 的 stubProspectRepo 與
// seller_prospect_reconcile_integration_test.go），這裡只是讓
// domain.SellerProspectRepository 編譯得過。
func (f *fakeProspectRepo) CurrentStoreExists(context.Context, int64) (bool, error) {
	return false, nil
}
func (f *fakeProspectRepo) ListTaxIDMatches(context.Context) ([]domain.ProspectTaxIDMatch, error) {
	return nil, nil
}

func TestMatch_TaxIDHitShortCircuitsContactTier(t *testing.T) {
	repo := &fakeProspectRepo{
		byTaxID: map[string]domain.SellerProspect{"12345678": {ID: 1, DisplayName: "梓官區漁會"}},
		byKeys:  []domain.SellerProspect{{ID: 2, DisplayName: "漁夫鮮撈"}},
	}
	uc := NewSellerProspectUseCase(repo, nil)

	got, err := uc.Match(context.Background(), domain.ProspectContactInput{
		TaxID: "12345678", Email: "marketing@example.org",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (T1 命中就不該再往下查)", len(got))
	}
	if got[0].Tier != ProspectMatchTierTaxID {
		t.Errorf("Tier = %q, want %q", got[0].Tier, ProspectMatchTierTaxID)
	}
	if repo.keysCalls != 0 {
		t.Errorf("FindByMatchKeys 被呼叫 %d 次，T1 命中時不該查 T2", repo.keysCalls)
	}
}

func TestMatch_FallsBackToContactTier(t *testing.T) {
	repo := &fakeProspectRepo{
		byTaxID: map[string]domain.SellerProspect{},
		byKeys: []domain.SellerProspect{{
			ID: 2, DisplayName: "漁夫鮮撈",
			MatchKeys: domain.ProspectMatchKeys{Email: "marketing@example.org"},
		}},
	}
	uc := NewSellerProspectUseCase(repo, nil)

	got, err := uc.Match(context.Background(), domain.ProspectContactInput{Email: "marketing@example.org"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Tier != ProspectMatchTierContact {
		t.Fatalf("got = %+v, want 單筆 T2_CONTACT", got)
	}
	if got[0].MatchedOn != "email" {
		t.Errorf("MatchedOn = %q, want email", got[0].MatchedOn)
	}
	if repo.taxIDCalls != 0 {
		t.Errorf("taxIDCalls = %d, want 0（這個情境沒給 TaxID，不該查 T1）", repo.taxIDCalls)
	}
}

func TestMatch_PlaceholderOnlyInputQueriesNothing(t *testing.T) {
	// 這是本檔最重要的一條：整份名單有 541 筆 email、48 筆電話填「待確認」。
	// 若佔位值沒被正規化成空、或空鍵仍然送進查詢，這幾百筆會互相命中、
	// 被判成同一家業者。
	repo := &fakeProspectRepo{byTaxID: map[string]domain.SellerProspect{}}
	uc := NewSellerProspectUseCase(repo, nil)

	got, err := uc.Match(context.Background(), domain.ProspectContactInput{
		Email: "待確認", Phone: "待確認", Website: "N/A",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
	if repo.taxIDCalls != 0 || repo.keysCalls != 0 {
		t.Errorf("查詢次數 taxID=%d keys=%d，全為佔位值時不該打任何一次 DB",
			repo.taxIDCalls, repo.keysCalls)
	}
}

func TestMatch_SocialHandleIsAUsableKey(t *testing.T) {
	// CRM1 的 454 筆只有 4 筆真 email、18 筆真電話，社群 handle 是它們唯一
	// 可用的決定性鍵。這條確認它真的會走到查詢。
	repo := &fakeProspectRepo{
		byTaxID: map[string]domain.SellerProspect{},
		byKeys:  []domain.SellerProspect{{ID: 3, DisplayName: "樹上的蛋"}},
	}
	uc := NewSellerProspectUseCase(repo, nil)

	got, err := uc.Match(context.Background(), domain.ProspectContactInput{
		Website: "https://www.threads.com/@tree_egg_",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if repo.lastKeysArg.Social != "threads:tree_egg_" {
		t.Errorf("Social key = %q, want threads:tree_egg_", repo.lastKeysArg.Social)
	}
	if repo.lastKeysArg.Domain != "" {
		t.Errorf("Domain = %q, want empty（社群網域絕不可成為 match_domain）", repo.lastKeysArg.Domain)
	}
}

func TestMatch_ReportsTheKeyThatActuallyMatched_NotTheFirstKeySupplied(t *testing.T) {
	// 輸入同時有 email 與電話，但這一列只因電話命中（email 完全不同）。
	// MatchedOn 必須說 phone。回報 email 等於對人謊報「這兩家共用 email」，
	// 而合併不可逆——假證據比沒有證據更糟。
	repo := &fakeProspectRepo{
		byTaxID: map[string]domain.SellerProspect{},
		byKeys: []domain.SellerProspect{{
			ID: 4, DisplayName: "多利多國際食品",
			MatchKeys: domain.ProspectMatchKeys{Email: "someone-else@example.com", Phone: "0425810586"},
		}},
	}
	uc := NewSellerProspectUseCase(repo, nil)

	got, err := uc.Match(context.Background(), domain.ProspectContactInput{
		Email: "buyer@example.com", Phone: "04-2581-0586",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].MatchedOn != "phone" {
		t.Errorf("MatchedOn = %q, want phone（只有電話真的相同，回報 email 是謊報）", got[0].MatchedOn)
	}
}

func TestMatch_TaxIDLookupError_PropagatesAndDoesNotFallThroughToContactTier(t *testing.T) {
	// 吞掉 DB 錯誤會讓既有業者看起來像「查無此人」＝新業者，呼叫端據此建一筆
	// 重複的主檔列。只有 ErrNoRows 代表「真的沒有」，其餘一律往上傳。
	boom := errors.New("connection refused")
	repo := &fakeProspectRepo{byTaxID: map[string]domain.SellerProspect{}, taxIDErr: boom}
	uc := NewSellerProspectUseCase(repo, nil)

	got, err := uc.Match(context.Background(), domain.ProspectContactInput{
		TaxID: "12345678", Email: "marketing@example.org",
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v（DB 錯誤必須原樣往上傳）", err, boom)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil", got)
	}
	if repo.keysCalls != 0 {
		t.Errorf("keysCalls = %d, want 0（T1 出錯時不得降級成 T2 查詢）", repo.keysCalls)
	}
}

func TestMatch_ContactLookupError_Propagates(t *testing.T) {
	boom := errors.New("connection refused")
	repo := &fakeProspectRepo{byTaxID: map[string]domain.SellerProspect{}, keysErr: boom}
	uc := NewSellerProspectUseCase(repo, nil)

	got, err := uc.Match(context.Background(), domain.ProspectContactInput{Email: "marketing@example.org"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil", got)
	}
}

// 統編格式不符必須在 usecase 就擋下並回 *ValidationError（→ InvalidArgument），
// 不能讓它掉到 DB 由 chk_sp_tax_id 擋。2026-09-06 dev 實測：沒有這一關時，
// 呼叫端拿到的是 `Internal: ... violates check constraint "chk_sp_tax_id"
// (SQLSTATE 23514)`——對階段二 admin 表單是「打錯一個字回 500」，而且把 DB
// 約束名洩漏到 service 邊界外。
func TestCreate_RejectsMalformedTaxIDBeforeTouchingTheRepository(t *testing.T) {
	for _, bad := range []string{"1234", "1234567a", "123456789", "  12345678"} {
		repo := &fakeProspectRepo{}
		uc := NewSellerProspectUseCase(repo, nil)
		taxID := bad
		// Status／OutreachStatus 必須填合法值：這條測試只想證明「統編格式
		// 不符會被擋」，其餘欄位不合法會讓 ValidateProspectFields 同時因
		// 別的理由回錯，使得刪掉統編正規表達式這條測試依然會通過——2026-09-06
		// review 抓到的既有缺口，見同名 commit 訊息。
		_, err := uc.Create(context.Background(), domain.SellerProspect{
			DisplayName: "測試業者", VendorType: "business",
			Status: "ACTIVE", OutreachStatus: "NOT_CONTACTED",
			TaxID: &taxID,
		})
		var verr *ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("Create(tax_id=%q)：期望 *ValidationError，得到 %v", bad, err)
		} else if !strings.Contains(verr.Error(), "統一編號格式不符") {
			t.Errorf("Create(tax_id=%q)：期望錯誤訊息包含統編格式不符的理由，得到 %q", bad, verr.Error())
		}
		if repo.createCalls != 0 {
			t.Errorf("Create(tax_id=%q)：格式不符不該打到 repository（createCalls=%d）", bad, repo.createCalls)
		}
	}
}

// 正對照：合法統編與「沒有統編」兩種情況都必須照常寫入——這一關是格式閘，
// 不是把統編變成必填。
func TestCreate_AllowsValidTaxIDAndNilTaxID(t *testing.T) {
	valid := "12345678"
	for name, in := range map[string]domain.SellerProspect{
		"有合法統編": {DisplayName: "測試業者", VendorType: "business", Status: "ACTIVE", OutreachStatus: "NOT_CONTACTED", TaxID: &valid},
		"無統編":   {DisplayName: "測試業者", VendorType: "individual", Status: "ACTIVE", OutreachStatus: "NOT_CONTACTED"},
	} {
		repo := &fakeProspectRepo{}
		uc := NewSellerProspectUseCase(repo, nil)
		if _, err := uc.Create(context.Background(), in); err != nil {
			t.Errorf("%s：不該回錯，得到 %v", name, err)
		}
		if repo.createCalls != 1 {
			t.Errorf("%s：應該打到 repository 一次，實際 %d 次", name, repo.createCalls)
		}
	}
}

func TestValidateProspectFields(t *testing.T) {
	taxOK := "12345678"
	taxBad := "1234"

	cases := []struct {
		name    string
		in      domain.SellerProspect
		wantErr bool
	}{
		{"合法", domain.SellerProspect{VendorType: "business", Status: "ACTIVE", OutreachStatus: "NOT_CONTACTED", TaxID: &taxOK}, false},
		{"vendor_type 非法", domain.SellerProspect{VendorType: "corp", Status: "ACTIVE", OutreachStatus: "NOT_CONTACTED"}, true},
		{"status 非法", domain.SellerProspect{VendorType: "business", Status: "DELETED", OutreachStatus: "NOT_CONTACTED"}, true},
		{"outreach_status 非法", domain.SellerProspect{VendorType: "business", Status: "ACTIVE", OutreachStatus: "MAYBE"}, true},
		{"tax_id 格式非法", domain.SellerProspect{VendorType: "business", Status: "ACTIVE", OutreachStatus: "NOT_CONTACTED", TaxID: &taxBad}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProspectFields(tc.in)
			if tc.wantErr {
				var verr *ValidationError
				require.ErrorAs(t, err, &verr, "必須是 *ValidationError 才會被 ErrorInterceptor 映射成 InvalidArgument")
				return
			}
			require.NoError(t, err)
		})
	}
}

// 這是本檔對 UpdateForAdmin 最重要的一條：呼叫端帶來的 MatchKeys 必須被
// 丟掉、改用 BuildProspectMatchKeys 從最新的 PublicEmail 重算。故意讓輸入的
// MatchKeys.Email 跟 PublicEmail 對不上（一個是 stale 舊值、一個帶大小寫與
// 前後空白），只有真的走過 BuildProspectMatchKeys（trim + 轉小寫）才會讓
// repository 收到 "new@example.com"；如果 UpdateForAdmin 退化成直接複製呼叫端
// 的 MatchKeys，這裡會看到 "stale@old.example" 原封不動地流進 repository。
func TestUpdateForAdmin_DiscardsCallerSuppliedMatchKeysAndRecomputes(t *testing.T) {
	repo := &fakeProspectRepo{}
	uc := NewSellerProspectUseCase(repo, nil)

	in := domain.SellerProspect{
		DisplayName:    "改信箱業者",
		VendorType:     "business",
		OutreachStatus: "NOT_CONTACTED",
		PublicEmail:    "NEW@Example.com ",
		MatchKeys:      domain.ProspectMatchKeys{Email: "stale@old.example"},
	}

	_, err := uc.UpdateForAdmin(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, 1, repo.updateCalls)
	require.Equal(t, "new@example.com", repo.lastUpdateIn.MatchKeys.Email,
		"UpdateForAdmin 必須丟掉呼叫端帶來的舊 MatchKeys，改用 BuildProspectMatchKeys 從 PublicEmail 重算")
}

// 驗證必須在寫入之前擋下，不是寫入之後才發現——「打到 repository 之後才
// 驗證」等於沒驗證：DB 已經寫壞了，回錯只是遮住結果。
func TestUpdateForAdmin_RejectsInvalidVendorTypeBeforeTouchingTheRepository(t *testing.T) {
	repo := &fakeProspectRepo{}
	uc := NewSellerProspectUseCase(repo, nil)

	_, err := uc.UpdateForAdmin(context.Background(), domain.SellerProspect{
		DisplayName:    "測試業者",
		VendorType:     "corp", // 不合法：只接受 individual／business
		OutreachStatus: "NOT_CONTACTED",
	})

	var verr *ValidationError
	require.ErrorAs(t, err, &verr, "必須是 *ValidationError 才會被 ErrorInterceptor 映射成 InvalidArgument")
	require.Equal(t, 0, repo.updateCalls, "驗證沒過就不該打到 repository")
}

// outreach_status = 'CONVERTED' 是 markProspectConverted（唯一寫入點）的
// 既成事實，不是 admin 表單可以手動選的值：放行會產生「outreach_status
// 說已入駐，但 status 還是 ACTIVE、沒有 converted_store_id」的矛盾列，這種
// 列會繼續出現在 SearchSellerProspects（它只濾 status）餵給 ai-service 的
// EXTERNAL lane，對買家宣稱一個從未真的發生過的入駐（spec §2.1）。同上一個
// 測試，驗證必須在寫入之前擋下。
func TestUpdateForAdmin_RejectsConvertedOutreachStatusBeforeTouchingTheRepository(t *testing.T) {
	repo := &fakeProspectRepo{}
	uc := NewSellerProspectUseCase(repo, nil)

	_, err := uc.UpdateForAdmin(context.Background(), domain.SellerProspect{
		DisplayName:    "測試業者",
		VendorType:     "business",
		OutreachStatus: "CONVERTED",
	})

	var verr *ValidationError
	require.ErrorAs(t, err, &verr, "必須是 *ValidationError 才會被 ErrorInterceptor 映射成 InvalidArgument")
	require.Equal(t, 0, repo.updateCalls, "驗證沒過就不該打到 repository")
}

// rows == 0 是 SetArchived 述詞沒放行的唯一訊號（封存要求 ACTIVE；還原額外
// 要求 merged_into_prospect_id 為 NULL），usecase 必須把它轉成具名錯誤，
// 不能讓呼叫端誤以為「沒回錯」就是操作生效了。
func TestSetArchived_ZeroRowsMapsToErrProspectMerged(t *testing.T) {
	repo := &fakeProspectRepo{setArchivedRows: 0}
	uc := NewSellerProspectUseCase(repo, nil)

	err := uc.SetArchived(context.Background(), 1, false, nil)
	require.ErrorIs(t, err, ErrProspectMerged)
}

// 正對照：真的改到一列時不該回錯——上一條測試才有意義。
func TestSetArchived_NonZeroRowsSucceeds(t *testing.T) {
	repo := &fakeProspectRepo{setArchivedRows: 1}
	uc := NewSellerProspectUseCase(repo, nil)

	err := uc.SetArchived(context.Background(), 1, true, nil)
	require.NoError(t, err)
}

// 還原把列改回 ACTIVE，可能撞上 uq_seller_prospects_tax_id（narrow 過的
// index 讓封存期間別的業者能合法拿走同一個統編，見
// repository.SetArchived 的註解）。這裡驗證 repository 回的
// *domain.ErrDuplicate 有被 translateProspectDuplicate 收斂成具名
// ErrProspectTaxIDConflict，不是原樣往上傳——同 Create／UpdateForAdmin
// 的既有慣例，讓 handler 能用同一份 prospectConflictStatus 判斷。
func TestSetArchived_TaxIDDuplicateTranslatesToNamedConflict(t *testing.T) {
	repo := &fakeProspectRepo{setArchivedErr: &domain.ErrDuplicate{ConstraintName: "uq_seller_prospects_tax_id"}}
	uc := NewSellerProspectUseCase(repo, nil)

	err := uc.SetArchived(context.Background(), 1, false, nil)
	require.ErrorIs(t, err, ErrProspectTaxIDConflict)
}

func TestListDuplicatesReportsMatchedKeyAndTruncation(t *testing.T) {
	mk := func(email, phone string) domain.ProspectMatchKeys {
		return domain.ProspectMatchKeys{Email: email, Phone: phone}
	}

	pairs := make([]domain.ProspectDuplicatePair, 0, 501)
	pairs = append(pairs, domain.ProspectDuplicatePair{
		Left:  domain.SellerProspect{ID: 1, MatchKeys: mk("", "0912345678")},
		Right: domain.SellerProspect{ID: 2, MatchKeys: mk("", "0912345678")},
	})
	for i := 0; i < 500; i++ {
		pairs = append(pairs, domain.ProspectDuplicatePair{
			Left:  domain.SellerProspect{ID: int64(100 + i), MatchKeys: mk("a@example.com", "")},
			Right: domain.SellerProspect{ID: int64(1000 + i), MatchKeys: mk("a@example.com", "")},
		})
	}

	repo := &stubProspectRepo{duplicatePairs: pairs}
	uc := NewSellerProspectUseCase(repo, nil)

	got, truncated, err := uc.ListDuplicates(context.Background())
	require.NoError(t, err)
	require.True(t, truncated)
	require.Len(t, got, 500)
	require.Equal(t, "phone", got[0].MatchedOn, "email 為空時不得回報 email")
}

// token 不可預測性是 spec §5.3 明列的四個性質之一：兩次呼叫必須產出不同
// token，且長度足以反映 32 bytes 的 CSPRNG 輸出（base64url 編碼後至少 43
// 字元）。有效期上限（180 天）另外由 lastInviteValidDays() 斷言——這條防的
// 是呼叫端傳一個超大 validDays 就能讓邀請連結長期有效。
func TestCreateInviteTokenIsUnpredictableAndCapped(t *testing.T) {
	repo := &stubProspectRepo{}
	uc := NewSellerProspectUseCase(repo, nil)

	_, tok1, err := uc.CreateInvite(context.Background(), 1, 0)
	require.NoError(t, err)
	_, tok2, err := uc.CreateInvite(context.Background(), 1, 0)
	require.NoError(t, err)

	require.NotEqual(t, tok1, tok2)
	require.GreaterOrEqual(t, len(tok1), 40, "32 bytes 的 base64url 至少 43 字")

	_, _, err = uc.CreateInvite(context.Background(), 1, 9999)
	require.NoError(t, err)
	require.LessOrEqual(t, repo.lastInviteValidDays(), 180, "有效期上限 180 天")
}

// 合併第 3 步（把邀請搬去 winner）存在的理由，正是不讓邀請繼續指向一筆
// 會被封存的列；CreateInvite 若不自己也擋非 ACTIVE 的 prospect，一筆已被
// 合併掉但仍可經 ?status=ARCHIVED 開啟詳情頁的 loser，還是能繼續建邀請、
// 發連結——vendor 用它註冊時 MarkConverted 的 WHERE status = 'ACTIVE'
// 影響 0 列，歸因悄悄失敗（CreateSeller 只記 log warning、不擋建店），
// token 卻已消費，合併想擋的事又重新發生一次。
func TestCreateInvite_RejectsNonActiveProspect(t *testing.T) {
	for _, status := range []string{"ARCHIVED", "CONVERTED"} {
		t.Run(status, func(t *testing.T) {
			repo := &stubProspectRepo{getByIDStatus: status}
			uc := NewSellerProspectUseCase(repo, nil)

			_, _, err := uc.CreateInvite(context.Background(), 1, 0)

			require.ErrorIs(t, err, ErrProspectNotInvitable)
			require.Equal(t, 0, repo.createInviteCalls, "驗證沒過就不該打到 repository")
		})
	}
}

// rows == 0 是 RevokeProspectInvite 述詞沒放行的唯一訊號（id 不存在、
// prospect_id 對不上、已撤銷，或已消費），usecase 必須把它轉成具名錯誤，
// 不能讓呼叫端誤以為「沒回錯」就是撤銷生效了。
func TestRevokeInvite_ZeroRowsMapsToErrProspectInviteNotRevocable(t *testing.T) {
	repo := &stubProspectRepo{revokeInviteRows: 0}
	uc := NewSellerProspectUseCase(repo, nil)

	err := uc.RevokeInvite(context.Background(), 1, 2)
	require.ErrorIs(t, err, ErrProspectInviteNotRevocable)
}

// 正對照：真的改到一列時不該回錯——上一條測試才有意義。
func TestRevokeInvite_NonZeroRowsSucceeds(t *testing.T) {
	repo := &stubProspectRepo{revokeInviteRows: 1}
	uc := NewSellerProspectUseCase(repo, nil)

	err := uc.RevokeInvite(context.Background(), 1, 2)
	require.NoError(t, err)
}
