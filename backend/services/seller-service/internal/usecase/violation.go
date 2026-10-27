package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// PublicViolationCache D8 公開查詢 cache 抽象。Phase 6 由 infrastructure/redis 實作；
// usecase 層 fail-open 處理（cache miss 或 Redis 不可用都打 DB）。
type PublicViolationCache interface {
	Get(ctx context.Context, sellerID int64) ([]*domain.SellerViolation, bool)
	Set(ctx context.Context, sellerID int64, violations []*domain.SellerViolation)
	Invalidate(ctx context.Context, sellerID int64)
}

// SellerViolationUseCase B-169 admin 違規處罰紀錄管理 + 賣家自助 + 公開查詢.
type SellerViolationUseCase struct {
	repo                domain.SellerViolationRepository
	sellerRepo          domain.SellerRepository
	logRepo             domain.SellerNotificationLogRepository
	violationReportRepo domain.SellerViolationReportRepository
	foodSafetyRepo      domain.FoodSafetyReportRepository
	publisher           SellerNotificationPublisher
	txMgr               database.TxManager
	cache               PublicViolationCache
	tsid                func() int64
	statusRecompute     *sellerStatusRecomputer
	mediaClient         MediaClient
}

// NewSellerViolationUseCase constructs the usecase with all dependencies.
// redis is forwarded into statusRecompute — governance full_suspension
// create/revoke/appeal and the cron restore path previously never touched
// Redis at all (P1 gap: only seller_suspension.go's admin Suspend/Lift did),
// so a governance-suspended seller was never BLOCKed by the Gateway
// capability layer. Now every path that calls statusRecompute.Recompute also
// calls statusRecompute.SyncRedis post-commit — see seller_status_recompute.go.
func NewSellerViolationUseCase(
	repo domain.SellerViolationRepository,
	sellerRepo domain.SellerRepository,
	logRepo domain.SellerNotificationLogRepository,
	violationReportRepo domain.SellerViolationReportRepository,
	foodSafetyRepo domain.FoodSafetyReportRepository,
	publisher SellerNotificationPublisher,
	txMgr database.TxManager,
	cache PublicViolationCache,
	tsid func() int64,
	suspensionRepo domain.SellerSuspensionRepository,
	redis SellerSuspensionRedis,
	mediaClient MediaClient,
	revalidateClient *revalidate.Client,
) *SellerViolationUseCase {
	return &SellerViolationUseCase{
		repo:                repo,
		sellerRepo:          sellerRepo,
		logRepo:             logRepo,
		violationReportRepo: violationReportRepo,
		foodSafetyRepo:      foodSafetyRepo,
		publisher:           publisher,
		txMgr:               txMgr,
		cache:               cache,
		tsid:                tsid,
		statusRecompute:     newSellerStatusRecomputer(sellerRepo, suspensionRepo, repo, redis, revalidateClient),
		mediaClient:         mediaClient,
	}
}

// AdminCreateViolationResult AdminCreateViolation 回傳，含自動升級資訊讓 admin 知情。
type AdminCreateViolationResult struct {
	Violation             *domain.SellerViolation
	SeverityAutoEscalated bool
	OriginalSeverity      domain.ViolationSeverity
	AppliedSeverity       domain.ViolationSeverity
}

