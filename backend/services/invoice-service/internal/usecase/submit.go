package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/eventwire"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// ClaimForSubmit：搶佔一列給 INV.op（§7.1.2 步驟 11）
// ---------------------------------------------------------------------------

// ClaimForSubmit 在**單一事務**內做兩件必須同時成立的事：
//
//	① operation READY／RETRY_SCHEDULED／SENDING → SENDING（J07，CAS，租約推到 now+120s）
//	② document PENDING／ISSUE_FAILED → ISSUING（M1 的對應邊）
//
// 為什麼綁在一起：M1 那條邊的觸發者就是 ①，拆開之後只要 ② 失敗，document 會停在 PENDING
// 而 operation 已經在 SENDING —— 賣家看到「待開立」而系統正在送，兩邊都不會報錯。
//
// 回 false 代表沒搶到（別的副本先拿到，或租約未到期）。呼叫端跳過這一列，**不得**改走
// 別的路徑重試：另開一條旁路等於製造第二個 SENDING 持有者（§8.4.1 I10）。
//
// 🔴 leaseUntil 逐筆在 claim 當下現算（不是在批次迴圈外算一次）：算一次會讓不變式從
// 「單筆最壞耗時 < 租約」退化成「批次總耗時 < 租約」，隨 batch-size 一起破。
func (u *Issuance) ClaimForSubmit(ctx context.Context, operationID int64) (bool, error) {
	now := u.now()
	return u.ClaimForSubmitAt(ctx, operationID, now, now.Add(u.cfg.IssueLease))
}

// ClaimForSubmitAt 是 ClaimForSubmit 的顯式租約版本，給 pkg/outbox.Worker 用。
//
// 存在的理由只有一個：`Repository[T].Claim` 的租約基準由 **worker 逐筆現算**
// （`pkg/outbox/worker.go` 的 processRow：`time.Now().Add(cfg.ClaimLease)`），
// 而 CAS 上界用的是該批次的 tickNow。兩個值必須從 worker 傳進來，不能在這裡各算一次
// ——各算一次的話，「租約基準逐筆取」這條不變式就會有兩份實作，而其中一份沒有測試守著。
//
// now 同時是 `started_at` 的 COALESCE 值與 CAS 的 `claimable_before` 上界。
func (u *Issuance) ClaimForSubmitAt(ctx context.Context, operationID int64, now, leaseUntil time.Time) (bool, error) {
	claimed := false
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		op, err := u.ops.GetByID(ctx, operationID)
		if err != nil {
			return err
		}
		ok, err := u.ops.Claim(ctx, operationID, now, leaseUntil)
		if err != nil || !ok {
			return err
		}
		if _, err := u.docs.MarkIssuing(ctx, op.DocumentID, now); err != nil {
			return err
		}
		// MarkIssuing 回 false 是**合法**的：租約到期被重新 claim 時 document 已經在
		// ISSUING（J07 的候選述詞含 SENDING，否則崩潰的租約永遠回不來）。
		// 所以這裡不把它當失敗——真正的守衛是 ① 的 CAS。
		if err := u.appendHistory(ctx, subjectOperation, op.ID, op.Status,
			codemeta.InvoiceOperationStatusSending.Value, "", "", now); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	return claimed, err
}

// ---------------------------------------------------------------------------
// Submit：送 f0401 並依四分類落點（§7.1.2 步驟 12–14、§4.5.6）
// ---------------------------------------------------------------------------

// Submit 送出一次開票。呼叫端（INV.op worker）必須已經 ClaimForSubmit 成功。
//
// 回 error 代表「這一輪沒有得到結論、且列上的狀態已經寫好」——呼叫端不需要再做補償。
// 本函式**不自己重試**：重試一律在 worker 層（照 PayUni client 慣例，§4.2.2）。
func (u *Issuance) Submit(ctx context.Context, operationID int64) error {
	op, err := u.ops.GetByID(ctx, operationID)
	if err != nil {
		return err
	}
	if op.Status != codemeta.InvoiceOperationStatusSending.Value {
		// 沒有租約就送 ＝ 可能有第二個持有者同時在打 AMEGO ⇒ 重複開票。
		return fmt.Errorf("invoice: operation %d is %s, expected SENDING (claim first)", op.ID, op.Status)
	}

	doc, err := u.docs.GetByID(ctx, op.DocumentID)
	if err != nil {
		return err
	}

	// resume checkpoint（§5.3.2）：已經拿到發票號碼就不再打 AMEGO。
	// 這是「重送不會產生第二張發票」的第 2 道防線，擋的是本地已知結果之後的任何重試。
	if doc.ProviderInvoiceNo != nil && *doc.ProviderInvoiceNo != "" {
		return u.settleIssued(ctx, doc, op, &amego.IssueResult{
			InvoiceNumber: *doc.ProviderInvoiceNo,
			RandomNumber:  derefString(doc.ProviderRandomCode),
		}, u.now())
	}

	// 平台代製收據：不打 AMEGO，改走本地 PDF 產生（§7.1.2 S-6，#3026）。
	// 這一格必須排在下面那道防線**之前** —— 防線的述詞是「不是 AMEGO_AUTO」，
	// 而 PLATFORM_RECEIPT 正好命中它。
	if doc.FulfillmentMode == codemeta.InvoiceFulfillmentModePlatformReceipt.Value {
		return u.settleReceipt(ctx, doc, op, u.now())
	}

	if doc.FulfillmentMode != codemeta.InvoiceFulfillmentModeAmegoAuto.Value {
		// 剩下的三種模式（SELLER_SELF_ISSUE／LEGACY_MANUAL，以及不存在的第四種）
		// **根本不該有 READY 的 ISSUE operation**——決策樹對它們一律落 NOT_APPLICABLE，
		// 而 NOT_APPLICABLE 不進開票候選集合。走到這裡代表有人繞過決策樹建了 operation。
		//
		// ⚠️ PLATFORM_RECEIPT **不再**命中這一格（#3026 之後它走上面那條本地產製）。
		// 這道防線因此從「三種模式」縮成「兩種模式」，語意不變。
		//
		// 當 PERMANENT 處置：讓它出現在 ADM 異常工作台，而不是每 10 秒空轉一次
		// 直到 max_attempts 用完。**這是防線不是正常路徑**——正常路徑上這一格取不到。
		return u.settlePermanent(ctx, doc, op, "", "", ErrUnexpectedIssueOperation.Error(), u.now())
	}

	lines, err := u.docs.ListLines(ctx, doc.ID)
	if err != nil {
		return err
	}

	var req amego.IssueRequest
	if err := u.docs.WithDecryptedRecipient(ctx, doc.ID, func(s domain.RecipientSecrets) error {
		var berr error
		req, berr = buildIssueRequest(doc, lines, s)
		return berr
	}); err != nil {
		return err
	}

	account, err := u.accounts.GetByIssuer(ctx, doc.IssuerID, providerAmego)
	if err != nil {
		return err
	}

	var result *amego.IssueResult
	var sendErr error
	// 🔴 AppKey 明文的活動範圍只到這個 callback：不回傳、不 log、不進任何 struct 欄位。
	// op.AttemptCount 是「先前失敗次數」（pkg/outbox 的 bumpAttempt 在 process 失敗**後**
	// 才加），首次為 0 ⇒ 1-based 的 attempt_seq 是它 +1。
	auditCtx := amego.WithAuditContext(ctx, op.ID, int(op.AttemptCount)+1)
	var carrierRejected bool
	if err := u.accounts.WithDecryptedCredential(ctx, account.ID, func(providerTaxID, appKey string) error {
		cred := amego.Credential{IssuerID: doc.IssuerID, Invoice: providerTaxID, AppKey: appKey}
		// §P12：手機條碼載具**送出前**先驗存在性。與 IssueInvoice 共用同一次憑證解密
		// ⇒ 不多一次解密、不把 AppKey 的活動範圍擴大到 callback 之外。
		if mobileBarcodeRejected(auditCtx, u.provider, cred, req) {
			carrierRejected = true
			return nil
		}
		result, sendErr = u.provider.IssueInvoice(auditCtx, cred, req)
		return nil
	}); err != nil {
		return err
	}

	now := u.now()
	// 載具確定無效 ⇒ **不送 f0401**，直接轉人工。
	//
	// 🔴 走 settleAwaitingManual 而不是 settleFailure：後者會落 ISSUE_FAILED ＋
	// FAILED_PERMANENT，而那一格**沒有自動出口**——法定上傳期限一路倒數到逾期，
	// 罰的是賣家（dev 上三張票就是這樣來的，2026-09-08）。載具打錯是買家的資料問題，
	// 需要人判定「改成無載具開立還是聯絡買家」，那正是 AWAITING_MANUAL 的語意。
	if carrierRejected {
		return u.settleAwaitingManual(ctx, doc, op,
			codemeta.InvoiceManualReasonCarrierInvalid.Value, now)
	}
	if sendErr == nil {
		return u.settleIssued(ctx, doc, op, result, now)
	}
	return u.settleFailure(ctx, doc, op, sendErr, now)
}

