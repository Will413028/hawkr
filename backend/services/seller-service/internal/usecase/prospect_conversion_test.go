package usecase

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// newSellerUseCaseForTest 依 certification_approve_payuni_test.go 既有的
// fake repo ＋ struct-literal 形狀撰寫（該檔用 &PayuniUseCase{...} 直接建構，
// 不經過 NewPayuniUseCase）。這裡只組裝 CreateSeller 路徑實際會碰到的欄位：
// sellerRepo／accountClient／tsid／storeCode／txManager／outboxRepo／prospectRepo。
func newSellerUseCaseForTest(t *testing.T, prospectRepo ProspectAttributionRepo) *SellerUseCase {
	t.Helper()
	return &SellerUseCase{
		sellerRepo: &mockSellerRepo{
			CreateFn: func(context.Context, *domain.Seller) error { return nil },
		},
		accountClient: testAccountClient,
		tsid:          testTSID,
		storeCode:     testStoreCode,
		txManager:     &mockTxManager{},
		outboxRepo:    &fakeSellerDeletedOutboxRepo{},
		prospectRepo:  prospectRepo,
	}
}

// stubProspectAttribution 記錄歸因側被呼叫的情形，讓每條測試斷言「有沒有
// 發生歸因」而不必碰真的 DB。
type stubProspectAttribution struct {
	invite                domain.ProspectInvite
	inviteErr             error
	markedRows            int64
	markedStore           int64
	markCalls             int
	consumeCalls          int
	markedOperatorSet     bool
	markedOperatorAccount *int64
}

func (s *stubProspectAttribution) FindUsableInviteByToken(context.Context, string) (domain.ProspectInvite, error) {
	return s.invite, s.inviteErr
}
func (s *stubProspectAttribution) MarkConverted(_ context.Context, _, storeID int64, operatorAccountID *int64) (int64, error) {
	s.markCalls++
	s.markedStore = storeID
	s.markedOperatorSet = true
	s.markedOperatorAccount = operatorAccountID
	return s.markedRows, nil
}
func (s *stubProspectAttribution) ConsumeInvite(context.Context, int64) error {
	s.consumeCalls++
	return nil
}

// spyCertRepo makes every domain.CertificationRepository call observable, so
// TestCreateSeller_TokenGrantsNoCertificationState can assert the token path
// touches ZERO certification state — not just that the *returned DTO* looks
// the same as the no-token path (review 2026-09-05, Important #1: that
// DTO-only version was tautological, since the in-memory DTO CreateSeller
// returns never reflects a certification write on either path — a
// regression that inserted a pre-approved certification_applications row
// through certRepo would still leave both DTOs zero-valued and the old test
// green).
type spyCertRepo struct {
	calls []string
}

func (s *spyCertRepo) GetApplicationByID(context.Context, int64) (*domain.CertificationApplication, error) {
	s.calls = append(s.calls, "GetApplicationByID")
	return nil, nil
}
func (s *spyCertRepo) GetApplicationByNo(context.Context, string) (*domain.CertificationApplication, error) {
	s.calls = append(s.calls, "GetApplicationByNo")
	return nil, nil
}
func (s *spyCertRepo) GetApplicationsBySeller(context.Context, int64) ([]*domain.CertificationApplication, error) {
	s.calls = append(s.calls, "GetApplicationsBySeller")
	return nil, nil
}
func (s *spyCertRepo) GetCurrentApplication(context.Context, int64) (*domain.CertificationApplication, error) {
	s.calls = append(s.calls, "GetCurrentApplication")
	return nil, nil
}
func (s *spyCertRepo) GetDraftApplication(context.Context, int64) (*domain.CertificationApplication, error) {
	s.calls = append(s.calls, "GetDraftApplication")
	return nil, nil
}
func (s *spyCertRepo) GetCertHolderNames(context.Context, int64) (domain.CertHolderNames, bool, error) {
	s.calls = append(s.calls, "GetCertHolderNames")
	return domain.CertHolderNames{}, false, nil
}
func (s *spyCertRepo) CreateApplication(context.Context, *domain.CertificationApplication) error {
	s.calls = append(s.calls, "CreateApplication")
	return nil
}
func (s *spyCertRepo) UpdateApplication(context.Context, *domain.CertificationApplication) error {
	s.calls = append(s.calls, "UpdateApplication")
	return nil
}
func (s *spyCertRepo) ListForAdmin(context.Context, domain.CertificationListFilter) ([]domain.AdminCertificationItem, error) {
	s.calls = append(s.calls, "ListForAdmin")
	return nil, nil
}
func (s *spyCertRepo) CountForAdmin(context.Context, domain.CertificationListFilter) (int64, error) {
	s.calls = append(s.calls, "CountForAdmin")
	return 0, nil
}
func (s *spyCertRepo) GetStatsForAdmin(context.Context) (*domain.CertificationStats, error) {
	s.calls = append(s.calls, "GetStatsForAdmin")
	return nil, nil
}
func (s *spyCertRepo) GetReviewsByApplication(context.Context, int64) ([]*domain.CertificationReview, error) {
	s.calls = append(s.calls, "GetReviewsByApplication")
	return nil, nil
}
func (s *spyCertRepo) GetLatestReview(context.Context, int64) (*domain.CertificationReview, error) {
	s.calls = append(s.calls, "GetLatestReview")
	return nil, nil
}
func (s *spyCertRepo) CreateReview(context.Context, *domain.CertificationReview) error {
	s.calls = append(s.calls, "CreateReview")
	return nil
}

