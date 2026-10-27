package amego

import (
	"errors"
	"strconv"
	"strings"

	gobreaker "github.com/sony/gobreaker/v2"
)

// ErrorClass 是 §4.5.6 的四分類。它決定 worker 怎麼處置一次失敗，**不決定業務**。
type ErrorClass string

const (
	// ClassRetryable：副作用必然未發生，或上游暫時性 ⇒ ProcessFunc 回 error，
	// 由 pkg/outbox 指數退避。
	ClassRetryable ErrorClass = "RETRYABLE"
	// ClassPermanent：請求本身錯，重送必然同樣失敗 ⇒ ProcessFunc 回 nil、
	// operation 落 FAILED_PERMANENT。
	ClassPermanent ErrorClass = "PERMANENT"
	// ClassReconcile：結果不明，必須先對帳（§4.5.7 決策樹）。
	// 這是**未知碼的 default**：多打一次唯讀 invoice_query，代價最小且會自己收斂。
	ClassReconcile ErrorClass = "RECONCILE"
	// ClassManual：帳號／資源／法規層面，程式無法自行恢復 ⇒ 回 nil ＋ 告警。
	// SellerActionRequired 為真時落 SELLER_ACTION_REQUIRED（賣家自己能解），
	// 否則落 FAILED_PERMANENT（只有 ADM 能解）——§7.5.1 與 §4.5.6 唯一有語意差的一格。
	ClassManual ErrorClass = "MANUAL"
)

// error_kind 的四個值（invoice_provider_audit_logs 的 CHECK，§4.5.12）。
// DECODE 必須與其他三者分開：打錯路徑時 AMEGO 回 HTTP 200 ＋ text/html ＋ body 字串
// `error`，吞成「未知錯誤」就再也查不出來。
const (
	ErrorKindTimeout  = "TIMEOUT"
	ErrorKindConn     = "CONNECTION"
	ErrorKindProvider = "PROVIDER_ERROR"
	ErrorKindDecode   = "DECODE"
)

// CodeEnvGuard 是我方的環境閘門碼（§4.5.8 第二層）。它不是 AMEGO 的碼，
// 也永遠不會產生一次外呼 ⇒ 不落 provider audit（那張表記的是「與光貿的交互」）。
const CodeEnvGuard = "ENV_GUARD"

// CodeOther 是未知 provider code 的 metric label。
//
// 🔴 **不得把 AMEGO 原始碼當 label**：它是 7 位數且錯誤碼表已知不完整（3040113、
// 4040145 都不在官方表內，另有 4 支 API 沒有錯誤碼分節，且 AMEGO 不發 changelog）
// ⇒ 無界基數，違反 §8.2.3。已登記的碼用自己的字面值當 label，其餘一律 "other"。
const CodeOther = "other"

// amegoReissuingMarker 是 3040171 兩義的唯一判別依據（§4.5.6a）。
//
// msg 是中文、不是契約 ⇒ 這個比對只是**省一次往返的最佳化，不是正確性的來源**：
// 認不出來時走 ClassReconcile 去問 AMEGO「到底開了沒有」，對帳會給出真相。
const amegoReissuingMarker = "註銷重開中"

// APIError 是本 package 對外的唯一錯誤型別。
//
// 它承載分類與落庫所需的每一欄，但**不承載業務決定**：該作廢還是該折讓、該不該
// 重排，由 usecase / worker 依 Class 自行決定。
type APIError struct {
	Endpoint string
	// Code 是 AMEGO 回的碼，一律字串（折讓類回 String、查詢類回 Number）。
	Code string
	// Msg 是原文，落庫供稅務舉證。**永不用來做分支**（3040171 是唯一例外）。
	Msg        string
	Class      ErrorClass
	Kind       string
	HTTPStatus int
	// SellerActionRequired：M 類中賣家自己解得掉的那一支（3040111／3040191 字軌不足）。
	SellerActionRequired bool
	// AccountLevel：要同時推進 invoice_provider_accounts.status 的帳號層碼。
	AccountLevel bool
	// SuccessEquivalent：這個「錯誤」其實代表目標已達成（3050122／4050132），
	// 或已被開立過而可對帳取回（3040171 永久衝突、4040161 折讓單號重複）。
	SuccessEquivalent bool
	// NotFound：code 71，是正常業務結果不是錯誤（「180 天內確定沒有」）。
	NotFound bool
	// KnownCode：這個碼命中了 §4.5.6／§7.5.1 的登記表。false ⇒ metric label 走 "other"。
	KnownCode bool

	wrapped error
}