// mobileBarcodeRejected 回報「AMEGO 明確說這個手機條碼不能用」。
//
// 🔴 **fail-open**：只有 AMEGO 明確回 9000112（格式錯誤）／9000113（不存在）才回 true。
// 逾時、限流、斷路器開啟、任何未知碼一律回 false 讓開票照常送出——驗證是為了把
// 「本來就會失敗的那些」擋在送出之前，不是給開票加一個新的單點故障。驗證服務掛掉時
// 擋住所有開票，比不驗更糟。
//
// 三個碼的分辨力是實測來的（2026-09-08，測試統編 12345678，設計書 §14 X13 結案）：
//
//	空字串   → 9000111 手機條碼不可為空
//	ABC1234  → 9000112 手機條碼格式錯誤
//	/ZZZZZZZ → 9000113 手機條碼不存在   ← 格式合法、確實查無此載具
//
// 9000111 不列入：我方在更正入口已有 carrierShapeValid 擋格式，送到這裡還是空字串
// 代表上游有 bug，那應該讓 f0401 去炸出來而不是被這裡靜默轉人工。
func mobileBarcodeRejected(ctx context.Context, p ProviderClient, cred amego.Credential, req amego.IssueRequest) bool {
	if req.CarrierType != domain.AmegoCarrierMobileBarcode || req.CarrierID1 == "" {
		return false
	}
	err := p.VerifyMobileBarcode(ctx, cred, req.CarrierID1)
	if err == nil {
		return false
	}
	var ae *amego.APIError
	if !errors.As(err, &ae) {
		return false
	}
	return ae.Code == amegoCodeBarcodeMalformed || ae.Code == amegoCodeBarcodeNotFound
}

const (
	// amegoCodeBarcodeMalformed／amegoCodeBarcodeNotFound 是 /json/barcode 的兩個
	// 「這個載具不能用」碼。字串型別與 amego.APIError.Code 一致（該欄位一律字串，
	// 因為折讓類回 String 而查詢類回 Number）。
	amegoCodeBarcodeMalformed = "9000112"
	amegoCodeBarcodeNotFound  = "9000113"
)

// SweepOverdueStatutoryDeadlines 把**已逾法定上傳期限**的文件標成 AWAITING_MANUAL ＋
// manual_reason_code=DEADLINE_MISSED（設計書 §5.2.1）。
//
// 🔴 這支在 2026-09-09 之前**不存在**，而 §5.2.1 逐字寫著「過期者由同一支掃描標
// AWAITING_MANUAL ＋ manual_reason_code='DEADLINE_MISSED'」。實查：
// `FetchApproachingStatutoryDeadlines` 零 usecase／worker 呼叫端、`DEADLINE_MISSED`
// 零生產者（business-alerts.yml 的 InvoiceStatutoryDeadlineOverdue 註解 2026-09-05
// 已記載，並警告「照那句話等掃描把它清掉的人會一直等下去」）。
//
// ⚠️ **這不是把 critical 降級**：逾期文件轉進 AWAITING_MANUAL 之後由 M32
// （invoice_documents_awaiting_manual）接手，而 `InvoiceAwaitingManualDeadlineAtRisk`
// 的述詞含 `bucket=~"overdue|..."` 且 severity 同樣是 **critical**。換的是桶不是等級，
// 而新桶的語意（「這件事在等人處理」）才是逾期文件的真實狀態。
//
// 🔴 只標 PENDING 與 ISSUE_FAILED：
//   - ISSUING 代表**正在送 AMEGO**，逾期了也該等它跑完（MarkAwaitingManual 的
//     allowed-from 是 ('PENDING','ISSUE_FAILED') ⇒ 天然擋住，這裡的 continue 只是
//     省一次無謂的 UPDATE 與一列誤導的 history）
//   - AWAITING_MANUAL 已經在目標狀態（本查詢的述詞含它，是為了「逼近告警」那個用途）
//
// **不動 operation**：它可能已是終態、也可能還在候選集合裡，而 document 轉
// AWAITING_MANUAL 是分類不是中止。動它會與 INV.op 的 lease 搶同一列。
func (u *Issuance) SweepOverdueStatutoryDeadlines(ctx context.Context, now time.Time) (int, error) {
	rows, err := u.docs.FetchApproachingStatutoryDeadlines(ctx, now, statutorySweepBatch)
	if err != nil {
		return 0, err
	}
	swept := 0
	for _, d := range rows {
		if d.Status != codemeta.InvoiceDocumentStatusPending.Value &&
			d.Status != codemeta.InvoiceDocumentStatusIssueFailed.Value {
			continue
		}
		ok, merr := u.docs.MarkAwaitingManual(ctx, d.ID,
			codemeta.InvoiceManualReasonDeadlineMissed.Value, now)
		if merr != nil {
			return swept, merr
		}
		if !ok {
			// rows=0 ＝ 這一列在我讀到它之後離開了 allowed-from（例如 INV.op 剛把它
			// 推進 ISSUING）。那是正常的併發結果，不是錯誤——下一輪再看。
			continue
		}
		if err := u.appendHistory(ctx, subjectDocument, d.ID, d.Status,
			codemeta.InvoiceDocumentStatusAwaitingManual.Value,
			codemeta.InvoiceManualReasonDeadlineMissed.Value, "", now); err != nil {
			return swept, err
		}
		swept++
	}
	return swept, nil
}

