package usecase

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/phone"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/go-pkg/twaddr"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
	"go.uber.org/zap"
)

type payuniOutcomeApplier interface {
	ReapplyOutcome(ctx context.Context, info *domain.PayuniRegistration, now time.Time, source string) bool
}

type CertificationUseCase struct {
	certRepo      domain.CertificationRepository
	sellerRepo    domain.SellerRepository
	payuniRepo    domain.PayuniRepository
	payuniClient  PayuniClient
	accountClient AccountClient
	mediaClient   MediaClient
	txManager     database.TxManager
	outboxRepo    domain.MerchantVerificationOutboxRepository
	payuniOutcome payuniOutcomeApplier
	tsid          func() int64
	socialClient  SocialClient
}

func NewCertificationUseCase(
	certRepo domain.CertificationRepository,
	sellerRepo domain.SellerRepository,
	payuniRepo domain.PayuniRepository,
	pc PayuniClient,
	ac AccountClient,
	mc MediaClient,
	txManager database.TxManager,
	outboxRepo domain.MerchantVerificationOutboxRepository,
	outcome payuniOutcomeApplier,
	tsid func() int64,
	sc SocialClient,
) *CertificationUseCase {
	return &CertificationUseCase{
		certRepo:      certRepo,
		sellerRepo:    sellerRepo,
		payuniRepo:    payuniRepo,
		payuniClient:  pc,
		accountClient: ac,
		mediaClient:   mc,
		txManager:     txManager,
		outboxRepo:    outboxRepo,
		payuniOutcome: outcome,
		tsid:          tsid,
		socialClient:  sc,
	}
}