// AdminCreateViolation 對應 spec：POST /admin/violations.
//
// 流程（D6 / D14 / D16 / 跨 service）：
//  1. application 層 validate（penalty_type allow-list 限 warning + full_suspension / penalty 欄位互斥 / source FK 互斥）
//  2. txMgr.WithTx 開 transaction：
//     2a. AcquireCreateLock(seller_id) — pg_advisory_xact_lock 串行同 seller 並發建立
//     2b. CountMediumLast90d(seller_id)；若 ≥3 且本次 medium → 自動升 high
//     2c. CreateViolation
//     2d. full_suspension 時：sellers.status='suspended_governance'（透過 UpdateStatusCurrentByAccountID）
//  3. tx commit 後：notification publish（RabbitMQ）+ cache invalidate（fire-and-forget）+ log audit
func (u *SellerViolationUseCase) AdminCreateViolation(ctx context.Context, in *domain.SellerViolation) (*AdminCreateViolationResult, error) {
	// Phase 1 admin 限 warning + full_suspension（其他 enum 仍 allow at DB 層留作 Phase 2）
	if !in.PenaltyType.IsPhase1Enabled() {
		return nil, pkgerrors.NewInvalidInput("penalty_type '" + string(in.PenaltyType) + "' is not enabled in current phase")
	}
	now := time.Now()
	if in.CreatedAt.IsZero() {
		in.CreatedAt = now
	}
	if in.UpdatedAt.IsZero() {
		in.UpdatedAt = now
	}
	if in.EffectiveAt.IsZero() {
		in.EffectiveAt = now
	}
	if in.Status == "" {
		in.Status = domain.ViolationStatusEnforced
	}
	if in.ID == 0 {
		in.ID = u.tsid()
	}
	originalSeverity := in.Severity
	if err := in.Validate(); err != nil {
		return nil, pkgerrors.NewInvalidInput(err.Error())
	}

	var (
		created   *domain.SellerViolation
		escalated bool
		// 0 = no full_suspension recompute happened this call, skip the
		// post-commit Redis sync below (P1 fix: this is what makes governance
		// full_suspension propagate to seller:suspended:<id>, see
		// seller_status_recompute.go SyncRedis).
		redisSyncAccountID int64
	)
	err := u.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		// D6: advisory lock 串行同 seller 並發建立
		if err := u.repo.AcquireCreateLock(txCtx, in.SellerID); err != nil {
			return err
		}
		// 存在性檢查：in.SellerID 是 admin 手動輸入的 store surrogate，DB 層無 FK 約束，任何
		// penalty_type（含 warning）都必須先確認對應到真實現任賣家，否則會建出永久對不到任何
		// 店家的孤兒違規記錄（hawkr-admin issue #103）。解出的 ownerAccountID 供下方
		// full_suspension recompute 分支重用，避免重複查詢。
		ownerAccountID, err := u.sellerRepo.GetAccountIDByStoreID(txCtx, in.SellerID)
		if err != nil {
			return err
		}
		// inline auto-escalation：近 90 天 medium ≥3 且本次 medium → 升 high
		if in.Severity == domain.ViolationSeverityMedium {
			count, err := u.repo.CountMediumLast90d(txCtx, in.SellerID)
			if err != nil {
				return err
			}
			if count >= int64(domain.AutoEscalateThresholdMediumCount) {
				in.Severity = domain.ViolationSeverityHigh
				escalated = true
				logger.Info(txCtx, "violation severity auto-escalated",
					zap.Int64("seller_id", in.SellerID),
					zap.Int64("medium_count_last_90d", count),
					zap.String("from", string(originalSeverity)),
					zap.String("to", string(in.Severity)))
			}
		}
		v, err := u.repo.CreateViolation(txCtx, in)
		if err != nil {
			return err
		}
		// full_suspension：連動 sellers.status（32 §13.3 SSOT：改呼叫 recompute，不再寫死值——
		// 若該帳號同時有 active admin 停權，recompute 會維持 'suspended' 不誤降級）。
		// 連動現任 stint（is_current scope — 不加 scope 的話 re-onboard 後會蓋到舊 tombstone）。
		if v.PenaltyType == domain.PenaltyTypeFullSuspension {
			if err := u.statusRecompute.Recompute(txCtx, ownerAccountID); err != nil {
				return err
			}
			redisSyncAccountID = ownerAccountID
		}
		created = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	if redisSyncAccountID != 0 {
		u.statusRecompute.SyncRedis(ctx, redisSyncAccountID)
	}

	// post-commit side effects（fire-and-forget — 失敗不 rollback DB）
	u.invalidatePublicCache(ctx, created.SellerID)
	u.publishGovernanceEvent(ctx, "seller.governance.violation_created", created, in.CreatedByAdminID)
	u.writeNotificationLog(ctx, created, "violation_created", &in.CreatedByAdminID)

	return &AdminCreateViolationResult{
		Violation:             created,
		SeverityAutoEscalated: escalated,
		OriginalSeverity:      originalSeverity,
		AppliedSeverity:       created.Severity,
	}, nil
}

