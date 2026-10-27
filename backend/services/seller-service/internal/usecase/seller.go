package usecase

import (
	"context"
	stderrors "errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/phone"
	"github.com/Will413028/hawkr/go-pkg/revalidate"
	"github.com/Will413028/hawkr/go-pkg/syncutil"
	"github.com/Will413028/hawkr/go-pkg/twaddr"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
	"go.uber.org/zap"
)

type SellerProfileEventPublisher interface {
	PublishSellerProfileChanged(ctx context.Context, storeID int64) error
}

type ProspectAttributionRepo interface {
	FindUsableInviteByToken(ctx context.Context, token string) (domain.ProspectInvite, error)
	MarkConverted(ctx context.Context, prospectID, storeID int64, operatorAccountID *int64) (int64, error)
	ConsumeInvite(ctx context.Context, inviteID int64) error
}

type SellerUseCase struct {
	sellerRepo              domain.SellerRepository
	payuniRepo              domain.PayuniRepository
	certRepo                domain.CertificationRepository
	accountClient           AccountClient
	mediaClient             MediaClient
	mediaBaseURL            string
	revalidate              *revalidate.Client
	profileEvents           SellerProfileEventPublisher
	tsid                    func() int64
	txManager               database.TxManager
	outboxRepo              domain.SellerDeletedOutboxRepository
	payuniProfileOutboxRepo domain.PayuniMerchantProfileOutboxRepository
	storeCode               func() (int64, error)
	sandboxMerIDOverride    string
	prospectRepo            ProspectAttributionRepo
}

func NewSellerUseCase(sellerRepo domain.SellerRepository, payuniRepo domain.PayuniRepository, certRepo domain.CertificationRepository, accountClient AccountClient, mediaClient MediaClient, cfg *config.Config, revalidateClient *revalidate.Client, profileEvents SellerProfileEventPublisher, tsid func() int64, txManager database.TxManager, outboxRepo domain.SellerDeletedOutboxRepository, payuniProfileOutboxRepo domain.PayuniMerchantProfileOutboxRepository, storeCode func() (int64, error), prospectRepo domain.SellerProspectRepository) *SellerUseCase {
	return &SellerUseCase{
		sellerRepo:    sellerRepo,
		payuniRepo:    payuniRepo,
		certRepo:      certRepo,
		accountClient: accountClient,
		mediaClient:   mediaClient,
		mediaBaseURL:  cfg.MediaBaseURL,
		revalidate:    revalidateClient,
		profileEvents: profileEvents,
		tsid:          tsid,
		txManager:     txManager,
		outboxRepo:    outboxRepo,

		payuniProfileOutboxRepo: payuniProfileOutboxRepo,

		storeCode:    storeCode,
		prospectRepo: prospectRepo,

		sandboxMerIDOverride: cfg.Payuni.SandboxMerIDOverride,
	}
}

func (u *SellerUseCase) emitProfileChanged(storeID int64) {
	if u.profileEvents == nil {
		return
	}
	syncutil.SafeGo(context.Background(), func() {
		if err := u.profileEvents.PublishSellerProfileChanged(context.Background(), storeID); err != nil {
			logger.Warn(context.Background(), "failed to publish seller.profile.changed",
				zap.Int64("store_id", storeID), zap.Error(err))
		}
	})
}
func (u *SellerUseCase) sellerCacheTags(ctx context.Context, accountID int64) []string {
	return sellerCacheTagsByAccountID(ctx, u.sellerRepo, accountID)
}

func sellerCacheTagsByAccountID(ctx context.Context, repo domain.SellerRepository, accountID int64) []string {
	seller, err := repo.GetByAccountID(ctx, accountID)
	if err != nil || seller == nil {
		logger.Warn(ctx, "無法解析賣家公開識別碼，cache bust 退回僅 account_id tag",
			zap.Int64("account_id", accountID), zap.Error(err))
		return revalidate.SellerTags(accountID)
	}
	return sellerCacheTagsFrom(ctx, accountID, seller.StoreID, seller.StoreCode)
}

func sellerCacheTagsFrom(ctx context.Context, accountID, storeID, storeCode int64) []string {
	if storeCode == 0 {
		logger.Warn(ctx, "賣家無 store_code，canonical 店鋪頁 cache 無法精準失效",
			zap.Int64("account_id", accountID), zap.Int64("store_id", storeID))
	}
	return revalidate.SellerTags(accountID, storeID, storeCode)
}

func (u *SellerUseCase) bustSellerCache(ctx context.Context, accountID int64) {
	if !u.revalidate.Enabled() {
		return
	}
	u.revalidate.BustAsync(ctx, u.sellerCacheTags(ctx, accountID))
}

