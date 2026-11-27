// Package kyc 是賣家的 KYC 認證：賣家填寫草稿、送出與取消，管理員審核（核准、退回、讀公司補充文件），以及統編的身分比對
// （ADR 2026-11-13-seller-platform-review-and-identity-matching「申請、草稿與審核」）。申請以 store_id 為鍵；平台不收身分證號、銀行帳號與證件影像（收款開通 ADR D22），統編以 blind index 比對；
// 人工決定與稽核紀錄在寫入的同一個 transaction，補充文件的 media 引用由收斂 job 維護。
package kyc

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/agreement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/auditreason"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Identity 是 KYC 需要的 identity 公開 API：帳號的聯絡信箱（送出時要有，核准時快照到收款帳戶）。
type Identity interface {
	ContactEmail(ctx context.Context, accountID int64) (identity.ContactEmail, bool, error)
}

// Deps 外部依賴；clock 由組裝點注入。MAC 必填（seller.New 檢查）：統編的 blind index 要有金鑰。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.ReportingEnqueuer
	Media  model.Media
	// Exposure 是可曝光的公告：申請的狀態是開通閘門（平台審核核准）的輸入，每個改變它的寫入在 transaction 的最後呼叫。
	Exposure *exposure.Service
	// Payment 是收款帳戶的擁有者（payment 模組）：核准的 transaction 內寫「建立中」的列並排入建立 job（收款開通 ADR D16、D28）。
	Payment  Payment
	Identity Identity
	MAC      *seal.MAC
	// Registry 是登記資料來源（公司型申請送件時的查詢快照）；必填：公開資料，沒有「沒有設定」的情形。
	Registry Registry
	Now      func() time.Time
	Logger   *slog.Logger
}

// Payment 是 payment 模組的公開 API 中核准用到的部分。RequestAccount 在呼叫方的 transaction 內只 INSERT 帳戶列（既有的列不改）並排入建立 job，
// 不鎖 payment 的既有列（全域鎖順序 payment 先於 seller，核准已鎖住店家與申請列）。
type Payment interface {
	Configured() bool
	RequestAccount(ctx context.Context, storeID int64, now time.Time) error
}

// Service 是 KYC 的 use case。
type Service struct {
	d Deps
	p protector
}

// New 建立服務。
func New(d Deps) *Service {
	if d.Registry == nil || d.Payment == nil || d.Identity == nil {
		panic("kyc: Registry, Payment and Identity are required")
	}
	return &Service{d: d, p: protector{mac: d.MAC}}
}

func (s *Service) contactEmail(ctx context.Context, accountID int64) (string, bool, error) {
	c, ok, err := s.d.Identity.ContactEmail(ctx, accountID)
	if err != nil || !ok {
		return "", false, err
	}
	return c.Address, true, nil
}

