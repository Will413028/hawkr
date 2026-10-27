package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// CorrectionIntake 是端點 11（B06）的**組裝層**：把散在五個地方的事實湊齊，
// 交給 Correction 執行狀態機。
//
// 🔴 為什麼另開一層而不是把五個依賴掛進 Correction：
// Correction 的職責很窄——「吃事實 ＋ 吃改好的資料，執行分流後的那條邊」，
// 它的每一支測試都能用純 fake 跑完。把 document 讀取、order 快照、provider 憑證、
// draft 建行全掛進去，那句話就不成立了，而它的測試會變成要先擺好五個 repo 才跑得動。
//
// 「湊齊事實」與「執行狀態機」是兩件事，這一層只做前者。

// CorrectionIntakeDeps 是組裝需要的五個讀取面。
//
// 刻意逐一列出而不是收一個大 Repository：介面越窄，「這一層會碰哪些東西」在型別上
// 就越說得清楚（同 HealthProbe／TrackLister 的理由）。
type CorrectionIntakeDeps struct {
	Documents  domain.DocumentRepository
	Operations domain.OperationRepository
	Accounts   domain.ProviderAccountRepository
	Tracks     TrackLister
	// Corrections 供序列閘門判「這條鏈上有沒有進行中的更正」（#3087）。
	// nil 容忍：既有測試與非 R04 路徑不需要它。
	Corrections domain.CorrectionRequestRepository
	// Preferences 是 R03 推導 carrier_selection_mode／buyer_invoice_preference_id
	// 所需的常用發票資料（#3085）。
	//
	// 🔴 少了它，ReviseSnapshotInput 的四個欄位只能留空，而 order-service 驗
	// invoice_kind 必填 ⇒ R03 一路回 400（包成 500 給買家）。
	// 歸屬驗證在 SQL 述詞裡（identityFromPreference 的註解：Tier A），
	// 這一層不得自己補 `if pref.BuyerID != buyerID`。
	Preferences domain.BuyerPreferenceRepository
}

// CorrectionIntake 組裝並委派。
type CorrectionIntake struct {
	deps       CorrectionIntakeDeps
	correction *Correction
}

// NewCorrectionIntake 建組裝層。
func NewCorrectionIntake(deps CorrectionIntakeDeps, correction *Correction) *CorrectionIntake {
	return &CorrectionIntake{deps: deps, correction: correction}
}

// SubmitCorrectionRequest 是端點 11 的原始輸入（handler 只做 proto→這個型別的轉換）。
type SubmitCorrectionRequest struct {
	BuyerID                 int64
	OrderNo                 string
	ExpectedSnapshotVersion int32
	// NewInvoice 是買家改後的發票資料。沿用 checkout 的型別——那是本服務唯一一份
	// 買家發票輸入的形狀，另立一份會讓兩邊的驗證規則漂移。
	NewInvoice CheckoutInvoiceSpec
	Reason     string
	// IdempotencyKey 一路傳到 ORD.ReviseOrderInvoiceSnapshotInternal 當冪等鍵。
	IdempotencyKey string
}

// SubmitCorrectionOutput 是端點 11 的回應素材。
//
// 🔴 handling_mode 與 status 的值域**零交集**（proto 註解逐字）：前者是「選定的做法」、
// 後者是「做完了沒」。放同一格會讓前端分不出「要作廢重開」與「已作廢重開完成」。
type SubmitCorrectionOutput struct {
	CorrectionID int64
	Route        CorrectionRoute
	Status       string
	HandlingMode string
	RejectReason string
}

// handling_mode 的兩個值（chk_invcorr_mode）與 correction status 的三個終態。
//
// ⚠️ **這些值沒有 CodeMeta enum**，只活在 DB CHECK 與這裡——與
// InvoiceCorrectionRejectReason 在本次補進碼表之前的處境相同。沒有一併補是因為
// 它們不對前端可見於**選擇面**（前端讀到的是後端決定的結果，不會送上來）。
// 真要補時走 §8.5 的流程，不要在別處再加一份字面值。
const (
	handlingModeUpdateSnapshot = "UPDATE_SNAPSHOT"
	handlingModeVoidAndReissue = "VOID_AND_REISSUE"

	correctionStatusRejected           = "REJECTED"
	correctionStatusUpdatedBeforeIssue = "UPDATED_BEFORE_ISSUE"
	correctionStatusVoidReissue        = "VOID_REISSUE_IN_PROGRESS"
)

