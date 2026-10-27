package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// B-169 usecase tests — covers spec scenarios (D6 / D11 / D13 / D14 / D16 / B1 / D9 / D19 / D20).

// ========== ViolationReportUseCase ==========

func TestViolationReport_CreateLoggedInReport_HappyPath(t *testing.T) {
	repo := &mockSellerViolationReportRepository{}
	uc := NewViolationReportUseCase(repo, newCounterTSID(), &mockMediaClient{})

	got, err := uc.CreateLoggedInReport(context.Background(), &domain.SellerViolationReport{
		TargetSellerID:    42,
		ReporterAccountID: 7,
		Category:          domain.ViolationCategoryMisleadingInfo,
		Severity:          domain.ViolationSeverityMedium,
		Description:       "test",
	})
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if got.ID == 0 {
		t.Errorf("expected TSID generated, got 0")
	}
	if got.Status != domain.ReportStatusPending {
		t.Errorf("expected pending, got %s", got.Status)
	}
}

// issue #1516: staged_token 必須先 FinalizeImages 換成真正 asset_id 才落庫 /
// LinkImages，否則 30 分鐘 TTL 一過就永久遺失佐證圖片。
func TestViolationReport_CreateLoggedInReport_FinalizesEvidence(t *testing.T) {
	const stagedToken = "staged-token-abc"
	const finalAssetID = "asset-123"

	var finalizeCalledWith []string
	var finalizeRefType string
	var linkRefType, linkRefID string
	var linkAssetIDs []string

	repo := &mockSellerViolationReportRepository{
		CreateReportFn: func(_ context.Context, r *domain.SellerViolationReport) (*domain.SellerViolationReport, error) {
			// 斷言落庫前欄位已經是 asset_id，不是 staged_token（issue #1516 核心）。
			if len(r.EvidenceAssetIDs) != 1 || r.EvidenceAssetIDs[0] != finalAssetID {
				t.Errorf("expected EvidenceAssetIDs=[%s] before insert, got %v", finalAssetID, r.EvidenceAssetIDs)
			}
			// 真實 repository 落庫用的是 usecase 預先產生的 in.ID（見
			// seller_violation_report_repository.go 的 CreateSellerViolationReportParams.ID），
			// 因此這裡不再偽造另一個 ID —— LinkImages 必須用同一個 ID 當 ref_id，
			// 否則引用掛在不存在的檢舉單上，佐證圖 7 天後被 media GC 實體刪除。
			return r, nil
		},
	}
	media := &mockMediaClient{
		FinalizeImagesFn: func(_ context.Context, tokens []string, refType string) ([]AssetInfo, error) {
			finalizeCalledWith = tokens
			finalizeRefType = refType
			return []AssetInfo{{AssetId: finalAssetID, StagedToken: stagedToken}}, nil
		},
		LinkImagesFn: func(_ context.Context, refType, refID string, assetIDs []string) error {
			linkRefType, linkRefID, linkAssetIDs = refType, refID, assetIDs
			return nil
		},
	}
	uc := NewViolationReportUseCase(repo, newCounterTSID(), media)

	got, err := uc.CreateLoggedInReport(context.Background(), &domain.SellerViolationReport{
		TargetSellerID:    42,
		ReporterAccountID: 7,
		Category:          domain.ViolationCategoryMisleadingInfo,
		Severity:          domain.ViolationSeverityMedium,
		Description:       "test",
		EvidenceAssetIDs:  []string{stagedToken},
	})
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if len(finalizeCalledWith) != 1 || finalizeCalledWith[0] != stagedToken {
		t.Errorf("expected FinalizeImages called with staged token, got %v", finalizeCalledWith)
	}
	if finalizeRefType != "7118" {
		t.Errorf("expected ref_type=7118 (GOVERNANCE_EVIDENCE), got %s", finalizeRefType)
	}
	if got.EvidenceAssetIDs[0] != finalAssetID {
		t.Errorf("expected returned report to carry asset_id, got %v", got.EvidenceAssetIDs)
	}
	if linkRefType != "7118" || linkRefID != fmt.Sprintf("%d", got.ID) || len(linkAssetIDs) != 1 || linkAssetIDs[0] != finalAssetID {
		t.Errorf("expected LinkImages(7118, report_id=%d, [%s]), got refType=%s refID=%s assetIDs=%v",
			got.ID, finalAssetID, linkRefType, linkRefID, linkAssetIDs)
	}
}

// finalize 失敗（過期/無效 token）必須整筆檢舉失敗，不可悄悄存空佐證讓 reporter 誤以為送出成功。
func TestViolationReport_CreateLoggedInReport_FinalizeEvidenceFails(t *testing.T) {
	createCalled := false
	repo := &mockSellerViolationReportRepository{
		CreateReportFn: func(_ context.Context, r *domain.SellerViolationReport) (*domain.SellerViolationReport, error) {
			createCalled = true
			return r, nil
		},
	}
	media := &mockMediaClient{
		FinalizeImagesFn: func(_ context.Context, _ []string, _ string) ([]AssetInfo, error) {
			return nil, errors.New("staged token expired")
		},
	}
	uc := NewViolationReportUseCase(repo, newCounterTSID(), media)

	_, err := uc.CreateLoggedInReport(context.Background(), &domain.SellerViolationReport{
		TargetSellerID:    42,
		ReporterAccountID: 7,
		Category:          domain.ViolationCategoryMisleadingInfo,
		Severity:          domain.ViolationSeverityMedium,
		Description:       "test",
		EvidenceAssetIDs:  []string{"expired-token"},
	})
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected 400 invalid input on finalize failure, got %v", err)
	}
	if createCalled {
		t.Error("expected CreateReport NOT called when finalize fails")
	}
}

func TestViolationReport_CreateLoggedInReport_DuplicateD13(t *testing.T) {
	existing := &domain.SellerViolationReport{ID: 9999, ReporterAccountID: 7, TargetSellerID: 42}
	repo := &mockSellerViolationReportRepository{
		CreateReportFn: func(_ context.Context, _ *domain.SellerViolationReport) (*domain.SellerViolationReport, error) {
			return nil, errPgUniqueViolation
		},
		GetExistingByReporterAndTargetTodayFn: func(_ context.Context, _, _ int64, _ domain.ViolationCategory) (*domain.SellerViolationReport, error) {
			return existing, nil
		},
	}
	uc := NewViolationReportUseCase(repo, newCounterTSID(), &mockMediaClient{})

	got, err := uc.CreateLoggedInReport(context.Background(), &domain.SellerViolationReport{
		TargetSellerID:    42,
		ReporterAccountID: 7,
		Category:          domain.ViolationCategoryMisleadingInfo,
		Severity:          domain.ViolationSeverityMedium,
		Description:       "test",
	})
	// Expected: 409 conflict + existing report returned.
	if err == nil {
		t.Fatal("expected duplicate conflict error")
	}
	if pkgerrors.GetCode(err) != pkgerrors.CodeConflict {
		t.Errorf("expected 409 conflict, got code %d", pkgerrors.GetCode(err))
	}
	if got == nil || got.ID != 9999 {
		t.Errorf("expected existing report id=9999, got %v", got)
	}
}