// OwnerContactEmail 回傳店家所屬帳號目前的聯絡信箱（與送審檢查同一個解析：contactEmail）；店家不存在或帳號沒有可用的信箱時 ok 是 false。
func (s *Service) OwnerContactEmail(ctx context.Context, storeID int64) (string, bool, error) {
	owner, err := store.New(s.d.DB.Conn(ctx)).StoreByID(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return s.contactEmail(ctx, owner.AccountID)
}

func (s *Service) requireContactEmail(ctx context.Context, accountID int64) (string, error) {
	email, ok, err := s.contactEmail(ctx, accountID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", apperr.FailedPrecondition("請先填寫聯絡信箱").WithReason(apperr.ReasonContactEmailRequired)
	}
	return email, nil
}

func errVersionConflict() error { return apperr.VersionConflict("申請資料已被修改") }

func errAlreadyExists() error {
	return apperr.AlreadyExists("店家已經有申請，要修改請帶讀到的版本").WithReason(apperr.ReasonVersionConflict)
}

func (s *Service) actingStore(ctx context.Context, q *store.Queries, accountID int64, c model.Capability) (store.SellerStore, error) {
	row, err := q.CurrentStore(ctx, accountID)
	if err != nil {
		return row, model.Error(err)
	}
	current, err := stint.Load(ctx, q, row, s.d.Now().UTC())
	if err != nil {
		return row, err
	}
	return row, model.Authorize(current, c)
}

// SaveDraft 部分更新草稿（店家還沒有申請時建立，ExpectedVersion 為 0）：被退回的申請在這裡回到 DRAFT 重新編輯。
// 授權由模組做（申請以帳號目前的店家為準，不接受別人的申請 ID，盤點 D22）；需要 CapStorefrontWrite。全部輸入先驗證才寫入；
// 補充文件在 transaction 外轉正（不建立引用），寫入的 transaction 內把被換掉的文件記成退場並排入引用的同步。必須在 transaction 外呼叫。
func (s *Service) SaveDraft(ctx context.Context, in model.KYCDraftInput) (model.KYCApplication, error) {
	if err := model.Positive(in.AccountID); err != nil {
		return model.KYCApplication{}, err
	}
	if in.ExpectedVersion < 0 {
		return model.KYCApplication{}, apperr.InvalidInputOn([]string{"ExpectedVersion"}, "版本不合法")
	}
	if err := validateDocumentInput(ctx, in); err != nil {
		return model.KYCApplication{}, err
	}
	now := s.d.Now().UTC()
	q := store.New(s.d.DB.Conn(ctx))
	storeRow, err := s.actingStore(ctx, q, in.AccountID, model.CapStorefrontWrite)
	if err != nil {
		return model.KYCApplication{}, err
	}
	base, create, err := s.draftBase(ctx, q, storeRow.StoreID, in.ExpectedVersion)
	if err != nil {
		return model.KYCApplication{}, err
	}
	next, diff, err := applyDraft(base, in, create, now)
	if err != nil {
		return model.KYCApplication{}, err
	}
	var out model.KYCApplication
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, err := stint.Lock(ctx, q, storeRow.StoreID)
		if err != nil {
			return err
		}
		if _, err := s.actingStore(ctx, q, in.AccountID, model.CapStorefrontWrite); err != nil {
			return err
		}
		current, err := s.lockCurrent(ctx, q, locked.StoreID)
		if err != nil {
			return err
		}
		if current != nil && !create && current.ApplicationID == next.ApplicationID && current.Version == in.ExpectedVersion {
			if err := requireEditable(ctx, *current); err != nil {
				return err
			}
		}
		var row store.SellerKycApplication
		switch {
		case create && current != nil:
			return errAlreadyExists()
		case !create && (current == nil || current.ApplicationID != next.ApplicationID || current.Version != in.ExpectedVersion):
			return errVersionConflict()
		case create:
			row, err = q.InsertKYCApplication(ctx, insertParams(next, now))
		default:
			row, err = q.UpdateKYCDraft(ctx, updateParams(next, in.ExpectedVersion, now))
			if errors.Is(err, pgx.ErrNoRows) {
				return errVersionConflict()
			}
		}
		if err != nil {
			return err
		}
		if err := s.p.syncHashes(ctx, q, current, row, now); err != nil {
			return err
		}
		if current != nil && current.Status != row.Status {
			if err := s.d.Exposure.Announce(ctx, q, now, locked.StoreID); err != nil {
				return err
			}
			if err := stint.Invalidate(ctx, s.d.Jobs, locked); err != nil {
				return err
			}
		}
		docsBefore, err := q.KYCDocuments(ctx, row.ApplicationID)
		if err != nil {
			return err
		}
		uploaded, err := s.confirmUploads(ctx, in)
		if err != nil {
			return err
		}
		docsAfter, retired, err := s.mergeDocuments(ctx, q, row, in, uploaded)
		if err != nil {
			return err
		}
		if err := markDeleted(ctx, s.d.Media, retired); err != nil {
			return err
		}
		if current != nil {
			if err := resolveIssues(ctx, q, row.ApplicationID, now, diff, docsBefore, docsAfter); err != nil {
				return err
			}
		}
		out, err = s.sellerView(ctx, q, row, locked.AccountID)
		return err
	})
	if err != nil {
		return model.KYCApplication{}, err
	}
	return out, nil
}

func (s *Service) draftBase(ctx context.Context, q *store.Queries, storeID, expectedVersion int64) (row store.SellerKycApplication, create bool, err error) {
	current, err := q.CurrentKYCApplication(ctx, storeID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if expectedVersion != 0 {
			return row, false, errVersionConflict()
		}
		return store.SellerKycApplication{ApplicationID: uuid.NewV7(), StoreID: storeID}, true, nil
	case err != nil:
		return row, false, err
	case expectedVersion == 0:
		return row, false, errAlreadyExists()
	case current.Version != expectedVersion:
		return row, false, errVersionConflict()
	}
	if err := requireEditable(ctx, current); err != nil {
		return row, false, err
	}
	return current, false, nil
}