// statutorySweepBatch 讓單輪耗時有上界。逾期是時間到就成立的事實、不會突然湧入，
// 與 INV.dl 的 5 分鐘 tick 搭配足夠。
const statutorySweepBatch int32 = 200

// settleFailure 把一次失敗翻成 M2 的落點（§4.5.6 四分類 → §7.3.2 的 J 碼）。
//
// 分類的權威是 amego.APIError.Class，**不是** HTTP status、也不是 msg 字串
// （3040171 是全表唯一以 msg 分支的地方，而那個判斷已經在 amego/codes.go 內做完）。
func (u *Issuance) settleFailure(ctx context.Context, doc domain.Document, op domain.Operation, sendErr error, now time.Time) error {
	var apiErr *amego.APIError
	if !errors.As(sendErr, &apiErr) {
		// 非 provider 錯誤（ctx 取消、transport 尚未分類）⇒ 結果不明，**不得重送**：
		// 副作用可能已經發生。走對帳（J13）。
		return u.settleUnknown(ctx, doc, op, "", sendErr.Error(), now)
	}

	switch apiErr.Class {
	case amego.ClassRetryable:
		return u.settleRetry(ctx, doc, op, apiErr, now)

	case amego.ClassReconcile:
		// 含逾時、連線中斷、JSON 解析失敗、未知碼的 default，以及 3040171 的永久衝突。
		// 「已被開立過而可對帳取回」與「完全不知道送出去沒有」在這一層是同一種處置：
		// 去問 AMEGO。盲目重送才是會開出第二張發票的那條路。
		return u.settleUnknown(ctx, doc, op, apiErr.Code, apiErr.Error(), now)

	case amego.ClassManual:
		if apiErr.SellerActionRequired {
			return u.settleSellerAction(ctx, doc, op, apiErr, now)
		}
		return u.settlePermanent(ctx, doc, op, apiErr.Code, apiErr.Msg, apiErr.Error(), now)

	case amego.ClassPermanent:
		if apiErr.SellerActionRequired {
			return u.settleSellerAction(ctx, doc, op, apiErr, now)
		}
		return u.settlePermanent(ctx, doc, op, apiErr.Code, apiErr.Msg, apiErr.Error(), now)

	default:
		// 分類值域長胖時預設走對帳：多打一次唯讀查詢，代價最小且會自己收斂。
		return u.settleUnknown(ctx, doc, op, apiErr.Code, apiErr.Error(), now)
	}
}

// settleIssued 是 §7.1.2 步驟 14：**單一事務**寫齊五件事。
func (u *Issuance) settleIssued(ctx context.Context, doc domain.Document, op domain.Operation, res *amego.IssueResult, now time.Time) error {
	var issuedAt *time.Time
	if res.InvoiceTime > 0 {
		t := time.Unix(res.InvoiceTime, 0).UTC()
		issuedAt = &t
	}

	// 🔴 invoice_date_taipei 與 tax_period_code **原值帶回、不以 provider 回傳值覆蓋**（註 5）：
	// 我方認定的應開立日是收款日，document 建立當下就固定、不隨重試移動。用 provider_issued_at
	// 重算會讓「拖越久期限越晚」，而 statutory_deadline_at 正是由它推導的。
	// 兩者不一致（開票延遲到跨日、甚至跨期別）是**告警**的職責，不是在這裡靜默改值。
	invoiceDate := doc.InvoiceDateTaipei
	taxPeriod := derefString(doc.TaxPeriodCode)

	// 載具發票沒有 PDF 可下載 ⇒ 不進 INV.pdf 的候選集合（傳 nil 而不是 now）。
	var pdfNext *time.Time
	if doc.CarrierType == nil || *doc.CarrierType == "" {
		t := now
		pdfNext = &t
	}
	uploadNext := now

	// 🔴 store_id 走 invoice_issuers.issuer_ref_id，**不是 doc.IssuerID**（那是 PK）。
	// 讀在交易外：它是一列不可變的對照（issuer_ref_id 沒有任何寫入路徑會改），
	// 而放進交易只會讓一次逾時把整筆開票一起回滾。
	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return err
	}
	payload, err := u.issuedEventPayload(issuer, doc, res, issuedAt, now)
	if err != nil {
		return err
	}

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		// 🔴 outbox 排在**最前面**是刻意的，而且是這條不變式唯一可被機械驗證的形狀。
		//
		// 原子性不在乎順序，但**可測性**在乎：把它放最後的話，「事件寫入有沒有跟隨業務交易」
		// 只能靠「讓交易在它之後失敗」來驗，而它後面沒有任何一句 SQL ⇒ 那個變異是無法轉紅的
		// no-op。排在最前面之後，只要讓後面任何一句守衛失敗（例如 document 已被別人移出
		// ISSUING ⇒ MarkIssued rows=0），交易就會把這一列一起帶走 —— 而「用 pool 而不是交易寫」
		// 的那份實作會留下一列孤兒事件：發票沒開成，買家卻收到「已開立」的通知。
		if _, err := u.outbox.Insert(ctx, domain.NewOutboxEvent{
			AggregateType: outboxAggregateD,
			AggregateID:   doc.ID,
			EventType:     string(issuedEventType(doc.DocumentKind)),
			Exchange:      pkgmq.ExchangeInvoice,
			RoutingKey:    issuedRoutingKey(doc.DocumentKind),
			Payload:       payload,
			NextAttemptAt: now,
			Now:           now,
		}); err != nil {
			return err
		}

		ok, err := u.docs.MarkIssued(ctx, domain.MarkDocumentIssued{
			ID:                  doc.ID,
			Provider:            providerAmego,
			ProviderInvoiceNo:   res.InvoiceNumber,
			ProviderRandomCode:  res.RandomNumber,
			ProviderIssuedAt:    issuedAt,
			InvoiceDateTaipei:   invoiceDate,
			TaxPeriodCode:       taxPeriod,
			PdfFetchNextAt:      pdfNext,
			UploadRecheckNextAt: &uploadNext,
			Now:                 now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: document %d could not enter ISSUED (rows=0, status=%s)", doc.ID, doc.Status)
		}
		if _, err := u.ops.MarkDone(ctx, op.ID, now); err != nil {
			return err
		}
		if err := u.appendHistory(ctx, subjectDocument, doc.ID,
			codemeta.InvoiceDocumentStatusIssuing.Value,
			codemeta.InvoiceDocumentStatusIssued.Value, "", "", now); err != nil {
			return err
		}
		if err := u.appendHistory(ctx, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusSucceeded.Value, "", "", now); err != nil {
			return err
		}
		// R10：這張是作廢重開的新票 ⇒ 同一交易內把原票標 SUPERSEDED、更正請求標 COMPLETED。
		//
		// 🔴 **必須在這個交易內**，不能另開一輪：新票已經 ISSUED 而原票還停在 VOIDED、
		// correction 還停在 VOID_REISSUE_IN_PROGRESS 的中間狀態，沒有任何掃描器會把它收掉
		// ——那正是本服務反覆出現的那個 class（建了一列而沒有東西會推它）。
		return u.completeReissueIfAny(ctx, issuer, doc, res, now)
	})
}

