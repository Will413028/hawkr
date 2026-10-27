// Package kyc 是賣家的 KYC 認證：賣家填寫草稿、送出與取消，管理員審核（核准、退回、顯示全文），以及身分比對
// （ADR 2026-10-19-seller-kyc-payuni-consolidated）。申請以 store_id 為鍵；個資加密、以 blind index 比對；人工決定
// 與稽核紀錄在寫入的同一個 transaction，證件圖片的 media 引用由收斂 job 維護。
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

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/agreement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/payuni"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/auditreason"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// Deps 外部依賴；clock 由組裝點注入。Keys 與 MAC 必填（seller.New 檢查）：個資不能在沒有加密的情況下寫入。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.ReportingEnqueuer
	Media  media.Media
	// Exposure 是可曝光的公告：申請的狀態是 KYC 流程的輸入，每個改變它的寫入在 transaction 的最後呼叫。
	Exposure *exposure.Service
	Keys     *seal.Keyring
	MAC      *seal.MAC
	// Registry 是登記資料來源（公司型申請送件時的查詢快照）；必填：公開資料，沒有「沒有設定」的情形。
	Registry Registry
	Now      func() time.Time
	Logger   *slog.Logger
}

// Service 是 KYC 的 use case。
type Service struct {
	d Deps
	p protector
}

// New 建立服務。
func New(d Deps) *Service {
	if d.Registry == nil {
		panic("kyc: Registry is required")
	}
	return &Service{d: d, p: protector{keys: d.Keys, mac: d.MAC}}
}

func errVersionConflict() error { return apperr.VersionConflict("申請資料已被修改") }

// errAlreadyExists 是沒有帶版本（要建立）時店家已經有申請：與以 key 建立撞到既有的資源同一個形狀（ALREADY_EXISTS 加
// VERSION_CONFLICT，API 設計規範 §七），client 一樣顯示最新版本讓使用者對照。
func errAlreadyExists() error {
	return apperr.AlreadyExists("店家已經有申請，要修改請帶讀到的版本").WithReason(apperr.ReasonVersionConflict)
}