func TestViolationReport_AdminPatchStatus_InvalidTransition(t *testing.T) {
	repo := &mockSellerViolationReportRepository{
		AdminGetFn: func(_ context.Context, _ int64) (*domain.SellerViolationReport, error) {
			return &domain.SellerViolationReport{ID: 1, Status: domain.ReportStatusVerified}, nil
		},
	}
	uc := NewViolationReportUseCase(repo, newCounterTSID(), &mockMediaClient{})

	_, err := uc.AdminPatchStatus(context.Background(), 1, domain.ReportStatusPending, nil)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeConflict {
		t.Errorf("expected 409 invalid transition, got %v", err)
	}
}

// ========== FoodSafetyReportUseCase ==========

func newFoodSafetyUC(t *testing.T, repo *mockFoodSafetyReportRepository) *FoodSafetyReportUseCase {
	t.Helper()
	logRepo := &mockSellerNotificationLogRepository{}
	sellerRepo := &mockSellerRepo{} // 既有的 mock，不需 stub for these tests
	pub := &mockGovernancePublisher{}
	media := &mockMediaAssetDeleter{}
	return NewFoodSafetyReportUseCase(repo, logRepo, sellerRepo, pub, media, newCounterTSID())
}

func TestFoodSafety_CreatePublic_HappyPath(t *testing.T) {
	repo := &mockFoodSafetyReportRepository{}
	uc := newFoodSafetyUC(t, repo)

	got, ackEst, err := uc.CreatePublicReport(context.Background(), &domain.FoodSafetyReport{
		ReporterPhone: "0912345678",
		Description:   "食用後腹瀉",
	})
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if got.ID == 0 {
		t.Errorf("expected TSID generated")
	}
	if ackEst.IsZero() {
		t.Errorf("expected ack_estimated_at non-zero")
	}
	if got.ReporterPhone != "0912345678" {
		t.Errorf("phone normalized to %s", got.ReporterPhone)
	}
}

func TestFoodSafety_CreatePublic_TextOnlyAccepted(t *testing.T) {
	// issue #1527：純文字通報（電話 + 描述、無佐證附件）正常立案 — 匿名通報維持純文字。
	repo := &mockFoodSafetyReportRepository{}
	uc := newFoodSafetyUC(t, repo)

	got, _, err := uc.CreatePublicReport(context.Background(), &domain.FoodSafetyReport{
		ReporterPhone: "0912345678",
		Description:   "食用後腹瀉，懷疑沙門氏菌",
	})
	if err != nil {
		t.Fatalf("text-only report should be accepted: %v", err)
	}
	if got.ID == 0 {
		t.Error("expected report created with TSID")
	}
	if len(got.EvidenceAssetIDs) != 0 {
		t.Errorf("evidence_asset_ids must stay empty, got %v", got.EvidenceAssetIDs)
	}
}

func TestFoodSafety_CreatePublic_RejectsEvidence(t *testing.T) {
	// issue #1527：匿名端 deny-by-default 拒收佐證附件（fail-closed，繞 Gateway 直打 gRPC 亦擋）；
	// Validate 階段即拒，不得到達 repo.Create。
	repo := &mockFoodSafetyReportRepository{
		CreateFn: func(_ context.Context, _ *domain.FoodSafetyReport) (*domain.FoodSafetyReport, error) {
			t.Fatal("repo.Create must not be reached when evidence is supplied")
			return nil, nil
		},
	}
	uc := newFoodSafetyUC(t, repo)

	_, _, err := uc.CreatePublicReport(context.Background(), &domain.FoodSafetyReport{
		ReporterPhone:    "0912345678",
		Description:      "食用後腹瀉",
		EvidenceAssetIDs: []string{"asset_x"},
	})
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected 400 invalid input rejecting evidence, got %v", err)
	}
}

func TestFoodSafety_CreatePublic_PhoneNormalization(t *testing.T) {
	repo := &mockFoodSafetyReportRepository{}
	uc := newFoodSafetyUC(t, repo)

	cases := []struct{ input, want string }{
		{"+886912345678", "0912345678"},
		{"0912-345-678", "0912345678"},
		{"886912345678", "0912345678"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, _, err := uc.CreatePublicReport(context.Background(), &domain.FoodSafetyReport{
				ReporterPhone: tc.input,
				Description:   "test",
			})
			if err != nil {
				t.Fatalf("got %v", err)
			}
			if got.ReporterPhone != tc.want {
				t.Errorf("normalized phone got %s, want %s", got.ReporterPhone, tc.want)
			}
		})
	}
}

func TestFoodSafety_CreatePublic_InvalidPhone(t *testing.T) {
	repo := &mockFoodSafetyReportRepository{}
	uc := newFoodSafetyUC(t, repo)

	_, _, err := uc.CreatePublicReport(context.Background(), &domain.FoodSafetyReport{
		ReporterPhone: "12345",
		Description:   "test",
	})
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected 400 invalid input, got %v", err)
	}
}

func TestFoodSafety_CreatePublic_SilentDropB1(t *testing.T) {
	// B1: phone+seller 同 ISO week unique 觸發 → silent drop with existing report id (still 200 OK)
	existingID := int64(8888)
	targetSeller := int64(42)
	repo := &mockFoodSafetyReportRepository{
		CreateFn: func(_ context.Context, _ *domain.FoodSafetyReport) (*domain.FoodSafetyReport, error) {
			return nil, &domain.ErrDuplicate{ConstraintName: "uq_food_safety_reports_phone_seller_week"}
		},
		GetExistingByPhoneAndSellerThisWeekFn: func(_ context.Context, _ string, _ int64) (*domain.FoodSafetyReport, error) {
			return &domain.FoodSafetyReport{ID: existingID, CreatedAt: time.Now()}, nil
		},
	}
	uc := newFoodSafetyUC(t, repo)

	got, _, err := uc.CreatePublicReport(context.Background(), &domain.FoodSafetyReport{
		TargetSellerID: &targetSeller,
		ReporterPhone:  "0912345678",
		Description:    "重複通報",
	})
	if err != nil {
		t.Fatalf("silent drop should NOT error, got %v", err)
	}
	if got.ID != existingID {
		t.Errorf("expected existing id %d, got %d", existingID, got.ID)
	}
}