// completeReissueIfAny 是 §7.3.6 的 R10。
//
// 判準是 document 自己的 original_document_id：非 nil ⇒ 這張是重開票。
// 用它而不是「查有沒有 open correction」是因為退款折讓路徑也會作廢，但那條不重開
// ——它產不出帶 original_document_id 的 document。
//
// 找不到 open correction **不是錯誤**（同上：非更正來源的重開），這時只標 SUPERSEDED。
func (u *Issuance) completeReissueIfAny(
	ctx context.Context, issuer domain.Issuer, doc domain.Document,
	res *amego.IssueResult, now time.Time,
) error {
	if doc.OriginalDocumentID == nil {
		return nil
	}
	originalID := *doc.OriginalDocumentID
	// 先讀原票：P5 的文案要「原發票 X 已作廢並重新開立，新發票號碼 Y」，而 X 只有
	// 這一列答得出來。讀在 MarkSuperseded **之前**沒有差別（那句不動 provider_invoice_no），
	// 放前面是因為讀失敗時整筆 rollback 比發一則缺號碼的通知好。
	original, err := u.docs.GetByID(ctx, originalID)
	if err != nil {
		return err
	}
	superseded, err := u.docs.MarkSuperseded(ctx, originalID, doc.ID, now)
	if err != nil {
		return err
	}
	if !superseded {
		// 原票不在 allowed-from（VOIDED／ISSUED）⇒ 我對狀態的理解與 DB 不符。整筆 rollback：
		// 放行會讓同一筆交易存在兩張都不是 SUPERSEDED 的發票，而那是稅務上的重複開立。
		return fmt.Errorf(
			"invoice: original document %d could not enter SUPERSEDED (rows=0, snapshot status=%s)",
			originalID, original.Status)
	}
	// 🔴 from 取 `original.Status`，**不硬寫 VOIDED**：`MarkInvoiceDocumentSuperseded` 的
	// allowed-from 是 `('VOIDED', 'ISSUED')` 兩個值——後者是「作廢確認遲到」的補正路徑
	// （f0501 已受理但 invoice_query 還沒翻面時，重開票先 ISSUED 而原票仍停在 ISSUED）。
	// 硬寫 VOIDED 會在那條路徑上留下一條**沒有發生過**的邊，而 invoice_status_history
	// 正是 A10 稽核與 AuditTrailDrawer 呈現給人看的東西（同 fsm/transition.go 檔頭的紅字）。
	// `original` 是在 MarkSuperseded **之前**讀的 ⇒ 它就是真正的 from。
	if err := u.appendHistory(ctx, subjectDocument, originalID,
		original.Status,
		codemeta.InvoiceDocumentStatusSuperseded.Value, "", "", now); err != nil {
		return err
	}

	// P5：發在這裡而不是下面 correction 那一段，因為 58 §7 把它掛在**兩條**轉移上
	// （M1 `→ SUPERSEDED` 與 M6 R10，§8014 兩列都指向 P5），而「非更正來源的重開」
	// 只走得到前者。掛在 correction 那一段 ⇒ 那一類重開的買家收不到通知。
	//
	// 與本交易最前面那則 INVOICE_ISSUED 是**兩則**不是二選一：後者說「你有一張新發票」，
	// 這一則說「它取代了哪一張」。冪等鍵不同（都以 new_document_id 為底但 event_type
	// 不同），notification 端的唯一鍵含 event_type ⇒ 不會互相吃掉。
	reissuedPayload, err := u.reissuedEventPayload(issuer, doc, original, res, now)
	if err != nil {
		return err
	}
	if _, err := u.outbox.Insert(ctx, domain.NewOutboxEvent{
		AggregateType: outboxAggregateD,
		AggregateID:   doc.ID,
		EventType:     string(pkgmq.EventTypeInvoiceReissued),
		Exchange:      pkgmq.ExchangeInvoice,
		RoutingKey:    pkgmq.RoutingKeyInvoiceReissued,
		Payload:       reissuedPayload,
		NextAttemptAt: now,
		Now:           now,
	}); err != nil {
		return err
	}

	corr, err := u.corrections.GetOpenByDocument(ctx, originalID)
	if err != nil {
		if !isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
			return err
		}
		// 🔴 **第二道，不是備援**（#3089 A 組第 1 項的第二層）：
		// GetOpenByDocument 的述詞只有 REQUESTED／VOID_REISSUE_IN_PROGRESS，
		// 停在 REISSUE_FAILED 的查不到 ⇒ 在此之前這裡直接 `return nil` 靜默略過。
		// 後果是「ADM 重試成功、新票開出來了，而 correction 永遠停在 REISSUE_FAILED、
		// reissued_document_id 永遠 NULL」——Complete 的 allowed-from 明明含它，
		// 那段容納卻是一條永遠走不到的死路徑。
		//
		// ⚠️ 端點 52 已經會把 correction 推回 VOID_REISSUE_IN_PROGRESS（R09），
		// 走那條路時上面那一支就找得到。這一支接的是**繞過端點 52 的重開**
		// （worker 自己重試成功、或任何未來新增的重開路徑）——兩道各自獨立，
		// 拿掉任何一道都會讓對應的情境重新卡住。
		corr, err = u.corrections.GetReissueFailedByDocument(ctx, originalID)
		if err != nil {
			if isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
				return nil // 這張原始憑證上真的沒有待收尾的修改請求
			}
			return err
		}
	}
	done, err := u.corrections.Complete(ctx, corr.ID, doc.ID, now)
	if err != nil {
		return err
	}
	if !done {
		return fmt.Errorf("invoice: correction %d could not enter COMPLETED (rows=0, status=%s)", corr.ID, corr.Status)
	}
	return u.appendHistory(ctx, subjectCorrectionRequest, corr.ID,
		corr.Status, codemeta.InvoiceCorrectionStatusCompleted.Value, "", "", now)
}