func requireEditable(ctx context.Context, row store.SellerKycApplication) error {
	status := model.KYCStatus(row.Status)
	if model.CanEditKYC(status) {
		return nil
	}
	return model.ErrKYCNotEditable(status, uitext.LocaleFrom(ctx))
}

func (s *Service) lockCurrent(ctx context.Context, q *store.Queries, storeID int64) (*store.SellerKycApplication, error) {
	current, err := q.CurrentKYCApplication(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	locked, err := q.LockKYCApplication(ctx, current.ApplicationID)
	if err != nil {
		return nil, err
	}
	return &locked, nil
}

// Mine 回傳帳號目前店家的申請（帶補充文件與最近一次退回的理由）；沒有申請時回 NotFound。
func (s *Service) Mine(ctx context.Context, accountID int64) (model.KYCApplication, error) {
	if err := model.Positive(accountID); err != nil {
		return model.KYCApplication{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	storeRow, err := s.actingStore(ctx, q, accountID, model.CapAccountSelfRead)
	if err != nil {
		return model.KYCApplication{}, err
	}
	row, err := q.CurrentKYCApplication(ctx, storeRow.StoreID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.KYCApplication{}, apperr.NotFound("尚未建立申請")
	}
	if err != nil {
		return model.KYCApplication{}, err
	}
	return s.sellerView(ctx, q, row, storeRow.AccountID)
}

func (s *Service) sellerView(ctx context.Context, q *store.Queries, row store.SellerKycApplication, accountID int64) (model.KYCApplication, error) {
	return s.view(ctx, q, row, accountID)
}

func (s *Service) view(ctx context.Context, q *store.Queries, row store.SellerKycApplication, accountID int64) (model.KYCApplication, error) {
	out, _, err := s.viewWithReviews(ctx, q, row, accountID)
	return out, err
}

func (s *Service) viewWithReviews(ctx context.Context, q *store.Queries, row store.SellerKycApplication, accountID int64) (model.KYCApplication, []model.KYCReview, error) {
	out := toApplication(row, accountID)
	docs, err := q.KYCDocuments(ctx, row.ApplicationID)
	if err != nil {
		return out, nil, err
	}
	reviews, err := q.KYCReviews(ctx, row.ApplicationID)
	if err != nil {
		return out, nil, err
	}
	issueRows, err := q.KYCReviewIssues(ctx, row.ApplicationID)
	if err != nil {
		return out, nil, err
	}
	issues := issuesByReview(issueRows)
	out.Documents, out.LastRejection = toDocuments(docs), lastRejection(reviews, issues)
	out.OutstandingIssues = outstandingIssues(reviews, issueRows)
	out.MissingFields = missingFields(row)
	if out.MissingStoreFields, err = s.missingStoreFields(ctx, q, row.StoreID); err != nil {
		return out, nil, err
	}
	views := make([]model.KYCReview, len(reviews))
	for i, r := range reviews {
		views[i] = toReview(r, issues[r.ReviewID])
	}
	return out, views, nil
}

// Submit 送出申請：草稿與被退回的申請都能直接送出，不必先回到草稿；審核中與已核准的不能送出（model.CanEditKYC）。必填欄位齊全才能送出，並先比對身分（命中別的帳號生效中的處分時失敗，回泛用錯誤；權威的
// 比對在核准時）。同意的文件（AcceptedAgreements）先於一切業務檢查驗證（種類不對回 InvalidInput，版本不是現行版本回
// FailedPrecondition 的 AGREEMENT_OUTDATED）。寫入、同意紀錄、審核紀錄（SUBMIT）與 KYCSubmitted 在同一個 transaction。
func (s *Service) Submit(ctx context.Context, in model.SubmitKYCInput) (model.KYCApplication, error) {
	if err := model.Positive(in.AccountID); err != nil {
		return model.KYCApplication{}, err
	}
	if err := model.CheckAgreements(model.SubmitKYCAgreements, in.AcceptedAgreements); err != nil {
		return model.KYCApplication{}, err
	}
	var out model.KYCApplication
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		storeRow, err := s.actingStore(ctx, q, in.AccountID, model.CapStorefrontWrite)
		if err != nil {
			return err
		}
		locked, err := stint.Lock(ctx, q, storeRow.StoreID)
		if err != nil {
			return err
		}
		if _, err := s.actingStore(ctx, q, in.AccountID, model.CapStorefrontWrite); err != nil {
			return err
		}
		row, err := s.lockOwn(ctx, q, locked.StoreID, in.ExpectedVersion, model.KYCStatusDraft, model.KYCStatusSubmitted, model.KYCStatusRejected, model.KYCStatusApproved)
		if err != nil {
			return err
		}
		if err := requireEditable(ctx, row); err != nil {
			return err
		}
		if err := complete(row); err != nil {
			return err
		}
		if _, err := s.requireContactEmail(ctx, locked.AccountID); err != nil {
			return err
		}
		missingStore, err := s.missingStoreFields(ctx, q, row.StoreID)
		if err != nil {
			return err
		}
		if len(missingStore) > 0 {
			return storeIncomplete(missingStore)
		}
		now := s.d.Now().UTC()
		if blocked, err := s.match(ctx, q, row, locked.AccountID, now); err != nil {
			return err
		} else if blocked {
			return model.ErrIdentityNotEligible()
		}
		updated, err := s.transition(ctx, q, row, model.KYCStatusSubmitted, now)
		if err != nil {
			return err
		}
		review, err := q.InsertKYCReview(ctx, reviewParams(row.ApplicationID, model.ReviewSubmit, nil, "", "", nil, now))
		if err != nil {
			return err
		}
		if err := agreement.Record(ctx, q, row.StoreID, locked.AccountID, in.AcceptedAgreements, now, agreement.Subject{SubmissionReviewID: &review.ReviewID},
			agreement.Source{ClientIP: in.ClientIP, UserAgent: in.UserAgent}); err != nil {
			return err
		}
		if row.SellerType == string(model.KYCCompany) {
			if _, err := s.requestRegistryCheck(ctx, q, row.ApplicationID, model.RegistrationRequestSubmit, nil, now); err != nil {
				return err
			}
		}
		err = events.Publish(ctx, s.d.Events, model.KYCSubmitted{
			EventID: review.ReviewID, OccurredAt: now, ApplicationID: row.ApplicationID, StoreID: row.StoreID, AccountID: locked.AccountID,
		})
		if err != nil {
			return err
		}
		if err := s.d.Exposure.Announce(ctx, q, now, row.StoreID); err != nil {
			return err
		}
		out, err = s.sellerView(ctx, q, updated, locked.AccountID)
		return err
	})
	return out, err
}

// Cancel 取消草稿或審核中的申請（已核准的不能取消；被退回的申請就留著，不需要取消）。取消後申請的證件引用由收斂 job
// 全部釋放；店家可以重新建立申請。
func (s *Service) Cancel(ctx context.Context, in model.CancelKYCInput) (model.KYCApplication, error) {
	if err := model.Positive(in.AccountID); err != nil {
		return model.KYCApplication{}, err
	}
	reason, err := auditreason.Optional(in.Reason, "Reason")
	if err != nil {
		return model.KYCApplication{}, err
	}
	var out model.KYCApplication
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		storeRow, err := s.actingStore(ctx, q, in.AccountID, model.CapStorefrontWrite)
		if err != nil {
			return err
		}
		locked, err := stint.Lock(ctx, q, storeRow.StoreID)
		if err != nil {
			return err
		}
		if _, err := s.actingStore(ctx, q, in.AccountID, model.CapStorefrontWrite); err != nil {
			return err
		}
		row, err := s.lockOwn(ctx, q, locked.StoreID, in.ExpectedVersion, model.KYCStatusDraft, model.KYCStatusSubmitted)
		if err != nil {
			return err
		}
		now := s.d.Now().UTC()
		updated, err := s.transition(ctx, q, row, model.KYCStatusCancelled, now)
		if err != nil {
			return err
		}
		review, err := q.InsertKYCReview(ctx, reviewParams(row.ApplicationID, model.ReviewCancel, nil, reason, "", nil, now))
		if err != nil {
			return err
		}
		if err := retireApplicationDocuments(ctx, q, s.d.Media, row.ApplicationID); err != nil {
			return err
		}
		err = events.Publish(ctx, s.d.Events, model.KYCCancelled{
			EventID: review.ReviewID, OccurredAt: now, ApplicationID: row.ApplicationID, StoreID: row.StoreID, AccountID: locked.AccountID,
		})
		if err != nil {
			return err
		}
		if err := s.d.Exposure.Announce(ctx, q, now, row.StoreID); err != nil {
			return err
		}
		out, err = s.sellerView(ctx, q, updated, locked.AccountID)
		return err
	})
	return out, err
}