func (u *CertificationUseCase) CreateDraft(ctx context.Context, req *dto.CreateCertificationDraft) (*dto.CertificationApplication, error) {
	existing, err := u.certRepo.GetCurrentApplication(ctx, req.AccountID)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else if existing != nil && (existing.IsDraft() || existing.IsSubmitted()) {
		if req.SellerType != "" && existing.SellerType != req.SellerType && existing.IsDraft() {
			oldType := existing.SellerType
			hadStepData := len(existing.StepData) > 0
			existing.SellerType = req.SellerType
			existing.TotalSteps = totalStepsForSellerType(req.SellerType)
			existing.CurrentStep = 0
			existing.StepData = nil
			existing.CompanyName = ""
			existing.CompanyType = ""
			existing.TaxID = ""
			existing.CompanyCity = ""
			existing.CompanyDistrict = ""
			existing.CompanyAddressDetail = ""
			existing.CompanyEmail = ""
			existing.CompanyPhone = ""
			existing.CompanyRegisterDate = nil
			existing.ProductType = ""
			existing.IsProcessedFood = false
			existing.FoodBusinessRegNo = ""
			existing.UpdatedAt = time.Now().UTC()
			if err := u.certRepo.UpdateApplication(ctx, existing); err != nil {
				return nil, fmt.Errorf("switch seller type: %w", err)
			}
			logger.Info(ctx, "switched certification draft seller type",
				zap.Int64("application_id", existing.ID),
				zap.String("from", oldType),
				zap.String("to", req.SellerType))
			if hadStepData {
				if relErr := u.mediaClient.ReleaseReferences(ctx, media.RefTypeIdentityVerification,
					[]string{fmt.Sprintf("%d", existing.ID)}, nil); relErr != nil {
					logger.Warn(ctx, "failed to release identity verification references after seller type switch",
						zap.Int64("application_id", existing.ID), zap.Error(relErr))
				}
			}
		}
		return u.toResponse(existing), nil
	}

	if existing != nil {
		switch existing.Status {
		case domain.CertStatusForwarded:
			if u.payuniOutcomeFor(ctx, existing) == outcomeApproved {
				return nil, errors.NewPreconditionFailed("認證已通過，無需重新申請")
			}
			return nil, errors.NewPreconditionFailed("認證審核中，無法重新申請")
		case domain.CertStatusForwardFailed:
			return nil, errors.NewPreconditionFailed("認證審核中，無法重新申請")
		}
	}

	seller, err := u.sellerRepo.GetByAccountID(ctx, req.AccountID)
	if err != nil {
		return nil, fmt.Errorf("load seller: %w", err)
	}
	if err := u.enforceCertificationPrecondition(ctx, seller); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	app := &domain.CertificationApplication{
		ID:            u.tsid(),
		AccountID:     req.AccountID,
		ApplicationNo: fmt.Sprintf("CERT-%d-%d", now.Unix(), req.AccountID),
		Status:        domain.CertStatusDraft,
		SellerType:    req.SellerType,
		CurrentStep:   0,
		TotalSteps:    totalStepsForSellerType(req.SellerType),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	app.InitFSM()

	if err := u.certRepo.CreateApplication(ctx, app); err != nil {
		return nil, err
	}

	if err := u.sellerRepo.UpdateCurrentCertificationID(ctx, req.AccountID, app.ID); err != nil {
		logger.Warn(ctx, "failed to update current_certification_id", zap.Int64("account_id", req.AccountID), zap.Int64("cert_id", app.ID), zap.Error(err))
	}

	return u.toResponse(app), nil
}

func (u *CertificationUseCase) GetDraft(ctx context.Context, accountID int64) (*dto.CertificationApplication, error) {
	app, err := u.certRepo.GetDraftApplication(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, nil
	}
	return u.toResponse(app), nil
}

func (u *CertificationUseCase) GetApplication(ctx context.Context, id int64) (*dto.CertificationApplication, error) {
	app, err := u.certRepo.GetApplicationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return u.toResponse(app), nil
}

func (u *CertificationUseCase) GetApplicationForAdmin(ctx context.Context, id int64) (*dto.CertificationApplication, error) {
	app, err := u.certRepo.GetApplicationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return u.toResponseForAdmin(app), nil
}

func (u *CertificationUseCase) CancelDraft(ctx context.Context, applicationID int64, reason string) error {
	app, err := u.certRepo.GetApplicationByID(ctx, applicationID)
	if err != nil {
		return err
	}
	hadStepData := len(app.StepData) > 0
	if err := app.Cancel(time.Now().UTC(), reason); err != nil {
		return err
	}
	if err := u.certRepo.UpdateApplication(ctx, app); err != nil {
		return err
	}
	if hadStepData {
		if relErr := u.mediaClient.ReleaseReferences(ctx, media.RefTypeIdentityVerification,
			[]string{fmt.Sprintf("%d", applicationID)}, nil); relErr != nil {
			logger.Warn(ctx, "failed to release identity verification references after cancel",
				zap.Int64("application_id", applicationID), zap.Error(relErr))
		}
	}
	return nil
}

func (u *CertificationUseCase) payuniOutcomeFor(ctx context.Context, app *domain.CertificationApplication) payuniOutcome {
	if app.Status != domain.CertStatusForwarded {
		return ""
	}
	seller, err := u.sellerRepo.GetByAccountID(ctx, app.AccountID)
	if err != nil || seller == nil {
		return ""
	}
	reg, err := u.payuniRepo.GetByStoreID(ctx, seller.StoreID)
	if err != nil || reg == nil {
		return ""
	}
	return derivePayuniOutcome(reg.MerStatus, reg.MemberStatus)
}

func (u *CertificationUseCase) SaveProgress(ctx context.Context, req *dto.SaveProgress) error {
	app, err := u.certRepo.GetApplicationByID(ctx, req.ApplicationID)
	if err != nil {
		return err
	}
	outcome := u.payuniOutcomeFor(ctx, app)
	if !app.IsDraft() && !deriveCanResubmit(app.Status.String(), outcome) {
		logger.Warn(ctx, "can only save progress for draft or rejected applications", zap.Int64("account_id", app.AccountID))
		return errors.NewInvalidInput("can only save progress for draft or rejected applications")
	}

	if !app.IsDraft() {
		if err := app.ResetToDraft(time.Now().UTC(), outcome == outcomeRejected); err != nil {
			return err
		}
	}

	stepData, err := u.finalizeStepDataTokens(ctx, req.StepData, fmt.Sprintf("%d", app.ID))
	if err != nil {
		return err
	}
	app.UpdateProgress(req.CurrentStep, json.RawMessage(stepData), time.Now().UTC())
	return u.certRepo.UpdateApplication(ctx, app)
}

func (u *CertificationUseCase) finalizeStepDataTokens(ctx context.Context, stepDataStr, refID string) (string, error) {
	if stepDataStr == "" {
		return stepDataStr, nil
	}
	var d map[string]interface{}
	if err := json.Unmarshal([]byte(stepDataStr), &d); err != nil {
		return stepDataStr, nil
	}

	type singleField struct{ tokenKey, assetKey string }
	singleFields := []singleField{
		{"id_card_front_staged_token", "id_card_front_asset_id"},
		{"id_card_back_staged_token", "id_card_back_asset_id"},
		{"bank_book_staged_token", "bank_book_asset_id"},
	}

	var tokens []string
	type pendingEntry struct{ token, assetKey, tokenKey string }
	var pending []pendingEntry

	for _, f := range singleFields {
		if v, ok := d[f.tokenKey].(string); ok && v != "" {
			tokens = append(tokens, v)
			pending = append(pending, pendingEntry{token: v, assetKey: f.assetKey, tokenKey: f.tokenKey})
		}
	}

	var docTokens []string
	if v, ok := d["document_staged_tokens"].([]interface{}); ok {
		for _, t := range v {
			if s, ok := t.(string); ok && s != "" {
				docTokens = append(docTokens, s)
				tokens = append(tokens, s)
			}
		}
	}

	if len(tokens) == 0 {
		return stepDataStr, nil
	}

	assets, err := u.mediaClient.FinalizeImages(ctx, tokens, media.RefTypeIdentityVerification)
	if err != nil {
		return "", fmt.Errorf("finalize identity verification images (ref_id=%s): %w", refID, err)
	}

	tokenToAssetID := make(map[string]string, len(assets))
	assetIDs := make([]string, 0, len(assets))
	for _, a := range assets {
		tokenToAssetID[a.StagedToken] = a.AssetId
		assetIDs = append(assetIDs, a.AssetId)
	}

	if refID != "" && len(assetIDs) > 0 {
		if linkErr := u.mediaClient.LinkImages(ctx, media.RefTypeIdentityVerification, refID, assetIDs); linkErr != nil {
			return "", fmt.Errorf("link identity verification images (ref_id=%s): %w", refID, linkErr)
		}
	}

	for _, e := range pending {
		if assetID := tokenToAssetID[e.token]; assetID != "" {
			d[e.assetKey] = assetID
			delete(d, e.tokenKey)
		}
	}

	if len(docTokens) > 0 {
		var existing []string
		if v, ok := d["document_asset_ids"].([]interface{}); ok {
			for _, id := range v {
				if s, ok := id.(string); ok && s != "" {
					existing = append(existing, s)
				}
			}
		}
		for _, token := range docTokens {
			if assetID := tokenToAssetID[token]; assetID != "" {
				existing = append(existing, assetID)
			}
		}
		d["document_asset_ids"] = existing
		delete(d, "document_staged_tokens")
	}

	result, err := json.Marshal(d)
	if err != nil {
		return stepDataStr, nil
	}
	return string(result), nil
}

func (u *CertificationUseCase) Submit(ctx context.Context, req *dto.SubmitApplication) error {
	app, err := u.certRepo.GetApplicationByID(ctx, req.ID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	switch app.Status {
	case domain.CertStatusDraft, domain.CertStatusRejected:
	case domain.CertStatusSubmitted:
		return errors.NewPreconditionFailed("認證審核中，無法修改")
	default:
		return errors.NewPreconditionFailed("認證狀態不允許修改")
	}

	if req.SellerType != "" {
		app.SellerType = req.SellerType
	}
	app.RealName = req.RealName
	app.IDCardNo = req.IDCardNo
	app.IDCardIssueType = req.IDCardIssueType
	app.IDCardIssueLocation = req.IDCardIssueLocation
	app.EnglishName = req.EnglishName

	if req.BirthDate != "" {
		if birthDate, err := parseROCDate(req.BirthDate); err == nil {
			app.BirthDate = &birthDate
		} else {
			logger.Warn(ctx, "failed to parse birth_date", zap.Int("birth_date_len", len(req.BirthDate)), zap.Error(err))
		}
	}

	if req.IDCardIssueDate != "" {
		if issueDate, err := parseROCDate(req.IDCardIssueDate); err == nil {
			app.IDCardIssueDate = &issueDate
		} else {
			logger.Warn(ctx, "failed to parse id_card_issue_date", zap.Int("id_card_issue_date_len", len(req.IDCardIssueDate)), zap.Error(err))
		}
	}

	app.BankName = req.BankName
	app.BankCode = req.BankCode
	app.BankAccount = req.BankAccount

	app.CompanyType = req.CompanyType
	app.CompanyName = req.CompanyName
	app.TaxID = req.TaxID
	app.CompanyCity = req.CompanyCity
	app.CompanyDistrict = req.CompanyDistrict
	app.CompanyAddressDetail = req.CompanyAddressDetail
	app.CompanyEmail = req.CompanyEmail
	app.CompanyPhone = req.CompanyPhone
	app.ProductType = req.ProductType
	app.IsProcessedFood = req.IsProcessedFood
	app.FoodBusinessRegNo = req.FoodBusinessRegNo

	if req.CompanyRegisterDate != "" {
		if registerDate, err := parseROCDate(req.CompanyRegisterDate); err == nil {
			app.CompanyRegisterDate = &registerDate
		} else {
			logger.Warn(ctx, "failed to parse company_register_date", zap.Int("company_register_date_len", len(req.CompanyRegisterDate)), zap.Error(err))
		}
	}

	if app.StepData != nil {
		finalizedStepData, ferr := u.finalizeStepDataTokens(ctx, string(app.StepData), fmt.Sprintf("%d", app.ID))
		if ferr != nil {
			return ferr
		}
		app.StepData = json.RawMessage(finalizedStepData)
	}

	var sd map[string]interface{}
	if app.StepData != nil {
		_ = json.Unmarshal(app.StepData, &sd)
	}
	getStepStr := func(key string) string {
		if sd == nil {
			return ""
		}
		v, _ := sd[key].(string)
		return v
	}

	var finalizedAssetIDs []string
	if id := firstNonEmptyStr(req.IDCardFrontAssetID, getStepStr("id_card_front_asset_id")); id != "" {
		app.IDCardFrontAssetID = id
		finalizedAssetIDs = append(finalizedAssetIDs, id)
	}
	if id := firstNonEmptyStr(req.IDCardBackAssetID, getStepStr("id_card_back_asset_id")); id != "" {
		app.IDCardBackAssetID = id
		finalizedAssetIDs = append(finalizedAssetIDs, id)
	}
	if id := firstNonEmptyStr(req.BankBookAssetID, getStepStr("bank_book_asset_id")); id != "" {
		app.BankBookAssetID = id
		finalizedAssetIDs = append(finalizedAssetIDs, id)
	}
	if len(req.DocumentAssetIDs) > 0 {
		app.DocumentAssetIDs = req.DocumentAssetIDs
		finalizedAssetIDs = append(finalizedAssetIDs, req.DocumentAssetIDs...)
	} else if sd != nil {
		if v, ok := sd["document_asset_ids"].([]interface{}); ok {
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" {
					app.DocumentAssetIDs = append(app.DocumentAssetIDs, s)
					finalizedAssetIDs = append(finalizedAssetIDs, s)
				}
			}
		}
	}

	seller, err := u.sellerRepo.GetByAccountID(ctx, app.AccountID)
	if err != nil {
		return fmt.Errorf("load seller: %w", err)
	}

	acctInfo, err := u.accountClient.GetAccountInfo(ctx, app.AccountID)
	if err != nil {
		logger.Error(ctx, "failed to get account info from account-service",
			zap.Int64("account_id", app.AccountID), zap.Error(err))
		return errors.NewUnavailable("帳號資訊暫時無法取得，請稍後再試")
	}

	if err := u.enforceCertificationPrecondition(ctx, seller); err != nil {
		return err
	}

	if err := validatePayuniFields(app, seller, acctInfo.Email, acctInfo.Phone); err != nil {
		return err
	}

	if !app.IsDraft() {
		if err := app.ResetToDraft(now, u.payuniOutcomeFor(ctx, app) == outcomeRejected); err != nil {
			return err
		}
	}
	if err := app.Submit(now); err != nil {
		return err
	}

	logger.Info(ctx, "submitting certification application",
		zap.Int64("application_id", app.ID),
		zap.Int64("account_id", app.AccountID))

	if len(finalizedAssetIDs) > 0 {
		if err := u.mediaClient.LinkImages(ctx, media.RefTypeCertification,
			fmt.Sprintf("%d", app.ID), finalizedAssetIDs); err != nil {
			return fmt.Errorf("link certification images (application_id=%d): %w", app.ID, err)
		}
	}

	if err := u.txManager.WithTx(ctx, func(ctx context.Context) error {
		if err := u.certRepo.UpdateApplication(ctx, app); err != nil {
			return err
		}

		review := domain.NewReview(app.ID, nil, "", domain.ReviewActionSubmit, domain.ReviewResultPending, "Application submitted", now)
		if err := u.certRepo.CreateReview(ctx, review); err != nil {
			return err
		}
		if err := u.sellerRepo.UpdateCurrentCertificationID(ctx, app.AccountID, app.ID); err != nil {
			logger.Warn(ctx, "failed to set current_certification_id", zap.Int64("account_id", app.AccountID), zap.Error(err))
		}
		return nil
	}); err != nil {
		return err
	}

	if len(finalizedAssetIDs) > 0 {
		if relErr := u.mediaClient.ReleaseReferences(ctx, media.RefTypeIdentityVerification,
			[]string{fmt.Sprintf("%d", app.ID)}, nil); relErr != nil {
			logger.Warn(ctx, "failed to release identity verification references after submit",
				zap.Int64("application_id", app.ID), zap.Error(relErr))
		}
	}

	return nil
}