func TestFoodSafety_AdminAcknowledge_AlreadyAcked(t *testing.T) {
	now := time.Now()
	repo := &mockFoodSafetyReportRepository{
		AdminGetFn: func(_ context.Context, _ int64) (*domain.FoodSafetyReport, error) {
			return &domain.FoodSafetyReport{
				ID:             1,
				CreatedAt:      now.Add(-2 * time.Hour),
				AcknowledgedAt: ptrTimeT(now.Add(-1 * time.Hour)),
			}, nil
		},
	}
	uc := newFoodSafetyUC(t, repo)

	got, err := uc.AdminAcknowledge(context.Background(), 1, 99)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeConflict {
		t.Errorf("expected 409 already acked, got %v", err)
	}
	if got == nil || got.AcknowledgedAt == nil {
		t.Errorf("expected current record returned for idempotent friendliness")
	}
}

func TestFoodSafety_AdminPatchStatus_InvestigatingRequiresAck(t *testing.T) {
	repo := &mockFoodSafetyReportRepository{
		AdminGetFn: func(_ context.Context, _ int64) (*domain.FoodSafetyReport, error) {
			return &domain.FoodSafetyReport{
				ID:             1,
				Status:         domain.ReportStatusPending,
				AcknowledgedAt: nil, // 未 ack
			}, nil
		},
	}
	uc := newFoodSafetyUC(t, repo)

	_, err := uc.AdminPatchStatus(context.Background(), 1, domain.ReportStatusInvestigating, nil)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeConflict {
		t.Errorf("expected 409 must ack before investigating, got %v", err)
	}
}

func TestFoodSafety_AdminMarkFDAReported_AlreadyReported(t *testing.T) {
	now := time.Now()
	repo := &mockFoodSafetyReportRepository{
		AdminGetFn: func(_ context.Context, _ int64) (*domain.FoodSafetyReport, error) {
			return &domain.FoodSafetyReport{ID: 1, FDAReportedAt: &now}, nil
		},
	}
	uc := newFoodSafetyUC(t, repo)

	_, err := uc.AdminMarkFDAReported(context.Background(), 1, domain.FDAReportChannelHotline1919, "FDA-123")
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeConflict {
		t.Errorf("expected 409 already reported, got %v", err)
	}
}

func TestFoodSafety_AdminMarkFDAReported_InvalidChannel(t *testing.T) {
	repo := &mockFoodSafetyReportRepository{}
	uc := newFoodSafetyUC(t, repo)
	_, err := uc.AdminMarkFDAReported(context.Background(), 1, domain.FDAReportChannel("invalid"), "FDA-123")
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected 400 invalid channel, got %v", err)
	}
}

func TestFoodSafety_AdminNotifySeller_InvalidTemplate(t *testing.T) {
	repo := &mockFoodSafetyReportRepository{}
	uc := newFoodSafetyUC(t, repo)
	_, err := uc.AdminNotifySeller(context.Background(), 1, 99, "garbage_template")
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected 400 invalid template, got %v", err)
	}
}

func TestFoodSafety_AdminNotifySeller_DismissedReport(t *testing.T) {
	targetSeller := int64(42)
	repo := &mockFoodSafetyReportRepository{
		AdminGetFn: func(_ context.Context, _ int64) (*domain.FoodSafetyReport, error) {
			return &domain.FoodSafetyReport{ID: 1, Status: domain.ReportStatusDismissed, TargetSellerID: &targetSeller}, nil
		},
	}
	uc := newFoodSafetyUC(t, repo)
	_, err := uc.AdminNotifySeller(context.Background(), 1, 99, "assist_investigation")
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeConflict {
		t.Errorf("expected 409 cannot notify dismissed, got %v", err)
	}
}

// TestFoodSafety_AdminNotifySeller_PayloadUsesStoreIDKey pins both halves of
// AdminNotifySeller's identity contract: the MQ payload key AND the audit blob
// persisted into seller_notification_log.message_payload (same map, marshaled).
func TestFoodSafety_AdminNotifySeller_PayloadUsesStoreIDKey(t *testing.T) {
	const storeID int64 = 7487768729113814280 // 19-digit TSID, deliberately not account_id-shaped
	targetSeller := storeID
	repo := &mockFoodSafetyReportRepository{
		AdminGetFn: func(_ context.Context, _ int64) (*domain.FoodSafetyReport, error) {
			return &domain.FoodSafetyReport{ID: 1, Status: domain.ReportStatusInvestigating, TargetSellerID: &targetSeller}, nil
		},
	}
	var auditBlob []byte
	logRepo := &mockSellerNotificationLogRepository{
		CreateFn: func(_ context.Context, row *domain.SellerNotificationLog) (*domain.SellerNotificationLog, error) {
			auditBlob = row.MessagePayload
			return row, nil
		},
	}
	pub := &mockGovernancePublisher{}
	uc := NewFoodSafetyReportUseCase(repo, logRepo, &mockSellerRepo{}, pub, &mockMediaAssetDeleter{}, newCounterTSID())

	if _, err := uc.AdminNotifySeller(context.Background(), 1, 99, "assist_investigation"); err != nil {
		t.Fatalf("AdminNotifySeller: %v", err)
	}

	calls := awaitGovernancePublish(t, pub, 1)
	if calls[0].RoutingKey != "seller.governance.notify.assist_investigation" {
		t.Errorf("routing key = %s, want seller.governance.notify.assist_investigation", calls[0].RoutingKey)
	}
	payload, ok := calls[0].Payload.(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T, want map[string]any", calls[0].Payload)
	}
	if _, legacy := payload["sellerAccountId"]; legacy {
		t.Errorf(`payload still carries the legacy key "sellerAccountId": %v`, payload)
	}
	if got, ok := payload["storeId"].(int64); !ok || got != storeID {
		t.Errorf(`payload["storeId"] = %#v, want int64 %d`, payload["storeId"], storeID)
	}
	if !strings.Contains(string(auditBlob), `"storeId"`) {
		t.Errorf("audit message_payload missing storeId key: %s", auditBlob)
	}
	if strings.Contains(string(auditBlob), `"sellerAccountId"`) {
		t.Errorf("audit message_payload still carries sellerAccountId: %s", auditBlob)
	}
}

func TestFoodSafety_AdminGenerateFDAExport_HappyPath(t *testing.T) {
	targetSeller := int64(42)
	now := time.Now()
	repo := &mockFoodSafetyReportRepository{
		AdminGetFn: func(_ context.Context, _ int64) (*domain.FoodSafetyReport, error) {
			return &domain.FoodSafetyReport{
				ID:             1,
				TargetSellerID: &targetSeller,
				ReporterPhone:  "0912345678",
				Description:    "腹瀉",
				CreatedAt:      now,
			}, nil
		},
	}
	logRepo := &mockSellerNotificationLogRepository{}
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, _ int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: 42, StoreName: "Test Store", ContactPhone: "02-12345678"}, nil
		},
	}
	pub := &mockGovernancePublisher{}
	media := &mockMediaAssetDeleter{}
	uc := NewFoodSafetyReportUseCase(repo, logRepo, sellerRepo, pub, media, newCounterTSID())

	md, err := uc.AdminGenerateFDAExport(context.Background(), 1)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(md, "Ticket #1") {
		t.Errorf("markdown missing ticket id")
	}
	if !strings.Contains(md, "0912345678") {
		t.Errorf("markdown missing phone for admin internal use")
	}
	if !strings.Contains(md, "Test Store") {
		t.Errorf("markdown missing seller store_name")
	}
}