// settleRetry 是 J12（RETRYABLE 且 attempt_count < MaxAttempts）與 J14（達上限）。
//
// 兩者在同一個交易內連著做，而不是「這一輪只寫 J12、下一輪才 J14」：J14 的 allowed-from
// 是 RETRY_SCHEDULED，所以必須先走完 J12 才走得到它。分兩輪的代價是白燒一次 claim ＋
// 一次 AMEGO 呼叫額度，而那一次的結果已經知道是「預算用完」。
func (u *Issuance) settleRetry(ctx context.Context, doc domain.Document, op domain.Operation, apiErr *amego.APIError, now time.Time) error {
	newAttempt := op.AttemptCount + 1
	exhausted := newAttempt >= op.MaxAttempts
	delay := backoffFor(int(newAttempt), u.cfg.BaseBackoff, u.cfg.MaxBackoff)
	lastErr := apiErr.Error()

	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		if err := u.ops.UpdateAttempt(ctx, op.ID, newAttempt, now.Add(delay), &lastErr, now); err != nil {
			return err
		}
		// #3082 第二階段：reason_code 一律 CodeMeta 值；AMEGO 的原話走 reason_detail
		// （text），**不是 correlation_id**（varchar(64)，那是 PR #3081 修過的借位缺陷）。
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusRetryScheduled.Value,
			codemeta.InvoiceManualReasonProviderRetryableError.Value,
			lastErr, "", now); err != nil {
			return err
		}
		if !exhausted {
			// document 退回 PENDING（M1：ISSUING → PENDING，operation 退回 RETRY_SCHEDULED）。
			if _, err := u.docs.MarkPendingRetry(ctx, doc.ID, now); err != nil {
				return err
			}
			return appendStatusHistoryDetail(ctx, u.audit, subjectDocument, doc.ID,
				codemeta.InvoiceDocumentStatusIssuing.Value,
				codemeta.InvoiceDocumentStatusPending.Value,
				codemeta.InvoiceManualReasonProviderRetryableError.Value,
				lastErr, "", now)
		}

		ok, err := u.ops.MarkRetryExhausted(ctx, op.ID, lastErr, now)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter FAILED_RETRY_EXHAUSTED (rows=0)", op.ID)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusRetryScheduled.Value,
			codemeta.InvoiceOperationStatusFailedRetryExhausted.Value,
			codemeta.InvoiceManualReasonRetryBudgetExhausted.Value,
			lastErr, "", now); err != nil {
			return err
		}
		return u.failDocument(ctx, doc, op, failureClassRetryExhausted,
			codemeta.InvoiceManualReasonRetryBudgetExhausted.Value, apiErr.Code, lastErr, now)
	})
}

// settlePermanent 是 J11 SENDING → FAILED_PERMANENT。
//
// 🔴 走 MarkFailedWithProviderResult 而不是 MarkFailed（#3082）：
// provider_code／provider_message 兩個專用欄位在此之前**從未被任何路徑寫入**。
// 2026-09-04 dev 實機驗證抓到 —— 第二階段只改了 status_history 的 reason_code，
// 而 repository 呼叫還是舊的那支 ⇒ 實測 op 7500463275386077184 落
// FAILED_PERMANENT 之後 reason_code 對了（PROVIDER_REJECTED_PERMANENT）、
// 兩個 provider 欄仍是空的。**白盒推理看不到這一層，只有實打得到。**
func (u *Issuance) settlePermanent(ctx context.Context, doc domain.Document, op domain.Operation, providerCode, providerMessage, lastError string, now time.Time) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.ops.MarkFailedWithProviderResult(ctx, domain.MarkOperationProviderResult{
			// 🔴 ProviderMessage 是**原文**（APIError.Msg），與 LastError 不是同一件事
			// （#3082 收尾，2026-09-04 dev 實測發現）：LastError 是我方組的診斷字串
			// （`amego: /json/f0401: code=… class=… kind=…`），provider_message 要的是
			// 對方原樣回的那句話——§8.2.1 稱它「落庫供稅務舉證」。
			//
			// dev 實證：重試一筆 REISSUE 後 provider_code=3040122 有落庫、
			// provider_message 仍是 NULL ⇒ 本票的宣稱只修好了一半。
			ID:              op.ID,
			ProviderCode:    providerCode,
			ProviderMessage: providerMessage,
			LastError:       lastError,
			Now:             now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter FAILED_PERMANENT (rows=0)", op.ID)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusFailedPermanent.Value,
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value,
			lastError, "", now); err != nil {
			return err
		}
		return u.failDocument(ctx, doc, op, failureClassPermanent,
			codemeta.InvoiceManualReasonProviderRejectedPermanent.Value, providerCode, lastError, now)
	})
}

// settleSellerAction 是 J32 SENDING → SELLER_ACTION_REQUIRED（3040111 字軌不足這一類）。
//
// ⚠️ 它**不叫 FAILED_***，但 document 一樣要落 ISSUE_FAILED：把觸發條件寫成「任一 FAILED_*」
// 會漏掉它，而漏掉的後果是 document 永遠停在 ISSUING，賣家看到「開立中」而系統早已停手
// 在等他去財政部配字軌。
func (u *Issuance) settleSellerAction(ctx context.Context, doc domain.Document, op domain.Operation, apiErr *amego.APIError, now time.Time) error {
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.ops.MarkSellerActionRequired(ctx, domain.MarkOperationProviderResult{
			ID:              op.ID,
			ProviderCode:    apiErr.Code,
			ProviderMessage: apiErr.Msg,
			LastError:       apiErr.Error(),
			Now:             now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter SELLER_ACTION_REQUIRED (rows=0)", op.ID)
		}
		if err := appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusSellerActionRequired.Value,
			codemeta.InvoiceManualReasonSellerTrackInsufficient.Value,
			apiErr.Error(), "", now); err != nil {
			return err
		}
		return u.failDocument(ctx, doc, op, failureClassPermanent,
			codemeta.InvoiceManualReasonSellerTrackInsufficient.Value, apiErr.Code, apiErr.Error(), now)
	})
}

