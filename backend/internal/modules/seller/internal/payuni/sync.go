package payuni

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"
)

// 一次 sync job 的結果種類。
type resultKind int

const (
	// resTransient 暫時性的失敗（傳輸錯誤、PayUni 內部的暫時性錯誤）：什麼都不寫，讓 River 退避重試。
	resTransient resultKind = iota
	// resBound 綁定成功（含 AGENT04017 認領成功）。
	resBound
	// resBindFailed 綁定失敗（永久）：FAILED 加錯誤碼與事件。
	resBindFailed
	// resSynced 聯絡欄位與地址同步成功。
	resSynced
	// resSyncRejected 同步被永久拒絕（或沒有設定）：記下碼，賣家再改一次才重送。
	resSyncRejected
)

type result struct {
	kind  resultKind
	code  string
	cause error
	// status 是認領時查到的 PayUni 狀態，結果的 transaction 內一併記成觀測並套用。
	status *pu.MerchantStatus
}

// claimed 是 job 開始時的快照：seq 是當時讀到的 intent_seq，結果的 transaction 以它記 synced_seq；之後有新的
// 改動時 intent_seq 會比它大，job 再跑一輪。
type claimed struct {
	reg   store.SellerPayuniRegistration
	seq   int64
	owner store.SellerStore
}

// Sync 是 sync job 的內容：把 PayUni 與目前的資料對齊。PENDING 的註冊向 PayUni 綁定（bind_merchant）；BOUND 而且有資料
// 沒送出的，送聯絡欄位與地址（update_merchant；PayUni 拒絕之後重新送出再核准的註冊也走這裡：MerID 已存在，再 bind 只會
// 得到 AGENT04017）。同一個店家同時只有一個 job 由 River 的唯一鍵保證（排隊中、執行中、等重試）。流程：
//
//  1. transaction：鎖住註冊列、確認有事要做、記下 intent_seq；
//  2. transaction 外：讀目前的資料（KYC 解密、店家地址、聯絡欄位）、呼叫 PayUni（有 deadline，不存請求本體）；
//  3. transaction：寫結果與事件（綁定成功、失敗、同步結果）。鎖順序與所有寫入者相同：店家列先、註冊列後。
//
// 暫時性的失敗（傳輸錯誤、PayUni 的暫時性錯誤碼）回傳錯誤，由 River 退避重試；final 為 true（最後一次嘗試）時改為記錄
// 結果：綁定記為 FAILED（PAYUNI_UNAVAILABLE），同步記為同步失敗，不再重試。永久的失敗（PayUni 以驗證類的碼拒絕、資料不
// 完整、沒有設定）直接記錄。again 為 true 表示結果寫入後 intent_seq 又前進了（期間有人改了資料）：呼叫方稍後再跑一輪。
// AGENT04017（MerID 已存在）先查詢狀態認領，所以結果不明（逾時）之後重送一律安全。
func (s *Service) Sync(ctx context.Context, storeID int64, final bool) (again bool, err error) {
	c, ok, err := s.claim(ctx, storeID)
	if err != nil || !ok {
		return false, err
	}
	bind := c.reg.BindStatus == string(model.BindPending)
	var res result
	if bind {
		res = s.bind(ctx, c)
	} else {
		res = s.update(ctx, c)
	}
	if res.kind == resTransient {
		if !final {
			return false, fmt.Errorf("seller：PayUni 暫時無法完成，稍後重試：%w", res.cause)
		}
		res = result{kind: resSyncRejected, code: model.ErrUnavailable, cause: res.cause}
		if bind {
			res.kind = resBindFailed
		}
	}
	return s.finish(ctx, c, res)
}

// claim 讀出這一輪要做的事；沒有事要做（沒有註冊列、已經失敗要管理員處理、沒有資料待送、店家已關閉）時 ok 為 false。
func (s *Service) claim(ctx context.Context, storeID int64) (out claimed, ok bool, err error) {
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		reg, err := q.LockPayUniRegistration(ctx, storeID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if reg.BindStatus == string(model.BindFailed) || reg.BindStatus == string(model.BindBound) && reg.SyncedSeq >= reg.IntentSeq {
			return nil
		}
		owner, err := q.StoreByID(ctx, storeID)
		if err != nil {
			return model.Error(err)
		}
		if owner.Status == string(model.Closed) {
			return nil
		}
		if reg.BindStatus == string(model.BindPending) {
			if reg, err = q.CountPayUniBindAttempt(ctx, store.CountPayUniBindAttemptParams{StoreID: storeID, Now: s.d.Now().UTC()}); err != nil {
				return err
			}
		}
		out, ok = claimed{reg: reg, seq: reg.IntentSeq, owner: owner}, true
		return nil
	})
	return out, ok, err
}