// ========== SellerViolationUseCase ==========

func newViolationUC(t *testing.T, opts ...func(*sellerViolationUCDeps)) *SellerViolationUseCase {
	t.Helper()
	d := &sellerViolationUCDeps{
		repo: &mockSellerViolationRepository{},
		sellerRepo: &mockSellerRepo{GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: accountID, Status: domain.SellerStatusActive}, nil
		}},
		logRepo:             &mockSellerNotificationLogRepository{},
		violationReportRepo: &mockSellerViolationReportRepository{},
		foodSafetyRepo:      &mockFoodSafetyReportRepository{},
		publisher:           &mockGovernancePublisher{},
		cache:               &mockPublicViolationCache{},
		suspensionRepo:      &mockSellerSuspensionRepository{},
		redis:               &mockSellerSuspensionRedis{},
		mediaClient:         &mockMediaClient{},
	}
	for _, opt := range opts {
		opt(d)
	}
	return NewSellerViolationUseCase(
		d.repo, d.sellerRepo, d.logRepo, d.violationReportRepo, d.foodSafetyRepo,
		d.publisher, inlineTxManager{}, d.cache, newCounterTSID(), d.suspensionRepo, d.redis,
		d.mediaClient,
		nil,
	)
}

type sellerViolationUCDeps struct {
	repo                domain.SellerViolationRepository
	sellerRepo          domain.SellerRepository
	logRepo             domain.SellerNotificationLogRepository
	violationReportRepo domain.SellerViolationReportRepository
	foodSafetyRepo      domain.FoodSafetyReportRepository
	publisher           SellerNotificationPublisher
	cache               PublicViolationCache
	suspensionRepo      domain.SellerSuspensionRepository
	redis               SellerSuspensionRedis
	mediaClient         MediaClient
}

func TestViolation_AdminCreate_DisabledPenalty(t *testing.T) {
	uc := newViolationUC(t)
	_, err := uc.AdminCreateViolation(context.Background(), &domain.SellerViolation{
		SellerID:         42,
		CreatedByAdminID: 99,
		Category:         domain.ViolationCategoryFraud,
		Severity:         domain.ViolationSeverityHigh,
		PenaltyType:      domain.PenaltyTypeFine, // Phase 1 disabled
		Description:      "test",
	})
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected 400 disabled penalty, got %v", err)
	}
}

func TestViolation_AdminCreate_AutoEscalateD6(t *testing.T) {
	// 近 90 天 medium ≥ 3 + 本次 medium → 自動升 high。
	repo := &mockSellerViolationRepository{
		CountMediumLast90dFn: func(_ context.Context, _ int64) (int64, error) { return 3, nil },
		CreateViolationFn: func(_ context.Context, v *domain.SellerViolation) (*domain.SellerViolation, error) {
			return v, nil
		},
	}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) { d.repo = repo })

	result, err := uc.AdminCreateViolation(context.Background(), &domain.SellerViolation{
		SellerID:         42,
		CreatedByAdminID: 99,
		Category:         domain.ViolationCategoryMisleadingInfo,
		Severity:         domain.ViolationSeverityMedium,
		PenaltyType:      domain.PenaltyTypeWarning,
		Description:      "test",
	})
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !result.SeverityAutoEscalated {
		t.Error("expected severity auto-escalated")
	}
	if result.OriginalSeverity != domain.ViolationSeverityMedium {
		t.Errorf("original=%s, want medium", result.OriginalSeverity)
	}
	if result.AppliedSeverity != domain.ViolationSeverityHigh {
		t.Errorf("applied=%s, want high", result.AppliedSeverity)
	}
}

// hawkr-admin issue #103: in.SellerID 是 admin 手動輸入的 store surrogate，
// DB 層 seller_violations.seller_id 無 FK 約束。修前只有 penalty_type=full_suspension
// 才會經 GetAccountIDByStoreID 驗證存在性，warning 之類的其他 penalty_type
// 會靜默建出永久對不到任何店家的孤兒違規記錄。這裡驗證 warning-type + 不存在的
// SellerID 一律先擋下，且 CreateViolation 完全沒被呼叫到。
func TestViolation_AdminCreate_WarningType_UnknownSeller_NotFound(t *testing.T) {
	createViolationCalled := false
	repo := &mockSellerViolationRepository{
		CreateViolationFn: func(_ context.Context, v *domain.SellerViolation) (*domain.SellerViolation, error) {
			createViolationCalled = true
			return v, nil
		},
	}
	sellerRepo := &mockSellerRepo{
		GetAccountIDByStoreIDFn: func(_ context.Context, storeID int64) (int64, error) {
			return 0, pkgerrors.NewNotFound("seller not found")
		},
	}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) {
		d.repo = repo
		d.sellerRepo = sellerRepo
	})

	_, err := uc.AdminCreateViolation(context.Background(), &domain.SellerViolation{
		SellerID:         999999, // 不存在的 store surrogate（admin 手動輸入打錯）
		CreatedByAdminID: 99,
		Category:         domain.ViolationCategoryMisleadingInfo,
		Severity:         domain.ViolationSeverityLow,
		PenaltyType:      domain.PenaltyTypeWarning,
		Description:      "test",
	})
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeNotFound {
		t.Errorf("expected 404 not-found error, got %v", err)
	}
	if createViolationCalled {
		t.Error("expected CreateViolation to never be called — must not create an orphaned violation record")
	}
}

const (
	testViolationSurrogateSellerID = 777
	testViolationOwnerAccountID    = 42
)