func (u *CertificationUseCase) enforceCertificationPrecondition(ctx context.Context, seller *domain.Seller) error {
	if domain.MeetsCertificationPrecondition(false, seller.Description, seller.WebsiteURL) {
		return nil
	}
	hasPost, err := u.socialClient.HasPublishedPost(ctx, seller.StoreID)
	if err != nil {
		return err
	}
	if !hasPost {
		return errors.NewPreconditionFailed("請先發布一則動態，或同時填寫自我介紹與官方網站網址")
	}
	return nil
}

func (u *CertificationUseCase) registerPayuni(
	ctx context.Context,
	app *domain.CertificationApplication,
	seller *domain.Seller,
	acctInfo *AccountInfo,
) (*domain.PayuniRegistration, error) {
	now := time.Now().UTC()

	merID := domain.FormatMerID(seller.StoreCode)

	var postalCode string
	var address string

	if app.CompanyCity != "" && app.CompanyDistrict != "" {
		postalCode = twaddr.LookupPostalCodeByCityDistrict(app.CompanyCity, app.CompanyDistrict)
		address = app.CompanyCity + app.CompanyDistrict + app.CompanyAddressDetail
	} else {
		if seller.PostalCode != "" {
			postalCode = seller.PostalCode
		}
		address = seller.FullAddress()
		if address == "" && app.CompanyCity != "" {
			address = app.CompanyCity + app.CompanyDistrict + app.CompanyAddressDetail
		}
		if postalCode == "" {
			postalCode = twaddr.LookupPostalCode(address)
		}
	}

	logger.Info(ctx, "postal code lookup",
		zap.Int("address_len", len(address)),
		zap.String("postal_code", postalCode),
		zap.String("mer_id", merID))

	contactPhone := normalizePhoneNumber(acctInfo.Phone)
	if contactPhone == "" {
		contactPhone = normalizePhoneNumber(seller.ContactPhone)
	}

	payuniReq := &BindSellerRequest{
		SellerID:         merID,
		StoreID:          fmt.Sprintf("%d", seller.StoreID),
		StoreCode:        fmt.Sprintf("%d", seller.StoreCode),
		WebsiteURL:       app.PayuniWebsiteURL,
		SellerType:       app.SellerType,
		RealName:         app.RealName,
		IDCardNo:         app.IDCardNo,
		Email:            acctInfo.Email,
		CompanyName:      app.CompanyName,
		TaxID:            app.TaxID,
		CompanyPhone:     app.CompanyPhone,
		CompanyAddress:   app.CompanyCity + app.CompanyDistrict + app.CompanyAddressDetail,
		EnglishName:      app.EnglishName,
		StoreName:        seller.StoreName,
		StoreDescription: seller.Description,
		ContactPhone:     contactPhone,
		Address:          seller.FullAddress(),
		PostalCode:       postalCode,
	}

	rawRequest, marshalErr := json.Marshal(payuniReq)
	if marshalErr != nil {
		logger.Error(ctx, "failed to marshal PAYUNi request",
			zap.String("mer_id", merID),
			zap.Error(marshalErr))
	}

	resp, err := u.payuniClient.BindSeller(ctx, payuniReq)
	if err != nil {
		logger.Error(ctx, "PAYUNi bind_merchant call failed",
			zap.String("mer_id", merID),
			zap.Error(err))
		info := &domain.PayuniRegistration{
			ID:            u.tsid(),
			AccountID:     seller.AccountID,
			StoreID:       seller.StoreID,
			ApplicationID: app.ID,
			RawRequest:    rawRequest,
		}
		info.SetFailed("", fmt.Sprintf("bind_merchant call failed: %v", err), now)
		if upsertErr := u.payuniRepo.Upsert(ctx, info); upsertErr != nil {
			logger.Error(ctx, "failed to save payuni info", zap.Error(upsertErr))
		}
		return nil, newPayuniBindErrorFromNetwork(err)
	}

	if !resp.Success {
		errMsg := resp.Message
		if resp.ErrorMsg != "" {
			errMsg = resp.ErrorMsg
		}
		errCode := resp.ErrorCode
		if errCode == "" {
			errCode = parsePayuniErrorCode(errMsg)
		}
		logger.Warn(ctx, "PAYUNi bind_merchant returned failure",
			zap.String("mer_id", merID),
			zap.String("code", errCode),
			zap.String("message", errMsg))

		if IsPayuniAlreadyExistsError(errCode) {
			recInfo, recErr := u.recoverFromExistingMerchant(ctx, app, seller, merID, now)
			if recErr == nil {
				return recInfo, nil
			}
			if stderrors.Is(recErr, errPayuniTerminalMerchant) {
				errMsg = "PayUni 端商店代號已存在且為退件/關閉狀態，需人工向 PayUni 處理後再重試"
			}
			logger.Warn(ctx, "AGENT04017 self-heal failed, falling through",
				zap.String("mer_id", merID), zap.Error(recErr))
		}

		info := &domain.PayuniRegistration{
			ID:            u.tsid(),
			AccountID:     seller.AccountID,
			StoreID:       seller.StoreID,
			ApplicationID: app.ID,
			RawRequest:    rawRequest,
			RawResponse:   resp.RawResponse,
		}
		info.SetFailed(errCode, errMsg, now)
		if err := u.payuniRepo.Upsert(ctx, info); err != nil {
			logger.Error(ctx, "failed to save payuni info", zap.String("mer_id", merID), zap.Error(err))
		}
		return nil, newPayuniBindErrorFromBusiness(errCode, errMsg)
	}

	info := &domain.PayuniRegistration{
		ID:            u.tsid(),
		AccountID:     seller.AccountID,
		StoreID:       seller.StoreID,
		ApplicationID: app.ID,
		PayuniMerID:   merID,
		RawRequest:    rawRequest,
		RawResponse:   resp.RawResponse,
	}
	info.SetRegistered(merID, now)

	logger.Info(ctx, "PAYUNi seller registered successfully",
		zap.String("mer_id", merID))

	if err := u.payuniRepo.Upsert(ctx, info); err != nil {
		logger.Error(ctx, "failed to save payuni info", zap.String("mer_id", merID), zap.Error(err))
		return nil, fmt.Errorf("save PAYUNi registration info: %w", err)
	}

	return info, nil
}