func (u *SellerUseCase) bustSellerCacheFor(ctx context.Context, m *domain.Seller) {
	if !u.revalidate.Enabled() {
		return
	}
	u.revalidate.BustAsync(ctx, sellerCacheTagsFrom(ctx, m.AccountID, m.StoreID, m.StoreCode))
}

func (u *SellerUseCase) resolveTarget(ctx context.Context, t dto.SellerTarget) (*domain.Seller, error) {
	switch {
	case t.StoreID() > 0:
		return u.sellerRepo.GetCurrentByStoreID(ctx, t.StoreID())
	case t.AccountID() > 0:
		return u.sellerRepo.GetByAccountID(ctx, t.AccountID())
	default:
		return nil, errors.NewInvalidInput("seller target is empty")
	}
}

func (u *SellerUseCase) GetByAccountID(ctx context.Context, accountID int64) (*dto.SellerProfile, error) {
	logger.Info(ctx, "查詢賣家資料", zap.Int64("account_id", accountID))
	p, err := u.sellerRepo.GetProfileByAccountID(ctx, accountID)
	if err != nil {
		logger.Warn(ctx, "查詢賣家資料失敗", zap.Int64("account_id", accountID), zap.Error(err))
		return nil, err
	}
	m := p.Seller
	if m.ContactPhone == "" {
		if acctInfo, err := u.accountClient.GetAccountInfo(ctx, accountID); err == nil && acctInfo.Phone != "" {
			m.ContactPhone = phone.NormalizeToLocal(acctInfo.Phone)
			m.UpdatedAt = time.Now().UTC()
			if updateErr := u.sellerRepo.Update(ctx, m, nil); updateErr != nil {
				logger.Warn(ctx, "failed to backfill contact_phone", zap.Int64("account_id", accountID), zap.Error(updateErr))
			}
		}
	}
	logger.Info(ctx, "查詢賣家資料成功", zap.Int64("account_id", accountID), zap.String("store_name", m.StoreName))

	if p.PayuniRegistrationStatus == domain.PayuniStatusActive.String() && p.PayuniApprovedAt != nil {
		m.CertifiedAt = p.PayuniApprovedAt
	}
	if p.CertSellerType != "" {
		if st, err := domain.SellerTypeString(p.CertSellerType); err == nil {
			m.SellerType = st
		}
	}
	return u.toResponse(m, deriveOverallStatus(p.CertStatus, derivePayuniOutcome(p.PayuniMerStatus, p.PayuniMemberStatus)), deriveApprovalStatus(p.CertStatus), payuniMerStatusToInt(p.PayuniMerStatus), p.PayuniRegistrationStatus), nil
}

func (u *SellerUseCase) GetPublicByStoreIDOrCode(ctx context.Context, storeIDOrCode int64) (*dto.SellerProfile, error) {
	p, err := u.sellerRepo.GetPublicProfileByStoreIDOrCode(ctx, storeIDOrCode)
	if err != nil {
		return nil, err
	}
	m := p.Seller
	if p.PayuniRegistrationStatus == domain.PayuniStatusActive.String() && p.PayuniApprovedAt != nil {
		m.CertifiedAt = p.PayuniApprovedAt
	}
	if p.CertSellerType != "" {
		if st, err := domain.SellerTypeString(p.CertSellerType); err == nil {
			m.SellerType = st
		}
	}
	return u.toResponse(m, deriveOverallStatus(p.CertStatus, derivePayuniOutcome(p.PayuniMerStatus, p.PayuniMemberStatus)), deriveApprovalStatus(p.CertStatus), payuniMerStatusToInt(p.PayuniMerStatus), p.PayuniRegistrationStatus), nil
}

func (u *SellerUseCase) GetByStoreID(ctx context.Context, storeID int64) (*dto.SellerProfile, error) {
	p, err := u.sellerRepo.GetProfileByStoreID(ctx, storeID)
	if err != nil {
		logger.Warn(ctx, "查詢賣家資料失敗（surrogate）", zap.Int64("store_id", storeID), zap.Error(err))
		return nil, err
	}
	m := p.Seller

	if p.PayuniRegistrationStatus == domain.PayuniStatusActive.String() && p.PayuniApprovedAt != nil {
		m.CertifiedAt = p.PayuniApprovedAt
	}
	if p.CertSellerType != "" {
		if st, err := domain.SellerTypeString(p.CertSellerType); err == nil {
			m.SellerType = st
		}
	}
	return u.toResponse(m, deriveOverallStatus(p.CertStatus, derivePayuniOutcome(p.PayuniMerStatus, p.PayuniMemberStatus)), deriveApprovalStatus(p.CertStatus), payuniMerStatusToInt(p.PayuniMerStatus), p.PayuniRegistrationStatus), nil
}

