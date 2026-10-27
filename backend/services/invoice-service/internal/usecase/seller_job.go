package usecase

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 本檔是 §7.4 A4「受控人工出口」最後一步的賣家入口：§6.1.10 端點 62
// （POST /app/seller/invoices/jobs/{job_id}/external_issuances ＝ §7.3.2 的 J30）。
//
// 🔴 它與 ADM 代錄（端點 56）**共用同一段 domain 邏輯**（§6.1.14 逐字「兩支都要走同一個
// transition helper，不得各寫一份 CAS」）。所以本型別持有 *AdminJob 並呼叫它的
// resolveExternally，而不是把那四件事（operation CAS ＋ document CAS ＋ 兩列狀態歷程 ＋
// outbox P12）再抄一份。抄一份的後果不是重複碼，是**兩份會漂移**——而漂移的那一天，
// 兩個入口對同一件事寫出不同的資料。
//
// 為什麼是新型別而不是 AdminJob 上多一個方法：歸屬鍵不同。ADM 三支的述詞裡沒有
// store_id（ADM 本來就看得到全部案件），本支的每一次讀取都必須帶 store_id 進 SQL 的
// WHERE。同一個型別上並存兩種歸屬語意，正是 Tier A 最容易破的地方。
//
// 🔴 **這一支對停權賣家放行**（§3 D16）：它是賣家履行法定開票義務的唯一回報管道，
// 而且它**完全不碰 AMEGO**——只記錄一件已經發生在平台之外的事實。「賣家沒有重試端點」
// 與「賣家有回報端點」不衝突，差別就是有沒有打 provider。

const (
	// sellerExternalEvidenceMax 是 evidenceAssetKeys 的筆數上限（§6.1 端點 62 逐字的 ≤ 5）。
	sellerExternalEvidenceMax = 5
)

// 型別邊界錯誤（契約違規 ⇒ 400，不借業務碼）。
//
// 🔴 號碼格式不合回 **400 而不是 9349**（§6.1 端點 62 逐字）：9349 的語意是「本作業尚未
// 開放自行開立回報」，賣家的下一步是等 ADM 解鎖；格式錯的下一步是把號碼重打一次。
var (
	ErrExternalEvidenceTooMany = errors.New("invoice: evidence_asset_keys exceeds 5 items")
	ErrExternalEvidenceInvalid = errors.New("invoice: evidence_asset_keys must be media object keys, not URLs")
)

// SellerJob 是端點 62 的寫入 usecase。
type SellerJob struct {
	jobs *AdminJob
	ops  domain.OperationRepository
	docs domain.DocumentRepository
}

// NewSellerJob 組出端點 62 的寫入 usecase。
//
// 它刻意**不持有自己的 TxManager**：交易邊界必須與 resolveExternally 用的是同一把，
// 否則那四件事會落在兩個交易裡，而「只轉 operation 不轉 document」正是它要防的組合。
func NewSellerJob(jobs *AdminJob, ops domain.OperationRepository, docs domain.DocumentRepository) *SellerJob {
	return &SellerJob{jobs: jobs, ops: ops, docs: docs}
}

// ReportExternalIssuanceInput 是端點 62 的輸入。
type ReportExternalIssuanceInput struct {
	StoreID           int64
	JobID             int64
	InvoiceNo         string
	InvoiceDate       string // YYYY-MM-DD（UTC+8 曆日）
	EvidenceAssetKeys []string
	Note              string
	ExpectedVersion   int64
	IdempotencyKey    string
}