// AdminUpdateMutable 對應 spec：PATCH /admin/violations/{id}.
//
// 只允許改 description / public_summary / expires_at；其他欄位 immutable。
func (u *SellerViolationUseCase) AdminUpdateMutable(ctx context.Context, id int64, description string, publicSummary *string, expiresAt *time.Time) (*domain.SellerViolation, error) {
	updated, err := u.repo.AdminUpdateMutable(ctx, id, description, publicSummary, expiresAt)
	if err != nil {
		return nil, err
	}
	u.invalidatePublicCache(ctx, updated.SellerID)
	return updated, nil
}

// AdminRevoke 對應 spec：admin 主動撤銷 violation（不走 appeal 路徑）.
//
// full_suspension violation 撤銷時還原 sellers.status='active'；發 notification + log audit。
func (u *SellerViolationUseCase) AdminRevoke(ctx context.Context, id int64, reason domain.RevokedReason, adminID int64) (*domain.SellerViolation, error) {
	var revoked *domain.SellerViolation
	var redisSyncAccountID int64 // 0 = no full_suspension recompute happened, skip Redis sync
	err := u.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		v, err := u.repo.AdminRevoke(txCtx, id, reason)
		if err != nil {
			return err
		}
		if v.PenaltyType == domain.PenaltyTypeFullSuspension {
			ownerAccountID, err := u.sellerRepo.GetAccountIDByStoreID(txCtx, v.SellerID)
			if err != nil {
				return err
			}
			if err := u.statusRecompute.Recompute(txCtx, ownerAccountID); err != nil {
				return err
			}
			redisSyncAccountID = ownerAccountID
		}
		revoked = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	if redisSyncAccountID != 0 {
		u.statusRecompute.SyncRedis(ctx, redisSyncAccountID)
	}
	u.invalidatePublicCache(ctx, revoked.SellerID)
	u.publishGovernanceEvent(ctx, "seller.governance.violation_revoked", revoked, adminID)
	u.writeNotificationLog(ctx, revoked, "violation_revoked", &adminID)
	return revoked, nil
}

// AdminUnionListReports 對應 spec：GET /admin/violation_reports（合併兩張 source 表）.
//
// 回傳「假 union row」(*UnionReport) — admin UI 依 source 組正確 PATCH URL.
type UnionReport struct {
	Source       string
	Severity     domain.ViolationSeverity // food_safety_reports 一律 'critical'
	CreatedAt    time.Time
	SellerReport *domain.SellerViolationReport `json:",omitempty"`
	FoodSafety   *domain.FoodSafetyReport      `json:",omitempty"`
}

// AdminUnionListReports 合併查兩張 source 表後 in-memory sort by (severity DESC, created_at ASC).
//
// Phase 1 簡化：兩張表各取 limit 筆再 in-memory merge；admin 量小可接受。Phase 2 量大再考慮 UNION SQL.
func (u *SellerViolationUseCase) AdminUnionListReports(ctx context.Context, source string, severity *domain.ViolationSeverity, status *domain.ReportStatus, since *time.Time, limit, offset int32) ([]*UnionReport, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	want := func(src string) bool {
		if source == "" {
			return true
		}
		return source == src
	}

	merged := make([]*UnionReport, 0, limit*2)

	if want("seller_violation_reports") {
		filter := domain.SellerViolationReportListFilter{
			Status:   status,
			Severity: severity,
			Since:    since,
			Limit:    limit,
			Offset:   offset,
		}
		rows, err := u.violationReportRepo.AdminList(ctx, filter)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			merged = append(merged, &UnionReport{
				Source:       "seller_violation_reports",
				Severity:     r.Severity,
				CreatedAt:    r.CreatedAt,
				SellerReport: r,
			})
		}
	}

	if want("food_safety_reports") {
		// food_safety severity 一律 critical；若 severity filter 指定非 critical 則跳過
		if severity == nil || *severity == domain.ViolationSeverityCritical {
			filter := domain.FoodSafetyReportListFilter{
				Status: status,
				Since:  since,
				Limit:  limit,
				Offset: offset,
			}
			rows, err := u.foodSafetyRepo.AdminList(ctx, filter)
			if err != nil {
				return nil, err
			}
			for _, r := range rows {
				merged = append(merged, &UnionReport{
					Source:     "food_safety_reports",
					Severity:   domain.ViolationSeverityCritical,
					CreatedAt:  r.CreatedAt,
					FoodSafety: r,
				})
			}
		}
	}

	// in-memory sort: severity DESC（critical 最前），same severity 取 created_at ASC（最舊優先處理）
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].Severity.Rank() != merged[j].Severity.Rank() {
			return merged[i].Severity.Rank() < merged[j].Severity.Rank()
		}
		return merged[i].CreatedAt.Before(merged[j].CreatedAt)
	})

	if int(limit) < len(merged) {
		merged = merged[:limit]
	}
	return merged, nil
}