func TestViolation_AdminCreate_FullSuspensionUpdatesSellersStatus(t *testing.T) {
	expires := time.Now().Add(30 * 24 * time.Hour)
	repo := &mockSellerViolationRepository{
		CreateViolationFn: func(_ context.Context, v *domain.SellerViolation) (*domain.SellerViolation, error) {
			return v, nil
		},
		// recompute reads this after CreateViolation — the mock doesn't persist,
		// so stub it to reflect the full_suspension just "created" (23 §13.3).
		HasActiveFullSuspensionBySellerFn: func(_ context.Context, _ int64) (bool, error) {
			return true, nil
		},
	}
	statusUpdates := make([]domain.SellerStatus, 0)
	var resolvedAccountIDs []int64
	// currentStatus models read-your-own-write: the post-tx SyncRedis (P1 fix)
	// re-reads sellers.status via GetByAccountID and must observe the value
	// Recompute just wrote, not the pre-call snapshot.
	currentStatus := domain.SellerStatusActive
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: testViolationSurrogateSellerID, Status: currentStatus}, nil
		},
		GetAccountIDByStoreIDFn: func(_ context.Context, storeID int64) (int64, error) {
			if storeID != testViolationSurrogateSellerID {
				t.Errorf("expected surrogate %d, got %d", testViolationSurrogateSellerID, storeID)
			}
			return testViolationOwnerAccountID, nil
		},
		UpdateStatusCurrentByAccountIDFn: func(_ context.Context, accountID int64, status domain.SellerStatus, _ time.Time) error {
			resolvedAccountIDs = append(resolvedAccountIDs, accountID)
			statusUpdates = append(statusUpdates, status)
			currentStatus = status
			return nil
		},
	}
	redis := &mockSellerSuspensionRedis{}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) {
		d.repo = repo
		d.sellerRepo = sellerRepo
		d.redis = redis
	})

	_, err := uc.AdminCreateViolation(context.Background(), &domain.SellerViolation{
		SellerID:         testViolationSurrogateSellerID,
		CreatedByAdminID: 99,
		Category:         domain.ViolationCategoryFoodSafety,
		Severity:         domain.ViolationSeverityCritical,
		PenaltyType:      domain.PenaltyTypeFullSuspension,
		ExpiresAt:        &expires,
		Description:      "test",
	})
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if len(statusUpdates) != 1 || statusUpdates[0] != domain.SellerStatusSuspendedGovernance {
		t.Errorf("expected sellers.status updated to suspended_governance, got %v", statusUpdates)
	}
	if len(resolvedAccountIDs) != 1 || resolvedAccountIDs[0] != testViolationOwnerAccountID {
		t.Errorf("expected UpdateStatusCurrentByAccountID to receive resolved accountID %d (not the surrogate %d), got %v",
			testViolationOwnerAccountID, testViolationSurrogateSellerID, resolvedAccountIDs)
	}
	// P1 regression guard (46 §11.1-4): governance full_suspension must
	// propagate to the Gateway Redis marker post-tx — dropping violation.go's
	// SyncRedis call would silently reintroduce the "Gateway never BLOCKs
	// governance-suspended sellers" gap while this suite stayed green.
	if len(redis.SetCalls) != 1 || redis.SetCalls[0] != testViolationOwnerAccountID {
		t.Errorf("expected Redis SetSuspended(%d) post-tx, got %v", testViolationOwnerAccountID, redis.SetCalls)
	}
	if len(redis.SetValues) != 1 || redis.SetValues[0] != governanceSuspendedRedisValue {
		t.Errorf("expected Redis value %q for governance suspension, got %v", governanceSuspendedRedisValue, redis.SetValues)
	}
	if len(redis.ClearCalls) != 0 {
		t.Errorf("expected no ClearSuspended on governance suspend, got %v", redis.ClearCalls)
	}
}

// S8: AdminRevoke's restore must go through the current-stint-scoped method
// (UpdateStatusCurrentByAccountID), not the removed unscoped UpdateStatus.
func TestViolation_AdminRevoke_FullSuspensionRestoresStatusViaCurrentStint(t *testing.T) {
	repo := &mockSellerViolationRepository{
		AdminRevokeFn: func(_ context.Context, _ int64, _ domain.RevokedReason) (*domain.SellerViolation, error) {
			return &domain.SellerViolation{
				ID:          1,
				SellerID:    testViolationSurrogateSellerID,
				PenaltyType: domain.PenaltyTypeFullSuspension,
				Status:      domain.ViolationStatusRevoked,
			}, nil
		},
	}
	statusUpdates := make([]domain.SellerStatus, 0)
	// Read-your-own-write: post-tx SyncRedis must observe the restored 'active'.
	currentStatus := domain.SellerStatusSuspendedGovernance
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: testViolationSurrogateSellerID, Status: currentStatus}, nil
		},
		GetAccountIDByStoreIDFn: func(_ context.Context, storeID int64) (int64, error) {
			if storeID != testViolationSurrogateSellerID {
				t.Errorf("expected surrogate %d, got %d", testViolationSurrogateSellerID, storeID)
			}
			return testViolationOwnerAccountID, nil
		},
		UpdateStatusCurrentByAccountIDFn: func(_ context.Context, accountID int64, status domain.SellerStatus, _ time.Time) error {
			if accountID != testViolationOwnerAccountID {
				t.Errorf("expected resolved accountID %d (not the surrogate %d), got %d",
					testViolationOwnerAccountID, testViolationSurrogateSellerID, accountID)
			}
			statusUpdates = append(statusUpdates, status)
			currentStatus = status
			return nil
		},
	}
	redis := &mockSellerSuspensionRedis{}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) {
		d.repo = repo
		d.sellerRepo = sellerRepo
		d.redis = redis
	})

	_, err := uc.AdminRevoke(context.Background(), 1, domain.RevokedReasonAdminCorrection, 99)
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if len(statusUpdates) != 1 || statusUpdates[0] != domain.SellerStatusActive {
		t.Errorf("expected sellers.status restored to active via current-stint scope, got %v", statusUpdates)
	}
	// P1 regression guard: revoke restored 'active' → the Gateway Redis marker
	// must be cleared post-tx, or the seller stays BLOCKed at the Gateway.
	if len(redis.ClearCalls) != 1 || redis.ClearCalls[0] != testViolationOwnerAccountID {
		t.Errorf("expected Redis ClearSuspended(%d) post-tx, got %v", testViolationOwnerAccountID, redis.ClearCalls)
	}
	if len(redis.SetCalls) != 0 {
		t.Errorf("expected no SetSuspended on revoke-to-active, got %v", redis.SetCalls)
	}
}

func TestViolation_AdminCreate_PenaltyMutexD14(t *testing.T) {
	uc := newViolationUC(t)
	// warning + expires_at → reject by validate
	expires := time.Now().Add(30 * 24 * time.Hour)
	_, err := uc.AdminCreateViolation(context.Background(), &domain.SellerViolation{
		SellerID:         42,
		CreatedByAdminID: 99,
		Category:         domain.ViolationCategoryOther,
		Severity:         domain.ViolationSeverityLow,
		PenaltyType:      domain.PenaltyTypeWarning,
		ExpiresAt:        &expires, // 違反 D14
		Description:      "test",
	})
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected 400 penalty mutex violation, got %v", err)
	}
}

func TestViolation_SellerSubmitAppeal_OwnershipCheck(t *testing.T) {
	repo := &mockSellerViolationRepository{
		GetByIDFn: func(_ context.Context, _ int64) (*domain.SellerViolation, error) {
			return &domain.SellerViolation{ID: 1, SellerID: 100, Status: domain.ViolationStatusEnforced}, nil
		},
	}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) { d.repo = repo })

	_, err := uc.SellerSubmitAppeal(context.Background(), 1, 999, "我不該被罰", nil)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeForbidden {
		t.Errorf("expected 403 forbidden, got %v", err)
	}
}