// Submit 是端點 11 的入口：組裝 → 委派 → 折成回應素材。
func (u *CorrectionIntake) Submit(ctx context.Context, req SubmitCorrectionRequest) (SubmitCorrectionOutput, error) {
	// ① document（授權讀取：sourceRef ＋ buyerID 兩段述詞在 SQL 內）
	//
	// 🔴 **一定要取「現行的那一份」，不能用 GetForBuyer**（2026-09-02 dev 實測）：
	// 一張訂單在 VOID_AND_REISSUE 之後有多份文件（舊的 SUPERSEDED、新的帶
	// original_document_id）。GetForBuyer 是 `:one` 且**沒有 ORDER BY 也沒有 LIMIT**
	// ⇒ 命中兩列時 pgx 取任意一列而不報錯 —— 實測它取到的是**作廢掉的原票**。
	//
	// 後果不是「顯示了舊發票」而是**買家再也無法更正重開後的發票**：原票是
	// SUPERSEDED（真終態）⇒ decideCorrectionRoute 落到終態婉拒 ⇒ 回
	// PAST_VOID_DEADLINE。這正是 #3010 描述的下游危害，只是根因是另一個
	// ——#3010 的欄位修好之後症狀仍在，換這一支才真的消失。
	//
	// ListOrderDocumentsForBuyer 的排序是「內容軸 → 實體軸 → id」全部 DESC
	// ⇒ 第一筆就是最新一次開立的那一份，且授權述詞仍在 SQL 內（Tier A 不變）。
	docs, err := u.deps.Documents.ListOrderDocumentsForBuyer(ctx, req.OrderNo, req.BuyerID)
	if err != nil {
		return SubmitCorrectionOutput{}, err
	}
	if len(docs) == 0 {
		// 查無列與越權共用同一個出口（§8.1.4 Tier A）——與 GetForBuyer 的行為一致。
		return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(
			bizcode.InvoiceDocumentNotFound, "文件不存在")
	}
	doc := docs[0]

	// 🔴 **作廢重開進行中時，這條鏈上的「當前那一份」不是排序最前的那一份**（#3087）。
	//
	// R04 一旦啟動就會**立刻**建出替代票（PENDING）。它的內容軸／實體軸都比原票新
	// ⇒ 排序把它排到第一，而序列閘門要看的是**正在被作廢的原票**。後果連鎖三層：
	//   ① 閘門看替代票 ⇒ 那張票上沒有進行中的 operation ⇒ 閘門不成立（**被繞過**）
	//   ② 路由看替代票 ⇒ PENDING 不在 {ISSUING, VOIDING} ⇒ 不走 in-progress 分支
	//   ③ 可作廢期限算替代票 ⇒ 回一個與使用者情境無關的 PAST_VOID_DEADLINE，
	//      而那張票的期限其實還在一週後 —— 買家會以為是自己太晚提出
	//
	// 序列閘門是「同一張發票同時只有一條更正在飛」的保證，繞過它 ⇒
	// 兩條更正可能同時對同一份憑證動作。
	//
	// ⚠️ 修在這裡而不是在路由層加一格狀態：再加狀態只會讓下一種中間態
	// （例如 ISSUE_FAILED 的替代票）重演同一個形狀。判準是**鏈上有沒有
	// 進行中的 correction**，那與替代票當下停在哪一格無關。
	if doc.OriginalDocumentID != nil && u.deps.Corrections != nil {
		_, err := u.deps.Corrections.GetOpenByDocument(ctx, *doc.OriginalDocumentID)
		switch {
		case err == nil:
			return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(
				bizcode.InvoiceOperationInProgress,
				"這張發票的上一次修改還在處理中，請等處理完成後再提出")
		case !isBizCode(err, bizcode.InvoiceCorrectionNotAllowed):
			return SubmitCorrectionOutput{}, err
		}
	}

	// 🔴 收據不走更正鏈路（#3026 收尾稽查發現，2026-09-02）。
	//
	// 這條鏈路整條是**發票語意**：`correction.go` 的 `buildReissueDraft` 寫死
	// `kind := B2C`（依買家統編切 B2B），而 R04 的執行面是「AMEGO 作廢 → 重開」。
	// 收據沒有 `provider_invoice_no`、不打 AMEGO ⇒ 一路走下去會在 provider 那一段才炸，
	// 而買家早就按下送出了。
	//
	// 設計書 §7.2 逐字：`PLATFORM_RECEIPT` ⇒「收據不是統一發票、沒有折讓單這種東西，
	// **只重開／註記收據**」—— 那是**另一條尚未實作的機制**，不是這一條。
	//
	// ⚠️ 為什麼是 fail-closed 而不是「先擋 UI 就好」：`canRequestCorrection` 只是把按鈕
	// 藏起來，而這支端點是公開的寫入面。§17.6 的收斂點原則要求防線放在真正的入口，
	// 不能賭「使用者只會從我們畫的按鈕進來」。
	if doc.DocumentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(
			bizcode.InvoiceCorrectionNotAllowed,
			"平台代製收據不適用發票更正流程，請聯繫客服")
	}

	// ② 這張憑證上待送出的那一筆 operation。
	//    🔴 「非終態」的判定用**正面列舉**而不是排除法：新增一個終態值時，
	//    排除法會靜默把它算成「還在進行中」，而正面列舉會漏掉它 —— 漏掉的後果是
	//    R03 走不成改走 R04（保守），反過來則是對一張已送出的發票改快照（危險）。
	ops, err := u.deps.Operations.ListByDocument(ctx, doc.ID)
	if err != nil {
		return SubmitCorrectionOutput{}, fmt.Errorf("invoice: list operations: %w", err)
	}
	var openOpID int64
	for _, op := range ops {
		if op.Status == codemeta.InvoiceOperationStatusReady.Value ||
			op.Status == codemeta.InvoiceOperationStatusRetryScheduled.Value {
			openOpID = op.ID
			break
		}
	}

	// ③ provider 帳戶（狀態供 ProviderAvailable，憑證供字軌探測）
	account, err := u.deps.Accounts.GetByIssuer(ctx, doc.IssuerID, derefString(doc.Provider))
	if err != nil {
		return SubmitCorrectionOutput{}, fmt.Errorf("invoice: get provider account: %w", err)
	}

	// ④ 既有明細。R03 要「重建 lines」（先刪後建），而**身分類更正的明細不會變**：
	//    tax_category／tax_rate_bps 來自商品，description／quantity／金額來自訂單，
	//    沒有一欄依賴買家身分。所以從既有列重建即可，不必回頭問 order-service。
	//
	//    🔴 那為什麼還要刪了再建？因為 DeleteInvoiceDocumentLines 的述詞限定
	//    PENDING／AWAITING_MANUAL——它同時是「這份文件還沒送出去」的斷言。
	//    跳過它等於少了一道檢查，而那道檢查擋的是「對已開立的發票改明細」＝偽造。
	existing, err := u.deps.Documents.ListLines(ctx, doc.ID)
	if err != nil {
		return SubmitCorrectionOutput{}, fmt.Errorf("invoice: list document lines: %w", err)
	}
	revisedLines := make([]domain.NewDocumentLine, 0, len(existing))
	for _, l := range existing {
		revisedLines = append(revisedLines, domain.NewDocumentLine{
			LineSeq: l.LineSeq, LineType: l.LineType,
			OrderItemID: int64OrZero(l.OrderItemID), SkuID: int64OrZero(l.SkuID),
			Description: l.Description, Unit: derefString(l.Unit),
			Quantity: l.Quantity, UnitPrice: l.UnitPrice, NetAmount: l.NetAmount,
			TaxCategory: l.TaxCategory, TaxRateBps: l.TaxRateBps,
			Remark:            derefString(l.Remark),
			AllocatedDiscount: l.AllocatedDiscount, AllowanceableBase: l.AllowanceableBase,
		})
	}

	// ⑤ 六項事實。憑證用 WithDecryptedCredential 取，且**只在已 ISSUED 時真的會被用到**
	//    （GatherCorrectionFacts 內部判斷）——這裡無條件取是因為解密是本地操作、
	//    比多一次分支便宜，而且它不外流：cred 只進 GatherCorrectionFacts。
	var cf CorrectionFacts
	err = u.deps.Accounts.WithDecryptedCredential(ctx, account.ID, func(taxID, appKey string) error {
		var gErr error
		cf, gErr = u.correction.GatherCorrectionFacts(ctx, u.deps.Tracks, CorrectionFactsInput{
			Document:        doc,
			OpenOperationID: openOpID,
			ProviderStatus:  account.Status,
			Credential:      amego.Credential{IssuerID: doc.IssuerID, Invoice: taxID, AppKey: appKey},
		})
		return gErr
	})
	if err != nil {
		return SubmitCorrectionOutput{}, err
	}
	// 載具格式由買家輸入決定，在這一層判（那是唯一有原始輸入的地方）。
	cf.CarrierValid = carrierShapeValid(req.NewInvoice)

	// ⑤-b 解析買家的發票身分（#3085）。**與結帳共用同一份推導**——
	// 另寫一份的症狀是「更正之後買家原本選的載具消失了」，而且不會報錯。
	//
	// 🔴 拒絕不是錯誤：identityFromPreference 對「這筆常用資料不是你的」與
	// 「不存在」刻意同碼同訊息（Tier A，§8.1.4(b)），這一層照樣不得分辨。
	id, rej, err := resolveInvoiceIdentity(ctx, u.deps.Preferences, req.BuyerID, req.NewInvoice)
	if err != nil {
		return SubmitCorrectionOutput{}, err
	}
	if !rej.ok() {
		return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(rej.Code, rej.Message)
	}
	// 🔴 來自常用發票資料的載具，明文要在這裡取出來（resolveInvoiceIdentity 不解密）。
	// 少了這一步，revisedCarrierFromIdentity 只拿得到類型、拿不到號碼，而
	// chk_invdoc_carrier_pair 是雙向等式 ⇒ 整條更正路徑 23514 回 500。
	// 自動線走的是 client/order 的 fromPreference ＋ OpenCarrierValue，同一個機制。
	if err := u.openCarrierValueInto(ctx, req.BuyerID, &id); err != nil {
		return SubmitCorrectionOutput{}, err
	}
	// 🔴 **逐欄形狀驗證**（#3089 B 組 A-3）：在此之前買家的輸入完全沒被驗過形狀，
	// 一路寫進 invoice_documents 交給 chk_invdoc_taxid_shape 擋 ⇒ buyerTaxId="1234567"
	// 或 ""（清掉統編）都是 HTTP 500。
	//
	// 走 validateIdentityShape 而不是另寫一份：req.NewInvoice 就是結帳那個
	// CheckoutInvoiceSpec（本檔開頭的註解「沿用 checkout 的型別——那是本服務唯一一份」），
	// 而**驗證函式當時沒有跟著沿用**。兩份規則必然漂移。
	if code := validateIdentityShape(id); code != 0 {
		return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(code, bizcode.Message(code))
	}
	// 🔴 抬頭長度要**涵蓋所有分支**（#3089 稽核 A-5）。
	// validateIdentityShape 只在 TAX_ID 那一格驗 BuyerName（1–60），而「無載具」
	// 那一格不驗——可是 recipient_name 是 varchar(60) NOT NULL，
	// 超長就是 22001 ⇒ 整個交易 aborted ⇒ HTTP 500。
	// 這裡不截斷：發票抬頭被默默截短，開出去的是一張抬頭不完整的憑證，
	// 而買家正在填表、告訴他比替他決定好。
	if utf8.RuneCountInString(req.NewInvoice.BuyerName) > buyerNameMaxRunes {
		return SubmitCorrectionOutput{}, pkgerrors.NewBusiness(
			bizcode.InvoiceCarrierInvalid, "發票抬頭不得超過 60 字")
	}

	// ⑤-c 依新的買受人重算 document_kind、金額四欄與法定期限（#3089 B 組 A-3）。
	//
	// 🔴 R04（作廢重開）2026-09-01 已因**同一個症狀**修好（見 buildReissueDocument 的
	// 註解：「原樣複製的症狀是建列當下 23514」），R03 沒有回灌。
	// 買家補打統編 ⇒ B2C 變 B2B，而 chk_invdoc_b2c 要求 B2C 的 recipient_tax_id
	// 恆為 '0000000000' ⇒ 就地 UPDATE 當場 23514。**那是端點 11 的主要使用情境。**
	revisedKind := invoiceKindOf(id.BuyerTaxID)
	revisedTaxID := recipientTaxID(revisedKind, id.BuyerTaxID)
	revisedCarrierType, revisedCarrierValue, revisedDonationCode := revisedCarrierFromIdentity(revisedKind, id)
	revisedHeader := domain.BuildHeader(reissueHeaderLines(revisedLines), revisedTaxID)
	// 期限跟著 kind 走（B2C 2 天／B2B 7 天）。基準日與建立當下同一份算式
	// （domain.InvoiceDateTaipei(paid_at)）——doc.PaidAt 為 nil 時算不出基準，
	// 留 nil 不動原值：那比用 now 硬算一個新的誠實（now 會把期限往後推）。
	var revisedDeadline *time.Time
	if doc.PaidAt != nil {
		if d, ok := domain.StatutoryDeadline(domain.InvoiceDateTaipei(*doc.PaidAt), revisedKind); ok {
			revisedDeadline = &d
		}
	}

	// ⑥ 委派。
	out, err := u.correction.Submit(ctx, SubmitCorrectionInput{
		DocumentID:              doc.ID,
		BuyerID:                 req.BuyerID,
		ExpectedSnapshotVersion: req.ExpectedSnapshotVersion,
		BeforePayload:           maskedCorrectionPayloadFromDoc(doc),
		AfterPayload:            maskedCorrectionPayloadFromSpec(req.NewInvoice),
		OperationID:             openOpID,
		IssueAttemptSeq:         doc.IssueAttemptSeq,
		OrderNo:                 req.OrderNo,
		Facts:                   cf,
		RevisedLines:            revisedLines,
		Revised: domain.ReviseDocumentSnapshot{
			ExpectedVersion: doc.Version,
			RecipientType:   doc.RecipientType,
			// 🔴 **收件人身分沿用原文件，不從更正請求取**：更正改的是「發票上寫什麼」
			// （抬頭、統編、通知信箱），不是「這張發票是誰的」。買家還是同一個人。
			//
			// 少了這一行，兩條路都會壞而且完全靜默（2026-09-01 dev 實測）：
			//   R04 作廢重開 ⇒ 新文件 recipient_ref_id 為 NULL ⇒ 買家在 B05／B09
			//     **看不到取代自己那張的新發票**，只看得到已 SUPERSEDED 的舊的。
			//   R03 開立前直接改 ⇒ ReviseInvoiceDocumentSnapshot 是就地 UPDATE，
			//     會把原本有值的 recipient_ref_id **抹成 NULL**。
			// 兩者都不會報錯：欄位可為 NULL，DB 約束不擋，測試餵什麼就是什麼。
			RecipientRefID: doc.RecipientRefID,
			RecipientName:  req.NewInvoice.BuyerName,
			// 🔴 落庫值走 recipientTaxID() 正規化，不是原樣寫入買家輸入：
			// B2C 的 recipient_tax_id 恆為 '0000000000'（chk_invdoc_b2c 雙向守著），
			// 而買家清掉統編時輸入是空字串 —— 原樣寫入違反 chk_invdoc_taxid_shape。
			// 自動線一直都走這支 helper，R03／R04 是唯二沒走的建/改路徑。
			RecipientTaxID: revisedTaxID,
			RecipientEmail: req.NewInvoice.Email,

			// 🔴 載具／捐贈四欄（2026-09-05 第 6 輪稽核）。在此之前**整組從未賦值**
			// ⇒ ReviseInvoiceDocumentSnapshot 的 `SET carrier_* = narg(...)`（沒有
			// 「保持原值」的選項）會把它們一併抹成 NULL，而 buildReissueDocument 也照抄
			// 空值 ⇒ **R03 就地改與 R04 作廢重開都會弄丟買家的載具與捐贈碼**。
			//
			// 症狀比「載具消失」更糟：carrier_type 由 revised.CarrierType 帶著、
			// 顯隱碼卻是空的 ⇒ 送 AMEGO 是「有載具類型、號碼皆空」＝ 3040132
			// 載具號碼不存在 ⇒ **重開票直接開不出來**。
			//
			// ⚠️ 本檔 ⑤-b 的註解早就寫著「與結帳共用同一份推導——另寫一份的症狀是
			// 『更正之後買家原本選的載具消失了』」，而 `id` 也真的解出來了
			// ——只是解完只用了 `id.SelectionMode`（order-service 那一側），
			// invoice_documents 這一側四欄全空。**解析做了、值沒接上。**
			//
			// 映射逐字照抄 checkout.go 的 draftFromIdentity：id.CarrierType 是
			// CodeMeta 的 Value，落庫與送 AMEGO 要的是原生詞彙（3J0002／CQ0001）。
			CarrierType:  revisedCarrierType,
			CarrierID1:   revisedCarrierValue,
			CarrierID2:   revisedCarrierValue, // 顯碼＝隱碼，理由見 normalizeBuyerIdentity 的紅字
			DonationCode: revisedDonationCode,

			DocumentKind:       revisedKind,
			SalesAmount:        revisedHeader.SalesAmount,
			FreeTaxSalesAmount: revisedHeader.FreeTaxSalesAmount,
			ZeroTaxSalesAmount: revisedHeader.ZeroTaxSalesAmount,
			TaxAmount:          revisedHeader.TaxAmount,
			TotalAmount:        revisedHeader.TotalAmount,
			StatutoryDeadline:  revisedDeadline,
		},
		OrderRevision: domain.ReviseSnapshotInput{
			OrderNo:                 req.OrderNo,
			ExpectedSnapshotVersion: int64(req.ExpectedSnapshotVersion),
			// #3085：以下四欄在 2026-09-04 之前**從未賦值**。
			// order-service 驗 invoice_kind 必填 ⇒ R03「開立前直接改」從上線起
			// 100% 失敗（invoice_correction_requests 全表沒有任何一筆到過
			// UPDATED_BEFORE_ISSUE），而買家看到的是泛用 500。
			//
			// 🔴 **四個必須一起補**：只補 invoice_kind 的話 carrier_selection_mode
			// 會落空字串 ⇒ 買家原本選的載具在更正後**消失**，而那不會報錯。
			InvoiceKind:              invoiceKindOf(id.BuyerTaxID),
			CarrierSelectionMode:     id.SelectionMode,
			BuyerInvoicePreferenceID: id.PreferenceID,
			BuyerTaxID:               id.BuyerTaxID,
			BuyerTitle:               id.BuyerName,
			DonationCode:             id.DonationCode,
			IdempotencyKey:           req.IdempotencyKey,
		},
	})
	if err != nil {
		return SubmitCorrectionOutput{}, err
	}
	return toSubmitOutput(out), nil
}