func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString("amego: ")
	if e.Endpoint != "" {
		b.WriteString(e.Endpoint)
		b.WriteString(": ")
	}
	b.WriteString("code=")
	b.WriteString(e.Code)
	b.WriteString(" class=")
	b.WriteString(string(e.Class))
	if e.Kind != "" {
		b.WriteString(" kind=")
		b.WriteString(e.Kind)
	}
	if e.Msg != "" {
		b.WriteString(" msg=")
		b.WriteString(e.Msg)
	}
	return b.String()
}

func (e *APIError) Unwrap() error { return e.wrapped }

// MetricCodeLabel 回這次回應在 metric 上的 code label（有界基數）。
func (e *APIError) MetricCodeLabel() string {
	if e.KnownCode {
		return e.Code
	}
	return CodeOther
}

// classification 是一個碼查表後的完整結論。
type classification struct {
	class             ErrorClass
	sellerAction      bool
	accountLevel      bool
	successEquivalent bool
	notFound          bool
	breaker           bool // 計入斷路器
	known             bool
}

// 通用碼（任何 endpoint 皆可能回）。逐條依據 §4.5.6 處置表 ＋ §7.5.1。
var genericCodes = map[string]classification{
	// 系統停機維護中：退避拉長到 15 分鐘級（維護窗可達 4 小時）＋ 告警。
	"10": {class: ClassRetryable, breaker: true, known: true},
	// invoice 不可為空：我方 bug（Content-Type 用錯也回這個碼）。
	"11": {class: ClassPermanent, known: true},
	// invoice 錯誤：該 provider 帳號轉 INVALID_CREDENTIAL。
	// **不計入斷路器**——單一賣家不得阻斷全平台。
	"12": {class: ClassManual, accountLevel: true, known: true},
	// status 未啟用（PROVIDER_REJECTED），無自助恢復途徑。
	"13": {class: ClassManual, accountLevel: true, known: true},
	// IP 錯誤：**全域故障**，我方出口 IP 不在白名單。
	// 歸 R 不歸 M 是刻意的：M 會把故障期間每一筆 operation 都變成需人工逐筆重排的
	// 死件，而它修好後會全部自動成功。計入斷路器（這是 AMEGO 側的連線健康）。
	"14": {class: ClassRetryable, breaker: true, known: true},
	// time 錯誤：clock.Resync 後重送同一請求一次；連續發生 ⇒ NTP 告警。
	"15": {class: ClassRetryable, known: true},
	// 簽名驗證錯誤：首次視為 P（多半是序列化做了兩次）。
	// 「同一賣家連續 3 次 ⇒ 轉 M」是呼叫端的計數，不是單次回應能判斷的事。
	"16": {class: ClassPermanent, known: true},
	"17": {class: ClassPermanent, known: true}, // data 為空
	"18": {class: ClassRetryable, breaker: true, known: true},
	"19": {class: ClassManual, accountLevel: true, known: true}, // 公司停權
	"20": {class: ClassPermanent, known: true},                  // data 非 JSON
	// 人數過多：指數退避 ＋ 我方限速自動降檔。無 Retry-After、無 X-RateLimit-*。
	"21": {class: ClassRetryable, breaker: true, known: true},
	"22": {class: ClassManual, accountLevel: true, known: true}, // 尚未申請 API 串接
	"23": {class: ClassPermanent, known: true},                  // data 應為陣列

	// ── 查詢／驗證類共用碼 ──
	// 超過 180 天查詢期限 ⇒ 改走無時間限制的 invoice_list（呼叫端決定）。
	"51": {class: ClassPermanent, known: true},
	// 有等待異動的排程 ⇒ 退避。
	"52": {class: ClassRetryable, known: true},
	// 查無資料：**這不是錯誤**，是「180 天內確定沒有」。
	"71": {class: ClassPermanent, notFound: true, known: true},
	// 分頁參數越界：我方 bug（limit 只收 20~500）。
	"34": {class: ClassPermanent, known: true},
	// 一碼多義，只能解析中文 msg ⇒ 一律告警，不自動處理。
	"99": {class: ClassManual, known: true},
}