func (s *Service) lockOwn(ctx context.Context, q *store.Queries, storeID, expectedVersion int64, from ...model.KYCStatus) (store.SellerKycApplication, error) {
	current, err := s.lockCurrent(ctx, q, storeID)
	if err != nil {
		return store.SellerKycApplication{}, err
	}
	if current == nil {
		return store.SellerKycApplication{}, apperr.NotFound("尚未建立申請")
	}
	if current.Version != expectedVersion {
		return store.SellerKycApplication{}, errVersionConflict()
	}
	for _, status := range from {
		if model.KYCStatus(current.Status) == status {
			return *current, nil
		}
	}
	return store.SellerKycApplication{}, apperr.FailedPrecondition("申請目前的狀態不允許這個動作")
}

func (s *Service) transition(ctx context.Context, q *store.Queries, row store.SellerKycApplication, to model.KYCStatus, now time.Time) (store.SellerKycApplication, error) {
	params := store.TransitionKYCApplicationParams{
		ApplicationID: row.ApplicationID, ExpectedVersion: row.Version, FromStatus: row.Status, ToStatus: string(to), Now: now,
	}
	switch to {
	case model.KYCStatusSubmitted:
		params.SubmittedAt = &now
	case model.KYCStatusApproved, model.KYCStatusRejected:
		params.DecidedAt = &now
	case model.KYCStatusDraft, model.KYCStatusCancelled:
	}
	updated, err := q.TransitionKYCApplication(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return updated, errVersionConflict()
	}
	if err != nil {
		return updated, err
	}
	return updated, stint.InvalidateByID(ctx, q, s.d.Jobs, row.StoreID)
}