func TestViolation_SellerSubmitAppeal_AlreadyAppealed(t *testing.T) {
	repo := &mockSellerViolationRepository{
		GetByIDFn: func(_ context.Context, _ int64) (*domain.SellerViolation, error) {
			return &domain.SellerViolation{
				ID: 1, SellerID: 100,
				Status:   domain.ViolationStatusEnforced,
				Appealed: true,
			}, nil
		},
	}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) { d.repo = repo })

	_, err := uc.SellerSubmitAppeal(context.Background(), 1, 100, "again", nil)
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeConflict {
		t.Errorf("expected 409 already appealed, got %v", err)
	}
}

// issue #1516: appeal_evidence_asset_ids 必須先 FinalizeImages 換成真正 asset_id 才落庫 /
// LinkImages — 舊版原封不動存 staged_token JWT，30 分鐘 TTL 一過 admin 決議前就讀不到佐證。
func TestViolation_SellerSubmitAppeal_FinalizesEvidence(t *testing.T) {
	const stagedToken = "staged-token-xyz"
	const finalAssetID = "asset-999"

	var submitAppealAssetIDs []string
	var linkRefType, linkRefID string
	var linkAssetIDs []string

	repo := &mockSellerViolationRepository{
		GetByIDFn: func(_ context.Context, _ int64) (*domain.SellerViolation, error) {
			return &domain.SellerViolation{ID: 1, SellerID: 100, Status: domain.ViolationStatusEnforced}, nil
		},
		SubmitAppealFn: func(_ context.Context, id int64, reason string, evidenceAssetIDs []string) (*domain.SellerViolation, error) {
			submitAppealAssetIDs = evidenceAssetIDs
			return &domain.SellerViolation{ID: id, SellerID: 100, AppealReason: &reason, AppealEvidenceAssetIDs: evidenceAssetIDs}, nil
		},
	}
	media := &mockMediaClient{
		FinalizeImagesFn: func(_ context.Context, tokens []string, refType string) ([]AssetInfo, error) {
			if refType != "7118" {
				t.Errorf("expected ref_type=7118 (GOVERNANCE_EVIDENCE), got %s", refType)
			}
			return []AssetInfo{{AssetId: finalAssetID, StagedToken: tokens[0]}}, nil
		},
		LinkImagesFn: func(_ context.Context, refType, refID string, assetIDs []string) error {
			linkRefType, linkRefID, linkAssetIDs = refType, refID, assetIDs
			return nil
		},
	}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) { d.repo = repo; d.mediaClient = media })

	got, err := uc.SellerSubmitAppeal(context.Background(), 1, 100, "我不該被罰", []string{stagedToken})
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if len(submitAppealAssetIDs) != 1 || submitAppealAssetIDs[0] != finalAssetID {
		t.Errorf("expected repo.SubmitAppeal to receive finalized asset_id [%s], got %v", finalAssetID, submitAppealAssetIDs)
	}
	if got.AppealEvidenceAssetIDs[0] != finalAssetID {
		t.Errorf("expected returned violation to carry asset_id, got %v", got.AppealEvidenceAssetIDs)
	}
	if linkRefType != "7118" || linkRefID != "1" || len(linkAssetIDs) != 1 || linkAssetIDs[0] != finalAssetID {
		t.Errorf("expected LinkImages(7118, violation_id=1, [%s]), got refType=%s refID=%s assetIDs=%v",
			finalAssetID, linkRefType, linkRefID, linkAssetIDs)
	}
}

// finalize 失敗（過期/無效 token）必須整筆申訴失敗，不可悄悄存空佐證讓賣家誤以為送出成功。
func TestViolation_SellerSubmitAppeal_FinalizeEvidenceFails(t *testing.T) {
	submitAppealCalled := false
	repo := &mockSellerViolationRepository{
		GetByIDFn: func(_ context.Context, _ int64) (*domain.SellerViolation, error) {
			return &domain.SellerViolation{ID: 1, SellerID: 100, Status: domain.ViolationStatusEnforced}, nil
		},
		SubmitAppealFn: func(_ context.Context, id int64, reason string, evidenceAssetIDs []string) (*domain.SellerViolation, error) {
			submitAppealCalled = true
			return &domain.SellerViolation{ID: id}, nil
		},
	}
	media := &mockMediaClient{
		FinalizeImagesFn: func(_ context.Context, _ []string, _ string) ([]AssetInfo, error) {
			return nil, errors.New("staged token expired")
		},
	}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) { d.repo = repo; d.mediaClient = media })

	_, err := uc.SellerSubmitAppeal(context.Background(), 1, 100, "我不該被罰", []string{"expired-token"})
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected 400 invalid input on finalize failure, got %v", err)
	}
	if submitAppealCalled {
		t.Error("expected repo.SubmitAppeal NOT called when finalize fails")
	}
}

func TestViolation_AdminDecideAppeal_RejectMissingReason(t *testing.T) {
	uc := newViolationUC(t)
	_, err := uc.AdminDecideAppeal(context.Background(), 1, 99, false, "")
	if err == nil || pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected 400 reject reason required, got %v", err)
	}
}

func TestViolation_AdminDecideAppeal_AcceptFullSuspensionRestoresStatus(t *testing.T) {
	pending := domain.AppealStatusPending
	expires := time.Now().Add(30 * 24 * time.Hour)
	repo := &mockSellerViolationRepository{
		GetByIDFn: func(_ context.Context, _ int64) (*domain.SellerViolation, error) {
			return &domain.SellerViolation{
				ID:           1,
				SellerID:     testViolationSurrogateSellerID,
				PenaltyType:  domain.PenaltyTypeFullSuspension,
				ExpiresAt:    &expires,
				Status:       domain.ViolationStatusEnforced,
				Appealed:     true,
				AppealStatus: &pending,
			}, nil
		},
		DecideAppealAcceptFn: func(_ context.Context, _, _ int64, _ string) (*domain.SellerViolation, error) {
			revokedAt := time.Now()
			return &domain.SellerViolation{
				ID:          1,
				SellerID:    testViolationSurrogateSellerID,
				PenaltyType: domain.PenaltyTypeFullSuspension,
				Status:      domain.ViolationStatusRevoked,
				RevokedAt:   &revokedAt,
			}, nil
		},
	}
	statusUpdates := make([]domain.SellerStatus, 0)
	// Read-your-own-write: post-tx SyncRedis must observe the restored 'active'.
	currentStatus := domain.SellerStatusSuspendedGovernance
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: testViolationSurrogateSellerID, Status: currentStatus}, nil
		},
		GetAccountIDByStoreIDFn: func(_ context.Context, storeID int64) (int64, error) {
			if storeID != testViolationSurrogateSellerID {
				t.Errorf("expected surrogate %d, got %d", testViolationSurrogateSellerID, storeID)
			}
			return testViolationOwnerAccountID, nil
		},
		UpdateStatusCurrentByAccountIDFn: func(_ context.Context, accountID int64, status domain.SellerStatus, _ time.Time) error {
			if accountID != testViolationOwnerAccountID {
				t.Errorf("expected resolved accountID %d (not the surrogate %d), got %d",
					testViolationOwnerAccountID, testViolationSurrogateSellerID, accountID)
			}
			statusUpdates = append(statusUpdates, status)
			currentStatus = status
			return nil
		},
	}
	cache := &mockPublicViolationCache{}
	redis := &mockSellerSuspensionRedis{}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) {
		d.repo = repo
		d.sellerRepo = sellerRepo
		d.cache = cache
		d.redis = redis
	})

	_, err := uc.AdminDecideAppeal(context.Background(), 1, 99, true, "證據充分")
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if len(statusUpdates) != 1 || statusUpdates[0] != domain.SellerStatusActive {
		t.Errorf("expected sellers.status restored to active, got %v", statusUpdates)
	}
	// cache invalidate
	if len(cache.InvalidateCalls) == 0 {
		t.Errorf("expected cache invalidate after appeal accept")
	}
	// P1 regression guard: appeal accept restored 'active' → Gateway Redis
	// marker must be cleared post-tx.
	if len(redis.ClearCalls) != 1 || redis.ClearCalls[0] != testViolationOwnerAccountID {
		t.Errorf("expected Redis ClearSuspended(%d) post-tx, got %v", testViolationOwnerAccountID, redis.ClearCalls)
	}
	if len(redis.SetCalls) != 0 {
		t.Errorf("expected no SetSuspended on appeal-accept-to-active, got %v", redis.SetCalls)
	}
}