// toSubmitOutput 折出回應素材。婉拒沒有 handling_mode（chk_invcorr_mode 只有兩值，
// 且 REJECTED 時該欄為 NULL）。
func toSubmitOutput(r ExecuteCorrectionResult) SubmitCorrectionOutput {
	switch r.Route {
	case RouteUpdateBeforeIssue:
		return SubmitCorrectionOutput{CorrectionID: r.CorrectionID, Route: r.Route,
			Status: correctionStatusUpdatedBeforeIssue, HandlingMode: handlingModeUpdateSnapshot}
	case RouteVoidAndReissue:
		return SubmitCorrectionOutput{CorrectionID: r.CorrectionID, Route: r.Route,
			Status: correctionStatusVoidReissue, HandlingMode: handlingModeVoidAndReissue}
	default:
		return SubmitCorrectionOutput{CorrectionID: r.CorrectionID, Route: r.Route,
			Status: correctionStatusRejected, RejectReason: r.RejectReason}
	}
}

// carrierShapeValid 判定買家改後的載具形狀是否成立。
//
// 這一層只判形狀（三選一互斥、統編長度），**不判歸屬**——歸屬是 profile 的
// buyer_id 述詞，由既有的 resolveIdentity 在寫入路徑上做。
func carrierShapeValid(spec CheckoutInvoiceSpec) bool {
	filled := 0
	if spec.BuyerTaxID != "" {
		filled++
	}
	if spec.ProfileID != 0 || spec.CarrierType != "" {
		filled++
	}
	// 🔴 `TAX_ID` ＋ `buyerTaxId` 是**同一個來源**，不是兩個（2026-09-04）。
	//
	// 結帳側的契約逐字寫在 identityFromManualInput：「統編必須明說
	// carrierType=TAX_ID，由欄位有沒有值去推語意，就是在憑證上猜買受人身分」
	// ⇒ 合法的「打統編」形狀**一定**同時帶這兩欄，而上面的計數會把它數成 2 ⇒
	// 每一筆打統編的更正都會被判 CARRIER_INVALID 並立刻 REJECTED。
	//
	// dev 實測（#3085 收尾）：`{carrierType:"TAX_ID", buyerTaxId:"22223223",
	// buyerName:"…"}` → 200 但 status=REJECTED / rejectReasonCode=CARRIER_INVALID；
	// 只送 buyerTaxId → 9305（被 validateIdentityShape 擋，那是對的）。
	// **兩種寫法都不通** ⇒ 打統編的更正整條走不到。
	//
	// ⚠️ 這一格與 validateIdentityShape 是「同一個判斷兩份實作」的形狀：
	// 那一份認 TAX_ID 要配 buyerTaxId，這一份認兩者互斥。修的是這一份，
	// 因為結帳側才是 carrierType 值域的 SSOT（CodeMeta InvoiceCarrierType 四值含 TAX_ID）。
	if spec.CarrierType == codemeta.InvoiceCarrierTypeTaxId.Value && spec.BuyerTaxID != "" && spec.ProfileID == 0 {
		filled--
	}
	return filled <= 1
}

