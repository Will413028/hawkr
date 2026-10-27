package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"go.uber.org/zap"
)

type PayuniUseCase struct {
	payuniRepo              domain.PayuniRepository
	payuniClient            PayuniClient
	sellerRepo              domain.SellerRepository
	certRepo                domain.CertificationRepository
	accountClient           AccountClient
	txManager               database.TxManager
	outboxRepo              domain.MerchantVerificationOutboxRepository
	payuniProfileUpdateRepo domain.PayuniMerchantProfileUpdateRepository
	tsid                    func() int64
}

func NewPayuniUseCase(
	payuniRepo domain.PayuniRepository,
	sellerRepo domain.SellerRepository,
	certRepo domain.CertificationRepository,
	ac AccountClient,
	pc PayuniClient,
	txManager database.TxManager,
	outboxRepo domain.MerchantVerificationOutboxRepository,
	payuniProfileUpdateRepo domain.PayuniMerchantProfileUpdateRepository,
	tsid func() int64,
) *PayuniUseCase {
	return &PayuniUseCase{
		payuniRepo:              payuniRepo,
		sellerRepo:              sellerRepo,
		certRepo:                certRepo,
		accountClient:           ac,
		payuniClient:            pc,
		txManager:               txManager,
		outboxRepo:              outboxRepo,
		payuniProfileUpdateRepo: payuniProfileUpdateRepo,
		tsid:                    tsid,
	}
}

func (u *PayuniUseCase) SyncPendingStatuses(ctx context.Context) error {
	now := time.Now().UTC()
	staleBefore := now.Add(-payuniPollMinInterval)

	infos, err := u.payuniRepo.GetPending(ctx, payuniSyncBatchSize, staleBefore)
	if err != nil {
		return fmt.Errorf("get pending payuni registrations: %w", err)
	}

	payuniPendingRegistrations.Set(float64(len(infos)))

	if len(infos) == 0 {
		return nil
	}

	logger.Info(ctx, "syncing PAYUNi pending statuses", zap.Int("count", len(infos)))

	for _, info := range infos {
		if info.PayuniMerID == "" {
			continue
		}

		claimedAt := time.Now().UTC()
		claimed, err := u.payuniRepo.ClaimForSync(ctx, info.ID, claimedAt, staleBefore)
		if err != nil {
			logger.Warn(ctx, "failed to claim payuni registration for sync",
				zap.Int64("account_id", info.AccountID),
				zap.Error(err))
			continue
		}
		if !claimed {
			continue
		}

		resp, err := u.payuniClient.GetSellerStatus(ctx, info.PayuniMerID)
		if err != nil {
			payuniSyncAPIErrorTotal.Inc()
			logger.Warn(ctx, "failed to get PAYUNi seller status",
				zap.String("payuni_mer_id", info.PayuniMerID),
				zap.Error(err))
			continue
		}

		u.warnIfStalePending(ctx, info, now)

		obs := observationFromAPI(resp, sourceSync, time.Now().UTC())
		info.RawResponse = resp.RawResponse

		if !u.ApplyObservation(ctx, info, obs) {
			continue
		}

		if err := u.payuniRepo.Update(ctx, info); err != nil {
			logger.Warn(ctx, "failed to update payuni registration",
				zap.Int64("account_id", info.AccountID),
				zap.Error(err))
		}
	}

	return nil
}

const (
	sourceSync    = "sync"    // background poll (SyncPendingStatuses)
	sourceWebhook = "webhook" // AGENT_NOTIFY
	sourceRefresh = "refresh" // admin / seller-triggered single refresh
	sourceApprove = "approve" // platform approval funnel — re-apply, not an observation
)

func observationFromAPI(resp *SellerStatusResponse, source string, observedAt time.Time) domain.PayuniStatusObservation {
	return domain.PayuniStatusObservation{
		ObservedAt:      observedAt,
		Source:          source,
		MerStatus:       resp.MerStatus,
		MemberStatus:    resp.MemberStatus,
		CreditStatus:    resp.CreditStatus,
		ApplePayStatus:  resp.ApplePayStatus,
		GooglePayStatus: resp.GooglePayStatus,
		ATMStatus:       resp.ATMStatus,
		Reason:          resp.Reason,
		Raw:             resp.RawResponse,
	}
}