// S8: task 9.2 cron restore must also go through the current-stint-scoped method.
func TestViolation_RestoreSellerStatusActive_UsesCurrentStintScope(t *testing.T) {
	statusUpdates := make([]domain.SellerStatus, 0)
	// Read-your-own-write: post-recompute SyncRedis must observe the restored 'active'.
	currentStatus := domain.SellerStatusSuspendedGovernance
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			return &domain.Seller{AccountID: accountID, StoreID: testViolationSurrogateSellerID, Status: currentStatus}, nil
		},
		GetAccountIDByStoreIDFn: func(_ context.Context, storeID int64) (int64, error) {
			if storeID != testViolationSurrogateSellerID {
				t.Errorf("expected surrogate %d, got %d", testViolationSurrogateSellerID, storeID)
			}
			return testViolationOwnerAccountID, nil
		},
		UpdateStatusCurrentByAccountIDFn: func(_ context.Context, accountID int64, status domain.SellerStatus, _ time.Time) error {
			if accountID != testViolationOwnerAccountID {
				t.Errorf("expected resolved accountID %d (not the surrogate %d), got %d",
					testViolationOwnerAccountID, testViolationSurrogateSellerID, accountID)
			}
			statusUpdates = append(statusUpdates, status)
			currentStatus = status
			return nil
		},
	}
	redis := &mockSellerSuspensionRedis{}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) {
		d.sellerRepo = sellerRepo
		d.redis = redis
	})

	if err := uc.RestoreSellerStatusActive(context.Background(), testViolationSurrogateSellerID); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(statusUpdates) != 1 || statusUpdates[0] != domain.SellerStatusActive {
		t.Errorf("expected sellers.status restored to active, got %v", statusUpdates)
	}
	// P1 regression guard: cron expiry restore must also clear the Gateway
	// Redis marker — pre-fix, this path never touched Redis and a marker
	// (had one existed) would stay stale until the next admin suspend/lift.
	if len(redis.ClearCalls) != 1 || redis.ClearCalls[0] != testViolationOwnerAccountID {
		t.Errorf("expected Redis ClearSuspended(%d) after cron restore, got %v", testViolationOwnerAccountID, redis.ClearCalls)
	}
	if len(redis.SetCalls) != 0 {
		t.Errorf("expected no SetSuspended on cron restore-to-active, got %v", redis.SetCalls)
	}
}

// B5 (11-分支覆蓋審計.md §三-2): direct assertion that the governance cron's
// expiry restore does NOT clobber a concurrently-active admin manual
// suspension. Previously only covered indirectly via the combination of A9
// (recompute precedence table) + B4 (RestoreSellerStatusActive delegates to
// Recompute) — this test drives the actual cron entrypoint with an active
// admin suspension present and asserts the write lands on `suspended`, not
// `active` (the original bug this SSOT fix targeted, per violation.go:448-450).
func TestViolation_RestoreSellerStatusActive_KeepsSuspendedWhenAdminSuspensionActive(t *testing.T) {
	statusUpdates := make([]domain.SellerStatus, 0)
	// Read-your-own-write: post-recompute SyncRedis must observe the corrected
	// 'suspended', not the stale pre-cron 'suspended_governance'.
	currentStatus := domain.SellerStatusSuspendedGovernance
	sellerRepo := &mockSellerRepo{
		GetByAccountIDFn: func(_ context.Context, accountID int64) (*domain.Seller, error) {
			// Stale stored status from before this cron run — still shows
			// suspended_governance even though the governance penalty just expired.
			return &domain.Seller{AccountID: accountID, StoreID: testViolationSurrogateSellerID, Status: currentStatus}, nil
		},
		GetAccountIDByStoreIDFn: func(_ context.Context, storeID int64) (int64, error) {
			return testViolationOwnerAccountID, nil
		},
		UpdateStatusCurrentByAccountIDFn: func(_ context.Context, _ int64, status domain.SellerStatus, _ time.Time) error {
			statusUpdates = append(statusUpdates, status)
			currentStatus = status
			return nil
		},
	}
	// The governance penalty itself has already expired (this call is the
	// cron reacting to that expiry), so HasActiveFullSuspensionBySeller must
	// return false here — the scenario under test is the *other*, still-active
	// admin suspension winning precedence, not a stale governance flag.
	violationRepo := &mockSellerViolationRepository{
		HasActiveFullSuspensionBySellerFn: func(_ context.Context, _ int64) (bool, error) { return false, nil },
	}
	suspensionRepo := &mockSellerSuspensionRepository{
		HasActiveByAccountIDFn: func(_ context.Context, accountID int64) (bool, error) {
			if accountID != testViolationOwnerAccountID {
				t.Errorf("expected resolved accountID %d, got %d", testViolationOwnerAccountID, accountID)
			}
			return true, nil // admin manual suspension still active (46 §5, §3.6)
		},
		// SyncRedis reads the winning admin suspension's reason_code for the
		// Redis value ('suspended' branch, seller_status_recompute.go).
		GetActiveByAccountIDFn: func(_ context.Context, accountID int64) (*domain.SellerSuspension, error) {
			return &domain.SellerSuspension{
				AccountID:  accountID,
				ReasonCode: domain.SellerSuspensionReasonRepeatedWarningIgnored,
				Status:     domain.SellerSuspensionActive,
			}, nil
		},
	}
	redis := &mockSellerSuspensionRedis{}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) {
		d.sellerRepo = sellerRepo
		d.repo = violationRepo
		d.suspensionRepo = suspensionRepo
		d.redis = redis
	})

	if err := uc.RestoreSellerStatusActive(context.Background(), testViolationSurrogateSellerID); err != nil {
		t.Fatalf("got %v", err)
	}
	if len(statusUpdates) != 1 || statusUpdates[0] != domain.SellerStatusSuspended {
		t.Errorf("expected sellers.status to stay suspended (admin suspension wins precedence), got %v", statusUpdates)
	}
	// P1 regression guard: the cron must NOT clear the Redis marker while an
	// admin suspension is still active — SyncRedis re-Sets it with the admin
	// suspension's reason_code (not the governance sentinel, which lost
	// precedence here).
	if len(redis.SetCalls) != 1 || redis.SetCalls[0] != testViolationOwnerAccountID {
		t.Errorf("expected Redis SetSuspended(%d) after cron recompute, got %v", testViolationOwnerAccountID, redis.SetCalls)
	}
	if len(redis.SetValues) != 1 || redis.SetValues[0] != domain.SellerSuspensionReasonRepeatedWarningIgnored {
		t.Errorf("expected Redis value = admin reason_code %q, got %v", domain.SellerSuspensionReasonRepeatedWarningIgnored, redis.SetValues)
	}
	if len(redis.ClearCalls) != 0 {
		t.Errorf("expected no ClearSuspended while admin suspension still active, got %v", redis.ClearCalls)
	}
}