type CreateSellerInput struct {
	AccountID           int64
	StoreName           string
	ContactPhone        string
	ProspectInviteToken string
}

func (u *SellerUseCase) Create(ctx context.Context, accountID int64, storeName string, contactPhone string) (*dto.SellerProfile, error) {
	return u.CreateSeller(ctx, CreateSellerInput{AccountID: accountID, StoreName: storeName, ContactPhone: contactPhone})
}

func (u *SellerUseCase) CreateSeller(ctx context.Context, in CreateSellerInput) (*dto.SellerProfile, error) {
	accountID := in.AccountID
	storeName := in.StoreName
	contactPhone := in.ContactPhone
	acctInfo, err := u.accountClient.GetAccountInfo(ctx, accountID)
	if err != nil {
		logger.Warn(ctx, "failed to get account info for seller creation", zap.Int64("account_id", accountID), zap.Error(err))
	}
	var email string
	if acctInfo != nil {
		email = acctInfo.Email
		if contactPhone == "" {
			contactPhone = acctInfo.Phone
		}
	}
	contactPhone = phone.NormalizeToLocal(contactPhone)
	if storeName == "" {
		storeName = fmt.Sprintf("Hawkr 賣家%04d", rand.Intn(10000))
	}
	now := time.Now().UTC()
	m := &domain.Seller{AccountID: accountID, StoreID: u.tsid(), StoreName: storeName, Email: email, ContactPhone: contactPhone, Logo: domain.DefaultSellerLogo, CoverKey: domain.DefaultSellerCover, CreatedAt: now, UpdatedAt: now}

	const maxStoreCodeAttempts = 5
	var txErr error
	for attempt := 0; attempt < maxStoreCodeAttempts; attempt++ {
		code, genErr := u.storeCode()
		if genErr != nil {
			return nil, genErr
		}
		m.StoreCode = code

		txErr = u.txManager.WithTx(ctx, func(ctx context.Context) error {
			existing, lookupErr := u.sellerRepo.GetByAccountIDForUpdate(ctx, accountID)
			if lookupErr != nil && !errors.IsNotFound(lookupErr) {
				return lookupErr
			}
			if existing != nil {
				if !existing.IsClosed() {
					return errors.NewConflict("您已經是賣家，無需重複申請")
				}
				m.StoreID = u.tsid()
				if err := u.sellerRepo.RetireCurrentStint(ctx, accountID, now); err != nil {
					return err
				}
				logger.Info(ctx, "store re-onboarded (new stint)",
					zap.Int64("account_id", accountID),
					zap.Int64("store_id", m.StoreID))
			}
			if err := u.sellerRepo.Create(ctx, m); err != nil {
				return err
			}
			row, err := domain.NewLifecycleOutboxRow(u.tsid(), accountID, domain.SellerLifecycleEventOnboarded, domain.SellerLifecyclePayload{
				AccountID: accountID,
				StoreID:   m.StoreID,
			}, now)
			if err != nil {
				return err
			}
			if err := u.outboxRepo.Insert(ctx, row); err != nil {
				return err
			}
			logger.Info(ctx, "merchant created", zap.Int64("account_id", accountID), zap.Int64("store_id", m.StoreID))

			return nil
		})

		var dupErr *domain.ErrDuplicate
		if !stderrors.As(txErr, &dupErr) || dupErr.ConstraintName != "uq_sellers_store_code" {
			break
		}
		logger.Warn(ctx, "store_code collision, retrying", zap.Int64("account_id", accountID), zap.Int("attempt", attempt+1))
	}
	if txErr != nil {
		return nil, txErr
	}

	if err := u.accountClient.BindSellerRole(ctx, accountID, m.StoreID); err != nil {
		logger.Warn(ctx, "seller: BindSellerRole failed after store creation; the seller.onboarded outbox row converges it",
			zap.Int64("account_id", accountID), zap.Int64("store_id", m.StoreID), zap.Error(err))
	}

	if token := in.ProspectInviteToken; token != "" && u.prospectRepo != nil {
		if err := u.attributeProspect(ctx, token, m.StoreID); err != nil {
			logger.Warn(ctx, "seller: prospect attribution skipped, seller creation unaffected",
				zap.Error(err), zap.Int64("store_id", m.StoreID))
		}
	}

	return u.toResponse(m, deriveOverallStatus("", payuniOutcome("")), deriveApprovalStatus(""), payuniMerStatusToInt(""), ""), nil
}

func (u *SellerUseCase) attributeProspect(ctx context.Context, token string, storeID int64) error {
	return u.txManager.WithTx(ctx, func(ctx context.Context) error {
		invite, err := u.prospectRepo.FindUsableInviteByToken(ctx, token)
		if err != nil {
			return fmt.Errorf("邀請憑證不可用: %w", err)
		}
		if err := markProspectConverted(ctx, u.prospectRepo.MarkConverted, invite.ProspectID, storeID, nil); err != nil {
			return err
		}
		return u.prospectRepo.ConsumeInvite(ctx, invite.ID)
	})
}

