package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// B-169 functional mocks for ViolationReportUseCase / FoodSafetyReportUseCase /
// SellerViolationUseCase tests. Pattern matches existing mock_test.go style.

// ========== mockSellerViolationReportRepository ==========

type mockSellerViolationReportRepository struct {
	CreateReportFn                        func(ctx context.Context, r *domain.SellerViolationReport) (*domain.SellerViolationReport, error)
	GetByIDFn                             func(ctx context.Context, id int64) (*domain.SellerViolationReport, error)
	GetExistingByReporterAndTargetTodayFn func(ctx context.Context, reporterID, targetSellerID int64, category domain.ViolationCategory) (*domain.SellerViolationReport, error)
	ListByReporterFn                      func(ctx context.Context, reporterID int64, since time.Time, limit, offset int32) ([]*domain.SellerViolationReport, error)
	AdminListFn                           func(ctx context.Context, filter domain.SellerViolationReportListFilter) ([]*domain.SellerViolationReport, error)
	AdminGetFn                            func(ctx context.Context, id int64) (*domain.SellerViolationReport, error)
	AdminUpdateStatusFn                   func(ctx context.Context, id int64, status domain.ReportStatus, investigatingAt, resolvedAt *time.Time, summary *string) (*domain.SellerViolationReport, error)
}

