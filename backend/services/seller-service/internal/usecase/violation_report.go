package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/media"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// ViolationReportUseCase B-169 登入端商家違規檢舉。
// dedup 由 schema D13 partial unique 強制；usecase 處理 ErrDuplicate sentinel + 回 既有 report_id。
type ViolationReportUseCase struct {
	repo        domain.SellerViolationReportRepository
	tsid        func() int64
	mediaClient MediaClient
}

// NewViolationReportUseCase constructs the usecase.
func NewViolationReportUseCase(repo domain.SellerViolationReportRepository, tsid func() int64, mediaClient MediaClient) *ViolationReportUseCase {
	return &ViolationReportUseCase{repo: repo, tsid: tsid, mediaClient: mediaClient}
}

// finalizeEvidence converts staged tokens to asset_id（issue #1516 同 root cause 一次全修，
// §17.7：SellerSubmitAppeal 的姊妹缺口 — POST /app/violation_reports 同樣 JWT 認證後可達，
// 前端可用既有 /app/media/upload_sessions 取得 staged_token，不是 dead path，故一併修）。
// 空輸入回 nil（no-op）；finalize 失敗直接回錯（400），避免過期佐證悄悄變空值落庫。
func (u *ViolationReportUseCase) finalizeEvidence(ctx context.Context, stagedTokens []string) ([]string, error) {
	if len(stagedTokens) == 0 {
		return nil, nil
	}
	assets, err := u.mediaClient.FinalizeImages(ctx, stagedTokens, media.RefTypeGovernanceEvidence)
	if err != nil {
		return nil, pkgerrors.NewInvalidInput("佐證圖片已過期或無效，請重新上傳: " + err.Error())
	}
	assetIDs := make([]string, 0, len(assets))
	for _, a := range assets {
		assetIDs = append(assetIDs, a.AssetId)
	}
	return assetIDs, nil
}

// CreateLoggedInReport 對應 spec：POST /app/violation_reports.
//
// 流程：
//  1. domain validate
//  2. INSERT；DB partial unique 觸發時 application 捕捉 ErrDuplicate
//  3. 重複時 GetExistingByReporterAndTargetToday → 回 既有 report_id + 9090 (DuplicateReport)
func (u *ViolationReportUseCase) CreateLoggedInReport(ctx context.Context, in *domain.SellerViolationReport) (*domain.SellerViolationReport, error) {
	now := time.Now()
	if in.CreatedAt.IsZero() {
		in.CreatedAt = now
	}
	if in.UpdatedAt.IsZero() {
		in.UpdatedAt = now
	}
	if in.Status == "" {
		in.Status = domain.ReportStatusPending
	}
	if in.ID == 0 {
		in.ID = u.tsid()
	}
	if err := in.Validate(); err != nil {
		return nil, pkgerrors.NewInvalidInput(err.Error())
	}

	// Finalize staged tokens → asset_id（issue #1516）：舊版把 staged_token JWT 原封不動存
	// 進 evidence_asset_ids，30 分鐘 TTL 一過 admin 就讀不到佐證。
	assetIDs, err := u.finalizeEvidence(ctx, in.EvidenceAssetIDs)
	if err != nil {
		return nil, err
	}
	in.EvidenceAssetIDs = assetIDs

	// Link 在 INSERT 之前且失敗即中止（#2222）：assetId 一旦寫進 evidence_asset_ids 就是
	// 「業務列指向」，此時若沒有 asset_reference，7 天後 media GC 會把檢舉佐證連同 R2
	// 物件實體刪除。ID 已於上面預先產生（TSID），故 Link 可先於 INSERT。
	if len(in.EvidenceAssetIDs) > 0 {
		if linkErr := u.mediaClient.LinkImages(ctx, media.RefTypeGovernanceEvidence,
			fmt.Sprintf("%d", in.ID), in.EvidenceAssetIDs); linkErr != nil {
			return nil, fmt.Errorf("link violation report evidence images (report_id=%d): %w", in.ID, linkErr)
		}
	}

	report, err := u.repo.CreateReport(ctx, in)
	if err != nil {
		// INSERT 沒成立 → 上面建立的引用沒有任何業務列指向它，釋放後由 GC 回收，
		// 否則會變成永遠不會被回收的引用（洩漏）。best-effort。
		if len(in.EvidenceAssetIDs) > 0 {
			if relErr := u.mediaClient.ReleaseReferences(ctx, media.RefTypeGovernanceEvidence,
				[]string{fmt.Sprintf("%d", in.ID)}, nil); relErr != nil {
				logger.Warn(ctx, "failed to release evidence references after failed report insert",
					zap.Int64("report_id", in.ID), zap.Error(relErr))
			}
		}
		var dup *domain.ErrDuplicate
		if errors.As(err, &dup) {
			// D13 calendar-day partial unique 觸發 — 拿既有 report_id 回 client。
			existing, getErr := u.repo.GetExistingByReporterAndTargetToday(ctx, in.ReporterAccountID, in.TargetSellerID, in.Category)
			if getErr != nil {
				logger.Warn(ctx, "duplicate report but failed to fetch existing",
					zap.Error(getErr),
					zap.Int64("reporter_account_id", in.ReporterAccountID),
					zap.Int64("target_seller_id", in.TargetSellerID))
				return nil, pkgerrors.NewConflict("duplicate report within same calendar day")
			}
			return existing, pkgerrors.NewConflict("duplicate report within same calendar day")
		}
		return nil, err
	}

	logger.Info(ctx, "violation report created",
		zap.Int64("report_id", report.ID),
		zap.Int64("target_seller_id", report.TargetSellerID),
		zap.String("category", string(report.Category)))
	return report, nil
}