func (u *SellerUseCase) UpdateProfile(ctx context.Context, req *dto.UpdateSellerProfile) error {
	m, err := u.resolveTarget(ctx, req.Target)
	if err != nil {
		return err
	}
	if err := validateWebsiteURL(req.WebsiteURL); err != nil {
		return err
	}
	storeName := req.StoreName
	if storeName == "" {
		storeName = m.StoreName
	}
	logo := m.Logo
	coverKey := m.CoverKey
	birthday := req.Birthday
	if birthday == "" {
		birthday = m.Birthday
	}
	gender := req.Gender
	if gender == "" {
		gender = m.Gender
	}
	city := strings.TrimSpace(req.City)
	if city != "" {
		canonical, ok := twaddr.CanonicalCity(city)
		if !ok {
			return errors.NewInvalidInput("invalid Taiwan city")
		}
		city = canonical
	}
	postalCode := twaddr.LookupPostalCodeByCityDistrict(city, req.District)
	contactPhone := phone.NormalizeToLocal(req.ContactPhone)

	if err := u.assertPayuniLockedFieldsUnchanged(ctx, m, storeName, req.Description); err != nil {
		return err
	}

	addressChanged := city != m.City || req.District != m.District || req.AddressDetail != m.AddressDetail
	enqueueSync := addressChanged && u.payuniHoldsAddressFor(ctx, m)

	m.UpdateProfile(storeName, req.Description, logo, coverKey, contactPhone, city, req.District, req.AddressDetail, postalCode, birthday, gender, req.Prize, req.WebsiteURL, time.Now().UTC())
	if err := u.txManager.WithTx(ctx, func(ctx context.Context) error {
		if err := u.sellerRepo.Update(ctx, m, req.ExpectedVersion); err != nil {
			return err
		}
		if !enqueueSync {
			return nil
		}
		return u.enqueuePayuniAddressSync(ctx, m)
	}); err != nil {
		return err
	}
	u.bustSellerCacheFor(ctx, m)
	u.emitProfileChanged(m.StoreID)
	return nil
}

var payuniLockedCertStatuses = map[domain.CertificationStatus]error{
	domain.CertStatusSubmitted: domain.ErrPayuniLockedFieldUnderReview,
	domain.CertStatusForwarded: domain.ErrPayuniLockedField,
}

func (u *SellerUseCase) assertPayuniLockedFieldsUnchanged(ctx context.Context, m *domain.Seller, storeName, description string) error {
	if storeName == m.StoreName && description == m.Description {
		return nil
	}
	if u.certRepo == nil {
		return nil
	}

	app, err := u.certRepo.GetCurrentApplication(ctx, m.AccountID)
	if err != nil {
		logger.Warn(ctx, "payuni locked-field check: 取認證申請失敗，放行",
			zap.Int64("store_id", m.StoreID), zap.Error(err))
		return nil
	}
	if app == nil {
		return nil
	}
	return payuniLockedCertStatuses[app.Status]
}

func (u *SellerUseCase) payuniHoldsAddressFor(ctx context.Context, m *domain.Seller) bool {
	if u.payuniProfileOutboxRepo == nil || u.certRepo == nil || u.payuniRepo == nil {
		return false
	}

	app, err := u.certRepo.GetCurrentApplication(ctx, m.AccountID)
	if err != nil {
		logger.Warn(ctx, "payuni address sync: 取認證申請失敗，跳過投遞",
			zap.Int64("store_id", m.StoreID), zap.Error(err))
		return false
	}
	if app == nil || app.SellerType == domain.SellerTypeBusiness.String() {
		return false
	}

	reg, err := u.payuniRepo.GetByStoreID(ctx, m.StoreID)
	if err != nil {
		logger.Warn(ctx, "payuni address sync: 取 PayUni 註冊失敗，跳過投遞",
			zap.Int64("store_id", m.StoreID), zap.Error(err))
		return false
	}
	return reg != nil && reg.Status == domain.PayuniStatusActive
}

func (u *SellerUseCase) enqueuePayuniAddressSync(ctx context.Context, m *domain.Seller) error {
	row, err := domain.NewPayuniMerchantProfileOutboxRow(
		u.tsid(), m.StoreID, m.AccountID, m.FullAddress(), m.PostalCode, time.Now().UTC())
	if err != nil {
		return err
	}
	return u.payuniProfileOutboxRepo.Insert(ctx, row)
}