// 端點專屬的精確碼。key 是 code 字面值。
//
// ⚠️ **精確碼一律優先於區間**。這不是我方發明的規則：§7.5.1 自己就把 3050131 列進
// RETRYABLE，同時把含它的 3050121~3050141 列進 PERMANENT ——精確列勝出是該表自身
// 的內部規則。
var exactCodes = map[string]classification{
	// f0401：字軌不足／無法取得下一張 ⇒ **重試多久都不會好**。
	// 落 SELLER_ACTION_REQUIRED（需賣家去財政部取號），不是 FAILED_PERMANENT：
	// 兩者的通知路徑與 ADM 佇列不同，混成同一個值就分不出該找誰。
	"3040111": {class: ClassManual, sellerAction: true, known: true},
	"3040191": {class: ClassManual, sellerAction: true, known: true},
	// f0401：data 形狀錯，我方 bug。3040113 不在官方錯誤碼表內（實測發現）。
	"3040112": {class: ClassPermanent, known: true},
	"3040113": {class: ClassPermanent, known: true},

	// f0501：CancelReason 過長（>20 字）⇒ 我方送出前截斷。
	"3050113": {class: ClassPermanent, known: true},
	// f0501：等待開立／等待開立·作廢·註銷 ⇒ 上一個異動還在排隊，延遲 30~60 秒重試。
	"3050121": {class: ClassRetryable, known: true},
	"3050131": {class: ClassRetryable, known: true},
	// f0501：已存在開立作廢 ⇒ 查詢確認後**視為成功**。
	"3050122": {class: ClassReconcile, successEquivalent: true, known: true},
	// f0501：已存在註銷 ⇒ 狀態機衝突，人工。
	"3050123": {class: ClassManual, known: true},
	// f0501：發票不存在 ⇒ 我方資料錯，告警。
	"3050125": {class: ClassPermanent, known: true},
	// f0501：已超過修改期限／已存在折讓單 ⇒ 法定窗口已關，usecase 自動改開全額折讓。
	"3050126": {class: ClassPermanent, known: true},
	"3050141": {class: ClassPermanent, known: true},

	// g0401：AllowanceNumber 重複 ⇒ allowance_query 取回既有折讓單，**視為成功**。
	//
	// ⚠️ 它落在 §7.5.1 的 4040151~4040163 PERMANENT 區間內，但 §4.5.6 明列為 Q。
	// 取 Q：它與 3040171 是同一個形狀（冪等鍵撞號），判 P 會丟掉一張已經送到國稅局
	// 的折讓單，正是 W-17 要擋的失敗。
	"4040161": {class: ClassReconcile, successEquivalent: true, known: true},
	// g0401：折讓金額（含累計）大於原發票 ⇒ DB CHECK 就該擋掉，走到這裡代表不變式有洞。
	"4040171": {class: ClassPermanent, known: true},
	"4040173": {class: ClassPermanent, known: true},

	// g0501：已存在作廢折讓 ⇒ 視為成功。
	"4050132": {class: ClassReconcile, successEquivalent: true, known: true},
	// g0501：折讓單已超過修改期限 ⇒ ADM 介入。
	"4050135": {class: ClassManual, known: true},
	// g0501：等待作廢 ⇒ 退避重試。
	"4050141": {class: ClassRetryable, known: true},
	// 4050131「折讓開立中」與 4050141 是**同一個暫時狀態的兩半**：第三方文件
	// 《AMEGO 電子發票 API 總覽》明文寫「4050131 ＋ 4050141 ⇒ 剛開立的折讓單不能
	// 立刻作廢，必須等 allowance_status 的 status 走到 99：完成」。
	//
	// ⚠️ 這是對 §7.5.1 字面的**刻意偏離**（2026-08-29）：該節只把 4050141 列進
	// RETRYABLE，4050131 留在 4050131~4050141 的 PERMANENT 區間。而第三方文件自己的
	// 兩張摘要表也互相矛盾（一張只列 4050141 進「等待後重試」，另一張把整個區間列進
	// 「不可重試」）——**語意陳述勝過區間摘要**，這與本檔既有的「精確碼勝過區間」
	// 同一條原則。
	//
	// 兩種判法的代價不對稱：判 P ⇒ 一個本來會自己成功的作廢落進 FAILED_PERMANENT、
	// 要 ADM 手動重排；判 R ⇒ 真的永久錯時也只是走完退避階梯（MaxAttempts=13，
	// 約 4 小時）後落 FAILED_RETRY_EXHAUSTED，同樣看得見。取 R。
	"4050131": {class: ClassRetryable, known: true},

	// barcode：手機條碼 ⇒ 送出前就該用 /json/barcode 驗過。
	"9000111": {class: ClassPermanent, known: true},
	"9000112": {class: ClassPermanent, known: true},
	"9000113": {class: ClassPermanent, known: true},
}