// settleUnknown 是 J13 SENDING → WAITING_CONFIRMATION。
//
// document **留在 ISSUING**：結果不明時它既不是失敗也不是成功，而 M1 上沒有
// ISSUING → ISSUING 的自轉移。這一列會離開 INV.op 的候選集合、落進 INV.rc 的
// （兩組狀態集合互斥＝I-55），由對帳給出真相。
//
// ⚠️ **不發 MQ**：invoice.issue.pending_confirmation 不在 P1–P12 內，發到沒有 binding 的
// routing key 會靜默消失、不進 DLQ、不報錯。
func (u *Issuance) settleUnknown(ctx context.Context, doc domain.Document, op domain.Operation, providerCode, lastError string, now time.Time) error {
	_ = doc
	return u.tx.WithTx(ctx, func(ctx context.Context) error {
		ok, err := u.ops.MarkWaitingConfirmation(ctx, domain.MarkOperationUnknown{
			ID:            op.ID,
			NextAttemptAt: now.Add(u.cfg.RecheckBase),
			ProviderCode:  providerCode,
			LastError:     lastError,
			Now:           now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("invoice: operation %d could not enter WAITING_CONFIRMATION (rows=0)", op.ID)
		}
		return appendStatusHistoryDetail(ctx, u.audit, subjectOperation, op.ID,
			codemeta.InvoiceOperationStatusSending.Value,
			codemeta.InvoiceOperationStatusWaitingConfirmation.Value,
			codemeta.InvoiceManualReasonProviderResultUnknown.Value,
			lastError, "", now)
	})
}

// failureClass 是 P2 payload 的 failure_class 欄（§6.3.3.3）。
const (
	failureClassPermanent      = "PERMANENT"
	failureClassRetryExhausted = "RETRYABLE"
)

// failDocument 把 document 推到 ISSUE_FAILED 並在同一交易內落一列 P2 outbox。
//
// **document 不自己發事件**：事件由對應的 operation 轉移發（J11／J14／J32 ⇒ P2），
// 同一件事只有一個 producer —— 所以這裡是唯一的 P2 落點，三個 settle* 共用它。
// 🔴 reasonCode 與 providerCode 是**兩件事**（#3082 第二階段）：
//
//	reasonCode   ＝ CodeMeta 的 InvoiceManualReason，落 invoice_status_history.reason_code
//	providerCode ＝ AMEGO 的原始碼，落**對外事件的 payload**（P2 的消費者要看它）
//
// 在此之前只有一個參數，AMEGO 的碼被同時當成兩者用 ⇒ reason_code 混進了
// 一個不屬於它值域的值。三個呼叫端各自傳自己那一格的分類。
func (u *Issuance) failDocument(ctx context.Context, doc domain.Document, op domain.Operation, failureClass, reasonCode, providerCode, detail string, now time.Time) error {
	ok, err := u.docs.MarkIssueFailed(ctx, doc.ID, reasonCode, now)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invoice: document %d could not enter ISSUE_FAILED (rows=0, status=%s)", doc.ID, doc.Status)
	}
	if err := appendStatusHistoryDetail(ctx, u.audit, subjectDocument, doc.ID,
		codemeta.InvoiceDocumentStatusIssuing.Value,
		codemeta.InvoiceDocumentStatusIssueFailed.Value, reasonCode, detail, "", now); err != nil {
		return err
	}
	issuer, err := u.issuers.GetByID(ctx, doc.IssuerID)
	if err != nil {
		return err
	}
	// 🔴 reasonCode 用**傳進來的那一個**，不是 doc 上的舊值：doc 是這一輪開始時
	// 載入的快照，上面那一句剛寫進去的值不在它身上 ⇒ 讀 doc 會讓 P2 事件的
	// FailureReasonCode 恆為空（2026-09-05 實查，與畫面缺原因是同一個根因）。
	payload, err := u.failedEventPayload(issuer, doc, op, failureClass, reasonCode, providerCode, now)
	if err != nil {
		return err
	}
	if _, err = u.outbox.Insert(ctx, domain.NewOutboxEvent{
		AggregateType: outboxAggregateD,
		AggregateID:   doc.ID,
		EventType:     string(pkgmq.EventTypeInvoiceIssueFailed),
		Exchange:      pkgmq.ExchangeInvoice,
		RoutingKey:    pkgmq.RoutingKeyInvoiceIssueFailed,
		Payload:       payload,
		NextAttemptAt: now,
		Now:           now,
	}); err != nil {
		return err
	}
	// R08：失敗的這一張是作廢重開的新票 ⇒ correction 落 REISSUE_FAILED。
	return u.markReissueFailedIfAny(ctx, doc, now)
}

// markReissueFailedIfAny 是 §7.3.6 的 R08，掛在**所有** document 失敗落點的共同出口。
//
// 判準與 R10 同一個（document 自己的 original_document_id），所以兩者不會漏掉對方
// 涵蓋的分支——failDocument 是 settlePermanent／settleRetry 用盡／settleSellerAction／
// settleUnknown 四條路的匯流點。
//
// 🔴 少了它，「作廢成功但新票開不出來」的 correction 會永遠停在
// VOID_REISSUE_IN_PROGRESS：那個狀態沒有任何掃描器，而買家看到的是「處理中」，
// 實際上他手上那張已經被作廢、新的一張不存在。設計書逐字稱 REISSUE_FAILED 為
// 「本設計書中最嚴重的單一狀態」，正是因為這個組合。
//
// ⚠️ 本函式只負責把狀態寫對。設計書要求的另外三件（A06 最高優先佇列、15 分鐘 SLA
// 告警、買家專屬文案通知）不在這裡：前兩件是 I8 唯讀掃描的範圍（#2959 ⑥），
// 第三件要等 B06 畫面。**不要因為狀態寫對了就以為那三件也做了。**
//
// 找不到 open correction 不是錯誤：退款折讓路徑不會產生帶 original_document_id 的
// document，而手動重開（端點 57）也可能沒有 correction。
func (u *Issuance) markReissueFailedIfAny(ctx context.Context, doc domain.Document, now time.Time) error {
	if doc.OriginalDocumentID == nil {
		return nil
	}
	corr, err := u.corrections.GetOpenByDocument(ctx, *doc.OriginalDocumentID)
	if err != nil {
		if isBizCode(err, bizcode.InvoiceCorrectionNotAllowed) {
			return nil
		}
		return err
	}
	ok, err := u.corrections.MarkReissueFailed(ctx, corr.ID, now)
	if err != nil {
		return err
	}
	if !ok {
		// 不在 VOID_REISSUE_IN_PROGRESS ⇒ 已經被推進過（例如 ADM 走端點 52 重試）。
		// 合法，不當失敗。
		u.log.Info("invoice.reissue.mark_failed_skipped",
			zap.Int64("correction_id", corr.ID), zap.String("status", corr.Status))
		return nil
	}
	u.log.Error("invoice.reissue.failed",
		zap.Int64("correction_id", corr.ID),
		zap.Int64("original_document_id", *doc.OriginalDocumentID),
		zap.Int64("reissue_document_id", doc.ID))
	return appendStatusHistory(ctx, u.audit, subjectCorrectionRequest, corr.ID,
		corr.Status, codemeta.InvoiceCorrectionStatusReissueFailed.Value, "", "", now)
}