// AdminListSellerViolations 對應 spec：GET /admin/violations.
func (u *SellerViolationUseCase) AdminListSellerViolations(ctx context.Context, filter domain.SellerViolationListFilter) ([]*domain.SellerViolation, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	return u.repo.AdminList(ctx, filter)
}

// AdminGetViolation 對應 spec：admin detail page.
func (u *SellerViolationUseCase) AdminGetViolation(ctx context.Context, id int64) (*domain.SellerViolation, error) {
	return u.repo.GetByID(ctx, id)
}

// SellerListMyViolations 對應 spec：GET /app/sellers/me/violations.
func (u *SellerViolationUseCase) SellerListMyViolations(ctx context.Context, sellerID int64, limit, offset int32) ([]*domain.SellerViolation, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return u.repo.ListBySellerSelf(ctx, sellerID, limit, offset)
}

// SellerSubmitAppeal 對應 spec：POST /app/sellers/me/violations/{id}/appeal.
//
// 限申訴一次 + 必須 status='enforced'（revoked 不可申訴）。
//
// 佐證圖片先 FinalizeImages（staged_token → asset_id）才落庫（issue #1516）：舊版把前端傳來
// 的 staged_token JWT 原封不動存進 appeal_evidence_asset_ids，30 分鐘 TTL 一過 admin 決議前
// 就完全讀不到佐證；32 號設計書該欄位註解本就是「media-service asset_id 列表」，比照
// certification.go / seller.go 既有 FinalizeImages→LinkImages 先例修正。CanSubmitAppeal 已
// 擋重複申訴（限一次），故不需處理 Unlink 舊圖 — LinkImages 對同一 violation 只會發生一次。
func (u *SellerViolationUseCase) SellerSubmitAppeal(ctx context.Context, violationID, sellerID int64, reason string, evidenceAssetIDs []string) (*domain.SellerViolation, error) {
	cur, err := u.repo.GetByID(ctx, violationID)
	if err != nil {
		return nil, err
	}
	// ownership check
	if cur.SellerID != sellerID {
		return nil, pkgerrors.NewForbidden("not your violation")
	}
	if err := cur.CanSubmitAppeal(); err != nil {
		// 已申訴 / 已 revoked — domain 層已 attribute 錯誤訊息（"appeal already submitted" 或 "cannot appeal a revoked violation"）
		return nil, pkgerrors.NewConflict(err.Error())
	}

	assetIDs, err := u.finalizeGovernanceEvidence(ctx, evidenceAssetIDs)
	if err != nil {
		return nil, err
	}

	// Link 在落庫之前且失敗即中止（#2222）：assetId 一旦寫進 appeal_evidence_asset_ids
	// 就是「業務列指向」，此時若沒有 asset_reference，7 天後 media GC 會把申訴佐證
	// 連同 R2 物件實體刪除。失敗時不落庫，剛轉正的資產由 GC 依孤兒規則回收。
	if len(assetIDs) > 0 {
		if linkErr := u.mediaClient.LinkImages(ctx, media.RefTypeGovernanceEvidence, fmt.Sprintf("%d", violationID), assetIDs); linkErr != nil {
			return nil, fmt.Errorf("link appeal evidence images (violation_id=%d): %w", violationID, linkErr)
		}
	}

	updated, err := u.repo.SubmitAppeal(ctx, violationID, reason, assetIDs)
	if err != nil {
		return nil, err
	}
	logger.Info(ctx, "appeal submitted",
		zap.Int64("violation_id", violationID),
		zap.Int64("seller_id", sellerID))
	return updated, nil
}

