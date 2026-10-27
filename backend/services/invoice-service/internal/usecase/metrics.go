package usecase

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// usecase 層自己的兩支業務指標：M13／M15（§8.2.3）
//
// 🔴 這兩支在 2026-09-05 之前**全 repo 零生產者**——設計書列了它們、
// business-alerts.yml 有對應告警、query.go 與 §8.1.5(e) 的註解也提到它們，
// 而沒有任何 code 註冊或遞增。Prometheus 對不存在的序列不會 fire，且
// `increase(X[15m]) > 20` 在 no data 與「一切正常」上輸出完全一樣
// ⇒ InvoicePiiRevealSpike 與 InvoicePdfMissing 是永久靜默的擺設。
//
// 走 package-level promauto（與 internal/mq/metrics.go、internal/client/amego/metrics.go
// 同一個理由）：**指標放在它唯一的觀測點所在的 package**。M13／M14 過去被放在
// internal/worker 上而觀測點在別的 package，結果是十個月沒有被遞增過一次。
// ---------------------------------------------------------------------------

// M13：ADM 揭露 P 類資料的次數。
//
// §8.1.3(c) 裁定 2 的代價那一半：「ADM 一律不遮罩，代價由稽核承擔」——
// 稽核指的是 invoice_admin_audit_logs 的列 **＋ 這支計數器**。只有列而沒有計數器時，
// 「有沒有人在大量翻買家資料」要有人主動去 query 才看得到，而沒有人會主動去。
//
// **無 label**：actor_account_id 是無界基數（§8.2.3 label 紅線）。「是誰」由告警觸發
// 之後撈同一時窗的 invoice_admin_audit_logs 依 actor_account_id 分組回答。
//
// ⚠️ 它偵測不到走既有訂單端點看同一份資料的旁路 ⇒ **不得據此宣稱「沒有人大量看過
// 買家資料」**（告警 ⑩ 的 description 逐字寫著同一句）。
// 缺一半的平台 AMEGO 憑證（#3269）。
//
// AMEGO_PLATFORM_INVOICE 與 AMEGO_PLATFORM_APP_KEY **兩者皆為選填**（前者自 #3230 起，
// 後者一直是 optional secret）⇒ **只有一半**是完全合法的啟動狀態，而它的後果是
// barcode verifier 靜默為 nil：
// 買家送一個明顯無效的手機條碼，HTTP 200 建立成功、零錯誤、零 log（2026-09-08 dev 實測）。
//
// 🔴 為什麼是 gauge 不是 fail-fast：prod 現在就缺這個 key，把它改成 required 會讓
// prod 一部署就 boot 失敗——那是把靜默失效換成全服務中斷，成本更高（backend#2813 同一課）。
// 這一支只負責讓它**被看見**；把 key 補上是 #3269 的另一半，需要向光貿取得正式憑證。
//
// ⚠️ 它守的是「配對缺一半」，不是「憑證正確」。AppKey 填錯字仍然是 0——那要等
// 第一次呼叫 /json/barcode 才會知道（M3 的 invoice_amego_response_code_total）。
var platformCredentialIncomplete = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "invoice_platform_credential_incomplete",
	Help: "1 when AMEGO platform credential is half-configured (invoice set, app key missing) and the mobile-barcode verifier is silently disabled",
})