// codeRange 是 §4.5.6／§7.5.1 以區間表述的那幾組。
//
// 🔴 **刻意不使用 `304xxxx` 這種前綴規則**：那會把杜撰的 3049999 也吃進 PERMANENT，
// 而未知碼的正解是 ClassReconcile（丟掉一張本來開得出來的發票 vs 多打一次唯讀查詢，
// 代價差一個數量級）。區間必須是閉區間、逐組登記。
var codeRanges = []struct {
	lo, hi int
	cls    classification
}{
	// f0401 買方欄位／載具／捐贈碼／品項／金額格式 ⇒ §4.5.5 就該擋掉。
	{3040121, 3040163, classification{class: ClassPermanent, known: true}},
	// f0401 金額計算錯誤／不應為負 ⇒ 我方算式與 AMEGO 不一致，是 bug 不是資料問題。
	{3040172, 3040184, classification{class: ClassPermanent, known: true}},
	// f0501 狀態機不允許（精確碼已在 exactCodes 覆蓋其中六個）。逐字取 §7.5.1 的
	// 3050121~3050141，不擅自往下擴到 3050111／3050112——那兩個碼兩張分類表都沒有，
	// 走 default 的 Reconcile 比我方自行推定安全。
	{3050121, 3050141, classification{class: ClassPermanent, known: true}},
	// g0401 折讓欄位格式／長度／AllowanceType ⇒ 我方送出前擋。
	{4040121, 4040145, classification{class: ClassPermanent, known: true}},
	// g0401 狀態機類（4040161 已由 exactCodes 覆蓋為 Q）。
	{4040151, 4040163, classification{class: ClassPermanent, known: true}},
	// g0501 狀態機類（4050131／4050132／4050135／4050141 已由 exactCodes 覆蓋）。
	{4050131, 4050141, classification{class: ClassPermanent, known: true}},
}