// finalizeGovernanceEvidence converts staged tokens to asset_id（比照 certification.go /
// seller.go 既有 FinalizeImages 先例）。空輸入直接回 nil（no-op），避免無佐證的申訴多打一次
// gRPC。與 UpdateLogo/UpdateCover 一致採嚴格失敗語意：finalize 失敗直接回錯（400），不會
// 讓已過期/無效的佐證悄悄變成空值落庫（那會讓賣家以為佐證已送出，實際遺失）。
func (u *SellerViolationUseCase) finalizeGovernanceEvidence(ctx context.Context, stagedTokens []string) ([]string, error) {
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

// SellerGetAppeal 對應 spec：GET /app/sellers/me/violations/{id}/appeal.
func (u *SellerViolationUseCase) SellerGetAppeal(ctx context.Context, violationID, sellerID int64) (*domain.SellerViolation, error) {
	v, err := u.repo.GetByID(ctx, violationID)
	if err != nil {
		return nil, err
	}
	if v.SellerID != sellerID {
		return nil, pkgerrors.NewForbidden("not your violation")
	}
	if !v.Appealed {
		return nil, pkgerrors.NewNotFound("no appeal submitted")
	}
	return v, nil
}

// AdminDecideAppeal 對應 spec：POST /admin/violations/{id}/appeal/decide.
//
// accept：撤銷 violation + revoked_reason='appeal_accepted'；full_suspension 還原 sellers.status='active'.
// reject：appeal_decision_reason 必填；violation 維持原效。
func (u *SellerViolationUseCase) AdminDecideAppeal(ctx context.Context, violationID, adminID int64, accept bool, reason string) (*domain.SellerViolation, error) {
	if !accept && reason == "" {
		return nil, pkgerrors.NewInvalidInput("decision_reason is required for reject")
	}
	cur, err := u.repo.GetByID(ctx, violationID)
	if err != nil {
		return nil, err
	}
	if err := cur.CanDecideAppeal(); err != nil {
		return nil, pkgerrors.NewConflict(err.Error())
	}

	var decided *domain.SellerViolation
	var redisSyncAccountID int64 // 0 = no full_suspension recompute happened, skip Redis sync
	err = u.txMgr.WithTx(ctx, func(txCtx context.Context) error {
		var v *domain.SellerViolation
		var dErr error
		if accept {
			v, dErr = u.repo.DecideAppealAccept(txCtx, violationID, adminID, reason)
		} else {
			v, dErr = u.repo.DecideAppealReject(txCtx, violationID, adminID, reason)
		}
		if dErr != nil {
			return dErr
		}
		if accept && v.PenaltyType == domain.PenaltyTypeFullSuspension {
			ownerAccountID, err := u.sellerRepo.GetAccountIDByStoreID(txCtx, v.SellerID)
			if err != nil {
				return err
			}
			if err := u.statusRecompute.Recompute(txCtx, ownerAccountID); err != nil {
				return err
			}
			redisSyncAccountID = ownerAccountID
		}
		decided = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	if redisSyncAccountID != 0 {
		u.statusRecompute.SyncRedis(ctx, redisSyncAccountID)
	}
	u.invalidatePublicCache(ctx, decided.SellerID)
	u.publishGovernanceEvent(ctx, "seller.governance.appeal_decided", decided, adminID)
	u.writeNotificationLog(ctx, decided, "appeal_decided", &adminID)
	return decided, nil
}

// GetPublicViolations 對應 spec：GET /sellers/{sellerId}/violations/public.
//
// D8: cache hit → 直接回；miss / fail-open → query DB + cache.set。
func (u *SellerViolationUseCase) GetPublicViolations(ctx context.Context, sellerID int64) ([]*domain.SellerViolation, error) {
	if u.cache != nil {
		if hit, ok := u.cache.Get(ctx, sellerID); ok {
			return hit, nil
		}
	}
	violations, err := u.repo.ListPublicLast90dHigh(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	if u.cache != nil {
		u.cache.Set(ctx, sellerID, violations)
	}
	return violations, nil
}

// ListExpiredFullSuspensions task 9.2 cron 用。
func (u *SellerViolationUseCase) ListExpiredFullSuspensions(ctx context.Context) ([]*domain.SellerViolation, error) {
	return u.repo.ListExpiredFullSuspensions(ctx)
}

func (u *SellerViolationUseCase) RestoreSellerStatusActive(ctx context.Context, sellerID int64) error {
	ownerAccountID, err := u.sellerRepo.GetAccountIDByStoreID(ctx, sellerID)
	if err != nil {
		return err
	}
	if err := u.statusRecompute.Recompute(ctx, ownerAccountID); err != nil {
		return err
	}
	u.statusRecompute.SyncRedis(ctx, ownerAccountID)
	return nil
}

// ========== Helpers ==========

// invalidatePublicCache 安全 wrapper（cache 為 nil 時 no-op）。
func (u *SellerViolationUseCase) invalidatePublicCache(ctx context.Context, sellerID int64) {
	if u.cache == nil {
		return
	}
	u.cache.Invalidate(ctx, sellerID)
}

// publishGovernanceEvent 安全 wrapper（publisher 失敗只 log，不 abort）。
//
// storeId was sellerAccountId until 2026-08-05 — pure naming debt (v.SellerID
// has always been the store surrogate: seller_violations.seller_id references
// sellers.store_id, and RestoreSellerStatusActive feeds the same value into
// GetAccountIDByStoreID to obtain the owning account). Renamed to match
// domain.SellerLifecyclePayload.StoreID, which got the identical treatment on
// 2026-08-02 (commit f45b082f2) — that pass covered the outbox-driven lifecycle
// events and missed this direct-publish sibling. Safe to rename outright with no
// rollout-window fallback: no notification-service template is registered for
// any seller.governance.* routing key yet (nothing consumes this), unlike
// seller.deleted which needed the store_id→sellerAccountId→accountId chain in
// SellerDeletedConsumer.java.
func (u *SellerViolationUseCase) publishGovernanceEvent(ctx context.Context, routingKey string, v *domain.SellerViolation, adminID int64) {
	if u.publisher == nil {
		return
	}
	payload := map[string]any{
		"storeId":       v.SellerID,
		"violationId":   v.ID,
		"category":      string(v.Category),
		"severity":      string(v.Severity),
		"penaltyType":   string(v.PenaltyType),
		"adminId":       adminID,
		"appealStatus":  nullableEnumString(v.AppealStatus),
		"revokedReason": nullableEnumString(v.RevokedReason),
	}
	syncutil.SafeGo(context.Background(), func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := u.publisher.Publish(bgCtx, mq.ExchangeGovernance, routingKey, payload); err != nil {
			logger.Warn(bgCtx, "governance event publish failed",
				zap.Error(err),
				zap.String("routing_key", routingKey),
				zap.Int64("violation_id", v.ID))
		}
	})
}

// writeNotificationLog 寫 audit log（失敗只 log；audit 為 best-effort）。
func (u *SellerViolationUseCase) writeNotificationLog(ctx context.Context, v *domain.SellerViolation, templateKey string, sentBy *int64) {
	if u.logRepo == nil {
		return
	}
	payload := map[string]any{
		"violationId": v.ID,
		"sellerId":    v.SellerID,
		"category":    string(v.Category),
		"severity":    string(v.Severity),
		"penaltyType": string(v.PenaltyType),
	}
	body, _ := json.Marshal(payload)
	logRow := &domain.SellerNotificationLog{
		ID:             u.tsid(),
		TemplateKey:    templateKey,
		SellerID:       v.SellerID,
		SentByAdminID:  sentBy,
		MessagePayload: body,
	}
	if _, err := u.logRepo.Create(ctx, logRow); err != nil {
		logger.Warn(ctx, "audit log write failed",
			zap.Error(err),
			zap.Int64("violation_id", v.ID),
			zap.String("template_key", templateKey))
	}
}

// nullableEnumString 支援 *AppealStatus / *RevokedReason 等 typed pointer enum.
func nullableEnumString[T ~string](p *T) string {
	if p == nil {
		return ""
	}
	return string(*p)
}