// ---------------------------------------------------------------------------
// AMEGO request 組裝（§5A.6.4 表頭四欄 ＋ §5A.3 明細行）
// ---------------------------------------------------------------------------

// buildIssueRequest 由**已落庫**的 document ＋ lines 組出 f0401 payload。
//
// 為什麼從 DB 讀而不是把 IssueDraft 留到這裡：送出與建立之間隔著租約、重試與可能的 pod
// 重啟，唯一能保證「送出去的就是落庫的那一份」的做法是從落庫那一份重建。
// §5A.11 的斷言在 Prepare 落庫**之前**跑完，本函式不重複驗（provider adapter 亦然）。
func buildIssueRequest(doc domain.Document, lines []domain.DocumentLine, secrets domain.RecipientSecrets) (amego.IssueRequest, error) {
	items := make([]amego.IssueProductItem, 0, len(lines))
	for _, l := range lines {
		// gross == 0 的行落庫但不輸出 ⇒ line_seq 與陣列索引必然錯開，兩者不得互相推導。
		if l.NetAmount == 0 {
			continue
		}
		taxType, err := domain.TaxCategoryToAmego(domain.TaxCategory(l.TaxCategory))
		if err != nil {
			return amego.IssueRequest{}, err
		}
		items = append(items, amego.IssueProductItem{
			Description: l.Description,
			Quantity:    strconv.FormatInt(int64(l.Quantity), 10),
			UnitPrice:   strconv.FormatInt(l.UnitPrice, 10),
			Amount:      strconv.FormatInt(l.NetAmount, 10),
			Remark:      derefString(l.Remark),
			TaxType:     taxType,
		})
	}

	return amego.IssueRequest{
		OrderID:           derefString(doc.ProviderRequestNo),
		BuyerIdentifier:   doc.RecipientTaxID,
		BuyerName:         doc.RecipientName,
		BuyerEmailAddress: secrets.Email,
		CarrierType:       derefString(doc.CarrierType),
		CarrierID1:        secrets.CarrierID1,
		CarrierID2:        secrets.CarrierID2,
		NPOBAN:            derefString(doc.DonationCode),
		ProductItem:       items,

		SalesAmount:        strconv.FormatInt(doc.SalesAmount, 10),
		FreeTaxSalesAmount: strconv.FormatInt(doc.FreeTaxSalesAmount, 10),
		ZeroTaxSalesAmount: strconv.FormatInt(doc.ZeroTaxSalesAmount, 10),
		TaxAmount:          strconv.FormatInt(doc.TaxAmount, 10),
		TotalAmount:        strconv.FormatInt(doc.TotalAmount, 10),
		TaxType:            strconv.FormatInt(int64(doc.TaxType), 10),
		// TaxRate 恆帶字面值 "0.05"，不由 tax_rate_bps 除回來（除法會產出 0.049999…）。
		TaxRate:           domain.TaxRateLiteral,
		DetailVat:         strconv.Itoa(domain.DetailVatInclusive),
		DetailAmountRound: strconv.Itoa(domain.DetailAmountRoundOn),
	}, nil
}

// ---------------------------------------------------------------------------
// outbox payload（§6.3.3.2 三條邊界 ＋ §6.3.3.4 冪等鍵）
// ---------------------------------------------------------------------------

// 三條 payload 邊界，逐條都是「拿到它能做什麼」而不是「是不是 PII」：
//
//	① 載具識別碼／買受人統編／email／電話 → 只放 has_* 布林
//	② random_code → invoice_no + random_code 就能在財政部平台查到整張發票 ⇒ 它是**憑證**
//	③ 任何 provider credential
//
// notify_seller_account_id 刻意**不在**這裡填：§6.3.3.2 明定它在 outbox 派送時解析，
// 不在業務交易內做（失敗帶 "0" 讓下游 skip、不阻斷派送）。
type issuedEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	InvoiceDocumentID string            `json:"invoice_document_id"`
	OrderNo           string            `json:"order_no"`
	BuyerID           eventwire.BuyerID `json:"buyer_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	DocumentKind      string            `json:"document_kind"`
	InvoiceNo         string            `json:"invoice_no"`
	InvoiceDate       string            `json:"invoice_date"`
	TotalAmount       string            `json:"total_amount"`
	HasCarrier        bool              `json:"has_carrier"`
	HasBuyerTaxID     bool              `json:"has_buyer_tax_id"`
	OccurredAt        string            `json:"occurred_at"`
}

// reissuedEvent 是 P5（`invoice.reissued`，58 §6.3.9 定版 13 列的第 6 列）。
//
// 🔴 它與 `invoice.issued` **不能互相取代**：重開票走的是正常開票路徑，那條路已經發過
// 一則 INVOICE_ISSUED；少了本事件，「這張是取代前一張的」在事件面完全不可區分，
// 下游（通知、對帳、CRM）分不出來。notification-service 的
// `invoice.reissued.buyer` 模板早就寫好了（2026-08-30 那批），停用理由逐字是
// 「publisher 根本還沒實作」。
//
// 欄位取自該模板實際用到的每一個 `#{...}` 與 data_fields[*].field，不是我猜的：
// notification_ref_id／invoice_document_id／voided_invoice_no／new_invoice_no。
type reissuedEvent struct {
	NotificationRefID string            `json:"notification_ref_id"`
	InvoiceDocumentID string            `json:"invoice_document_id"`
	OrderNo           string            `json:"order_no"`
	BuyerID           eventwire.BuyerID `json:"buyer_id"`
	StoreID           eventwire.StoreID `json:"store_id"`
	VoidedDocumentID  string            `json:"voided_document_id"`
	VoidedInvoiceNo   string            `json:"voided_invoice_no"`
	NewInvoiceNo      string            `json:"new_invoice_no"`
	OccurredAt        string            `json:"occurred_at"`
}