var errPayuniTerminalMerchant = stderrors.New("payuni merchant in terminal status")

func (u *CertificationUseCase) recoverFromExistingMerchant(
	ctx context.Context,
	app *domain.CertificationApplication,
	seller *domain.Seller,
	merID string,
	now time.Time,
) (*domain.PayuniRegistration, error) {
	statusResp, err := u.payuniClient.GetSellerStatus(ctx, merID)
	if err != nil {
		return nil, fmt.Errorf("search_merchant_status failed: %w", err)
	}

	switch statusResp.MerStatus {
	case "2", "8", "92", "99":
		return nil, fmt.Errorf("PayUni merchant %s is in terminal status %s; cannot relink a fresh application: %w",
			merID, statusResp.MerStatus, errPayuniTerminalMerchant)
	}

	info := &domain.PayuniRegistration{
		ID:            u.tsid(),
		AccountID:     seller.AccountID,
		StoreID:       seller.StoreID,
		ApplicationID: app.ID,
		PayuniMerID:   merID,
		RawResponse:   statusResp.RawResponse,
	}
	info.Apply(observationFromAPI(statusResp, sourceApprove, now))

	info.SetRegistered(merID, now)

	if err := u.payuniRepo.Upsert(ctx, info); err != nil {
		return nil, fmt.Errorf("save recovered payuni info: %w", err)
	}

	if err := u.payuniRepo.InsertStatusHistory(ctx, &domain.PayuniStatusHistory{
		ID:              u.tsid(),
		AccountID:       info.AccountID,
		StoreID:         info.StoreID,
		PayuniMerID:     info.PayuniMerID,
		Source:          "recover",
		MerStatus:       info.MerStatus,
		MemberStatus:    info.MemberStatus,
		CreditStatus:    info.CreditStatus,
		ApplePayStatus:  info.ApplePayStatus,
		GooglePayStatus: info.GooglePayStatus,
		ATMStatus:       info.ATMStatus,
		Reason:          info.Reason,
		RawResponse:     info.RawResponse,
		RecordedAt:      now,
	}); err != nil {
		logger.Warn(ctx, "failed to record PAYUNi status history after recovery",
			zap.Int64("account_id", info.AccountID),
			zap.String("payuni_mer_id", info.PayuniMerID),
			zap.Error(err))
	}

	logger.Warn(ctx, "self-healed from AGENT04017 state drift",
		zap.String("mer_id", merID),
		zap.String("payuni_mer_status", statusResp.MerStatus),
		zap.Int64("account_id", seller.AccountID))

	return info, nil
}