// ListMyReports 對應 spec：GET /app/violation_reports/me.
func (u *ViolationReportUseCase) ListMyReports(ctx context.Context, reporterID int64, since time.Time, limit, offset int32) ([]*domain.SellerViolationReport, error) {
	if since.IsZero() {
		since = time.Now().Add(-90 * 24 * time.Hour) // 預設近 90 天
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return u.repo.ListByReporter(ctx, reporterID, since, limit, offset)
}

// AdminListReports 對應 spec：GET /admin/violation_reports?source=seller_violation_reports
// （source=food_safety_reports 由 FoodSafetyReportUseCase 處理；admin handler 在 SellerViolationUseCase 統一 union）
func (u *ViolationReportUseCase) AdminListReports(ctx context.Context, filter domain.SellerViolationReportListFilter) ([]*domain.SellerViolationReport, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	return u.repo.AdminList(ctx, filter)
}

// AdminGetReport 對應 spec：admin detail page.
func (u *ViolationReportUseCase) AdminGetReport(ctx context.Context, id int64) (*domain.SellerViolationReport, error) {
	return u.repo.AdminGet(ctx, id)
}

// AdminPatchStatus 對應 spec：PATCH /admin/seller_violation_reports/{id}.
//
// 狀態推進合法性由 domain.ReportStatus.CanTransitionTo 檢查。
func (u *ViolationReportUseCase) AdminPatchStatus(ctx context.Context, id int64, next domain.ReportStatus, summary *string) (*domain.SellerViolationReport, error) {
	if !next.IsValid() {
		return nil, pkgerrors.NewInvalidInput("invalid status")
	}
	cur, err := u.repo.AdminGet(ctx, id)
	if err != nil {
		return nil, err
	}
	if !cur.Status.CanTransitionTo(next) {
		return nil, pkgerrors.NewConflict("invalid status transition")
	}

	now := time.Now()
	var investigatingAt, resolvedAt *time.Time
	if next == domain.ReportStatusInvestigating {
		investigatingAt = &now
	}
	if next.IsTerminal() {
		resolvedAt = &now
	}
	return u.repo.AdminUpdateStatus(ctx, id, next, investigatingAt, resolvedAt, summary)
}