// 手機條碼載具的存在性驗證有沒有掛上（#3230 M34）。
//
// 掛載條件**逐字等於** usecase/di.go 的第一個 case：AMEGO_PLATFORM_INVOICE 與
// AMEGO_PLATFORM_APP_KEY 兩者皆非空。任一為空 ⇒ 0 ⇒ 買家送的手機條碼不會被驗存在性，
// 無效載具被 HTTP 200 接受，錯誤要到開票前那一層才會被擋下——而平台未開票時永遠不會擋。
//
// 🔴 **這支不是「平台開票能力」的指標。** 平台自身開票用的 AppKey 來自 **DB**
// （usecase/submit.go 的 accounts.GetByIssuer ＋ WithDecryptedCredential），不是這兩個 env。
// 本指標 ＝ 0 時平台開票可能完全正常，不得據此宣稱「平台 issuer 停了」。
//
// 與 platformCredentialIncomplete 的分工：那支答「配對是不是只有一半」（＝配置錯誤），
// 這支答「verifier 到底掛上了沒」（＝功能可不可用）。兩半都缺時前者是 0（那不是錯誤，
// 是未啟用）而後者也是 0（功能確實不可用）——#3230 放寬必填後 prod 正好落在這一格，
// **只看前者會以為一切正常**，那正是需要這支的理由。
var mobileBarcodeVerifierEnabled = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "invoice_mobile_barcode_verifier_enabled",
	Help: "1 when the AMEGO platform credential pair is complete and the mobile-barcode existence verifier is wired; 0 means invalid carriers are accepted unchecked",
})

var adminPIIReveal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "invoice_admin_pii_reveal_total",
	Help: "ADM reveals of P-class invoice fields (58 M13)",
})

// M15：讀取時發現 pdf_object_key 對應的物件不存在。
//
// 守的是 §8.1.5(e) 的 asset_reference 到底有沒有真的建立：沒建的話 media GC 會在
// 7 天後把 R2 物件連同紀錄刪掉，而**AMEGO 的 invoice_file 只能查 180 天內**
// ⇒ 超過就再也抓不回來。這支是那件事唯一的事後檢查，時效性很硬。
//
// **無 label**：document_id／store_id 都是無界基數。「是哪一張」由同時刻的
// `invoice.document_file.media_read_failed` Error log 回答（它帶 document_id 與
// object_key），兩條缺一不可。
var documentsPDFMissing = promauto.NewCounter(prometheus.CounterOpts{
	Name: "invoice_documents_pdf_missing_total",
	Help: "Reads that found the stored pdf_object_key missing from media (58 M15)",
})

// M16：累計退款超過已開票金額（§6.3.2.3 的 clamp 觸發）。
//
// **真實業務異常，不是技術故障**：折讓本身沒有被擋（clamp 之後照樣開），
// 成因通常是同一張訂單開了兩張售後單而上游的 refundable_amount 沒有互扣。
//
// **無 label**（label 基數紅線）⇒「是哪一家、哪一張」由**同時刻的 Error log**
// `invoice.allowance.refund_exceeds_invoiced` 回答（帶 order_no／document_id／
// cumulative_refund_amount／invoiced_total／clamped_to）。§8.2.3 M16 逐字寫著
// 「那則 Error log 不是選配：只刪 label 不補 log 比帶著 label 更糟」。
var refundExceedsInvoiced = promauto.NewCounter(prometheus.CounterOpts{
	Name: "invoice_refund_exceeds_invoiced_total",
	Help: "Refunds whose cumulative amount exceeded the invoiced total and were clamped (58 M16)",
})

// init 讓三條序列在第一次觀測之前就存在。
//
// counter 在第一次觀測之前不會出現在 /metrics，而告警的 selector 對**不存在的序列**
// 永遠不會 fire。驗收判準是「序列存在」不是「值為 0」——值為 0 才是正確的初始狀態。
func init() {
	adminPIIReveal.Add(0)
	documentsPDFMissing.Add(0)
	refundExceedsInvoiced.Add(0)
}

// observeAdminPIIReveal 遞增 M13。呼叫點在 recordPIIReveal **成功之後**：
// 稽核列寫不進去時揭露不會發生（fail-closed），計進去會高估揭露次數。
func observeAdminPIIReveal() { adminPIIReveal.Inc() }

// observeDocumentPDFMissing 遞增 M15。
func observeDocumentPDFMissing() { documentsPDFMissing.Inc() }

// observeRefundExceedsInvoiced 遞增 M16。呼叫端必須在同一處補那則 Error log。
func observeRefundExceedsInvoiced() { refundExceedsInvoiced.Inc() }