func TestViolation_GetPublic_CacheHitMiss(t *testing.T) {
	cached := []*domain.SellerViolation{{ID: 1, SellerID: 42, Severity: domain.ViolationSeverityHigh}}

	t.Run("cache hit", func(t *testing.T) {
		cache := &mockPublicViolationCache{
			GetFn: func(_ context.Context, sellerID int64) ([]*domain.SellerViolation, bool) {
				if sellerID == 42 {
					return cached, true
				}
				return nil, false
			},
		}
		// repo 不該被打到 — 沒設 ListPublicLast90dHighFn。
		uc := newViolationUC(t, func(d *sellerViolationUCDeps) { d.cache = cache })
		got, err := uc.GetPublicViolations(context.Background(), 42)
		if err != nil {
			t.Fatalf("got %v", err)
		}
		if len(got) != 1 || got[0].ID != 1 {
			t.Errorf("expected cache hit returning cached, got %v", got)
		}
	})

	t.Run("cache miss → fallback to repo + Set cache", func(t *testing.T) {
		cache := &mockPublicViolationCache{
			GetFn: func(_ context.Context, _ int64) ([]*domain.SellerViolation, bool) {
				return nil, false
			},
		}
		repo := &mockSellerViolationRepository{
			ListPublicLast90dHighFn: func(_ context.Context, _ int64) ([]*domain.SellerViolation, error) {
				return []*domain.SellerViolation{{ID: 2}}, nil
			},
		}
		uc := newViolationUC(t, func(d *sellerViolationUCDeps) {
			d.repo = repo
			d.cache = cache
		})
		got, err := uc.GetPublicViolations(context.Background(), 42)
		if err != nil {
			t.Fatalf("got %v", err)
		}
		if len(got) != 1 || got[0].ID != 2 {
			t.Errorf("expected miss fallback to repo")
		}
		if len(cache.SetCalls) != 1 || cache.SetCalls[0] != 42 {
			t.Errorf("expected cache.Set called for seller 42, got %v", cache.SetCalls)
		}
	})
}

func TestViolation_AdminNotifySeller_PublishesGovernanceEvent(t *testing.T) {
	expires := time.Now().Add(30 * 24 * time.Hour)
	pub := &mockGovernancePublisher{}
	repo := &mockSellerViolationRepository{
		CreateViolationFn: func(_ context.Context, v *domain.SellerViolation) (*domain.SellerViolation, error) { return v, nil },
	}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) {
		d.repo = repo
		d.publisher = pub
	})

	_, err := uc.AdminCreateViolation(context.Background(), &domain.SellerViolation{
		SellerID:         42,
		CreatedByAdminID: 99,
		Category:         domain.ViolationCategoryFraud,
		Severity:         domain.ViolationSeverityHigh,
		PenaltyType:      domain.PenaltyTypeFullSuspension,
		ExpiresAt:        &expires,
		Description:      "test",
	})
	if err != nil {
		t.Fatalf("got %v", err)
	}
	// fire-and-forget publish 是 goroutine — 等一下確認。
	calls := awaitGovernancePublish(t, pub, 1)
	if calls[0].RoutingKey != "seller.governance.violation_created" {
		t.Errorf("routing key = %s, want seller.governance.violation_created", calls[0].RoutingKey)
	}
}

// TestViolation_GovernanceEventPayload_UsesStoreIDKey pins the wire contract of
// the seller.governance.* payload: the surrogate goes out under "storeId", and
// the pre-2026-08-05 misnomer "sellerAccountId" must never come back. This is
// the assertion whose absence let commit f45b082f2 (2026-08-02) rename the
// outbox lifecycle payload while leaving this direct-publish sibling on the old
// key for three more days.
func TestViolation_GovernanceEventPayload_UsesStoreIDKey(t *testing.T) {
	const storeID int64 = 7487768729113814280 // 19-digit TSID — deliberately NOT an account_id-shaped 10-digit number
	expires := time.Now().Add(30 * 24 * time.Hour)
	pub := &mockGovernancePublisher{}
	repo := &mockSellerViolationRepository{
		CreateViolationFn: func(_ context.Context, v *domain.SellerViolation) (*domain.SellerViolation, error) { return v, nil },
	}
	uc := newViolationUC(t, func(d *sellerViolationUCDeps) {
		d.repo = repo
		d.publisher = pub
	})

	_, err := uc.AdminCreateViolation(context.Background(), &domain.SellerViolation{
		SellerID:         storeID,
		CreatedByAdminID: 99,
		Category:         domain.ViolationCategoryFraud,
		Severity:         domain.ViolationSeverityHigh,
		PenaltyType:      domain.PenaltyTypeFullSuspension,
		ExpiresAt:        &expires,
		Description:      "test",
	})
	if err != nil {
		t.Fatalf("AdminCreateViolation: %v", err)
	}

	calls := awaitGovernancePublish(t, pub, 1)
	payload, ok := calls[0].Payload.(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T, want map[string]any", calls[0].Payload)
	}
	if _, legacy := payload["sellerAccountId"]; legacy {
		t.Errorf(`payload still carries the legacy key "sellerAccountId": %v`, payload)
	}
	got, ok := payload["storeId"].(int64)
	if !ok {
		t.Fatalf(`payload["storeId"] = %#v (type %T), want int64`, payload["storeId"], payload["storeId"])
	}
	if got != storeID {
		t.Errorf(`payload["storeId"] = %d, want %d (the store surrogate, not the owning account_id)`, got, storeID)
	}
}

// ========== helpers ==========

func ptrTimeT(t time.Time) *time.Time { return &t }

// silence unused import linter — errors used elsewhere
var _ = errors.New