type requirement struct {
	field, message string
	absent         func(row store.SellerKycApplication) bool
}

var requirements = []requirement{
	{"business.company_name", "請填寫公司名稱", func(r store.SellerKycApplication) bool { return r.CompanyName == nil }},
	{"business.tax_id", "請填寫統一編號", func(r store.SellerKycApplication) bool { return r.TaxID == nil }},
}

func missingFields(row store.SellerKycApplication) []string {
	out := []string{}
	for _, r := range requirements {
		if model.FieldAppliesTo(r.field, model.KYCSellerType(row.SellerType)) && r.absent(row) {
			out = append(out, r.field)
		}
	}
	return out
}

func complete(row store.SellerKycApplication) error {
	missing := missingFields(row)
	if len(missing) == 0 {
		return nil
	}
	violations := make([]apperr.FieldViolation, 0, len(missing))
	for _, field := range missing {
		i := slices.IndexFunc(requirements, func(r requirement) bool { return r.field == field })
		violations = append(violations, apperr.FieldViolation{Field: field, Message: requirements[i].message})
	}
	return apperr.InvalidInputFields(violations, "申請資料不完整")
}

var storeFieldLabels = map[string]string{
	model.StoreFieldAddress:      "店家地址",
	model.StoreFieldSupportEmail: "客服電子信箱",
}

func storeIncomplete(missing []string) error {
	labels := make([]string, 0, len(missing))
	for _, f := range missing {
		labels = append(labels, storeFieldLabels[f])
	}
	return apperr.FailedPrecondition("請先到店家設定補齊%s，再送出申請", strings.Join(labels, "、"))
}

func (s *Service) missingStoreFields(ctx context.Context, q *store.Queries, storeID int64) ([]string, error) {
	row, err := q.StoreByID(ctx, storeID)
	if err != nil {
		return nil, model.Error(err)
	}
	return model.MissingStoreFields(stint.ToStore(row).Profile), nil
}

func reviewParams(applicationID uuid.UUID, action model.ReviewAction, admin *int64, text, internalNote string, registrationCheckID *uuid.UUID, now time.Time) store.InsertKYCReviewParams {
	p := store.InsertKYCReviewParams{
		ReviewID: uuid.NewV7(), ApplicationID: applicationID, Action: string(action), ReviewerAdminID: admin,
		RegistrationCheckID: registrationCheckID, Now: now,
	}
	if text != "" {
		p.Reason = &text
	}
	if internalNote != "" {
		p.InternalNote = &internalNote
	}
	return p
}