func TestCreateSeller_ValidTokenAttributesProspectInSameTransaction(t *testing.T) {
	stub := &stubProspectAttribution{
		invite: domain.ProspectInvite{ID: 11, ProspectID: 7001}, markedRows: 1,
	}
	uc := newSellerUseCaseForTest(t, stub)

	created, err := uc.CreateSeller(context.Background(), CreateSellerInput{
		AccountID: 3001, StoreName: "梓官區漁會", ProspectInviteToken: "tok_valid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.markCalls != 1 || stub.markedStore != created.StoreID {
		t.Errorf("歸因未指向新建 store：markCalls=%d markedStore=%d storeID=%d",
			stub.markCalls, stub.markedStore, created.StoreID)
	}
	if stub.consumeCalls != 1 {
		t.Errorf("consumeCalls = %d, want 1（token 單次消費）", stub.consumeCalls)
	}
	// CreateSeller 的 token 路徑沒有 admin 操作者：attributeProspect 必須傳
	// nil，不能把買家/賣家自己的 account_id 或任何猜測值當成操作者記進
	// updated_by_account_id——那會偽造一次「有人手動操作」的稽核紀錄。
	if !stub.markedOperatorSet || stub.markedOperatorAccount != nil {
		t.Errorf("token 歸因路徑的 operatorAccountID = %v, want nil", stub.markedOperatorAccount)
	}
}

func TestCreateSeller_InvalidTokenStillCreatesSeller(t *testing.T) {
	// 這條防的是「把行銷歸因變成註冊阻塞點」——讓一個無效 token 擋住賣家
	// 註冊，是把功能變成故障。
	stub := &stubProspectAttribution{inviteErr: errors.New("no such token")}
	uc := newSellerUseCaseForTest(t, stub)

	created, err := uc.CreateSeller(context.Background(), CreateSellerInput{
		AccountID: 3002, StoreName: "測試店", ProspectInviteToken: "tok_garbage",
	})
	if err != nil {
		t.Fatalf("建店不得因歸因失敗而失敗，got err = %v", err)
	}
	if created.StoreID == 0 {
		t.Fatal("seller 應該照樣建立")
	}
	if stub.markCalls != 0 {
		t.Errorf("markCalls = %d, want 0（token 不可用時不得歸因）", stub.markCalls)
	}
}

func TestCreateSeller_ExpiredTokenStillCreatesSellerAndDoesNotAttribute(t *testing.T) {
	// 過期由 FindUsableInviteByToken 的 SQL 述詞擋掉（expires_at > now()），
	// 這裡以「查不到」表達，行為與無效 token 相同：不阻塞、不歸因。
	stub := &stubProspectAttribution{inviteErr: errors.New("no rows")}
	uc := newSellerUseCaseForTest(t, stub)

	created, err := uc.CreateSeller(context.Background(), CreateSellerInput{
		AccountID: 3003, StoreName: "測試店", ProspectInviteToken: "tok_expired",
	})
	if err != nil {
		t.Fatalf("建店不得因過期 token 而失敗，got err = %v", err)
	}
	// review 2026-09-05 Important #3：不能只看 err == nil——那連 (nil, nil) 都會放行。
	if created == nil || created.StoreID == 0 {
		t.Fatal("seller 應該照樣建立")
	}
	if stub.markCalls != 0 {
		t.Errorf("markCalls = %d, want 0", stub.markCalls)
	}
}

func TestCreateSeller_SecondConversionOfSameProspectDoesNotOverwrite(t *testing.T) {
	// MarkConverted 回 0 列＝這筆 prospect 已被別的 store 認領過
	// （WHERE status = 'ACTIVE' 擋下）。不覆蓋、不重試，但也不阻塞建店。
	stub := &stubProspectAttribution{
		invite: domain.ProspectInvite{ID: 12, ProspectID: 7001}, markedRows: 0,
	}
	uc := newSellerUseCaseForTest(t, stub)

	created, err := uc.CreateSeller(context.Background(), CreateSellerInput{
		AccountID: 3004, StoreName: "後來的店", ProspectInviteToken: "tok_reused",
	})
	if err != nil {
		t.Fatalf("建店不得因歸因衝突而失敗，got err = %v", err)
	}
	// review 2026-09-05 Important #3：不能只看 err == nil——那連 (nil, nil) 都會放行。
	if created == nil || created.StoreID == 0 {
		t.Fatal("seller 應該照樣建立")
	}
	// review 2026-09-05 Important #2：MarkConverted 一定要被呼叫過一次（且回 0
	// 列）才算「已被別的 store 認領」；沒斷言 markCalls 的話，一個把衝突當成
	// 「重試」的回歸（例如重打 MarkConverted 三次）一樣會通過這條測試。
	if stub.markCalls != 1 {
		t.Errorf("markCalls = %d, want 1（衝突不代表沒查，是查到後不覆蓋、不重試）", stub.markCalls)
	}
	if stub.consumeCalls != 0 {
		t.Errorf("consumeCalls = %d, want 0（歸因沒成立就不該消費 token）", stub.consumeCalls)
	}
}

func TestCreateSeller_TokenGrantsNoCertificationState(t *testing.T) {
	// 🔴 安全不變式（spec §5.3）：帶 token 與不帶 token 建立出來的 seller，
	// 其認證相關狀態必須完全相同。這條若轉紅，代表邀請連結成了繞過賣家
	// 認證閘門的路徑。
	stub := &stubProspectAttribution{
		invite: domain.ProspectInvite{ID: 13, ProspectID: 7002}, markedRows: 1,
	}
	uc := newSellerUseCaseForTest(t, stub)
	spy := &spyCertRepo{}
	uc.certRepo = spy

	withToken, err := uc.CreateSeller(context.Background(), CreateSellerInput{
		AccountID: 3005, StoreName: "A店", ProspectInviteToken: "tok_valid",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// review 2026-09-05 Important #1：這是本檔最重要的斷言。前面幾條 DTO 欄位
	// 比對是「兩條路徑長得一樣」的第二層佐證，便宜且值得留著；但真正抓「token
	// 路徑偷寫了認證狀態」的是這一條——certRepo 完全沒被碰過。
	if len(spy.calls) != 0 {
		t.Errorf("certRepo 被呼叫了 %v，token 路徑不得碰任何 certification_applications 狀態", spy.calls)
	}
	without, err := uc.CreateSeller(context.Background(), CreateSellerInput{
		AccountID: 3006, StoreName: "B店",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if withToken.CurrentCertificationID != without.CurrentCertificationID {
		t.Errorf("current_certification_id 不同：帶 token=%v 不帶=%v",
			withToken.CurrentCertificationID, without.CurrentCertificationID)
	}
	if withToken.CertifiedAt != nil {
		t.Error("帶 token 建立的 seller 不得帶有 certified_at——邀請連結只做歸因不授權")
	}
	if withToken.OnboardingCompleted != without.OnboardingCompleted {
		t.Errorf("onboarding_completed 不同：帶 token=%v 不帶=%v",
			withToken.OnboardingCompleted, without.OnboardingCompleted)
	}
}

// stubProspectRepo 是 domain.SellerProspectRepository 的最小 stub，只給
// SellerProspectUseCase 的單元測試用。嵌入的 domain.SellerProspectRepository
// 是 nil，只借它的方法集合過編譯檢查——除了下面明確覆寫的方法，其餘方法
// 這些測試從不呼叫。
type stubProspectRepo struct {
	domain.SellerProspectRepository
	markConvertedRows int64
	markConvertedErr  error

	// duplicatePairs 讓 TestListDuplicatesReportsMatchedKeyAndTruncation 能
	// 控制 ListDuplicatePairs 回傳的原始配對，藉此驗證 usecase 層的截斷與
	// MatchedOn 計算，不必碰真的 DB。
	duplicatePairs    []domain.ProspectDuplicatePair
	duplicatePairsErr error

	// storeExists 讓 TestConvertForAdminRejectsMissingStore／
	// TestConvertForAdminRejectsNonActiveProspect 能控制 CurrentStoreExists
	// 回傳值，藉此驗證 ConvertForAdmin 在綁定前先擋不存在的 store，
	// 不必碰真的 DB。
	storeExists    bool
	storeExistsErr error

	// lastInviteExpiresAt／lastInviteProspectID 記錄最後一次 CreateInvite 實際
	// 收到的引數，供 TestCreateInviteTokenIsUnpredictableAndCapped 透過
	// lastInviteValidDays() 換算天數，斷言 usecase 有把 180 天上限套用在
	// 傳給 repository 的到期時間上——不必碰真的 DB 或系統時鐘 mock。
	createInviteCalls    int
	lastInviteProspectID int64
	lastInviteToken      string
	lastInviteExpiresAt  time.Time

	// revokeInviteRows／revokeInviteErr 讓 TestRevokeInvite_* 能控制
	// RevokeInvite 的回傳值，藉此驗證 usecase 把 rows == 0 轉成
	// ErrProspectInviteNotRevocable，不必碰真的 DB。
	revokeInviteRows int64
	revokeInviteErr  error

	// lastMarkConvertedOperator 記錄最後一次 MarkConverted 實際收到的
	// operatorAccountID，供 TestConvertForAdminRecordsOperatorAccountID
	// 斷言 admin 手動歸因路徑有把操作者傳到底，不必碰真的 DB。
	lastMarkConvertedOperator *int64

	// getByIDStatus 讓 TestCreateInvite_RejectsNonActiveProspect 能控制
	// GetByID 回傳的 status，藉此驗證 usecase.CreateInvite 擋下非 ACTIVE
	// 的 prospect；空字串（零值，其餘既有測試不設定這個欄位）當 "ACTIVE"，
	// 維持這些測試建邀請前不關心 status 的既有假設。
	getByIDStatus string
}

func (s *stubProspectRepo) MarkConverted(ctx context.Context, prospectID, storeID int64, operatorAccountID *int64) (int64, error) {
	s.lastMarkConvertedOperator = operatorAccountID
	return s.markConvertedRows, s.markConvertedErr
}

func (s *stubProspectRepo) ListDuplicatePairs(context.Context) ([]domain.ProspectDuplicatePair, error) {
	return s.duplicatePairs, s.duplicatePairsErr
}

func (s *stubProspectRepo) CurrentStoreExists(context.Context, int64) (bool, error) {
	return s.storeExists, s.storeExistsErr
}

// GetByID 恆放行：CreateInvite 在產生 token 之前一定先呼叫這支方法確認
// prospect 存在，這個 stub 只負責讓 CreateInvite 能繼續往下跑，不驗證
// 「prospect 不存在」的拒絕路徑。status 預設 ACTIVE（見 getByIDStatus 欄位
// 註解），可由個別測試覆寫成其他值驗證 CreateInvite 的狀態拒絕路徑。
func (s *stubProspectRepo) GetByID(context.Context, int64) (domain.SellerProspect, error) {
	status := s.getByIDStatus
	if status == "" {
		status = "ACTIVE"
	}
	return domain.SellerProspect{ID: 1, Status: status}, nil
}

// CreateInvite 只記錄呼叫端實際傳入的引數並回一筆看起來合理的
// domain.ProspectInvite——token 的不可預測性與有效期上限都是
// usecase.SellerProspectUseCase.CreateInvite 自己的責任，這個 stub 不驗證，
// 只負責讓斷言能看到 usecase 傳了什麼下來。
func (s *stubProspectRepo) CreateInvite(_ context.Context, prospectID int64, token string, expiresAt time.Time) (domain.ProspectInvite, error) {
	s.createInviteCalls++
	s.lastInviteProspectID = prospectID
	s.lastInviteToken = token
	s.lastInviteExpiresAt = expiresAt
	return domain.ProspectInvite{ID: int64(s.createInviteCalls), ProspectID: prospectID, ExpiresAt: expiresAt}, nil
}

// lastInviteValidDays 把最後一次 CreateInvite 收到的 expiresAt 換算回「幾天後
// 到期」，四捨五入到最近的整數天——expiresAt 是 time.Now().Add(days*24h)
// 算出來的，呼叫這支方法與 usecase 呼叫 CreateInvite 之間的系統時鐘漂移是
// 微秒等級，四捨五入足以吸收掉。
func (s *stubProspectRepo) lastInviteValidDays() int {
	return int(math.Round(time.Until(s.lastInviteExpiresAt).Hours() / 24))
}

func (s *stubProspectRepo) RevokeInvite(context.Context, int64, int64) (int64, error) {
	return s.revokeInviteRows, s.revokeInviteErr
}

// 被合併掉的 loser 是 ARCHIVED：歸因必須拒絕，而不是讓 UPDATE 撞上
// chk_sp_merged_into 回 23514（DB 約束名洩漏到 service 邊界外，且對呼叫端
// 是 500）。這是 b99d7785b 同一類錯誤的第二個入口。
func TestMarkConvertedRejectsArchivedProspect(t *testing.T) {
	repo := &stubProspectRepo{markConvertedRows: 0}
	uc := NewSellerProspectUseCase(repo, nil)

	err := uc.MarkConverted(context.Background(), 123, 456, nil)

	require.ErrorIs(t, err, ErrProspectNotConvertible)
}

func TestConvertForAdminRejectsMissingStore(t *testing.T) {
	repo := &stubProspectRepo{storeExists: false}
	uc := NewSellerProspectUseCase(repo, nil)

	err := uc.ConvertForAdmin(context.Background(), 1, 999, nil)

	require.ErrorIs(t, err, ErrStoreNotFound)
}

func TestConvertForAdminRejectsNonActiveProspect(t *testing.T) {
	repo := &stubProspectRepo{storeExists: true, markConvertedRows: 0}
	uc := NewSellerProspectUseCase(repo, nil)

	err := uc.ConvertForAdmin(context.Background(), 1, 2, nil)

	require.ErrorIs(t, err, ErrProspectNotConvertible)
}

// TestConvertForAdminRecordsOperatorAccountID 是 admin 手動歸因這條路徑的
// 對照組（同檔 TestCreateSeller_ValidTokenAttributesProspectInSameTransaction
// 驗證 token 路徑傳 nil）：人工歸因決定一筆真實 store 認哪一筆 prospect 當
// 前身，不可逆，spec §5.3／§6 要求記下操作者。這裡斷言操作者的 account_id
// 有原樣傳到 repository.MarkConverted，會落地到 updated_by_account_id。
func TestConvertForAdminRecordsOperatorAccountID(t *testing.T) {
	repo := &stubProspectRepo{storeExists: true, markConvertedRows: 1}
	uc := NewSellerProspectUseCase(repo, nil)
	operatorID := int64(9001)

	err := uc.ConvertForAdmin(context.Background(), 1, 2, &operatorID)

	require.NoError(t, err)
	require.NotNil(t, repo.lastMarkConvertedOperator)
	require.Equal(t, operatorID, *repo.lastMarkConvertedOperator)
}