func (u *SellerUseCase) CompleteOnboarding(ctx context.Context, accountID int64) error {
	if err := u.sellerRepo.CompleteOnboarding(ctx, accountID); err != nil {
		return err
	}
	u.bustSellerCache(ctx, accountID)
	return nil
}

func (u *SellerUseCase) UpdateSEOFields(ctx context.Context, accountID int64, seoTitle, seoDescription string) error {
	if _, err := u.sellerRepo.GetByAccountID(ctx, accountID); err != nil {
		return err
	}
	if err := u.sellerRepo.UpdateSEO(ctx, accountID, seoTitle, seoDescription); err != nil {
		return err
	}
	u.bustSellerCache(ctx, accountID)
	return nil
}

func (u *SellerUseCase) UpdateFoodBusinessRegistration(ctx context.Context, accountID int64, isProcessedFood bool, regNo string) (bool, string, time.Time, error) {
	seller, err := u.sellerRepo.GetByAccountID(ctx, accountID)
	if err != nil {
		return false, "", time.Time{}, err
	}
	if !seller.IsActive() {
		return false, "", time.Time{}, errors.NewBusiness(bizcode.SellerNotActive, "賣家未啟用，無法補登食品業者登錄字號")
	}
	if !isProcessedFood {
		regNo = ""
	} else {
		reg := strings.TrimSpace(regNo)
		if reg == "" || len([]rune(reg)) > 50 {
			return false, "", time.Time{}, errors.NewBusiness(bizcode.FoodBusinessRegNoInvalid, "食品業者登錄字號格式不正確或未填")
		}
		regNo = reg
	}
	now := time.Now().UTC()
	if err := u.sellerRepo.UpdateFoodBusinessRegistration(ctx, accountID, isProcessedFood, regNo, now); err != nil {
		return false, "", time.Time{}, err
	}
	u.bustSellerCache(ctx, accountID)
	return isProcessedFood, regNo, now, nil
}

func (u *SellerUseCase) GetPayuniStatus(ctx context.Context, accountID int64) (*dto.PayuniStatus, error) {
	seller, err := u.sellerRepo.GetByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	info, err := u.payuniRepo.GetByStoreID(ctx, seller.StoreID)
	if err != nil {
		return nil, err
	}
	if info == nil || seller.CurrentCertificationID == nil || info.ApplicationID != *seller.CurrentCertificationID {
		return nil, errors.NewNotFound("payuni registration not found")
	}
	now := time.Now().UTC()
	display := PayuniErrorDisplayFor(info)
	if display.Code != "" && IsPayuniUnknownErrorMessage(display.Message) {
		logger.Warn(ctx, "PayUni 回了目錄未登錄的錯誤碼，需補 CodeMeta",
			zap.String("error_code", display.Code),
			zap.Int64("store_id", info.StoreID))
	}
	outcome := derivePayuniOutcome(info.MerStatus, info.MemberStatus)
	return &dto.PayuniStatus{
		AccountID: info.AccountID, PayuniMerID: info.PayuniMerID, Status: info.Status.String(),
		PaymentEnabled: info.PaymentEnabled, MerStatus: info.MerStatus, MemberStatus: info.MemberStatus, CreditStatus: info.CreditStatus,
		Reason:         info.Reason,
		ApplePayStatus: info.ApplePayStatus, GooglePayStatus: info.GooglePayStatus, ATMStatus: info.ATMStatus,
		ErrorMessage: info.ErrorMessage,
		RegisteredAt: info.RegisteredAt, ApprovedAt: info.ApprovedAt, LastSyncedAt: info.LastSyncedAt,
		Freshness:              computePayuniFreshness(info.Status, outcome, info.LastSyncedAt, now),
		AllowedActions:         computePayuniAllowedActions(info.Status, outcome, info.LastSyncedAt, now),
		MerchantProfileVersion: info.MerchantProfileVersion,
		ContactName:            info.ContactName,
		ContactMobile:          info.ContactMobile,
		ContactTel:             info.ContactTel,
		MerEmail:               info.MerEmail,
		MerAddr:                info.MerAddr,
		MerAddrPost:            info.MerAddrPost,
		MerTel:                 info.MerTel,
		ErrorCode:              display.Code,
		ErrorDisplayMessage:    display.Message,
		ErrorRemedy:            string(display.Remedy),
		RetryStep:              display.RetryStep,
	}, nil
}

func payuniNeedsReconciliation(status domain.PayuniStatus, outcome payuniOutcome) bool {
	return status == domain.PayuniStatusPending || outcome == outcomePendingInfo || outcome == outcomeUnderReview
}