func (m *mockSellerViolationReportRepository) CreateReport(ctx context.Context, r *domain.SellerViolationReport) (*domain.SellerViolationReport, error) {
	if m.CreateReportFn != nil {
		return m.CreateReportFn(ctx, r)
	}
	return r, nil
}
func (m *mockSellerViolationReportRepository) GetByID(ctx context.Context, id int64) (*domain.SellerViolationReport, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (m *mockSellerViolationReportRepository) GetExistingByReporterAndTargetToday(ctx context.Context, reporterID, targetSellerID int64, category domain.ViolationCategory) (*domain.SellerViolationReport, error) {
	if m.GetExistingByReporterAndTargetTodayFn != nil {
		return m.GetExistingByReporterAndTargetTodayFn(ctx, reporterID, targetSellerID, category)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (m *mockSellerViolationReportRepository) ListByReporter(ctx context.Context, reporterID int64, since time.Time, limit, offset int32) ([]*domain.SellerViolationReport, error) {
	if m.ListByReporterFn != nil {
		return m.ListByReporterFn(ctx, reporterID, since, limit, offset)
	}
	return nil, nil
}
func (m *mockSellerViolationReportRepository) AdminList(ctx context.Context, filter domain.SellerViolationReportListFilter) ([]*domain.SellerViolationReport, error) {
	if m.AdminListFn != nil {
		return m.AdminListFn(ctx, filter)
	}
	return nil, nil
}
func (m *mockSellerViolationReportRepository) AdminGet(ctx context.Context, id int64) (*domain.SellerViolationReport, error) {
	if m.AdminGetFn != nil {
		return m.AdminGetFn(ctx, id)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (m *mockSellerViolationReportRepository) AdminUpdateStatus(ctx context.Context, id int64, status domain.ReportStatus, investigatingAt, resolvedAt *time.Time, summary *string) (*domain.SellerViolationReport, error) {
	if m.AdminUpdateStatusFn != nil {
		return m.AdminUpdateStatusFn(ctx, id, status, investigatingAt, resolvedAt, summary)
	}
	return nil, nil
}

// ========== mockFoodSafetyReportRepository ==========

type mockFoodSafetyReportRepository struct {
	CreateFn                              func(ctx context.Context, r *domain.FoodSafetyReport) (*domain.FoodSafetyReport, error)
	GetByIDFn                             func(ctx context.Context, id int64) (*domain.FoodSafetyReport, error)
	GetExistingByPhoneAndSellerThisWeekFn func(ctx context.Context, phone string, targetSellerID int64) (*domain.FoodSafetyReport, error)
	AdminListFn                           func(ctx context.Context, filter domain.FoodSafetyReportListFilter) ([]*domain.FoodSafetyReport, error)
	AdminGetFn                            func(ctx context.Context, id int64) (*domain.FoodSafetyReport, error)
	AdminUpdateStatusFn                   func(ctx context.Context, id int64, status domain.ReportStatus, investigatingAt, resolvedAt *time.Time, summary *string) (*domain.FoodSafetyReport, error)
	AcknowledgeFn                         func(ctx context.Context, id, adminID int64) (*domain.FoodSafetyReport, error)
	MarkFDAReportedFn                     func(ctx context.Context, id int64, channel domain.FDAReportChannel, refNo string) (*domain.FoodSafetyReport, error)
	AnonymizeOldPhonesFn                  func(ctx context.Context) ([]domain.AnonymizedFoodSafetyRecord, error)
}

func (m *mockFoodSafetyReportRepository) Create(ctx context.Context, r *domain.FoodSafetyReport) (*domain.FoodSafetyReport, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, r)
	}
	return r, nil
}
func (m *mockFoodSafetyReportRepository) GetByID(ctx context.Context, id int64) (*domain.FoodSafetyReport, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (m *mockFoodSafetyReportRepository) GetExistingByPhoneAndSellerThisWeek(ctx context.Context, phone string, targetSellerID int64) (*domain.FoodSafetyReport, error) {
	if m.GetExistingByPhoneAndSellerThisWeekFn != nil {
		return m.GetExistingByPhoneAndSellerThisWeekFn(ctx, phone, targetSellerID)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (m *mockFoodSafetyReportRepository) AdminList(ctx context.Context, filter domain.FoodSafetyReportListFilter) ([]*domain.FoodSafetyReport, error) {
	if m.AdminListFn != nil {
		return m.AdminListFn(ctx, filter)
	}
	return nil, nil
}
func (m *mockFoodSafetyReportRepository) AdminGet(ctx context.Context, id int64) (*domain.FoodSafetyReport, error) {
	if m.AdminGetFn != nil {
		return m.AdminGetFn(ctx, id)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (m *mockFoodSafetyReportRepository) AdminUpdateStatus(ctx context.Context, id int64, status domain.ReportStatus, investigatingAt, resolvedAt *time.Time, summary *string) (*domain.FoodSafetyReport, error) {
	if m.AdminUpdateStatusFn != nil {
		return m.AdminUpdateStatusFn(ctx, id, status, investigatingAt, resolvedAt, summary)
	}
	return nil, nil
}
func (m *mockFoodSafetyReportRepository) Acknowledge(ctx context.Context, id, adminID int64) (*domain.FoodSafetyReport, error) {
	if m.AcknowledgeFn != nil {
		return m.AcknowledgeFn(ctx, id, adminID)
	}
	return nil, nil
}
func (m *mockFoodSafetyReportRepository) MarkFDAReported(ctx context.Context, id int64, channel domain.FDAReportChannel, refNo string) (*domain.FoodSafetyReport, error) {
	if m.MarkFDAReportedFn != nil {
		return m.MarkFDAReportedFn(ctx, id, channel, refNo)
	}
	return nil, nil
}
func (m *mockFoodSafetyReportRepository) AnonymizeOldPhones(ctx context.Context) ([]domain.AnonymizedFoodSafetyRecord, error) {
	if m.AnonymizeOldPhonesFn != nil {
		return m.AnonymizeOldPhonesFn(ctx)
	}
	return nil, nil
}

// ========== mockSellerViolationRepository ==========

type mockSellerViolationRepository struct {
	AcquireCreateLockFn               func(ctx context.Context, sellerID int64) error
	CountMediumLast90dFn              func(ctx context.Context, sellerID int64) (int64, error)
	CreateViolationFn                 func(ctx context.Context, v *domain.SellerViolation) (*domain.SellerViolation, error)
	GetByIDFn                         func(ctx context.Context, id int64) (*domain.SellerViolation, error)
	AdminUpdateMutableFn              func(ctx context.Context, id int64, description string, publicSummary *string, expiresAt *time.Time) (*domain.SellerViolation, error)
	AdminRevokeFn                     func(ctx context.Context, id int64, reason domain.RevokedReason) (*domain.SellerViolation, error)
	ListBySellerSelfFn                func(ctx context.Context, sellerID int64, limit, offset int32) ([]*domain.SellerViolation, error)
	ListPublicLast90dHighFn           func(ctx context.Context, sellerID int64) ([]*domain.SellerViolation, error)
	AdminListFn                       func(ctx context.Context, filter domain.SellerViolationListFilter) ([]*domain.SellerViolation, error)
	ListExpiredFullSuspensionsFn      func(ctx context.Context) ([]*domain.SellerViolation, error)
	HasActiveFullSuspensionBySellerFn func(ctx context.Context, sellerID int64) (bool, error)
	SubmitAppealFn                    func(ctx context.Context, id int64, reason string, evidenceAssetIDs []string) (*domain.SellerViolation, error)
	DecideAppealAcceptFn              func(ctx context.Context, id, adminID int64, reason string) (*domain.SellerViolation, error)
	DecideAppealRejectFn              func(ctx context.Context, id, adminID int64, reason string) (*domain.SellerViolation, error)
}

func (m *mockSellerViolationRepository) AcquireCreateLock(ctx context.Context, sellerID int64) error {
	if m.AcquireCreateLockFn != nil {
		return m.AcquireCreateLockFn(ctx, sellerID)
	}
	return nil
}
func (m *mockSellerViolationRepository) CountMediumLast90d(ctx context.Context, sellerID int64) (int64, error) {
	if m.CountMediumLast90dFn != nil {
		return m.CountMediumLast90dFn(ctx, sellerID)
	}
	return 0, nil
}
func (m *mockSellerViolationRepository) CreateViolation(ctx context.Context, v *domain.SellerViolation) (*domain.SellerViolation, error) {
	if m.CreateViolationFn != nil {
		return m.CreateViolationFn(ctx, v)
	}
	return v, nil
}
func (m *mockSellerViolationRepository) GetByID(ctx context.Context, id int64) (*domain.SellerViolation, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (m *mockSellerViolationRepository) AdminUpdateMutable(ctx context.Context, id int64, description string, publicSummary *string, expiresAt *time.Time) (*domain.SellerViolation, error) {
	if m.AdminUpdateMutableFn != nil {
		return m.AdminUpdateMutableFn(ctx, id, description, publicSummary, expiresAt)
	}
	return nil, nil
}
func (m *mockSellerViolationRepository) AdminRevoke(ctx context.Context, id int64, reason domain.RevokedReason) (*domain.SellerViolation, error) {
	if m.AdminRevokeFn != nil {
		return m.AdminRevokeFn(ctx, id, reason)
	}
	return nil, nil
}
func (m *mockSellerViolationRepository) ListBySellerSelf(ctx context.Context, sellerID int64, limit, offset int32) ([]*domain.SellerViolation, error) {
	if m.ListBySellerSelfFn != nil {
		return m.ListBySellerSelfFn(ctx, sellerID, limit, offset)
	}
	return nil, nil
}
func (m *mockSellerViolationRepository) ListPublicLast90dHigh(ctx context.Context, sellerID int64) ([]*domain.SellerViolation, error) {
	if m.ListPublicLast90dHighFn != nil {
		return m.ListPublicLast90dHighFn(ctx, sellerID)
	}
	return nil, nil
}
func (m *mockSellerViolationRepository) AdminList(ctx context.Context, filter domain.SellerViolationListFilter) ([]*domain.SellerViolation, error) {
	if m.AdminListFn != nil {
		return m.AdminListFn(ctx, filter)
	}
	return nil, nil
}
func (m *mockSellerViolationRepository) ListExpiredFullSuspensions(ctx context.Context) ([]*domain.SellerViolation, error) {
	if m.ListExpiredFullSuspensionsFn != nil {
		return m.ListExpiredFullSuspensionsFn(ctx)
	}
	return nil, nil
}
func (m *mockSellerViolationRepository) HasActiveFullSuspensionBySeller(ctx context.Context, sellerID int64) (bool, error) {
	if m.HasActiveFullSuspensionBySellerFn != nil {
		return m.HasActiveFullSuspensionBySellerFn(ctx, sellerID)
	}
	return false, nil
}
func (m *mockSellerViolationRepository) SubmitAppeal(ctx context.Context, id int64, reason string, evidenceAssetIDs []string) (*domain.SellerViolation, error) {
	if m.SubmitAppealFn != nil {
		return m.SubmitAppealFn(ctx, id, reason, evidenceAssetIDs)
	}
	return nil, nil
}
func (m *mockSellerViolationRepository) DecideAppealAccept(ctx context.Context, id, adminID int64, reason string) (*domain.SellerViolation, error) {
	if m.DecideAppealAcceptFn != nil {
		return m.DecideAppealAcceptFn(ctx, id, adminID, reason)
	}
	return nil, nil
}
func (m *mockSellerViolationRepository) DecideAppealReject(ctx context.Context, id, adminID int64, reason string) (*domain.SellerViolation, error) {
	if m.DecideAppealRejectFn != nil {
		return m.DecideAppealRejectFn(ctx, id, adminID, reason)
	}
	return nil, nil
}

// ========== mockSellerNotificationLogRepository ==========

type mockSellerNotificationLogRepository struct {
	CreateFn func(ctx context.Context, log *domain.SellerNotificationLog) (*domain.SellerNotificationLog, error)
}

func (m *mockSellerNotificationLogRepository) Create(ctx context.Context, log *domain.SellerNotificationLog) (*domain.SellerNotificationLog, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, log)
	}
	return log, nil
}
func (m *mockSellerNotificationLogRepository) ListBySourceReport(ctx context.Context, sourceTable string, sourceReportID int64) ([]*domain.SellerNotificationLog, error) {
	return nil, nil
}
func (m *mockSellerNotificationLogRepository) ListBySeller(ctx context.Context, sellerID int64, limit, offset int32) ([]*domain.SellerNotificationLog, error) {
	return nil, nil
}

// ========== mockGovernancePublisher ==========

// mutex-guarded：usecase publish 是 fire-and-forget goroutine，測試 goroutine 會並發讀。
type mockGovernancePublisher struct {
	mu           sync.Mutex
	publishCalls []mockPublishCall
}

type mockPublishCall struct {
	Exchange   string
	RoutingKey string
	// Payload 保留 usecase 傳進來的原始物件（未經 JSON round-trip），
	// 讓測試能直接斷言 wire key —— governance 事件的 payload key 是跨服務契約
	// （notification-service 未來的消費端照這些 key 讀），2026-08-02 那輪
	// 命名債修法就是因為沒有這層斷言才漏掉這條 direct-publish 路徑。
	Payload interface{}
}

func (m *mockGovernancePublisher) Publish(_ context.Context, exchange, routingKey string, event interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishCalls = append(m.publishCalls, mockPublishCall{Exchange: exchange, RoutingKey: routingKey, Payload: event})
	return nil
}

func (m *mockGovernancePublisher) Calls() []mockPublishCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]mockPublishCall(nil), m.publishCalls...)
}

// awaitGovernancePublish 等 fire-and-forget publish goroutine 至少發出 want 筆，
// 逾時直接 t.Fatal。取代各測試各寫一份 2 秒輪詢迴圈。
func awaitGovernancePublish(t *testing.T, pub *mockGovernancePublisher, want int) []mockPublishCall {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if calls := pub.Calls(); len(calls) >= want {
			return calls
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected at least %d governance publish call(s), got %d", want, len(pub.Calls()))
	return nil
}

// ========== mockPublicViolationCache ==========

type mockPublicViolationCache struct {
	GetFn           func(ctx context.Context, sellerID int64) ([]*domain.SellerViolation, bool)
	SetCalls        []int64
	InvalidateCalls []int64
}

func (m *mockPublicViolationCache) Get(ctx context.Context, sellerID int64) ([]*domain.SellerViolation, bool) {
	if m.GetFn != nil {
		return m.GetFn(ctx, sellerID)
	}
	return nil, false
}
func (m *mockPublicViolationCache) Set(_ context.Context, sellerID int64, _ []*domain.SellerViolation) {
	m.SetCalls = append(m.SetCalls, sellerID)
}
func (m *mockPublicViolationCache) Invalidate(_ context.Context, sellerID int64) {
	m.InvalidateCalls = append(m.InvalidateCalls, sellerID)
}

// ========== mockSellerSuspensionRepository ==========

type mockSellerSuspensionRepository struct {
	InsertFn               func(ctx context.Context, s *domain.SellerSuspension) error
	GetActiveByAccountIDFn func(ctx context.Context, accountID int64) (*domain.SellerSuspension, error)
	HasActiveByAccountIDFn func(ctx context.Context, accountID int64) (bool, error)
	GetByIDFn              func(ctx context.Context, id int64) (*domain.SellerSuspension, error)
	LiftFn                 func(ctx context.Context, id, liftedByAdminID int64, now time.Time) (*domain.SellerSuspension, error)
	ListByAccountIDFn      func(ctx context.Context, accountID int64, limit, offset int32) ([]*domain.SellerSuspension, error)
}

func (m *mockSellerSuspensionRepository) Insert(ctx context.Context, s *domain.SellerSuspension) error {
	if m.InsertFn != nil {
		return m.InsertFn(ctx, s)
	}
	return nil
}
func (m *mockSellerSuspensionRepository) GetActiveByAccountID(ctx context.Context, accountID int64) (*domain.SellerSuspension, error) {
	if m.GetActiveByAccountIDFn != nil {
		return m.GetActiveByAccountIDFn(ctx, accountID)
	}
	return nil, nil
}
func (m *mockSellerSuspensionRepository) HasActiveByAccountID(ctx context.Context, accountID int64) (bool, error) {
	if m.HasActiveByAccountIDFn != nil {
		return m.HasActiveByAccountIDFn(ctx, accountID)
	}
	return false, nil
}
func (m *mockSellerSuspensionRepository) GetByID(ctx context.Context, id int64) (*domain.SellerSuspension, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, pkgerrors.NewNotFound("not found")
}
func (m *mockSellerSuspensionRepository) Lift(ctx context.Context, id, liftedByAdminID int64, now time.Time) (*domain.SellerSuspension, error) {
	if m.LiftFn != nil {
		return m.LiftFn(ctx, id, liftedByAdminID, now)
	}
	return nil, nil
}
func (m *mockSellerSuspensionRepository) ListByAccountID(ctx context.Context, accountID int64, limit, offset int32) ([]*domain.SellerSuspension, error) {
	if m.ListByAccountIDFn != nil {
		return m.ListByAccountIDFn(ctx, accountID, limit, offset)
	}
	return nil, nil
}

// ========== mockSellerSuspensionRedis ==========

type mockSellerSuspensionRedis struct {
	SetCalls   []int64
	SetValues  []string // parallel to SetCalls — the reasonCode/sentinel passed to each SetSuspended
	ClearCalls []int64
}

func (m *mockSellerSuspensionRedis) SetSuspended(_ context.Context, accountID int64, reasonCode string) {
	m.SetCalls = append(m.SetCalls, accountID)
	m.SetValues = append(m.SetValues, reasonCode)
}
func (m *mockSellerSuspensionRedis) ClearSuspended(_ context.Context, accountID int64) {
	m.ClearCalls = append(m.ClearCalls, accountID)
}

// ========== mockTxManager ==========

// Inline TxManager — fn 直接執行（測試不需真實 transaction），ctx 直通。
type inlineTxManager struct{}

func (inlineTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// ========== mockMediaAssetDeleter ==========

type mockMediaAssetDeleter struct {
	DeletedBatches [][]string
}

func (m *mockMediaAssetDeleter) BatchDeleteAssets(_ context.Context, assetIDs []string) error {
	m.DeletedBatches = append(m.DeletedBatches, assetIDs)
	return nil
}

// ========== Test TSID generator ==========

func newCounterTSID() func() int64 {
	var counter int64 = 1_000_000
	return func() int64 {
		counter++
		return counter
	}
}

// ========== sentinel ==========

var errPgUniqueViolation = &domain.ErrDuplicate{
	ConstraintName: "uq_seller_violation_reports_dedup_calendar_day",
	Underlying:     errors.New("23505"),
}