func (u *CertificationUseCase) Approve(ctx context.Context, req *dto.ReviewApplication) error {
	app, err := u.certRepo.GetApplicationByID(ctx, req.ApplicationID)
	if err != nil {
		return err
	}
	if err := validateWebsiteURL(req.PayuniWebsiteURL); err != nil {
		return err
	}
	app.PayuniWebsiteURL = req.PayuniWebsiteURL
	now := time.Now().UTC()

	if !app.Can(domain.EventForward) {
		switch app.Status {
		case domain.CertStatusForwarded:
			return errors.NewPreconditionFailed("認證已送出至金流商，請勿重複核准")
		default:
			return errors.NewPreconditionFailed("認證狀態不允許核准")
		}
	}

	seller, err := u.sellerRepo.GetByAccountIDForUpdate(ctx, app.AccountID)
	if err != nil {
		return fmt.Errorf("load seller: %w", err)
	}
	existing, err := u.payuniRepo.GetByStoreID(ctx, seller.StoreID)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("lookup payuni registration: %w", err)
	}
	relink := existing != nil &&
		(existing.Status == domain.PayuniStatusPending || existing.Status == domain.PayuniStatusActive)

	logger.Info(ctx, "approving certification application",
		zap.Int64("application_id", app.ID), zap.Int64("reviewer_id", req.ReviewerID),
		zap.Bool("relink", relink))

	var info *domain.PayuniRegistration
	if relink {
		existing.ApplicationID = app.ID
		existing.UpdatedAt = now
		info = existing
	} else {
		acctInfo, err := u.accountClient.GetAccountInfo(ctx, app.AccountID)
		if err != nil {
			logger.Error(ctx, "failed to get account info", zap.Int64("account_id", app.AccountID), zap.Error(err))
			return errors.NewUnavailable("帳號資訊暫時無法取得，請稍後再試")
		}
		info, err = u.registerPayuni(ctx, app, seller, acctInfo)
		if err != nil {
			if ffErr := app.ForwardFailed(now); ffErr != nil {
				return ffErr
			}
			if upErr := u.certRepo.UpdateApplication(ctx, app); upErr != nil {
				logger.Warn(ctx, "failed to persist forward_failed", zap.Int64("application_id", app.ID), zap.Error(upErr))
			}
			review := domain.NewReview(app.ID, &req.ReviewerID, req.ReviewerName, domain.ReviewActionApprove, domain.ReviewResultForwardFailed, req.Comment, now)
			reason := []rune(err.Error())
			if len(reason) > 255 {
				reason = reason[:255]
			}
			review.RejectionReason = string(reason)
			review.RequiredFields = marshalRequiredFields(req.RequiredFields)
			if crErr := u.certRepo.CreateReview(ctx, review); crErr != nil {
				logger.Warn(ctx, "failed to create forward_failed review", zap.Int64("application_id", app.ID), zap.Error(crErr))
			}
			return err
		}
	}

	if err := app.Forward(now); err != nil {
		return err
	}
	accountID := app.AccountID
	if err := u.txManager.WithTx(ctx, func(ctx context.Context) error {
		if err := u.certRepo.UpdateApplication(ctx, app); err != nil {
			return err
		}
		review := domain.NewReview(app.ID, &req.ReviewerID, req.ReviewerName, domain.ReviewActionApprove, domain.ReviewResultApproved, req.Comment, now)
		review.RequiredFields = marshalRequiredFields(req.RequiredFields)
		if err := u.certRepo.CreateReview(ctx, review); err != nil {
			return err
		}
		if relink {
			if err := u.payuniRepo.Upsert(ctx, info); err != nil {
				return err
			}
		}
		outboxRow, err := domain.NewMerchantVerificationOutboxRow(u.tsid(), accountID, seller.StoreID, "forwarded_to_payuni", "", now)
		if err != nil {
			return err
		}
		return u.outboxRepo.Insert(ctx, outboxRow)
	}); err != nil {
		return err
	}

	if app.IsProcessedFood && app.FoodBusinessRegNo != "" {
		if err := u.sellerRepo.UpdateFoodBusinessRegistration(ctx, app.AccountID, true, app.FoodBusinessRegNo, now); err != nil {
			logger.Warn(ctx, "failed to project food business registration onto seller",
				zap.Int64("account_id", app.AccountID), zap.Int64("application_id", app.ID), zap.Error(err))
		}
	}

	if info.MerStatus != "" && u.payuniOutcome.ReapplyOutcome(ctx, info, now, sourceApprove) {
		if err := u.payuniRepo.Upsert(ctx, info); err != nil {
			logger.Warn(ctx, "failed to persist registration after outcome funnel",
				zap.Int64("account_id", accountID), zap.Error(err))
		}
	}
	return nil
}