func computePayuniFreshness(status domain.PayuniStatus, outcome payuniOutcome, lastSyncedAt *time.Time, now time.Time) string {
	if !payuniNeedsReconciliation(status, outcome) {
		return "not_applicable"
	}
	if lastSyncedAt == nil {
		return "very_stale"
	}
	age := now.Sub(*lastSyncedAt)
	switch {
	case age < payuniPollMinInterval:
		return "fresh"
	case age < payuniFreshnessStaleThreshold:
		return "stale"
	default:
		return "very_stale"
	}
}

func computePayuniAllowedActions(status domain.PayuniStatus, outcome payuniOutcome, lastSyncedAt *time.Time, now time.Time) []string {
	var actions []string
	if status == domain.PayuniStatusActive {
		actions = append(actions, "edit_merchant_profile")
	}
	if payuniNeedsReconciliation(status, outcome) && (lastSyncedAt == nil || now.Sub(*lastSyncedAt) >= payuniManualRefreshMinInterval) {
		actions = append(actions, "resync")
	}
	return actions
}

func (u *SellerUseCase) ListForAdmin(ctx context.Context, req *dto.AdminListSellersRequest) (*dto.AdminSellerListResponse, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	limit := req.Limit
	if limit < 1 {
		limit = domain.DefaultLimit
	}
	offset := (int64(page) - 1) * int64(limit)
	if offset > math.MaxInt32 {
		page = 1
		limit = domain.DefaultLimit
		offset = 0
	}

	filter := domain.SellerListFilter{
		CertStatuses:    expandApprovalStatusFilter(req.ApprovalStatus),
		LifecycleStatus: req.LifecycleStatus,
		Limit:           limit,
		Offset:          int32(offset),
	}
	if req.Keyword != "" {
		filter.Keyword = &req.Keyword
	}

	merchants, err := u.sellerRepo.ListForAdmin(ctx, filter)
	if err != nil {
		return nil, err
	}

	total, err := u.sellerRepo.CountForAdmin(ctx, filter)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.AdminSellerItem, 0, len(merchants))
	for _, m := range merchants {
		items = append(items, &dto.AdminSellerItem{
			AccountID:                  m.AccountID,
			StoreID:                    m.StoreID,
			StoreName:                  m.StoreName,
			StoreCode:                  m.StoreCode,
			Description:                m.Description,
			Logo:                       m.Logo,
			CoverKey:                   m.CoverKey,
			ContactPhone:               m.ContactPhone,
			Email:                      m.Email,
			City:                       m.City,
			District:                   m.District,
			AddressDetail:              m.AddressDetail,
			PostalCode:                 m.PostalCode,
			Rating:                     m.Rating,
			SellerType:                 m.SellerType.String(),
			PayuniStatus:               payuniMerStatusToInt(m.PayuniMerStatus),
			MerStatus:                  m.PayuniMerStatus,
			MemberStatus:               m.PayuniMemberStatus,
			ApprovalStatus:             deriveApprovalStatus(m.CertStatus),
			OverallCertificationStatus: deriveOverallStatus(m.CertStatus, derivePayuniOutcome(m.PayuniMerStatus, m.PayuniMemberStatus)),
			LifecycleStatus:            string(m.Status),
			SeoTitle:                   m.SeoTitle,
			SeoDescription:             m.SeoDescription,
			CertifiedAt:                m.CertifiedAt,
			CreatedAt:                  m.CreatedAt,
			UpdatedAt:                  m.UpdatedAt,
			CurrentCertificationID:     m.CurrentCertificationID,
		})
	}

	return &dto.AdminSellerListResponse{
		List:  items,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

func (u *SellerUseCase) GetStatsForAdmin(ctx context.Context) (*dto.SellerStats, error) {
	stats, err := u.sellerRepo.GetStatsForAdmin(ctx)
	if err != nil {
		return nil, err
	}
	return &dto.SellerStats{
		Total:      stats.Total,
		Pending:    stats.Pending,
		Approved:   stats.Approved,
		Rejected:   stats.Rejected,
		MonthlyNew: stats.MonthlyNew,
	}, nil
}

func (u *SellerUseCase) toResponse(m *domain.Seller, overall, approval string, payuni int, payuniRegStatus string) *dto.SellerProfile {
	return &dto.SellerProfile{
		AccountID: m.AccountID, StoreID: m.StoreID, StoreCode: m.StoreCode, StoreName: m.StoreName, Description: m.Description, Logo: m.Logo, CoverKey: m.CoverKey,
		Email: m.Email, ContactPhone: m.ContactPhone, Phone: m.ContactPhone, Nickname: m.StoreName,
		City: m.City, District: m.District, AddressDetail: m.AddressDetail, PostalCode: m.PostalCode,
		Birthday: m.Birthday, Gender: m.Gender, Prize: m.Prize, Rating: m.Rating,
		SellerType: m.SellerType.String(), PayuniStatus: payuni, PayuniRegistrationStatus: payuniRegStatus, ApprovalStatus: approval,
		OverallCertificationStatus: overall,
		SeoTitle:                   m.SeoTitle, SeoDescription: m.SeoDescription,
		WebsiteURL:      m.WebsiteURL,
		IsProcessedFood: m.IsProcessedFood, FoodBusinessRegNo: m.FoodBusinessRegNo,
		OnboardingCompleted: m.OnboardingCompleted, CertifiedAt: m.CertifiedAt, CurrentCertificationID: m.CurrentCertificationID,
		Status: string(m.Status), Version: m.Version,
	}
}

func (u *SellerUseCase) GetShippingProfile(ctx context.Context, accountID, storeID int64) (*dto.SellerShippingProfile, error) {
	var m *domain.Seller
	if storeID > 0 {
		p, err := u.sellerRepo.GetPublicProfileByStoreIDOrCode(ctx, storeID)
		if err != nil {
			return nil, err
		}
		m = p.Seller
	} else {
		var err error
		m, err = u.sellerRepo.GetByAccountID(ctx, accountID)
		if err != nil {
			return nil, err
		}
	}
	if err := validateShippingProfileFields(m); err != nil {
		return nil, err
	}
	merID := domain.FormatMerID(m.StoreCode)
	if u.sandboxMerIDOverride != "" {
		merID = u.sandboxMerIDOverride
	}
	return &dto.SellerShippingProfile{
		StoreID:       fmt.Sprintf("%d", m.StoreID),
		SenderName:    m.StoreName,
		SenderPhone:   m.ContactPhone,
		ContactEmail:  m.Email,
		City:          m.City,
		District:      m.District,
		AddressDetail: m.AddressDetail,
		ZipCode:       m.PostalCode,
		MerID:         merID,
	}, nil
}

func validateShippingProfileFields(m *domain.Seller) error {
	type field struct {
		name  string
		value string
	}
	required := []field{
		{"sender_phone", m.ContactPhone},
		{"city", m.City},
		{"district", m.District},
		{"address_detail", m.AddressDetail},
		{"zip_code", m.PostalCode},
	}
	var missing []string
	for _, f := range required {
		if f.value == "" {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return errors.NewPreconditionFailed(fmt.Sprintf("seller profile missing required fields: %v", missing))
	}
	return nil
}

func (u *SellerUseCase) ResolvePrincipals(ctx context.Context, ids []int64) (map[int64]int64, []int64, error) {
	if len(ids) == 0 {
		return map[int64]int64{}, nil, nil
	}
	result, unresolvedIDs, err := u.sellerRepo.GetPrincipalsByStoreIDs(ctx, ids)
	if err != nil {
		logger.Warn(ctx, "resolve principals 失敗", zap.Error(err))
		return nil, nil, err
	}
	return result, unresolvedIDs, nil
}

func (u *SellerUseCase) ListStoreIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	storeIDs, err := u.sellerRepo.GetStoreIDsByAccountID(ctx, accountID)
	if err != nil {
		logger.Warn(ctx, "列舉帳號歷史 store_id 失敗", zap.Error(err), zap.Int64("account_id", accountID))
		return nil, err
	}
	return storeIDs, nil
}

func (u *SellerUseCase) GetAccountIDByStoreID(ctx context.Context, storeID int64) (int64, error) {
	accountID, err := u.sellerRepo.GetAccountIDByStoreID(ctx, storeID)
	if err != nil {
		logger.Warn(ctx, "解析 store_id 對應 account_id 失敗", zap.Error(err), zap.Int64("store_id", storeID))
		return 0, err
	}
	return accountID, nil
}

func (u *SellerUseCase) BatchGetSellerMeta(ctx context.Context, storeIDs []int64) (map[int64]*dto.SellerMeta, error) {
	if len(storeIDs) == 0 {
		return map[int64]*dto.SellerMeta{}, nil
	}

	logger.Info(ctx, "批次查詢賣家 meta", zap.Int64s("store_ids", storeIDs))
	rows, err := u.sellerRepo.GetSellerMetaByStoreIDs(ctx, storeIDs)
	if err != nil {
		logger.Warn(ctx, "批次查詢賣家 meta 失敗", zap.Error(err))
		return nil, err
	}
	logger.Info(ctx, "批次查詢賣家 meta 成功", zap.Int("count", len(rows)))

	result := make(map[int64]*dto.SellerMeta, len(rows))
	for _, r := range rows {
		result[r.StoreID] = &dto.SellerMeta{
			AccountID:         r.AccountID,
			StoreID:           r.StoreID,
			StoreCode:         r.StoreCode,
			SellerName:        r.StoreName,
			LogoURL:           r.Logo,
			CanAcceptOrders:   domain.CanAcceptOrders(r.PayuniStatus, domain.SellerStatus(r.Status)),
			CanManageProducts: domain.CanManageProducts(domain.SellerStatus(r.Status)),
			ZipCode:           r.ZipCode,
			City:              r.City,
			District:          r.District,
			PayuniMerID:       domain.FormatMerID(r.StoreCode),
		}
	}
	return result, nil
}

func (u *SellerUseCase) ListPublicStoreIDsByCity(ctx context.Context, city string) (string, []int64, error) {
	canonical, ok := twaddr.CanonicalCity(city)
	if !ok {
		canonical, ok = twaddr.CityFromCode(city)
	}
	if !ok {
		return "", nil, errors.NewInvalidInput("invalid Taiwan city")
	}
	ids, err := u.sellerRepo.GetPublicStoreIDsByCity(ctx, twaddr.CityAliases(canonical))
	return canonical, ids, err
}

func (u *SellerUseCase) ListPublicStoresByCity(ctx context.Context) ([]domain.PublicCityStores, error) {
	grouped, err := u.sellerRepo.GetPublicStoreIDsGroupedByCity(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.PublicCityStores, 0, len(twaddr.TaiwanCities))
	for _, city := range twaddr.TaiwanCities {
		code, _ := twaddr.CityCode(city)
		ids := grouped[city]
		previewCount := len(ids)
		if previewCount > 6 {
			previewCount = 6
		}
		preview := append([]int64(nil), ids[:previewCount]...)
		result = append(result, domain.PublicCityStores{
			CityCode: code, CityName: city, StoreIDs: ids, PreviewStoreIDs: preview,
		})
	}
	return result, nil
}

func (u *SellerUseCase) UpdateLogo(ctx context.Context, target dto.SellerTarget, stagedToken string, expectedVersion *int64) (logoKey string, err error) {
	m, err := u.resolveTarget(ctx, target)
	if err != nil {
		return "", err
	}

	assets, err := u.mediaClient.FinalizeImages(ctx, []string{stagedToken}, media.RefTypeSellerLogo)
	if err != nil {
		return "", fmt.Errorf("finalize logo: %w", err)
	}
	if len(assets) == 0 {
		return "", errors.NewInvalidInput("no asset returned from finalize")
	}

	logoKey = assets[0].ObjectKey
	refID := fmt.Sprintf("%d", m.StoreID)

	if linkErr := u.mediaClient.LinkImages(ctx, media.RefTypeSellerLogo, refID, []string{assets[0].AssetId}); linkErr != nil {
		return "", fmt.Errorf("link logo image: %w", linkErr)
	}

	if err := u.sellerRepo.UpdateLogo(ctx, m.StoreID, logoKey, expectedVersion); err != nil {
		return "", err
	}

	u.bustSellerCacheFor(ctx, m)
	u.emitProfileChanged(m.StoreID)

	if relErr := u.mediaClient.ReleaseReferences(ctx, media.RefTypeSellerLogo,
		[]string{refID}, []string{assets[0].AssetId}); relErr != nil {
		logger.Warn(ctx, "failed to release superseded logo references", zap.Int64("store_id", m.StoreID), zap.Error(relErr))
	}

	return logoKey, nil
}

func (u *SellerUseCase) UpdateCover(ctx context.Context, target dto.SellerTarget, stagedToken string, expectedVersion *int64) (coverKey string, err error) {
	m, err := u.resolveTarget(ctx, target)
	if err != nil {
		return "", err
	}

	assets, err := u.mediaClient.FinalizeImages(ctx, []string{stagedToken}, media.RefTypeSellerCover)
	if err != nil {
		return "", fmt.Errorf("finalize cover: %w", err)
	}
	if len(assets) == 0 {
		return "", errors.NewInvalidInput("no asset returned from finalize")
	}

	coverKey = assets[0].ObjectKey
	refID := fmt.Sprintf("%d", m.StoreID)

	if linkErr := u.mediaClient.LinkImages(ctx, media.RefTypeSellerCover, refID, []string{assets[0].AssetId}); linkErr != nil {
		return "", fmt.Errorf("link cover image: %w", linkErr)
	}

	if err := u.sellerRepo.UpdateCover(ctx, m.StoreID, coverKey, expectedVersion); err != nil {
		return "", err
	}

	u.bustSellerCacheFor(ctx, m)
	u.emitProfileChanged(m.StoreID)

	if relErr := u.mediaClient.ReleaseReferences(ctx, media.RefTypeSellerCover,
		[]string{refID}, []string{assets[0].AssetId}); relErr != nil {
		logger.Warn(ctx, "failed to release superseded cover references", zap.Int64("store_id", m.StoreID), zap.Error(relErr))
	}

	return coverKey, nil
}
