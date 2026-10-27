package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// SellerNotificationPublisher abstracts RabbitMQ publish for B-169 notifications.
// 由 *mq.Publisher 實作（既有 pkg/mq.Publisher.Publish）。
type SellerNotificationPublisher interface {
	Publish(ctx context.Context, exchange, routingKey string, event interface{}) error
}

// MediaAssetDeleter abstracts media-service OSS object deletion (used by phone anonymize cron, B3 cleanup sync).
type MediaAssetDeleter interface {
	BatchDeleteAssets(ctx context.Context, assetIDs []string) error
}

// AdminInfoFetcher 拿 admin 資訊用於 NotifySeller / Audit (optional — 簡化版不要求)。
// 此處 placeholder，未來若需要 adminID → 名稱可注入。
//
// FoodSafetyReportUseCase B-169 公開無登入食安通報 + admin 處理.
type FoodSafetyReportUseCase struct {
	repo       domain.FoodSafetyReportRepository
	logRepo    domain.SellerNotificationLogRepository
	sellerRepo domain.SellerRepository
	publisher  SellerNotificationPublisher
	media      MediaAssetDeleter
	tsid       func() int64
}

// NewFoodSafetyReportUseCase constructs the usecase.
func NewFoodSafetyReportUseCase(
	repo domain.FoodSafetyReportRepository,
	logRepo domain.SellerNotificationLogRepository,
	sellerRepo domain.SellerRepository,
	publisher SellerNotificationPublisher,
	media MediaAssetDeleter,
	tsid func() int64,
) *FoodSafetyReportUseCase {
	return &FoodSafetyReportUseCase{
		repo:       repo,
		logRepo:    logRepo,
		sellerRepo: sellerRepo,
		publisher:  publisher,
		media:      media,
		tsid:       tsid,
	}
}

// CreatePublicReport 對應 spec：POST /public/food_safety_reports.
//
// 流程：
//  1. NormalizePhone (必填，台灣手機格式)
//  2. domain validate
//  3. INSERT；DB B1 ISO-week partial unique 觸發 ErrDuplicate → silent drop 路徑回 既有 report_id 與 200 OK
//
// admin UI list 視覺強調是新建 record 的後續觸發點（D9 不 push 任何 channel）。
func (u *FoodSafetyReportUseCase) CreatePublicReport(ctx context.Context, in *domain.FoodSafetyReport) (*domain.FoodSafetyReport, time.Time, error) {
	normalizedPhone, err := domain.NormalizePhone(in.ReporterPhone)
	if err != nil {
		return nil, time.Time{}, pkgerrors.NewInvalidInput(err.Error())
	}
	in.ReporterPhone = normalizedPhone

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
		return nil, time.Time{}, pkgerrors.NewInvalidInput(err.Error())
	}

	report, err := u.repo.Create(ctx, in)
	ackEstimated := in.CreatedAt.Add(4 * time.Hour)
	if err != nil {
		var dup *domain.ErrDuplicate
		if errors.As(err, &dup) {
			// B1 silent drop 路徑：撈既有 record，仍回 200 OK 與既有 report_id。
			if in.TargetSellerID != nil {
				existing, getErr := u.repo.GetExistingByPhoneAndSellerThisWeek(ctx, in.ReporterPhone, *in.TargetSellerID)
				if getErr == nil {
					return existing, ackEstimated, nil
				}
				logger.Warn(ctx, "silent drop but failed to fetch existing",
					zap.Error(getErr),
					zap.String("reporter_phone_suffix", phoneSuffix(in.ReporterPhone)))
			}
			// fallback：silent drop 但無法撈既有 — 仍回 200 與 0 id（避免暴露差異）。
			return &domain.FoodSafetyReport{ID: 0, CreatedAt: now}, ackEstimated, nil
		}
		return nil, time.Time{}, err
	}
	logger.Info(ctx, "food safety report created",
		zap.Int64("report_id", report.ID),
		zap.String("reporter_phone_suffix", phoneSuffix(report.ReporterPhone)))
	return report, ackEstimated, nil
}

// AdminListReports 對應 spec：GET /admin/violation_reports?source=food_safety_reports（or critical filter）.
func (u *FoodSafetyReportUseCase) AdminListReports(ctx context.Context, filter domain.FoodSafetyReportListFilter) ([]*domain.FoodSafetyReport, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	return u.repo.AdminList(ctx, filter)
}