func observationFromAgentNotify(merStatus string, payload map[string]string, observedAt time.Time) domain.PayuniStatusObservation {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = nil
	}
	return domain.PayuniStatusObservation{
		ObservedAt:      observedAt,
		Source:          sourceWebhook,
		MerStatus:       merStatus,
		MemberStatus:    payload["MemberStatus"],
		CreditStatus:    payload["CreditStatus"],
		ApplePayStatus:  payload["ApplePayStatus"],
		GooglePayStatus: payload["GooglePayStatus"],
		ATMStatus:       payload["ATMStatus"],
		Reason:          payload["Reason"],
		Raw:             raw,
	}
}

func (u *PayuniUseCase) recordStatusHistory(ctx context.Context, info *domain.PayuniRegistration, obs domain.PayuniStatusObservation) {
	if info == nil || info.PayuniMerID == "" || u.tsid == nil {
		return
	}
	if err := u.payuniRepo.InsertStatusHistory(ctx, info.NewStatusHistory(u.tsid(), obs)); err != nil {
		logger.Warn(ctx, "failed to record PAYUNi status history",
			zap.String("source", obs.Source),
			zap.Int64("account_id", info.AccountID),
			zap.String("payuni_mer_id", info.PayuniMerID),
			zap.Error(err))
	}
}

const (
	payuniPollMinInterval          = 2 * time.Hour
	payuniManualRefreshMinInterval = 30 * time.Minute
	payuniSyncBatchSize            = 100
)

const payuniFreshnessStaleThreshold = 24 * time.Hour

const payuniPendingStaleThreshold = 30 * 24 * time.Hour

func (u *PayuniUseCase) warnIfStalePending(ctx context.Context, info *domain.PayuniRegistration, now time.Time) {
	if info.RegisteredAt == nil {
		return
	}
	age := now.Sub(*info.RegisteredAt)
	if age <= payuniPendingStaleThreshold {
		return
	}
	logger.Warn(ctx, "PAYUNi registration stuck in pending past SLA threshold",
		zap.Int64("account_id", info.AccountID),
		zap.String("payuni_mer_id", info.PayuniMerID),
		zap.String("mer_status", info.MerStatus),
		zap.String("member_status", info.MemberStatus),
		zap.Duration("pending_for", age))
}

type payuniOutcome string

const (
	outcomeApproved    payuniOutcome = "approved"
	outcomePendingInfo payuniOutcome = "pending_info"
	outcomeRejected    payuniOutcome = "rejected"
	outcomeUnderReview payuniOutcome = "under_review"
)

var payuniAmbiguousOutcomes = map[[2]string]payuniOutcome{
	{"1", "1"}: outcomeApproved, {"1", "5"}: outcomePendingInfo,
	{"1", "9"}: outcomeUnderReview, {"1", ""}: outcomeUnderReview,

	{"9", "1"}: outcomeUnderReview, {"9", "5"}: outcomePendingInfo,
	{"9", "9"}: outcomeUnderReview, {"9", ""}: outcomeUnderReview,
}

func derivePayuniOutcome(merStatus, memberStatus string) payuniOutcome {
	switch merStatus {
	case "2", "8", "92", "99": // terminal — MemberStatus is irrelevant
		return outcomeRejected
	case "91": // merchant-level pending — MemberStatus is irrelevant
		return outcomePendingInfo
	}
	if merStatus == "1" || merStatus == "9" {
		switch memberStatus {
		case "8", "92", "99":
			return outcomeRejected
		}
	}
	if outcome, ok := payuniAmbiguousOutcomes[[2]string{merStatus, memberStatus}]; ok {
		return outcome
	}
	if merStatus == "1" || merStatus == "9" {
		return outcomeUnderReview
	}
	if memberStatus == "5" {
		return outcomePendingInfo
	}
	return payuniOutcome("") // fully unrecognized MerStatus and MemberStatus != "5" — fail closed
}

func payuniStatusCode(merStatus, memberStatus string) string {
	switch merStatus {
	case "2", "8", "91", "92", "99":
		return "MER_" + merStatus
	}
	switch memberStatus {
	case "5", "8", "92", "99":
		return "MEMBER_" + memberStatus
	}
	if merStatus != "" {
		return "MER_" + merStatus
	}
	return ""
}

func payuniPendingInfoReason(reason string) string {
	if strings.TrimSpace(reason) != "" {
		return reason
	}
	return codemeta.PayuniSellerFacingErrorLabel("MEMBER_5")
}

func payuniRejectNotificationReason(info *domain.PayuniRegistration) string {
	return ResolvePayuniError(payuniStatusCode(info.MerStatus, info.MemberStatus), info.Reason).Message
}