// maskedCorrectionPayload／…FromSpec 產出 before／after 快照。
//
// 🔴 **只存遮罩值，不存明文**：這兩欄會被 ADM 讀到，而 invoice_correction_requests
// 沒有加密欄。存明文等於把載具與 email 複製一份到一張沒有保護的表上（§5.1.1）。
func maskedCorrectionPayloadFromDoc(d domain.Document) []byte {
	// before 取自 document 而不是 order 快照：那才是「這張憑證現在印的是什麼」，
	// 而爭議時買家問的正是這個。email 只有密文與 hash ⇒ 這裡不放。
	return marshalMasked(d.RecipientTaxID, d.RecipientName, derefString(d.CarrierType), derefString(d.DonationCode), "")
}

func maskedCorrectionPayloadFromSpec(s CheckoutInvoiceSpec) []byte {
	return marshalMasked(s.BuyerTaxID, s.BuyerName, s.CarrierType, "", s.Email)
}

func marshalMasked(taxID, name, carrierType, donation, email string) []byte {
	b, _ := json.Marshal(map[string]string{
		"buyerTaxId":   taxID,
		"buyerName":    name,
		"carrierType":  carrierType,
		"donationCode": donation,
		"emailMasked":  maskEmail(email),
	})
	return b
}

// int64OrZero：nil ⇒ 0。刻意不叫 derefInt64——測試檔已有同名函式（只在測試可見），
// 同名會讓 production 編譯不過而錯誤訊息指向 redeclared，離真因很遠。
func int64OrZero(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// maskEmail 只保留首字與網域，其餘以 * 取代。
func maskEmail(s string) string {
	at := -1
	for i, r := range s {
		if r == '@' {
			at = i
			break
		}
	}
	if at <= 0 {
		return ""
	}
	return s[:1] + "***" + s[at:]
}

// revisedCarrierFromIdentity 把已解析的買家身分折成 invoice_documents 的載具／捐贈四欄。
//
// 🔴 這四欄在 2026-09-05 之前**整組從未賦值**，而 ReviseInvoiceDocumentSnapshot 的
// `SET carrier_* = narg(...)` 沒有「保持原值」的選項 ⇒ R03 就地改與 R04 作廢重開
// 都會把買家的載具與捐贈碼抹掉。症狀比「載具消失」更糟：carrier_type 由
// revised.CarrierType 帶著、顯隱碼卻是空的 ⇒ 送 AMEGO 是「有載具類型、號碼皆空」
// ＝ 3040132 載具號碼不存在 ⇒ 重開票直接開不出來。
//
// 抽成純函式而不是留在 SubmitCorrection 裡：那支要跑起來需要一整組 repository 替身，
// 而這裡真正要釘住的是三個分支的**折法**，不是流程。
func revisedCarrierFromIdentity(documentKind string, id checkoutIdentity) (carrierType *string, carrierValue string, donationCode *string) {
	// B2B 一律清空（chk_invdoc_exclusive：統編／載具／捐贈三選一，
	// 而買家補打統編正是端點 11 的主要使用情境 ⇒ 舊載具必須跟著消失）。
	if documentKind == codemeta.InvoiceDocumentKindB2b.Value {
		return nil, "", nil
	}
	// 映射逐字照抄 checkout.go 的 draftFromIdentity：id.CarrierType 是 CodeMeta 的
	// Value，落庫與送 AMEGO 要的是原生詞彙（3J0002／CQ0001）。
	if kind, ok := codemeta.InvoiceCarrierTypeFromValue(id.CarrierType); ok {
		if amego, ok := domain.CarrierTypeToAmego(kind); ok && amego != "" && id.CarrierValue != "" {
			// 🔴 **兩者同進同出**：chk_invdoc_carrier_pair 是雙向等式
			// `(carrier_type IS NULL) = (carrier_id1_ciphertext IS NULL)`
			// ⇒ 只填類型不填號碼會 23514，整條更正路徑回 500。
			//
			// ⚠️ `id.CarrierValue` 對**來自常用發票資料**的載具是空的
			// （resolveInvoiceIdentity 那一層不解密——checkout.go 的
			// draftFromIdentity 為此用 carrierPresentPlaceholder 繞過）
			// ⇒ 呼叫端必須先把明文取出來塞回 id，見 openCarrierValueInto。
			carrierType = &amego
			carrierValue = id.CarrierValue
		}
	}
	if id.DonationCode != "" {
		d := id.DonationCode
		donationCode = &d
	}
	return carrierType, carrierValue, donationCode
}

// openCarrierValueInto 把常用發票資料裡的載具顯碼明文補進已解析的身分。
//
// 🔴 明文只活在 fn 的作用域與 id 這個區域變數上（§5.1.1 的 write-only 邊界）：
// 不進 log、不進任何會被回傳或序列化的 struct——它接著只被 revisedCarrierFromIdentity
// 讀去交給 repository 重新加密。
//
// 買家在更正時直接打字給的載具（CarrierType ＋ Value）不走這裡：那條路的明文本來就在
// request 上，id.CarrierValue 已經有值。
func (u *CorrectionIntake) openCarrierValueInto(ctx context.Context, buyerID int64, id *checkoutIdentity) error {
	if id.PreferenceID == 0 || id.CarrierValue != "" || !id.HasCarrier {
		return nil
	}
	if u.deps.Preferences == nil {
		return fmt.Errorf("invoice: preferences repository unavailable (preference=%d)", id.PreferenceID)
	}
	var plain string
	if err := u.deps.Preferences.WithDecryptedCarrierValue(ctx, id.PreferenceID, buyerID,
		func(_, v string) error { plain = v; return nil }); err != nil {
		return fmt.Errorf("invoice: open carrier value (preference=%d): %w", id.PreferenceID, err)
	}
	if plain == "" {
		// chk_bip_shape 保證這兩種 kind 的密文非空 ⇒ 空明文代表解密路徑壞了。
		// 硬錯而不是放行：放行會開出一張沒有號碼的載具發票，AMEGO 收下而買家歸不到戶
		// （與 client/order 的 fromPreference 同一句判準）。
		return fmt.Errorf("invoice: preference %d decrypted to an empty carrier value", id.PreferenceID)
	}
	id.CarrierValue = plain
	return nil
}