// AdminGetReport 對應 spec：admin detail page.
func (u *FoodSafetyReportUseCase) AdminGetReport(ctx context.Context, id int64) (*domain.FoodSafetyReport, error) {
	return u.repo.AdminGet(ctx, id)
}

// AdminPatchStatus 對應 spec：PATCH /admin/food_safety_reports/{id}.
func (u *FoodSafetyReportUseCase) AdminPatchStatus(ctx context.Context, id int64, next domain.ReportStatus, summary *string) (*domain.FoodSafetyReport, error) {
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

	// food_safety extra: 推到 investigating 前必須已 ack（食安事件流程要求）
	if next == domain.ReportStatusInvestigating && cur.AcknowledgedAt == nil {
		return nil, pkgerrors.NewConflict("food safety report must be acknowledged before investigating")
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

// AdminAcknowledge 對應 spec：POST /admin/food_safety_reports/{id}/ack.
//
// idempotent friendly：已 ack 過的 record 回 409 + 既有 acknowledged_at（透過先 GetByID 判斷）。
func (u *FoodSafetyReportUseCase) AdminAcknowledge(ctx context.Context, id, adminID int64) (*domain.FoodSafetyReport, error) {
	cur, err := u.repo.AdminGet(ctx, id)
	if err != nil {
		return nil, err
	}
	if cur.IsAcknowledged() {
		return cur, pkgerrors.NewConflict("report already acknowledged")
	}
	updated, err := u.repo.Acknowledge(ctx, id, adminID)
	if err != nil {
		return nil, err
	}
	logger.Info(ctx, "food safety report acknowledged",
		zap.Int64("report_id", id),
		zap.Int64("admin_id", adminID),
		zap.Duration("ack_latency", updated.AckLatency()))
	return updated, nil
}

// AdminMarkFDAReported 對應 spec：POST /admin/food_safety_reports/{id}/mark_fda_reported (D-Q3).
func (u *FoodSafetyReportUseCase) AdminMarkFDAReported(ctx context.Context, id int64, channel domain.FDAReportChannel, refNo string) (*domain.FoodSafetyReport, error) {
	if !channel.IsValid() {
		return nil, pkgerrors.NewInvalidInput("invalid fda_report_channel")
	}
	if strings.TrimSpace(refNo) == "" {
		return nil, pkgerrors.NewInvalidInput("fda_reference_no is required (use 'n/a' if not assigned)")
	}
	cur, err := u.repo.AdminGet(ctx, id)
	if err != nil {
		return nil, err
	}
	if cur.FDAReportedAt != nil {
		return cur, pkgerrors.NewConflict("FDA already reported")
	}
	return u.repo.MarkFDAReported(ctx, id, channel, refNo)
}

// AdminGenerateFDAExport D19: admin 一鍵 export Markdown 拿去拼 1919 / 線上表單 / 傳真。
//
// 即時 query DB 拼 Markdown，每次重新生成（不 cache）。phone / email 對 anonymize 後的 record
// 顯示 hash 標記。
func (u *FoodSafetyReportUseCase) AdminGenerateFDAExport(ctx context.Context, id int64) (string, error) {
	r, err := u.repo.AdminGet(ctx, id)
	if err != nil {
		return "", err
	}

	var seller *domain.Seller
	if r.TargetSellerID != nil {
		s, sErr := u.sellerRepo.GetByAccountID(ctx, *r.TargetSellerID)
		if sErr == nil {
			seller = s
		} else {
			logger.Warn(ctx, "fda export: seller fetch failed", zap.Error(sErr))
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# FDA 食安通報 — Ticket #%d\n\n", r.ID)
	if anonymized := strings.HasPrefix(r.ReporterPhone, "phone_hash:"); anonymized {
		fmt.Fprintf(&b, "> ⚠️  此通報已超過 90 天 PII 保留期限，原始 phone / email 已 anonymize。\n\n")
	}

	fmt.Fprintf(&b, "## 1. 事件時間\n\n")
	fmt.Fprintf(&b, "- 通報時間：%s\n", r.CreatedAt.Format("2006-01-02 15:04:05 MST"))
	if r.AcknowledgedAt != nil {
		fmt.Fprintf(&b, "- 確認時間：%s（admin id %d）\n", r.AcknowledgedAt.Format("2006-01-02 15:04:05 MST"), int64Ptr(r.AcknowledgedByAdminID))
	}
	if r.ResolvedAt != nil {
		fmt.Fprintf(&b, "- 結案時間：%s\n", r.ResolvedAt.Format("2006-01-02 15:04:05 MST"))
	}

	fmt.Fprintf(&b, "\n## 2. 涉案賣家\n\n")
	switch {
	case seller != nil:
		fmt.Fprintf(&b, "- 店名：%s\n", seller.StoreName)
		fmt.Fprintf(&b, "- account_id：%d\n", seller.AccountID)
		fmt.Fprintf(&b, "- 聯絡電話：%s\n", seller.ContactPhone)
		fmt.Fprintf(&b, "- 地址：%s\n", seller.FullAddress())
		fmt.Fprintf(&b, "- status：%s\n", seller.Status)
	case r.TargetSellerID != nil:
		fmt.Fprintf(&b, "- account_id：%d（賣家資訊查詢失敗）\n", *r.TargetSellerID)
	default:
		fmt.Fprintf(&b, "- 通報者未指定賣家（事件早期不知道哪個商家）\n")
	}

	fmt.Fprintf(&b, "\n## 3. 涉案商品 / 訂單\n\n")
	if r.TargetProductID != nil {
		fmt.Fprintf(&b, "- product_id：%d\n", *r.TargetProductID)
	}
	if r.TargetOrderNo != nil {
		fmt.Fprintf(&b, "- 訂單號：%s\n", *r.TargetOrderNo)
	}
	if r.TargetProductID == nil && r.TargetOrderNo == nil {
		fmt.Fprintf(&b, "- （未指定 — 通報者未提供具體商品 / 訂單）\n")
	}

	fmt.Fprintf(&b, "\n## 4. 通報者聯絡（admin 內部使用）\n\n")
	fmt.Fprintf(&b, "- phone：%s\n", r.ReporterPhone)
	if r.ReporterEmail != nil {
		fmt.Fprintf(&b, "- email：%s\n", *r.ReporterEmail)
	}
	if r.ReporterName != nil {
		fmt.Fprintf(&b, "- 姓名：%s\n", *r.ReporterName)
	}

	fmt.Fprintf(&b, "\n## 5. 描述全文\n\n")
	fmt.Fprintf(&b, "```\n%s\n```\n", r.Description)

	fmt.Fprintf(&b, "\n## 6. 證據資產 ID\n\n")
	if len(r.EvidenceAssetIDs) == 0 {
		fmt.Fprintf(&b, "- （無）\n")
	} else {
		for _, aid := range r.EvidenceAssetIDs {
			fmt.Fprintf(&b, "- `%s`（admin 透過 media-service 解析為 OSS URL 後下載附帶通報）\n", aid)
		}
	}

	fmt.Fprintf(&b, "\n---\n")
	fmt.Fprintf(&b, "_本檔案由 admin UI Generate FDA Report 即時產生（%s）。FDA 通報參考用，請以實際遞送內容為準。_\n",
		time.Now().Format("2006-01-02 15:04:05 MST"))

	return b.String(), nil
}

// AdminNotifySeller 對應 spec：POST /admin/food_safety_reports/{id}/notify_seller (D20).
//
// 模板限制：assist_investigation / temporary_takedown_self_check。
// 發送：(1) 寫 seller_notification_log audit；(2) publish RabbitMQ governance event 給 notification-service。
// dismissed report 拒絕通知；報告無 target_seller_id 也拒絕（不知道通知誰）。
func (u *FoodSafetyReportUseCase) AdminNotifySeller(ctx context.Context, reportID, adminID int64, templateKey string) (*domain.SellerNotificationLog, error) {
	allowedTemplates := map[string]string{
		"assist_investigation":          "seller.governance.notify.assist_investigation",
		"temporary_takedown_self_check": "seller.governance.notify.temporary_takedown_self_check",
	}
	routingKey, ok := allowedTemplates[templateKey]
	if !ok {
		return nil, pkgerrors.NewInvalidInput("invalid template_key (allowed: assist_investigation / temporary_takedown_self_check)")
	}

	r, err := u.repo.AdminGet(ctx, reportID)
	if err != nil {
		return nil, err
	}
	if r.Status == domain.ReportStatusDismissed {
		return nil, pkgerrors.NewConflict("cannot notify seller for dismissed report")
	}
	if r.TargetSellerID == nil {
		return nil, pkgerrors.NewInvalidInput("report has no target_seller_id; cannot notify")
	}

	// storeId was sellerAccountId until 2026-08-05 — same pure naming debt as
	// publishGovernanceEvent in violation.go (TargetSellerID is the store
	// surrogate, food_safety_reports.target_seller_id, not the login account).
	// No rollout-window fallback needed: no notification-service template is
	// registered for either seller.governance.notify.* routing key yet. Note
	// this map is ALSO the audit blob persisted below as
	// seller_notification_log.message_payload — pre-existing rows keep the old
	// key, which nothing reads (no RPC surfaces message_payload today).
	payload := map[string]any{
		"storeId":     *r.TargetSellerID,
		"reportId":    r.ID,
		"templateKey": templateKey,
		"productId":   r.TargetProductID, // pointer ok in JSON (null if NULL)
		"orderNo":     r.TargetOrderNo,
	}
	payloadBytes, _ := json.Marshal(payload)

	sourceTable := "food_safety_reports"
	logRow := &domain.SellerNotificationLog{
		ID:                u.tsid(),
		SourceReportID:    &r.ID,
		SourceReportTable: &sourceTable,
		TemplateKey:       templateKey,
		SellerID:          *r.TargetSellerID,
		SentByAdminID:     &adminID,
		MessagePayload:    payloadBytes,
	}
	created, err := u.logRepo.Create(ctx, logRow)
	if err != nil {
		return nil, err
	}

	// RabbitMQ publish — fire-and-forget；失敗只 log 不阻塞 admin 操作（同 Discord 失敗策略）。
	syncutil.SafeGo(context.Background(), func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := u.publisher.Publish(bgCtx, mq.ExchangeGovernance, routingKey, payload); err != nil {
			logger.Warn(bgCtx, "governance notify publish failed",
				zap.Error(err),
				zap.String("routing_key", routingKey),
				zap.Int64("report_id", r.ID))
		}
	})
	return created, nil
}

// AnonymizeOldPhones 對應 task 9.1 cron worker 入口。
//
// 流程：(1) anonymize phone / email；(2) 同步刪除對應 evidence_asset_ids 的 OSS 實體（B3 attached cleanup）。
func (u *FoodSafetyReportUseCase) AnonymizeOldPhones(ctx context.Context) (int, error) {
	rows, err := u.repo.AnonymizeOldPhones(ctx)
	if err != nil {
		return 0, err
	}
	// 收集所有要刪的 asset_id。
	allAssetIDs := make([]string, 0)
	for _, r := range rows {
		allAssetIDs = append(allAssetIDs, r.EvidenceAssetIDs...)
	}
	if len(allAssetIDs) > 0 && u.media != nil {
		if err := u.media.BatchDeleteAssets(ctx, allAssetIDs); err != nil {
			// 不 abort：DB 已 anonymize；OSS 刪除失敗只 log 留待下次 cron retry。
			logger.Warn(ctx, "anonymize phones: media batch delete failed",
				zap.Error(err),
				zap.Int("asset_count", len(allAssetIDs)))
		}
	}
	logger.Info(ctx, "anonymize old phones",
		zap.Int("rows", len(rows)),
		zap.Int("assets_deleted", len(allAssetIDs)))
	return len(rows), nil
}

// ListNotificationsBySource 給 admin UI detail 頁顯示「此 report 已發過哪些通知」。
func (u *FoodSafetyReportUseCase) ListNotificationsBySource(ctx context.Context, reportID int64) ([]*domain.SellerNotificationLog, error) {
	return u.logRepo.ListBySourceReport(ctx, "food_safety_reports", reportID)
}

// ========== helpers ==========

// phoneSuffix 取後 4 碼用於 log（避免暴露完整 phone）。
func phoneSuffix(phone string) string {
	if len(phone) < 4 {
		return "****"
	}
	return phone[len(phone)-4:]
}

func int64Ptr(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