// bind 組出請求並呼叫 bind_merchant（transaction 外）。
func (s *Service) bind(ctx context.Context, c claimed) result {
	if s.d.Client == nil {
		return result{kind: resBindFailed, code: model.ErrNotConfigured}
	}
	req, missing, err := s.bindRequest(ctx, c)
	if err != nil {
		return result{kind: resTransient, cause: err}
	}
	if len(missing) > 0 {
		// 資料不完整：不補假資料送出（盤點 D15）。只記欄位名，不記值。
		s.d.Logger.WarnContext(ctx, "seller：PayUni 綁定的資料不完整", "store_id", c.reg.StoreID, "missing", strings.Join(missing, ","))
		return result{kind: resBindFailed, code: model.ErrIncompleteData}
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	err = s.d.Client.BindMerchant(callCtx, req)
	if err == nil {
		return result{kind: resBound}
	}
	apiErr, ok := pu.AsAPIError(err)
	switch {
	case !ok:
		// 傳輸錯誤原樣當成錯誤（盤點 D16），結果不明時重試安全：MerID 已存在時走認領。
		return result{kind: resTransient, cause: err}
	case model.PayUniAlreadyBound(apiErr.Code):
		return s.claimExisting(ctx, c)
	case model.PayUniRetryable(apiErr.Code):
		return result{kind: resTransient, cause: err}
	}
	s.d.Logger.WarnContext(ctx, "seller：PayUni 拒絕綁定", "store_id", c.reg.StoreID, "code", apiErr.Code)
	return result{kind: resBindFailed, code: apiErr.Code}
}

// claimExisting 是 AGENT04017（MerID 已存在）的自癒：查詢 PayUni 端的狀態並認領。商家層級的終止狀態（退件、關閉）無法再
// 綁定：記為失敗（MERCHANT_TERMINAL），要關店重新入駐。
func (s *Service) claimExisting(ctx context.Context, c claimed) result {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	st, err := s.d.Client.MerchantStatus(callCtx, c.reg.MerID)
	if err != nil {
		if apiErr, ok := pu.AsAPIError(err); ok && !model.PayUniRetryable(apiErr.Code) {
			return result{kind: resBindFailed, code: apiErr.Code}
		}
		return result{kind: resTransient, cause: err}
	}
	if model.TerminalMerStatus(st.MerStatus) {
		return result{kind: resBindFailed, code: model.ErrMerchantTerminal, status: &st}
	}
	return result{kind: resBound, status: &st}
}

// update 呼叫 update_merchant 送店家目前的聯絡資料與地址（transaction 外）。必填的店家資料有空的時候不呼叫 PayUni：記成同步失敗
// （INCOMPLETE_DATA，賣家補齊再改一次才重送），不把「PayUni 還是舊值」記成同步成功。
func (s *Service) update(ctx context.Context, c claimed) result {
	if s.d.Client == nil {
		return result{kind: resSyncRejected, code: model.ErrNotConfigured}
	}
	owner, err := store.New(s.d.DB.Conn(ctx)).StoreByID(ctx, c.reg.StoreID)
	if err != nil {
		return result{kind: resTransient, cause: err}
	}
	data := StoreDataOf(stint.ToStore(owner).Profile)
	if missing := data.Missing(); len(missing) > 0 {
		s.d.Logger.WarnContext(ctx, "seller：PayUni 同步的店家資料不完整", "store_id", c.reg.StoreID, "missing", strings.Join(missing, ","))
		return result{kind: resSyncRejected, code: model.ErrIncompleteData}
	}
	contactTel, err := s.d.KYC.CompanyPhone(ctx, c.reg.ApplicationID)
	if err != nil {
		return result{kind: resTransient, cause: err}
	}
	addr, post, _ := data.Address()
	req := pu.UpdateMerchantRequest{
		MerID: c.reg.MerID, ContactName: data.ContactName, ContactMobile: data.ContactMobile, MerEmail: data.SupportEmail,
		ContactTel: contactTel, MerTel: data.ContactPhone, MerAddr: addr, MerAddrPost: post,
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	err = s.d.Client.UpdateMerchant(callCtx, req)
	if err == nil {
		return result{kind: resSynced}
	}
	apiErr, ok := pu.AsAPIError(err)
	switch {
	case !ok, model.PayUniRetryable(apiErr.Code):
		return result{kind: resTransient, cause: err}
	}
	s.d.Logger.WarnContext(ctx, "seller：PayUni 拒絕更新聯絡資料", "store_id", c.reg.StoreID, "code", apiErr.Code)
	return result{kind: resSyncRejected, code: apiErr.Code}
}

// bindLocation 組出 bind_merchant 的地址、3 碼郵遞區號與商店網址，並列出缺的必要資料（欄位名；不補預設值）：店家要有地址與聯絡資料
// （StoreData.Missing），申請要有官網、或設定了店家頁的網站根網址。bindRequest 與「資料補齊了沒有」（requestSync）共用，完整性只有這一份定義。
func bindLocation(owner store.SellerStore, websiteURL, storefrontBaseURL string) (addr, post, merURL string, missing []string) {
	data := StoreDataOf(stint.ToStore(owner).Profile)
	addr, post, _ = data.Address()
	missing = data.Missing()
	merURL = websiteURL
	if merURL == "" && storefrontBaseURL != "" {
		merURL = strings.TrimRight(storefrontBaseURL, "/") + "/sellers/" + strconv.FormatInt(owner.StoreCode, 10)
	}
	if merURL == "" {
		missing = append(missing, "storefront_url")
	}
	return addr, post, merURL, missing
}

// bindRequest 在 transaction 外、執行當下讀取目前的資料組出 bind_merchant 的請求：KYC（解密）、店家（地址、店名、
// 介紹、地址、聯絡資料）。缺的必要資料（地址、聯絡資料、店家頁網址）回在 missing（欄位名），不補預設值。
func (s *Service) bindRequest(ctx context.Context, c claimed) (req pu.BindMerchantRequest, missing []string, err error) {
	kyc, err := s.d.KYC.BindingData(ctx, c.reg.ApplicationID)
	if err != nil {
		return req, nil, err
	}
	reg, err := store.New(s.d.DB.Conn(ctx)).PayUniRegistrationByStore(ctx, c.reg.StoreID)
	if err != nil {
		return req, nil, err
	}
	owner, err := store.New(s.d.DB.Conn(ctx)).StoreByID(ctx, c.reg.StoreID)
	if err != nil {
		return req, nil, err
	}
	contactTel, err := s.d.KYC.CompanyPhone(ctx, c.reg.ApplicationID)
	if err != nil {
		return req, nil, err
	}
	data := StoreDataOf(stint.ToStore(owner).Profile)
	addr, post, merURL, missing := bindLocation(owner, kyc.WebsiteURL, s.d.StorefrontBaseURL)
	req = pu.BindMerchantRequest{
		MerID: reg.MerID, MerType: merType, MerMCC: merMCC, GoodsRate: goodsRate, Payment: payment, DeliverTime: deliverTime,
		EscrowType: escrowType, MerUnitPrice: merUnitPrice, SellType: sellType,
		MerName: model.Deref(owner.StoreName), MerEngName: kyc.EnglishName, MerURL: merURL, MerAddr: addr, MerAddrPost: post,
		MerDesc: model.FirstNonEmpty(model.Deref(owner.Description), kyc.ProductType, model.Deref(owner.StoreName)),
		Email:   data.SupportEmail, MerEmail: data.SupportEmail, ContactName: data.ContactName,
		ContactMobile: data.ContactMobile, ContactTel: contactTel, MerTel: data.ContactPhone,
	}
	if kyc.SellerType == model.KYCCompany {
		// 企業：商店名稱登記在公司，統編是公司的，負責人另外帶（MC*、MBC*）。
		req.Name, req.UBN = kyc.CompanyName, kyc.TaxID
		req.MBCName, req.MBCTel, req.MBCMail = kyc.RealName, contactTel, data.SupportEmail
		req.MCName, req.MCID = kyc.RealName, kyc.IDNumber
	} else {
		req.Name, req.UBN = kyc.RealName, kyc.IDNumber
	}
	return req, missing, nil
}

// finish 在一個 transaction 內寫結果與事件（盤點 D7：失敗的狀態與它的事件同一個 transaction，不是只 warn）。ctx 逾時被
// 取消之後仍要能寫入，所以用自己的 deadline。鎖順序是店家列、再註冊列（與核准、重新綁定、賣家改資料相同）：先鎖註冊列再碰
// 店家（寫觀測列的外鍵會對店家列取 KEY SHARE），會和「先鎖店家列、再鎖註冊列」的寫入者互鎖。
func (s *Service) finish(ctx context.Context, c claimed, res result) (again bool, err error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.d.Now().UTC()
		if _, err := q.LockStore(ctx, c.reg.StoreID); err != nil {
			return model.Error(err)
		}
		locked, err := q.LockPayUniRegistration(ctx, c.reg.StoreID)
		if err != nil {
			return err
		}
		// 結果只套到自己讀到的 intent：註冊列已經記過比 c.seq 更新的結果（synced_seq > c.seq，例如開發種子重建了註冊列、
		// intent_seq 延續加一並把 synced_seq 記到新值），這個 job 是舊 intent 的在途 job，不寫入（寫了會把新列退回舊結果，
		// 或把重建的列記成綁定失敗）。賣家改資料只讓 intent_seq 前進、不動 synced_seq，所以正常的「呼叫期間有新資料」不受影響。
		if locked.SyncedSeq > c.seq {
			s.d.Logger.WarnContext(ctx, "seller：PayUni 的結果屬於舊的 intent，不寫入", "store_id", c.reg.StoreID, "job_seq", c.seq, "synced_seq", locked.SyncedSeq)
			return nil
		}
		var row store.SellerPayuniRegistration
		switch res.kind {
		case resTransient:
			return errors.New("seller：暫時性的失敗不寫入結果（Sync 不會把它交給 finish）")
		case resBound:
			if row, err = q.MarkPayUniBound(ctx, store.MarkPayUniBoundParams{StoreID: c.reg.StoreID, SyncedSeq: c.seq, Now: &now}); err != nil {
				return err
			}
			marker, err := q.InsertPayUniObservation(ctx, store.InsertPayUniObservationParams{
				ObservationID: uuid.NewV7(), StoreID: c.reg.StoreID, Source: string(model.SourceBind), ExecutedAt: now, Now: now,
			})
			if err != nil {
				return err
			}
			err = events.Publish(ctx, s.d.Events, model.PayUniForwarded{
				OccurredAt: now, EventID: marker.ObservationID, ApplicationID: c.reg.ApplicationID, StoreID: c.reg.StoreID, AccountID: c.owner.AccountID,
			})
			if err != nil {
				return err
			}
		case resBindFailed:
			code := res.code
			if row, err = q.MarkPayUniBindFailed(ctx, store.MarkPayUniBindFailedParams{StoreID: c.reg.StoreID, SyncedSeq: c.seq, ErrorCode: &code, Now: now}); err != nil {
				return err
			}
			err = events.Publish(ctx, s.d.Events, model.PayUniBindFailed{
				OccurredAt: now, EventID: uuid.NewV7(), ApplicationID: c.reg.ApplicationID, StoreID: c.reg.StoreID,
				AccountID: c.owner.AccountID, ErrorCode: code,
			})
			if err != nil {
				return err
			}
		case resSynced, resSyncRejected:
			var code *string
			if res.kind == resSyncRejected {
				code = &res.code
			}
			if row, err = q.MarkPayUniSynced(ctx, store.MarkPayUniSyncedParams{StoreID: c.reg.StoreID, SyncedSeq: c.seq, ErrorCode: code, Now: now}); err != nil {
				return err
			}
		}
		if res.status != nil {
			if _, err := s.record(ctx, q, c.reg.StoreID, fromStatus(*res.status, model.SourceBind, now)); err != nil {
				return err
			}
		}
		if res.kind == resBound || res.kind == resBindFailed {
			if err := s.exposureChanged(ctx, q, c.reg.StoreID); err != nil {
				return err
			}
		}
		again = row.IntentSeq > c.seq
		return nil
	})
	return again, err
}
