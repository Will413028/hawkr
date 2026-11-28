package intake

import (
	"context"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

// OnStoreSuspended 訂閱 seller.StoreSuspended。
func (s *Service) OnStoreSuspended(ctx context.Context, e seller.StoreSuspended) error {
	return s.Accept(ctx, "store_suspended", registry.StoreSuspendedParams{StoreID: e.StoreID, PeriodID: e.PeriodID, OccurredAt: e.OccurredAt})
}

// OnStoreUnsuspended 訂閱 seller.StoreUnsuspended。
func (s *Service) OnStoreUnsuspended(ctx context.Context, e seller.StoreUnsuspended) error {
	return s.Accept(ctx, "store_unsuspended", registry.StoreUnsuspendedParams{StoreID: e.StoreID, PeriodID: e.PeriodID, OccurredAt: e.OccurredAt})
}

// OnMeasureCreated 訂閱 seller.MeasureCreated；停權措施不另發。
func (s *Service) OnMeasureCreated(ctx context.Context, e seller.MeasureCreated) error {
	name, params, ok := measureCreatedIntent(e)
	if !ok {
		return nil
	}
	return s.Accept(ctx, name, params)
}

// OnMeasureRevised 訂閱 seller.MeasureRevised。
func (s *Service) OnMeasureRevised(ctx context.Context, e seller.MeasureRevised) error {
	return s.Accept(ctx, "store_measure_revised", registry.StoreMeasureRevisedParams{StoreID: e.StoreID, MeasureID: e.MeasureID, Revision: e.Revision, OccurredAt: e.OccurredAt})
}

// OnMeasureEnded 訂閱 seller.MeasureEnded；接受解除請求的結束不另發（由裁定通知）。
func (s *Service) OnMeasureEnded(ctx context.Context, e seller.MeasureEnded) error {
	if !measureEndedNotified(e) {
		return nil
	}
	return s.Accept(ctx, "store_measure_ended", registry.StoreMeasureStateParams{StoreID: e.StoreID, MeasureID: e.MeasureID, OccurredAt: e.OccurredAt})
}

// OnMeasureRevoked 訂閱 seller.MeasureRevoked；接受申覆的撤銷不另發（由裁定通知）。
func (s *Service) OnMeasureRevoked(ctx context.Context, e seller.MeasureRevoked) error {
	if !measureRevokedNotified(e) {
		return nil
	}
	return s.Accept(ctx, "store_measure_revoked", registry.StoreMeasureStateParams{StoreID: e.StoreID, MeasureID: e.MeasureID, OccurredAt: e.OccurredAt})
}

// OnMeasureRequestDecided 訂閱 seller.MeasureRequestDecided。
func (s *Service) OnMeasureRequestDecided(ctx context.Context, e seller.MeasureRequestDecided) error {
	name, params, err := requestDecidedIntent(e)
	if err != nil {
		return err
	}
	return s.Accept(ctx, name, params)
}

// OnKYCApproved 訂閱 seller.KYCApproved。
func (s *Service) OnKYCApproved(ctx context.Context, e seller.KYCApproved) error {
	return s.Accept(ctx, "kyc_approved", registry.KYCDecisionParams{StoreID: e.StoreID, ApplicationID: e.ApplicationID, OccurredAt: e.OccurredAt})
}

// OnKYCRejected 訂閱 seller.KYCRejected。
func (s *Service) OnKYCRejected(ctx context.Context, e seller.KYCRejected) error {
	return s.Accept(ctx, "kyc_rejected", registry.KYCDecisionParams{StoreID: e.StoreID, ApplicationID: e.ApplicationID, OccurredAt: e.OccurredAt})
}

// OnCaseInquiryCreated 訂閱 seller.CaseInquiryCreated。
func (s *Service) OnCaseInquiryCreated(ctx context.Context, e seller.CaseInquiryCreated) error {
	return s.Accept(ctx, "store_case_inquiry", registry.StoreCaseInquiryParams{
		StoreID: e.StoreID, InquiryID: e.InquiryID, Kind: string(e.Kind), DueAt: e.DueAt, OccurredAt: e.OccurredAt,
	})
}

// OnStorePendingDeletion 訂閱 seller.StorePendingDeletion（確認信）。
func (s *Service) OnStorePendingDeletion(ctx context.Context, e seller.StorePendingDeletion) error {
	return s.Accept(ctx, "store_pending_deletion", registry.StorePendingDeletionParams{StoreID: e.StoreID, EventID: e.EventID, OccurredAt: e.OccurredAt})
}

// OnBuyerDeletionRequested 訂閱 buyer.BuyerDeletionRequested（確認信）。
func (s *Service) OnBuyerDeletionRequested(ctx context.Context, e buyer.BuyerDeletionRequested) error {
	return s.Accept(ctx, "buyer_deletion_requested", registry.BuyerDeletionRequestedParams{BuyerID: e.BuyerID, EventID: e.EventID, OccurredAt: e.OccurredAt})
}

// OnSellerReportReceived 訂閱 seller.ReportReceived（檢舉人一律是買家）。
func (s *Service) OnSellerReportReceived(ctx context.Context, e seller.ReportReceived) error {
	return s.Accept(ctx, "report_received", registry.ReportReceivedParams{
		ReportSource: registry.ReportSourceSeller, ReportID: e.ReportID, SupplementID: e.SupplementID, BuyerID: e.ReporterBuyerID, OccurredAt: e.OccurredAt,
	})
}

// OnSellerReportResolved 訂閱 seller.ReportResolved：母意圖展開成一位買家檢舉人的 report_resolved。
func (s *Service) OnSellerReportResolved(ctx context.Context, e seller.ReportResolved) error {
	return s.Accept(ctx, "seller_report_resolved", registry.SellerReportResolvedParams{
		ReportID: e.ReportID, BuyerID: e.ReporterBuyerID, Outcome: string(e.Outcome), OccurredAt: e.OccurredAt,
	})
}

// OnModerationReportReceived 訂閱 moderation.ReportReceived；檢舉人可為買家或店家。
func (s *Service) OnModerationReportReceived(ctx context.Context, e moderation.ReportReceived) error {
	name, params, err := moderationReportReceivedIntent(e)
	if err != nil {
		return err
	}
	return s.Accept(ctx, name, params)
}

// OnCaseClosed 訂閱 moderation.CaseClosedEvent：買家與店家檢舉人各一個母意圖，由 worker 分頁展開。
func (s *Service) OnCaseClosed(ctx context.Context, e moderation.CaseClosedEvent) error {
	for _, parent := range registry.ModerationCaseClosedParents {
		if err := s.Accept(ctx, parent, registry.ModerationCaseClosedParams{CaseID: e.CaseID, Outcome: string(e.Outcome), OccurredAt: e.OccurredAt}); err != nil {
			return err
		}
	}
	return nil
}

// OnContentHidden 訂閱 social.ContentHidden（只通知買家作者）。
func (s *Service) OnContentHidden(ctx context.Context, e social.ContentHidden) error {
	buyerID, ok := socialAuthorBuyer(e.Author)
	if !ok {
		return nil
	}
	return s.Accept(ctx, "content_hidden", registry.ContentHiddenParams{
		SanctionID: e.SanctionID, BuyerID: buyerID, Source: registry.ContentSourceSocial, Category: e.Category, AppealPath: e.AppealPath, OccurredAt: e.OccurredAt,
	})
}

// OnContentRestored 訂閱 social.ContentRestored。
func (s *Service) OnContentRestored(ctx context.Context, e social.ContentRestored) error {
	buyerID, ok := socialAuthorBuyer(e.Author)
	if !ok {
		return nil
	}
	return s.Accept(ctx, "content_restored", registry.ContentRestoredParams{SanctionID: e.SanctionID, BuyerID: buyerID, Source: registry.ContentSourceSocial, OccurredAt: e.OccurredAt})
}

// OnBuyerWarned 訂閱 social.BuyerWarned。
func (s *Service) OnBuyerWarned(ctx context.Context, e social.BuyerWarned) error {
	return s.Accept(ctx, "author_warned", registry.AuthorWarnedParams{
		SanctionID: e.SanctionID, BuyerID: e.BuyerID, Category: e.Category, AppealPath: e.AppealPath, OccurredAt: e.OccurredAt,
	})
}

// OnReviewHidden 訂閱 review.ReviewHidden（評價作者一定是買家）。
func (s *Service) OnReviewHidden(ctx context.Context, e review.ReviewHidden) error {
	return s.Accept(ctx, "content_hidden", registry.ContentHiddenParams{
		SanctionID: e.SanctionID, BuyerID: e.BuyerID, Source: registry.ContentSourceReview, Category: e.Category, AppealPath: e.AppealPath, OccurredAt: e.OccurredAt,
	})
}

// OnReviewRestored 訂閱 review.ReviewRestored。
func (s *Service) OnReviewRestored(ctx context.Context, e review.ReviewRestored) error {
	return s.Accept(ctx, "content_restored", registry.ContentRestoredParams{SanctionID: e.SanctionID, BuyerID: e.BuyerID, Source: registry.ContentSourceReview, OccurredAt: e.OccurredAt})
}

// OnMessageHidden 訂閱 im.MessageHidden（只通知買家作者）。
func (s *Service) OnMessageHidden(ctx context.Context, e im.MessageHidden) error {
	buyerID, ok := messageAuthorBuyer(e.AuthorKind, e.AuthorID)
	if !ok {
		return nil
	}
	return s.Accept(ctx, "content_hidden", registry.ContentHiddenParams{
		SanctionID: e.SanctionID, BuyerID: buyerID, Source: registry.ContentSourceMessage, Category: e.Category, AppealPath: e.AppealPath, OccurredAt: e.OccurredAt,
	})
}

// OnMessageRestored 訂閱 im.MessageRestored。
func (s *Service) OnMessageRestored(ctx context.Context, e im.MessageRestored) error {
	buyerID, ok := messageAuthorBuyer(e.AuthorKind, e.AuthorID)
	if !ok {
		return nil
	}
	return s.Accept(ctx, "content_restored", registry.ContentRestoredParams{SanctionID: e.SanctionID, BuyerID: buyerID, Source: registry.ContentSourceMessage, OccurredAt: e.OccurredAt})
}

// OnPayoutAccountChanged 訂閱 payment.PayoutAccountChanged：只有轉帳能力第一次啟用的那一次同步通知。
func (s *Service) OnPayoutAccountChanged(ctx context.Context, e payment.PayoutAccountChanged) error {
	if !e.FirstActivation {
		return nil
	}
	return s.Accept(ctx, "payout_account_activated", registry.PayoutAccountActivatedParams{StoreID: e.StoreID, OccurredAt: e.OccurredAt})
}

// OnPayoutDeadlineApproaching 訂閱 payment.PayoutDeadlineApproaching（每日掃描）；去重靠型別的冪等鍵（店家、期限、檔位）。
func (s *Service) OnPayoutDeadlineApproaching(ctx context.Context, e payment.PayoutDeadlineApproaching) error {
	return s.Accept(ctx, "payout_account_deadline", registry.PayoutAccountDeadlineParams{
		StoreID: e.StoreID, Deadline: e.Deadline, Tier: string(e.Tier), OccurredAt: e.OccurredAt,
	})
}

// OnDelegationOperationCompleted 訂閱 delegation.DelegationOperationCompleted：參數只取 ID 與列舉，不取名稱、標籤與自由文字。
func (s *Service) OnDelegationOperationCompleted(ctx context.Context, e delegation.DelegationOperationCompleted) error {
	return s.Accept(ctx, "delegation_operation_completed", registry.DelegationOperationCompletedParams{
		AuditLogID: e.AuditLogID, TargetStoreID: e.TargetStoreID, TargetSellerAccountID: e.TargetSellerAccountID,
		Capability: string(e.Capability), Action: string(e.Action), OccurredAt: e.OccurredAt,
	})
}

// OnRfqPublished 訂閱 procurement.RfqPublished：每個邀請的店家一個 rfq_invited 意圖（邀請 ID 是冪等鍵）。
func (s *Service) OnRfqPublished(ctx context.Context, e procurement.RfqPublished) error {
	for _, invite := range e.Invites {
		if err := s.Accept(ctx, "rfq_invited", registry.RfqInvitedParams{
			StoreID: invite.StoreID, PublicationID: e.PublicationID, InviteID: invite.InviteID, OccurredAt: e.PublishedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}