func (u *PayuniUseCase) notifyVerification(ctx context.Context, info *domain.PayuniRegistration, eventType, reason string, now time.Time) bool {
	if info.ApplicationID == 0 {
		return true
	}
	outboxRow, err := domain.NewMerchantVerificationOutboxRow(u.tsid(), info.AccountID, info.StoreID, eventType, reason, now)
	if err != nil {
		logger.Warn(ctx, "notifyVerification: failed to build outbox row",
			zap.Int64("account_id", info.AccountID), zap.Error(err))
		return false
	}
	if err := u.txManager.WithTx(ctx, func(ctx context.Context) error {
		return u.outboxRepo.Insert(ctx, outboxRow)
	}); err != nil {
		logger.Warn(ctx, "notifyVerification: failed to record outbox",
			zap.Int64("account_id", info.AccountID),
			zap.String("payuni_mer_id", info.PayuniMerID),
			zap.Error(err))
		return false
	}
	return true
}

func (u *PayuniUseCase) ApplyObservation(ctx context.Context, info *domain.PayuniRegistration, obs domain.PayuniStatusObservation) bool {
	changed := info.ChangedBy(obs)
	kycChanged := info.KYCChangedBy(obs)
	// 舊 outcome 必須在 Apply 之前算，它是「從哪個狀態轉過來」的唯一來源。
	// 先前是讀 cert 的 pending_info 狀態代答，那在 cert 不再鏡射 PayUni 之後不成立。
	prevOutcome := derivePayuniOutcome(info.MerStatus, info.MemberStatus)
	info.Apply(obs)

	recordPayuniObservation(obs.Source, changed)

	if changed {
		u.recordStatusHistory(ctx, info, obs)
	}
	return u.applyOutcome(ctx, info, kycChanged, prevOutcome, obs.ObservedAt, obs.Source)
}

// ReapplyOutcome 由 Approve 的 relink／AGENT04017 自癒路徑呼叫：PayUni 的狀態是
// 先前就回報過的，這裡不是觀測到新變化，而是把它重新套用一次。
//
// kycChanged 傳 true：通知在新設計下由這個旗標驅動，而 relink 正是賣家該收到
// 「認證通過」的時刻。傳 false 會讓這條路徑的通知整個消失——舊設計的通知掛在 FSM
// 轉換成功上，不看這個旗標，所以搬家時這裡是會漏的那一格。
func (u *PayuniUseCase) ReapplyOutcome(ctx context.Context, info *domain.PayuniRegistration, now time.Time, source string) bool {
	return u.applyOutcome(ctx, info, true, payuniOutcome(""), now, source)
}

func (u *PayuniUseCase) applyOutcome(ctx context.Context, info *domain.PayuniRegistration, kycChanged bool, prevOutcome payuniOutcome, now time.Time, source string) bool {
	accountID := info.AccountID
	outcome := derivePayuniOutcome(info.MerStatus, info.MemberStatus)

	switch outcome {
	case outcomeApproved:
		if kycChanged && !u.notifyVerification(ctx, info, string(outcomeApproved), info.Reason, now) {
			return false
		}
		info.SetActive(now)
		if kycChanged {
			logger.Info(ctx, "PAYUNi approved",
				zap.Int64("account_id", accountID),
				zap.String("payuni_mer_id", info.PayuniMerID),
				zap.String("source", source))
		}

	case outcomePendingInfo:
		if kycChanged && !u.notifyVerification(ctx, info, string(outcomePendingInfo), payuniPendingInfoReason(info.Reason), now) {
			return false
		}

	case outcomeRejected:
		if kycChanged && !u.notifyVerification(ctx, info, string(outcomeRejected), payuniRejectNotificationReason(info), now) {
			return false
		}
		info.SetFailed(payuniStatusCode(info.MerStatus, info.MemberStatus), info.Reason, now)
		if kycChanged {
			switch info.MerStatus {
			case "92", "8":
				logger.Info(ctx, "PAYUNi rejected",
					zap.Int64("account_id", accountID),
					zap.String("payuni_mer_id", info.PayuniMerID),
					zap.String("mer_status", info.MerStatus),
					zap.String("source", source))
			case "2":
				logger.Info(ctx, "PAYUNi closed",
					zap.Int64("account_id", accountID),
					zap.String("payuni_mer_id", info.PayuniMerID),
					zap.String("source", source))
			case "99":
				logger.Info(ctx, "PAYUNi other closed",
					zap.Int64("account_id", accountID),
					zap.String("payuni_mer_id", info.PayuniMerID),
					zap.String("source", source))
			case "1", "9":
				logger.Info(ctx, "PAYUNi member-level rejected",
					zap.Int64("account_id", accountID),
					zap.String("payuni_mer_id", info.PayuniMerID),
					zap.String("mer_status", info.MerStatus),
					zap.String("member_status", info.MemberStatus),
					zap.String("source", source))
			}
		}

	case outcomeUnderReview:
		if info.MerStatus == "1" {
			logger.Warn(ctx, "PAYUNi approval withheld: merchant approved but member status unconfirmed",
				zap.Int64("account_id", accountID),
				zap.String("payuni_mer_id", info.PayuniMerID),
				zap.String("member_status", info.MemberStatus),
				zap.String("source", source))
		}
		if kycChanged && prevOutcome == outcomePendingInfo {
			u.applyUnderReviewOutbox(ctx, info, now, source)
		}

	default:
		logger.Warn(ctx, "PAYUNi unrecognized MerStatus, skipping",
			zap.String("mer_status", info.MerStatus),
			zap.String("member_status", info.MemberStatus),
			zap.Int64("account_id", accountID),
			zap.String("payuni_mer_id", info.PayuniMerID),
			zap.String("source", source))
		return false
	}
	return true
}

