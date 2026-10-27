package domain

import (
	"context"
	"time"
)

// B-169 Seller Governance — repository interfaces。對應 db/queries/{seller_violation_reports,food_safety_reports,seller_violations,seller_notification_log}.sql。

// ErrDuplicateReport B-169：D13 partial unique 觸發時 application 收到的 sentinel error；
// usecase 應接住並轉成 409 Conflict + 既有 report_id（calling site 用 GetExistingReport... query 拿 id）。
// PG 錯誤代碼 23505 (unique_violation) 由 repository 轉成此 error。
type ErrDuplicate struct {
	ConstraintName string
	Underlying     error
}

func (e *ErrDuplicate) Error() string { return "duplicate (constraint: " + e.ConstraintName + ")" }
func (e *ErrDuplicate) Unwrap() error { return e.Underlying }

// SellerViolationReportRepository 對應 seller_violation_reports table。
//
// CreateReport 在 D13 calendar-day partial unique 觸發時應回傳 *ErrDuplicate；
// usecase 收到後呼叫 GetExistingByReporterAndTargetToday 拿既有 id。
type SellerViolationReportRepository interface {
	CreateReport(ctx context.Context, r *SellerViolationReport) (*SellerViolationReport, error)
	GetByID(ctx context.Context, id int64) (*SellerViolationReport, error)
	GetExistingByReporterAndTargetToday(ctx context.Context, reporterID, targetSellerID int64, category ViolationCategory) (*SellerViolationReport, error)
	ListByReporter(ctx context.Context, reporterID int64, since time.Time, limit, offset int32) ([]*SellerViolationReport, error)
	AdminList(ctx context.Context, filter SellerViolationReportListFilter) ([]*SellerViolationReport, error)
	AdminGet(ctx context.Context, id int64) (*SellerViolationReport, error)
	AdminUpdateStatus(ctx context.Context, id int64, status ReportStatus, investigatingAt, resolvedAt *time.Time, summary *string) (*SellerViolationReport, error)
}

// FoodSafetyReportRepository 對應 food_safety_reports table。
//
// Create 在 B1 ISO-week partial unique 觸發時應回傳 *ErrDuplicate；
// usecase 收到後呼叫 GetExistingByPhoneAndSellerThisWeek 拿既有 id（silent drop 路徑）。
type FoodSafetyReportRepository interface {
	Create(ctx context.Context, r *FoodSafetyReport) (*FoodSafetyReport, error)
	GetByID(ctx context.Context, id int64) (*FoodSafetyReport, error)
	GetExistingByPhoneAndSellerThisWeek(ctx context.Context, phone string, targetSellerID int64) (*FoodSafetyReport, error)
	AdminList(ctx context.Context, filter FoodSafetyReportListFilter) ([]*FoodSafetyReport, error)
	AdminGet(ctx context.Context, id int64) (*FoodSafetyReport, error)
	AdminUpdateStatus(ctx context.Context, id int64, status ReportStatus, investigatingAt, resolvedAt *time.Time, summary *string) (*FoodSafetyReport, error)
	Acknowledge(ctx context.Context, id, adminID int64) (*FoodSafetyReport, error)
	MarkFDAReported(ctx context.Context, id int64, channel FDAReportChannel, refNo string) (*FoodSafetyReport, error)
	// AnonymizeOldPhones 回傳 anonymize 過的 records (id + evidence_asset_ids)，cron worker 用 evidence_asset_ids 呼叫 media-service 刪 OSS 實體。
	AnonymizeOldPhones(ctx context.Context) ([]AnonymizedFoodSafetyRecord, error)
}

// AnonymizedFoodSafetyRecord cron worker 用，配合 media-service 同步刪除 OSS。
type AnonymizedFoodSafetyRecord struct {
	ID               int64
	EvidenceAssetIDs []string
}

// SellerViolationRepository 對應 seller_violations table。
//
// CreateViolation 必須在 transaction 內呼叫（usecase 透過 pkg/database.TxManager.WithTx 包裝），
// 且呼叫前先 AcquireCreateLock(seller_id) 串行同 seller 並發建立（D6）。
// CountMediumLast90d 用於 inline auto-escalation 判定（同 transaction 內，lock 已串行不需要 SELECT FOR UPDATE）。
type SellerViolationRepository interface {
	// Transactional helpers — methods auto-pick up tx from ctx via pkg/database.GetDBTX.
	AcquireCreateLock(ctx context.Context, sellerID int64) error
	CountMediumLast90d(ctx context.Context, sellerID int64) (int64, error)

	// Mutate
	CreateViolation(ctx context.Context, v *SellerViolation) (*SellerViolation, error)
	GetByID(ctx context.Context, id int64) (*SellerViolation, error)
	AdminUpdateMutable(ctx context.Context, id int64, description string, publicSummary *string, expiresAt *time.Time) (*SellerViolation, error)
	AdminRevoke(ctx context.Context, id int64, reason RevokedReason) (*SellerViolation, error)

	// Listing
	ListBySellerSelf(ctx context.Context, sellerID int64, limit, offset int32) ([]*SellerViolation, error)
	ListPublicLast90dHigh(ctx context.Context, sellerID int64) ([]*SellerViolation, error)
	AdminList(ctx context.Context, filter SellerViolationListFilter) ([]*SellerViolation, error)

	// Cron
	ListExpiredFullSuspensions(ctx context.Context) ([]*SellerViolation, error)

	// HasActiveFullSuspensionBySeller 32 §13.3 recomputeSellerStatus 讀端：sellerID
	// 為 store surrogate（②）。生效中定義同 §6.2 cron：penalty_type='full_suspension'
	// AND revoked_at IS NULL AND expires_at > NOW()。
	HasActiveFullSuspensionBySeller(ctx context.Context, sellerID int64) (bool, error)

	// Appeal
	SubmitAppeal(ctx context.Context, id int64, reason string, evidenceAssetIDs []string) (*SellerViolation, error)
	DecideAppealAccept(ctx context.Context, id, adminID int64, reason string) (*SellerViolation, error)
	DecideAppealReject(ctx context.Context, id, adminID int64, reason string) (*SellerViolation, error)
}

// SellerNotificationLogRepository 對應 seller_notification_log table。audit-only。
type SellerNotificationLogRepository interface {
	Create(ctx context.Context, log *SellerNotificationLog) (*SellerNotificationLog, error)
	ListBySourceReport(ctx context.Context, sourceTable string, sourceReportID int64) ([]*SellerNotificationLog, error)
	ListBySeller(ctx context.Context, sellerID int64, limit, offset int32) ([]*SellerNotificationLog, error)
}

// SellerNotificationLog audit row。
type SellerNotificationLog struct {
	SentAt            time.Time
	SourceReportID    *int64
	SourceReportTable *string
	SentByAdminID     *int64

	MessagePayload []byte // jsonb raw
	TemplateKey    string

	ID       int64
	SellerID int64
}