// ReportExternalIssuance 讓賣家回報自己開立的發票號碼（端點 62 ＝ J30）。
//
// 🔴 三條不可協商的落點（與端點 56 逐字相同，因為走的是同一個 helper）：
//
//	operation.status → RESOLVED_EXTERNALLY（終態）
//	document.status  → RESOLVED_EXTERNALLY，號碼寫進 external_invoice_no
//	provider_invoice_no **必須留空** —— 平台不冒充開立人；責任歸屬在爭議時是法律問題
//
// 🔴 唯一的來源狀態是 MANUAL_PROCESS_ALLOWED（ADM 雙人解鎖的產物）。不在那一格就回
// 9349，**不是**靜默成功：賣家按了沒反應而 operation 永遠停在非終態，買家端永遠顯示
// 「處理中」，法定期限照樣倒數。
func (u *SellerJob) ReportExternalIssuance(ctx context.Context, in ReportExternalIssuanceInput) (domain.OperationView, error) {
	if !externalInvoiceNoPattern.MatchString(in.InvoiceNo) {
		return domain.OperationView{}, ErrExternalInvoiceNoInvalid
	}
	invoiceDate, err := time.Parse(time.DateOnly, in.InvoiceDate)
	if err != nil {
		return domain.OperationView{}, ErrExternalInvoiceDateInvalid
	}
	if utf8.RuneCountInString(in.Note) > adminNoteMaxRunes {
		return domain.OperationView{}, ErrJobNoteTooLong
	}
	if err := validateEvidenceAssetKeys(in.EvidenceAssetKeys); err != nil {
		return domain.OperationView{}, err
	}

	var out domain.OperationView
	err = u.jobs.tx.WithTx(ctx, func(ctx context.Context) error {
		// 🔴 GetViewForSeller 不是 GetByID：store_id 是述詞的一部分（經 invoice_documents
		// → invoice_issuers 解析），越權與查無由同一行程式碼回同一個 9340。
		// invoice_operations 沒有 store_id 欄，擁有者判定一律經 invoice_documents（§8.1.4 末段）。
		view, err := u.ops.GetViewForSeller(ctx, in.JobID, in.StoreID)
		if err != nil {
			return err
		}
		op := view.Operation
		if op.Version != in.ExpectedVersion {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		doc, err := u.docs.GetByID(ctx, op.DocumentID)
		if err != nil {
			return err
		}
		// 9349 的語意逐字就是這一格：「本作業尚未開放自行開立回報」。
		//
		// 🔴 **走共用的 canReportExternalIssuance，不在這裡自己比一次**：
		// 這個判斷有兩個呼叫端（端點 56 ADM／端點 62 賣家），2026-09-05 修 #3166 時
		// 我只改了 admin_job.go 那一份，dev 實測仍回 9349 —— 兩份各自比對就是
		// 「同一個判準兩份實作」，改一份不會讓另一份轉紅。
		if !canReportExternalIssuance(op, doc) {
			return jobBiz(bizcode.InvoiceExternalIssuanceNotAllowed)
		}
		// 發票日不得早於付款日（§6.1 端點 62 的 400 清單）。issue_not_before ＝ 收款時點，
		// 兩者的曆日都以 UTC+8 比較——拿 UTC 日期比會讓台北時間 08:00 之前的付款被誤判。
		if invoiceDate.Before(domain.InvoiceDateTaipei(doc.IssueNotBefore)) {
			return ErrExternalInvoiceDateTooEarly
		}

		now := u.jobs.now()
		if rerr := u.jobs.resolveExternally(ctx, op, doc, externalIssuance{
			InvoiceNo:   in.InvoiceNo,
			InvoiceDate: invoiceDate,
			// ⚠️ 端點 62 的 request **沒有** totalAmount（proto 逐字），所以金額取該
			// document 的既有總額。端點 56（ADM 代錄）才收 ADM 打進來的數字。
			TotalAmount: doc.TotalAmount,
			Note:        externalNoteWithEvidence(in.Note, in.EvidenceAssetKeys),
			// actor 是 SELLER：P12 事件本身不帶 actor（對買家而言兩種情形是同一個事實），
			// 誰做的留在 invoice_status_history 供稽核。
			ActorType:      providerActorSeller,
			ActorAccountID: in.StoreID,
			Now:            now,
		}); rerr != nil {
			return rerr
		}

		// ⚠️ **不寫 invoice_admin_audit_logs**：那張表是 ADM 動作的 5 年保存紀錄
		// （resource 的 actor 恆為管理員）。賣家的動作留在 invoice_status_history，
		// 兩者的保存期與讀者都不同，混寫會讓 A11 的稽核清單出現不是 ADM 做的事。

		// 重讀而不是就地拼裝：status 與 version 的新值由 SQL 算出。
		updated, err := u.ops.GetViewForSeller(ctx, in.JobID, in.StoreID)
		if err != nil {
			return err
		}
		out = updated
		return nil
	})
	if err != nil {
		return domain.OperationView{}, err
	}
	return out, nil
}

// validateEvidenceAssetKeys 檢查佐證截圖的形狀。
//
// 🔴 只收 objectKey、不收 URL（《多媒體與圖片 URL 規範》／49 號）：收 URL 回存的後果是
// 有人把外部網址塞進來，而下游組 URL 時會把它原樣吐給買家或 ADM。
//
// ⚠️ **本波沒有把這些 key 落進獨立欄位**：invoice_documents 沒有承載外部開立佐證的欄位
// （§5 逐欄查過），它們目前只出現在 invoice_status_history 的 reason_detail 裡——
// 與外部發票金額同一個處置（externalHistoryDetail 的註解逐字）。schema 缺口登記在交付回報。
func validateEvidenceAssetKeys(keys []string) error {
	if len(keys) > sellerExternalEvidenceMax {
		return ErrExternalEvidenceTooMany
	}
	for _, k := range keys {
		if k == "" ||
			strings.HasPrefix(k, "http://") || strings.HasPrefix(k, "https://") ||
			strings.HasPrefix(k, "/") || strings.Contains(k, "..") {
			return ErrExternalEvidenceInvalid
		}
	}
	return nil
}

// externalNoteWithEvidence 把佐證 key 併進歷程備註。
//
// 這不是美化：那幾把 key 是「賣家說他開了這張票」的唯一佐證，而歷程表是 append-only、
// 保存期同稽核。放進 note 之後 externalHistoryDetail 會連同號碼／日期／金額一起寫進去。
func externalNoteWithEvidence(note string, keys []string) string {
	if len(keys) == 0 {
		return note
	}
	joined := "evidence=" + strings.Join(keys, ",")
	if note == "" {
		return joined
	}
	return note + " / " + joined
}