// actingStore 回傳帳號目前的店家（停權狀態在這一刻推導），只在它有能力 c 時成功。
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
// 授權由模組做（申請以帳號目前的店家為準，不接受別人的申請 ID，盤點 D22）；需要 CapStorefrontWrite。全部輸入先驗證、
// 個資加密之後才寫入；證件圖片在 transaction 外轉正（不建立引用），寫入的 transaction 內把被換掉的證件記成退場並排入引用的同步。必須在 transaction 外呼叫。
func (s *Service) SaveDraft(ctx context.Context, in model.KYCDraftInput) (model.KYCApplication, error) {
	if err := model.Positive(in.AccountID); err != nil {
		return model.KYCApplication{}, err
	}
	if in.ExpectedVersion < 0 {
		return model.KYCApplication{}, apperr.InvalidInputOn([]string{"ExpectedVersion"}, "版本不合法")
	}
	if _, ok := db.TxFrom(ctx); ok {
		return model.KYCApplication{}, apperr.FailedPrecondition("儲存草稿必須在 transaction 外開始")
	}
	if err := validateDocumentInput(in); err != nil {
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
	next, diff, err := s.p.applyDraft(base, in, create, now)
	if err != nil {
		return model.KYCApplication{}, err
	}
	uploaded, err := s.finalizeUploads(ctx, in)
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
		if current != nil {
			// 鎖住 PayUni 註冊列（鎖順序：店家、申請、註冊）：已核准的申請要回到草稿時再確認一次它仍是 PayUni 拒絕的狀態，
			// 註冊已有 MerID 時身分欄位不能改。
			if _, err := q.LockPayUniRegistration(ctx, locked.StoreID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if current.Status == string(model.KYCStatusApproved) {
				if err := payuni.CheckResubmit(ctx, q, *current); err != nil {
					return err
				}
			}
			if !create && current.ApplicationID == next.ApplicationID && current.Version == in.ExpectedVersion {
				if err := checkIdentityUnchanged(ctx, q, current.StoreID, diff); err != nil {
					return err
				}
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
		if err := s.p.syncHashes(ctx, q, current, row); err != nil {
			return err
		}
		if current != nil && current.Status != row.Status {
			// 已核准的申請回到草稿：KYC 流程不再是 APPROVED，店面頁隨之不公開，可曝光重新公告。
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
		docsAfter, retired, err := s.mergeDocuments(ctx, q, row, in, uploaded)
		if err != nil {
			return err
		}
		if current != nil {
			// 退回指出的問題：這次寫入改掉的標成已處理（同一個 transaction，比對寫入前後的列與證件）。
			if err := resolveIssues(ctx, q, row.ApplicationID, now, diff, docsBefore, docsAfter); err != nil {
				return err
			}
		}
		if len(uploaded) > 0 || len(retired) > 0 {
			if err := mediaref.Changed(ctx, q, s.d.Jobs, now, mediaref.CertificationRef(row.ApplicationID), retired...); err != nil {
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

// draftBase 回傳要在它上面合併的列：已有申請時是目前的那一筆（版本要符合、狀態要能編輯），沒有時是空白的新列。
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
	if err := payuni.CheckResubmit(ctx, q, current); err != nil {
		return row, false, err
	}
	return current, false, nil
}

// checkIdentityUnchanged 在註冊已有 MerID（PayUni 已收下）時，拒絕改動 bind_merchant 送出的身分欄位：update_merchant 只能改聯絡
// 方式與地址（PayUni 代理商專用 API 總覽 §8），改了我們的紀錄就和 PayUni 分歧。對照 payuni.bindRequest：Name／UBN（姓名或
// 公司名稱、身分證號或統編，所以賣家型態也鎖）、MCName／MCID（負責人姓名與身分證號）、MerEngName、MerURL（官網）、MerDesc
// 的後備（商品類型）。可改的是聯絡信箱、聯絡手機、公司電話（update_merchant 的聯絡欄位）與不送給 PayUni 的資料（生日、證件
// 資訊、銀行、公司地址、證件圖片）。哪些欄位變了由寫入路徑回報（draftDiff，比明文）。
func checkIdentityUnchanged(ctx context.Context, q *store.Queries, storeID int64, diff draftDiff) error {
	if !diff.identity() {
		return nil
	}
	locked, err := payuni.IdentityLocked(ctx, q, storeID)
	if err != nil {
		return err
	}
	if locked {
		return apperr.FailedPrecondition("商店已送交金流審核，身分資料不能修改；只能更正聯絡資料與地址，身分的錯誤要關閉商店後重新入駐")
	}
	return nil
}

// lockCurrent 鎖住店家目前的申請（呼叫方已鎖住店家列：鎖順序店家列先於申請列）；沒有申請時回 nil。
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

// Mine 回傳帳號目前店家的申請（遮蔽的檢視，帶證件圖片與最近一次退回的理由）；沒有申請時回 NotFound。
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

// sellerView 是賣家讀自己的申請：view 加上賣家看到的 PayUni 狀態與不能重新送出的原因（後台的檢視不帶，admin 的 PayUni 呈現在註冊）。
// 流程、PayUni 狀態與原因出自同一次 payuni.SellerViewOf。
func (s *Service) sellerView(ctx context.Context, q *store.Queries, row store.SellerKycApplication, accountID int64) (model.KYCApplication, error) {
	pv, err := payuni.SellerViewOf(ctx, q, row)
	if err != nil {
		return model.KYCApplication{}, err
	}
	out, _, err := s.viewWith(ctx, q, row, accountID, pv)
	if err != nil {
		return out, err
	}
	out.PayUni, out.ResubmitReasons = pv.PayUni, pv.Reasons
	return out, nil
}

// view 轉成遮蔽檢視並帶上證件圖片與最近一次退回。
func (s *Service) view(ctx context.Context, q *store.Queries, row store.SellerKycApplication, accountID int64) (model.KYCApplication, error) {
	out, _, err := s.viewWithReviews(ctx, q, row, accountID)
	return out, err
}

// viewWithReviews 是 view，另外回傳讀到的審核紀錄（含逐項問題；後台的檢視要它，不必再讀一次）。
func (s *Service) viewWithReviews(ctx context.Context, q *store.Queries, row store.SellerKycApplication, accountID int64) (model.KYCApplication, []model.KYCReview, error) {
	pv, err := payuni.SellerViewOf(ctx, q, row)
	if err != nil {
		return model.KYCApplication{}, nil, err
	}
	return s.viewWith(ctx, q, row, accountID, pv)
}

// viewWith 是 viewWithReviews，流程取自呼叫方已推導的 pv（同一份註冊）。
func (s *Service) viewWith(ctx context.Context, q *store.Queries, row store.SellerKycApplication, accountID int64, pv payuni.SellerView) (model.KYCApplication, []model.KYCReview, error) {
	out, err := s.p.toApplication(row, accountID)
	if err != nil {
		return out, nil, err
	}
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
	flow := pv.Flow
	out.Documents, out.LastRejection, out.Flow = toDocuments(docs), lastRejection(reviews, issues), &flow
	out.OutstandingIssues = outstandingIssues(reviews, issueRows)
	out.MissingFields = missingFields(row, docs)
	if out.MissingStoreFields, err = s.missingStoreFields(ctx, q, row.StoreID); err != nil {
		return out, nil, err
	}
	views := make([]model.KYCReview, len(reviews))
	for i, r := range reviews {
		views[i] = toReview(r, issues[r.ReviewID])
	}
	return out, views, nil
}

// Submit 送出申請：草稿，以及流程允許重新送出（CanResubmit）的申請——被退回的、已核准但 PayUni 拒絕而且還能重新綁定的——
// 都能直接送出，不必先回到草稿。必填欄位與三張證件齊全才能送出，並先比對身分（命中別的帳號生效中的處分時失敗，回泛用錯誤；權威的
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
		// 鎖住之後再檢查一次能力：讀取與拿到鎖之間，帳號可能已經被停權或店家已進入等待刪除（同 SaveDraft）。
		if _, err := s.actingStore(ctx, q, in.AccountID, model.CapStorefrontWrite); err != nil {
			return err
		}
		row, err := s.lockOwn(ctx, q, locked.StoreID, in.ExpectedVersion, model.KYCStatusDraft, model.KYCStatusSubmitted, model.KYCStatusRejected, model.KYCStatusApproved)
		if err != nil {
			return err
		}
		if _, err := q.LockPayUniRegistration(ctx, locked.StoreID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := payuni.CheckResubmit(ctx, q, row); err != nil {
			return err
		}
		docs, err := q.KYCDocuments(ctx, row.ApplicationID)
		if err != nil {
			return err
		}
		if err := complete(row, docs); err != nil {
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
		if blocked, _, err := s.match(ctx, q, row, locked.AccountID, now); err != nil {
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
		// 公司型的申請在同一個 transaction 內排入登記資料的查詢（外部呼叫在 job，不在 transaction 內）；個人型沒有登記資料。
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
		// 鎖住之後再檢查一次能力（理由同 Submit）。
		if _, err := s.actingStore(ctx, q, in.AccountID, model.CapStorefrontWrite); err != nil {
			return err
		}
		row, err := s.lockOwn(ctx, q, locked.StoreID, in.ExpectedVersion, model.KYCStatusDraft, model.KYCStatusSubmitted)
		if err != nil {
			return err
		}
		// 鎖順序：店家、申請、註冊。身分已鎖定時取消會讓店家以新申請繞過身分鎖。
		if _, err := q.LockPayUniRegistration(ctx, locked.StoreID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		identityLocked, err := payuni.IdentityLocked(ctx, q, locked.StoreID)
		if err != nil {
			return err
		}
		if identityLocked {
			return apperr.FailedPrecondition("金流服務商已收下您的商店資料，申請不能取消。身分資料有誤，請關閉商店後重新入駐。")
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
		if err := retireApplicationDocuments(ctx, q, s.d.Jobs, now, row.ApplicationID); err != nil {
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

// lockOwn 鎖住店家目前的申請並確認版本與狀態（呼叫方已鎖住店家列）。
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

// transition 以目前的狀態與版本為條件轉移狀態；0 列（狀態或版本在鎖內被改過）當成版本衝突。
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
	// 申請的狀態是「可公開曝光」的輸入（KYC 流程的 APPROVED 由它決定）：送出、取消、核准、退回都在同一個 transaction 內排入
	// website 店家頁的快取失效；可曝光的公告由各個呼叫方在 transaction 的最後做（Announce 是最後一個鎖，transition 之後
	// 核准還要鎖 PayUni 註冊列）。
	return updated, stint.InvalidateByID(ctx, q, s.d.Jobs, row.StoreID)
}

// complete 檢查送出必填的欄位與三張證件（DB 的 kyc_applications_complete_check 是保底）；一次列出全部缺的欄位。
type requirement struct {
	field, message string
	absent         func(row store.SellerKycApplication, have map[string]bool) bool
}

// requirements 是送出之前必填的項目（欄位路徑與缺少時的說明）；完整性的規則只有這一份：送出（complete）與申請檢視的
// missing_fields（missingFields）都由它推導。個人與企業各自專屬的項目依 model.FieldAppliesTo 過濾。
var requirements = []requirement{
	{"real_name", "請填寫姓名", func(r store.SellerKycApplication, _ map[string]bool) bool { return r.RealName == nil }},
	{"english_name", "請填寫商店英文名稱", func(r store.SellerKycApplication, _ map[string]bool) bool { return r.EnglishName == nil }},
	{"bank_name", "請填寫銀行名稱", func(r store.SellerKycApplication, _ map[string]bool) bool { return r.BankName == nil }},
	{"bank_code", "請填寫銀行代碼", func(r store.SellerKycApplication, _ map[string]bool) bool { return r.BankCode == nil }},
	{"bank_account", "請填寫銀行帳號", func(r store.SellerKycApplication, _ map[string]bool) bool { return r.BankAccountEnc == nil }},
	{"id_card_front", "請上傳身分證正面照片", func(_ store.SellerKycApplication, have map[string]bool) bool {
		return !have[string(model.SlotIDCardFront)]
	}},
	{"id_card_back", "請上傳身分證反面照片", func(_ store.SellerKycApplication, have map[string]bool) bool {
		return !have[string(model.SlotIDCardBack)]
	}},
	{"bank_book", "請上傳存摺封面照片", func(_ store.SellerKycApplication, have map[string]bool) bool {
		return !have[string(model.SlotBankBook)]
	}},
	{"individual.id_card_no", "請填寫身分證字號", func(r store.SellerKycApplication, _ map[string]bool) bool { return r.IDCardNoEnc == nil }},
	{"business.company_name", "請填寫公司名稱", func(r store.SellerKycApplication, _ map[string]bool) bool { return r.CompanyName == nil }},
	{"business.tax_id", "請填寫統一編號", func(r store.SellerKycApplication, _ map[string]bool) bool { return r.TaxID == nil }},
	{"business.rep_id_card_no", "請填寫負責人身分證字號", func(r store.SellerKycApplication, _ map[string]bool) bool { return r.RepIDCardNoEnc == nil }},
}

// missingFields 回傳還缺的欄位路徑（依 requirements 的順序）。
func missingFields(row store.SellerKycApplication, docs []store.SellerKycDocument) []string {
	have := map[string]bool{}
	for _, d := range docs {
		have[d.Slot] = true
	}
	out := []string{}
	for _, r := range requirements {
		if model.FieldAppliesTo(r.field, model.KYCSellerType(row.SellerType)) && r.absent(row, have) {
			out = append(out, r.field)
		}
	}
	return out
}

// complete 在送出時確認必填的項目與證件齊全：缺的每一項都是一個欄位違規（欄位是 KYCFieldNames 的路徑）。
func complete(row store.SellerKycApplication, docs []store.SellerKycDocument) error {
	missing := missingFields(row, docs)
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

// storeFieldLabels 是送出時缺少店家欄位的名稱。
var storeFieldLabels = map[string]string{
	model.StoreFieldAddress:       "店家地址",
	model.StoreFieldContactName:   "聯絡人姓名",
	model.StoreFieldContactMobile: "聯絡人手機",
	model.StoreFieldSupportEmail:  "客服電子信箱",
}

// storeIncomplete 是送出時店家的 PayUni 綁定資料還有缺：店家的狀態不在請求裡，所以是 failed_precondition（不是欄位違規）；
// 缺哪些由申請檢視的 missing_store_fields 取得。
func storeIncomplete(missing []string) error {
	labels := make([]string, 0, len(missing))
	for _, f := range missing {
		labels = append(labels, storeFieldLabels[f])
	}
	return apperr.FailedPrecondition("請先到店家設定補齊%s，再送出申請", strings.Join(labels, "、"))
}

// missingStoreFields 讀店家目前的資料，回傳送出之前還缺的店家欄位（model.MissingStoreFields）。
func (s *Service) missingStoreFields(ctx context.Context, q *store.Queries, storeID int64) ([]string, error) {
	row, err := q.StoreByID(ctx, storeID)
	if err != nil {
		return nil, model.Error(err)
	}
	return payuni.MissingStoreFields(stint.ToStore(row).Profile), nil
}

// reviewParams 組審核紀錄的寫入參數。admin 是管理員的 ID（賣家自己的動作是 nil）；text 是理由或 comment，已經由呼叫方依
// auditreason 正規化（空字串是沒有）；internalNote 同樣（只有退回有）。registrationCheckID 是核准或退回當下顯示的登記資料查詢（其他動作與沒有查詢時是 nil）。
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