func (u *CertificationUseCase) Reject(ctx context.Context, req *dto.ReviewApplication) error {
	app, err := u.certRepo.GetApplicationByID(ctx, req.ApplicationID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if err := app.Reject(now); err != nil {
		return err
	}

	accountID := app.AccountID

	seller, err := u.sellerRepo.GetByAccountIDForUpdate(ctx, accountID)
	if err != nil {
		return fmt.Errorf("load seller: %w", err)
	}

	rejectionReason := req.RejectionReason
	if err := u.txManager.WithTx(ctx, func(ctx context.Context) error {
		if err := u.certRepo.UpdateApplication(ctx, app); err != nil {
			return err
		}

		review := &domain.CertificationReview{
			ApplicationID:   app.ID,
			ReviewerID:      &req.ReviewerID,
			ReviewerName:    req.ReviewerName,
			Action:          domain.ReviewActionReject,
			Result:          domain.ReviewResultRejected,
			Comment:         req.Comment,
			RejectionReason: req.RejectionReason,
			RequiredFields:  marshalRequiredFields(req.RequiredFields),
			CreatedAt:       now,
		}
		if err := u.certRepo.CreateReview(ctx, review); err != nil {
			return err
		}

		outboxRow, err := domain.NewMerchantVerificationOutboxRow(u.tsid(), accountID, seller.StoreID, "rejected", rejectionReason, now)
		if err != nil {
			return err
		}
		return u.outboxRepo.Insert(ctx, outboxRow)
	}); err != nil {
		return err
	}

	return nil
}

func (u *CertificationUseCase) GetReviews(ctx context.Context, applicationID int64) ([]*dto.CertificationReview, error) {
	reviews, err := u.certRepo.GetReviewsByApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	result := make([]*dto.CertificationReview, 0, len(reviews))
	for _, r := range reviews {
		result = append(result, &dto.CertificationReview{
			ID:              r.ID,
			ApplicationID:   r.ApplicationID,
			ReviewerID:      r.ReviewerID,
			ReviewerName:    r.ReviewerName,
			Action:          r.Action.String(),
			Result:          r.Result.String(),
			Comment:         r.Comment,
			RejectionReason: r.RejectionReason,
			RequiredFields:  unmarshalRequiredFields(r.RequiredFields),
			CreatedAt:       r.CreatedAt,
		})
	}
	return result, nil
}

func (u *CertificationUseCase) toResponse(app *domain.CertificationApplication) *dto.CertificationApplication {
	return u.buildResponse(app, maskIDCardNo(app.IDCardNo))
}

func (u *CertificationUseCase) toResponseForAdmin(app *domain.CertificationApplication) *dto.CertificationApplication {
	return u.buildResponse(app, app.IDCardNo)
}

func maskIDCardNo(id string) string {
	runes := []rune(id)
	n := len(runes)
	if n == 0 {
		return ""
	}
	if n <= 4 {
		return strings.Repeat("*", n)
	}
	return string(runes[:2]) + strings.Repeat("*", n-4) + string(runes[n-2:])
}

func (u *CertificationUseCase) buildResponse(app *domain.CertificationApplication, idCardNo string) *dto.CertificationApplication {
	return &dto.CertificationApplication{
		ID:                   app.ID,
		AccountID:            app.AccountID,
		ApplicationNo:        app.ApplicationNo,
		Status:               app.Status.String(),
		SellerType:           app.SellerType,
		RealName:             app.RealName,
		IDCardNo:             idCardNo,
		BirthDate:            app.BirthDate,
		IDCardIssueDate:      app.IDCardIssueDate,
		IDCardIssueType:      app.IDCardIssueType,
		IDCardIssueLocation:  app.IDCardIssueLocation,
		EnglishName:          app.EnglishName,
		IDCardFrontAssetID:   app.IDCardFrontAssetID,
		IDCardBackAssetID:    app.IDCardBackAssetID,
		BankBookAssetID:      app.BankBookAssetID,
		DocumentAssetIDs:     app.DocumentAssetIDs,
		BankName:             app.BankName,
		BankCode:             app.BankCode,
		BankAccount:          app.BankAccount,
		CompanyType:          app.CompanyType,
		CompanyName:          app.CompanyName,
		TaxID:                app.TaxID,
		CompanyRegisterDate:  app.CompanyRegisterDate,
		CompanyCity:          app.CompanyCity,
		CompanyDistrict:      app.CompanyDistrict,
		CompanyAddressDetail: app.CompanyAddressDetail,
		CompanyEmail:         app.CompanyEmail,
		CompanyPhone:         app.CompanyPhone,
		ProductType:          app.ProductType,
		IsProcessedFood:      app.IsProcessedFood,
		FoodBusinessRegNo:    app.FoodBusinessRegNo,
		CurrentStep:          app.CurrentStep,
		TotalSteps:           app.TotalSteps,
		StepDataJSON:         string(app.StepData),
		SubmittedAt:          app.SubmittedAt,
		RejectedAt:           app.RejectedAt,
		CreatedAt:            app.CreatedAt,
		UpdatedAt:            app.UpdatedAt,
		PayuniWebsiteURL:     app.PayuniWebsiteURL,
	}
}

func normalizePhoneNumber(p string) string {
	p = phone.NormalizeToLocal(p)
	if len(p) == 10 && strings.HasPrefix(p, "09") {
		return p
	}
	return ""
}

func parseROCDate(rocDate string) (time.Time, error) {
	parsed, err := timeutil.ParseROCDate(rocDate)
	if err != nil {
		return time.Time{}, err
	}
	if parsed == nil {
		return time.Time{}, fmt.Errorf("empty date string")
	}
	return *parsed, nil
}

func (u *CertificationUseCase) ListForAdmin(ctx context.Context, req *dto.AdminListCertificationsRequest) (*dto.AdminCertificationListResponse, error) {
	filter := domain.CertificationListFilter{
		Limit:  req.Limit,
		Offset: (req.Page - 1) * req.Limit,
	}
	if req.Status != "" {
		filter.Status = &req.Status
	}
	if req.Keyword != "" {
		filter.Keyword = &req.Keyword
	}

	items, err := u.certRepo.ListForAdmin(ctx, filter)
	if err != nil {
		return nil, err
	}

	total, err := u.certRepo.CountForAdmin(ctx, filter)
	if err != nil {
		return nil, err
	}

	stats, err := u.certRepo.GetStatsForAdmin(ctx)
	if err != nil {
		return nil, err
	}

	dtoItems := make([]dto.AdminCertificationItem, 0, len(items))
	for _, item := range items {
		dtoItems = append(dtoItems, dto.AdminCertificationItem{
			ID:            item.ID,
			AccountID:     item.AccountID,
			ApplicationNo: item.ApplicationNo,
			Status:        item.Status,
			SellerType:    item.SellerType,
			RealName:      item.RealName,
			StoreName:     item.StoreName,
			CurrentStep:   item.CurrentStep,
			TotalSteps:    item.TotalSteps,
			SubmittedAt:   item.SubmittedAt,
			RejectedAt:    item.RejectedAt,
			CreatedAt:     item.CreatedAt,
			UpdatedAt:     item.UpdatedAt,

			IdCardFrontAssetId: item.IDCardFrontAssetID,
			IdCardBackAssetId:  item.IDCardBackAssetID,
			BankBookAssetId:    item.BankBookAssetID,
			DocumentAssetIds:   item.DocumentAssetIDs,
		})
	}

	return &dto.AdminCertificationListResponse{
		List:  dtoItems,
		Total: total,
		Page:  req.Page,
		Limit: req.Limit,
		Stats: &dto.CertificationStats{
			Total:     stats.Total,
			Submitted: stats.Submitted,
			Approved:  stats.Approved,
			Rejected:  stats.Rejected,
		},
	}, nil
}

func totalStepsForSellerType(sellerType string) int {
	if sellerType == "business" {
		return 9
	}
	return 8
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func marshalRequiredFields(fields []string) json.RawMessage {
	if len(fields) == 0 {
		return nil
	}
	b, err := json.Marshal(fields)
	if err != nil {
		return nil
	}
	return b
}

func unmarshalRequiredFields(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var fields []string
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil
	}
	return fields
}