type issueFailedEvent struct {
	NotificationRefID  string            `json:"notification_ref_id"`
	InvoiceDocumentID  string            `json:"invoice_document_id"`
	InvoiceOperationID string            `json:"invoice_operation_id"`
	OrderNo            string            `json:"order_no"`
	BuyerID            eventwire.BuyerID `json:"buyer_id"`
	StoreID            eventwire.StoreID `json:"store_id"`
	FailureClass       string            `json:"failure_class"`
	FailureReasonCode  string            `json:"failure_reason_code"`
	ProviderErrorCode  string            `json:"provider_error_code"`
	OccurredAt         string            `json:"occurred_at"`
}

func (u *Issuance) issuedEventPayload(issuer domain.Issuer, doc domain.Document, res *amego.IssueResult, issuedAt *time.Time, now time.Time) (json.RawMessage, error) {
	invoiceDate := ""
	if doc.InvoiceDateTaipei != nil {
		invoiceDate = doc.InvoiceDateTaipei.Format("2006-01-02")
	}
	_ = issuedAt
	return json.Marshal(issuedEvent{
		// §6.3.3.4：reference_id 只填 order_no 的話，「開立 → 作廢 → 重開」的第二、三次
		// 通知會被 notification-service 靜默吞掉（它的冪等鍵含 reference_id）。
		NotificationRefID: "INV:" + strconv.FormatInt(doc.ID, 10),
		InvoiceDocumentID: strconv.FormatInt(doc.ID, 10),
		OrderNo:           derefString(doc.SourceRef),
		BuyerID:           eventwire.BuyerIDOfDocument(doc),
		StoreID:           eventwire.StoreIDOf(issuer),
		DocumentKind:      doc.DocumentKind,
		InvoiceNo:         res.InvoiceNumber,
		InvoiceDate:       invoiceDate,
		TotalAmount:       strconv.FormatInt(doc.TotalAmount, 10),
		HasCarrier:        doc.CarrierType != nil && *doc.CarrierType != "",
		HasBuyerTaxID:     doc.RecipientTaxID != domain.B2CBuyerIdentifier,
		OccurredAt:        timeutil.FormatISO8601(now),
	})
}

// reissuedEventPayload 組 P5。
//
// 冪等鍵 `INV:{new_document_id}`（CodeMeta `INVOICE_REISSUED` 逐字）：重開會產生新
// document ⇒ 天然與原票的 `INV:{原 id}` 不同，買家收得到第二則而不會被 notification
// 端的 `UNIQUE (user_id, user_type, event_type, reference_id)` 吞掉。
func (u *Issuance) reissuedEventPayload(
	issuer domain.Issuer, doc, original domain.Document, res *amego.IssueResult, now time.Time,
) (json.RawMessage, error) {
	return json.Marshal(reissuedEvent{
		NotificationRefID: "INV:" + strconv.FormatInt(doc.ID, 10),
		InvoiceDocumentID: strconv.FormatInt(doc.ID, 10),
		OrderNo:           derefString(doc.SourceRef),
		BuyerID:           eventwire.BuyerIDOfDocument(doc),
		StoreID:           eventwire.StoreIDOf(issuer),
		VoidedDocumentID:  strconv.FormatInt(original.ID, 10),
		// 原票號取自**原 document**：doc 是新票，它的號碼是 res.InvoiceNumber。
		// 兩者填反的症狀是買家收到「原發票 X 已作廢並重新開立，新發票號碼 X」——
		// 同一個號碼，而且不會有任何東西報錯。
		VoidedInvoiceNo: derefString(original.ProviderInvoiceNo),
		NewInvoiceNo:    res.InvoiceNumber,
		OccurredAt:      timeutil.FormatISO8601(now),
	})
}

func (u *Issuance) failedEventPayload(issuer domain.Issuer, doc domain.Document, op domain.Operation, failureClass, reasonCode, providerCode string, now time.Time) (json.RawMessage, error) {
	return json.Marshal(issueFailedEvent{
		NotificationRefID:  fmt.Sprintf("INVOP:%d:%d", op.ID, op.AttemptCount),
		InvoiceDocumentID:  strconv.FormatInt(doc.ID, 10),
		InvoiceOperationID: strconv.FormatInt(op.ID, 10),
		OrderNo:            derefString(doc.SourceRef),
		BuyerID:            eventwire.BuyerIDOfDocument(doc),
		StoreID:            eventwire.StoreIDOf(issuer),
		FailureClass:       failureClass,
		FailureReasonCode:  reasonCode,
		// 只放 AMEGO 原碼給稽核，**不放 provider 回傳的原始訊息字串**（可能含賣家憑證片段）。
		ProviderErrorCode: providerCode,
		OccurredAt:        timeutil.FormatISO8601(now),
	})
}

// issuedRoutingKey 依 document_kind 選 P1 或 P6。
//
// RECEIPT 走 invoice.receipt.issued：兩者的通知模板不同（收據不是統一發票，
// 文案不得讓買家以為拿到的是發票）。
func issuedRoutingKey(documentKind string) string {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return pkgmq.RoutingKeyInvoiceReceiptIssued
	}
	return pkgmq.RoutingKeyInvoiceIssued
}

func issuedEventType(documentKind string) pkgmq.EventType {
	if documentKind == codemeta.InvoiceDocumentKindReceipt.Value {
		return pkgmq.EventTypeInvoiceReceiptIssued
	}
	return pkgmq.EventTypeInvoiceIssued
}

// 這裡曾經有一支 `formatOptionalID(v *int64) string`，被 P1／P2 用來填 buyer_id。
// 它已刪除且**不得復活**：recipient_ref_id 是多義欄位（BUYER ⇒ buyer_id、
// SELLER ⇒ store_id、EXTERNAL ⇒ NULL），一支「把 *int64 印成字串」的通用 helper
// 正好把那個多義性藏起來。要 buyer_id 走 eventwire.BuyerIDOfDocument。

// backoffFor 回 2^attempt × base，上限 max（與 pkg/outbox/worker.go 逐字同義）。
//
// 兩份實作是刻意的：INV.op 這一支不走 pkg/outbox.Worker（它的 MarkDone 會覆蓋業務狀態欄），
// 但退避曲線必須與 §7.5.3 的表逐段相同 —— 表上的 60s／120s／240s… 是用這個算式推出來的。
func backoffFor(attempt int, base, max time.Duration) time.Duration {
	d := base
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= max {
			return max
		}
	}
	return d
}