// classify 把 (endpoint, code, msg) 映射成四分類。
//
// 🔴 **必須有 default 分支且一律回 ClassReconcile**（§4.5.6）。上表已知不是窮舉：
// 3040113、4040145 都不在官方錯誤碼表內，另有 4 支 API 完全沒有錯誤碼分節。
// default 選 Permanent ⇒ 丟掉一張本來開得出來的發票；選 Retryable ⇒ 白燒法定時限；
// 選 Reconcile ⇒ 多打一次唯讀查詢，由對帳結果決定真相。
func classify(endpoint, code, msg string) classification {
	// 3040171 一碼兩義，是全表唯一以 msg 分支的地方。
	if code == "3040171" {
		if strings.Contains(msg, amegoReissuingMarker) {
			// 「該訂單編號正在註銷重開中」＝暫時態。
			return classification{class: ClassRetryable, known: true}
		}
		// 其餘一律 Reconcile（**不是 Retryable**）：認不出措辭時去問 AMEGO
		// 「到底開了沒有」，對帳取回既有發票號碼 ⇒ 視為成功。
		return classification{class: ClassReconcile, successEquivalent: true, known: true}
	}
	if c, ok := exactCodes[code]; ok {
		return c
	}
	if c, ok := genericCodes[code]; ok {
		return c
	}
	if n, err := strconv.Atoi(code); err == nil {
		for _, r := range codeRanges {
			if n >= r.lo && n <= r.hi {
				return r.cls
			}
		}
	}
	_ = endpoint // endpoint 目前不改變分類；保留在簽章內是因為落庫必須 endpoint+code+msg 三者同時記。
	return classification{class: ClassReconcile}
}

// newProviderError 由一次 code != 0 的回應建構 APIError。
func newProviderError(endpoint, code, msg string, httpStatus int) *APIError {
	c := classify(endpoint, code, msg)
	return &APIError{
		Endpoint:             endpoint,
		Code:                 code,
		Msg:                  msg,
		Class:                c.class,
		Kind:                 ErrorKindProvider,
		HTTPStatus:           httpStatus,
		SellerActionRequired: c.sellerAction,
		AccountLevel:         c.accountLevel,
		SuccessEquivalent:    c.successEquivalent,
		NotFound:             c.notFound,
		KnownCode:            c.known,
	}
}

// countsTowardBreaker 決定一次失敗要不要餵給斷路器。
//
// 斷路器的目的是「AMEGO 掛了就不要繼續搥它」，代價是開著的期間本地拒絕每一次開票
// ⇒ 只有「AMEGO 自己的健康」可以計入。一個賣家送錯統編不該讓其他賣家開不出發票
// （pkg/sms 因 issue #2108 學到的同一課）。
//
// 用排除而不是「回報成功」：後者會把 ConsecutiveFailures 歸零，於是真實故障期間夾雜
// 幾筆格式錯誤就能讓斷路器永遠跳不起來。
func countsTowardBreaker(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		// 非 APIError 一律當成傳輸層問題計入（不該發生，但保守）。
		return true
	}
	switch apiErr.Kind {
	case ErrorKindTimeout, ErrorKindConn, ErrorKindDecode:
		// 傳輸層錯誤、非 2xx、JSON 解析失敗。
		return true
	case ErrorKindProvider:
		c := classify(apiErr.Endpoint, apiErr.Code, apiErr.Msg)
		return c.breaker
	default:
		// ENV_GUARD 這類我方閘門：根本沒有外呼，與 AMEGO 的健康無關。
		return false
	}
}

// isExcludedFromBreaker 是 gobreaker.Settings.IsExcluded 的實作。
func isExcludedFromBreaker(err error) bool { return !countsTowardBreaker(err) }

// asOpenStateError 把 gobreaker.ErrOpenState 映射成 ClassRetryable。
//
// 開路時 operation **不得標成失敗**（W-18）：上游暫時不可用被記成該筆的永久失敗
// ⇒ 法定期限內、完全可補救的票直接進終態。
func asOpenStateError(endpoint string, err error) *APIError {
	if !errors.Is(err, gobreaker.ErrOpenState) && !errors.Is(err, gobreaker.ErrTooManyRequests) {
		return nil
	}
	return &APIError{
		Endpoint: endpoint,
		Code:     "BREAKER_OPEN",
		Msg:      err.Error(),
		Class:    ClassRetryable,
		Kind:     ErrorKindConn,
		wrapped:  err,
	}
}