func (u *PayuniUseCase) applyUnderReviewOutbox(ctx context.Context, info *domain.PayuniRegistration, now time.Time, source string) {
	if info.ApplicationID == 0 {
		return
	}
	outboxRow, err := domain.NewMerchantVerificationOutboxRow(u.tsid(), info.AccountID, info.StoreID, string(outcomeUnderReview), "", now)
	if err != nil {
		logger.Warn(ctx, "applyUnderReviewOutbox: failed to build outbox row",
			zap.Int64("account_id", info.AccountID), zap.Error(err))
		return
	}

	if err := u.txManager.WithTx(ctx, func(ctx context.Context) error {
		return u.outboxRepo.Insert(ctx, outboxRow)
	}); err != nil {
		logger.Warn(ctx, "applyUnderReviewOutbox: failed to record outbox",
			zap.Int64("account_id", info.AccountID),
			zap.String("payuni_mer_id", info.PayuniMerID),
			zap.Error(err))
		return
	}

	logger.Info(ctx, "PAYUNi under review: pending_info application flagged for resubmission notice",
		zap.Int64("account_id", info.AccountID),
		zap.String("payuni_mer_id", info.PayuniMerID),
		zap.String("source", source))
}

func (u *PayuniUseCase) HandleAgentNotify(ctx context.Context, merId, merStatus string, payload map[string]string) error {
	if !strings.HasPrefix(merId, "HAWK") {
		return fmt.Errorf("invalid merId format: %s", merId)
	}
	if _, err := strconv.ParseInt(strings.TrimPrefix(merId, "HAWK"), 10, 64); err != nil {
		return fmt.Errorf("parse store_code from merId %s: %w", merId, err)
	}

	info, err := u.payuniRepo.GetByMerID(ctx, merId)
	if err != nil {
		return fmt.Errorf("get payuni registration for mer_id %s: %w", merId, err)
	}

	obs := observationFromAgentNotify(merStatus, payload, time.Now().UTC())
	if !u.ApplyObservation(ctx, info, obs) {
		return nil
	}

	if err := u.payuniRepo.Update(ctx, info); err != nil {
		logger.Warn(ctx, "failed to update payuni registration via webhook",
			zap.Int64("store_id", info.StoreID),
			zap.String("payuni_mer_id", merId),
			zap.Error(err))
	}
	return nil
}

func (u *PayuniUseCase) RefreshStatus(ctx context.Context, storeID int64) error {
	info, err := u.payuniRepo.GetByStoreID(ctx, storeID)
	if err != nil {
		return fmt.Errorf("get payuni registration: %w", err)
	}
	if info.PayuniMerID == "" {
		return fmt.Errorf("store_id %d has no PAYUNi MerID", storeID)
	}

	now := time.Now().UTC()
	claimed, err := u.payuniRepo.ClaimForSync(ctx, info.ID, now, now.Add(-payuniManualRefreshMinInterval))
	if err != nil {
		return fmt.Errorf("claim payuni registration for sync: %w", err)
	}
	if !claimed {
		return nil
	}

	resp, err := u.payuniClient.GetSellerStatus(ctx, info.PayuniMerID)
	if err != nil {
		return fmt.Errorf("get seller status from PAYUNi: %w", err)
	}

	info.RawResponse = resp.RawResponse

	if !u.ApplyObservation(ctx, info, observationFromAPI(resp, sourceRefresh, now)) {
		return nil
	}

	return u.payuniRepo.Update(ctx, info)
}
