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

type classification struct {
	class             ErrorClass
	sellerAction      bool
	accountLevel      bool
	successEquivalent bool
	notFound          bool
	breaker           bool
	known             bool
}

var genericCodes = map[string]classification{
	"10": {class: ClassRetryable, breaker: true, known: true},
	"11": {class: ClassPermanent, known: true},
	"12": {class: ClassManual, accountLevel: true, known: true},
	"13": {class: ClassManual, accountLevel: true, known: true},
	"14": {class: ClassRetryable, breaker: true, known: true},
	"15": {class: ClassRetryable, known: true},
	"16": {class: ClassPermanent, known: true},
	"17": {class: ClassPermanent, known: true},
	"18": {class: ClassRetryable, breaker: true, known: true},
	"19": {class: ClassManual, accountLevel: true, known: true},
	"20": {class: ClassPermanent, known: true},
	"21": {class: ClassRetryable, breaker: true, known: true},
	"22": {class: ClassManual, accountLevel: true, known: true},
	"23": {class: ClassPermanent, known: true},

	"51": {class: ClassPermanent, known: true},
	"52": {class: ClassRetryable, known: true},
	"71": {class: ClassPermanent, notFound: true, known: true},
	"34": {class: ClassPermanent, known: true},
	"99": {class: ClassManual, known: true},
}

var exactCodes = map[string]classification{
	"3040111": {class: ClassManual, sellerAction: true, known: true},
	"3040191": {class: ClassManual, sellerAction: true, known: true},
	"3040112": {class: ClassPermanent, known: true},
	"3040113": {class: ClassPermanent, known: true},

	"3050113": {class: ClassPermanent, known: true},
	"3050121": {class: ClassRetryable, known: true},
	"3050131": {class: ClassRetryable, known: true},
	"3050122": {class: ClassReconcile, successEquivalent: true, known: true},
	"3050123": {class: ClassManual, known: true},
	"3050125": {class: ClassPermanent, known: true},
	"3050126": {class: ClassPermanent, known: true},
	"3050141": {class: ClassPermanent, known: true},

	"4040161": {class: ClassReconcile, successEquivalent: true, known: true},
	"4040171": {class: ClassPermanent, known: true},
	"4040173": {class: ClassPermanent, known: true},

	"4050132": {class: ClassReconcile, successEquivalent: true, known: true},
	"4050135": {class: ClassManual, known: true},
	"4050141": {class: ClassRetryable, known: true},
	"4050131": {class: ClassRetryable, known: true},

	"9000111": {class: ClassPermanent, known: true},
	"9000112": {class: ClassPermanent, known: true},
	"9000113": {class: ClassPermanent, known: true},
}

var codeRanges = []struct {
	lo, hi int
	cls    classification
}{
	{3040121, 3040163, classification{class: ClassPermanent, known: true}},
	{3040172, 3040184, classification{class: ClassPermanent, known: true}},
	{3050121, 3050141, classification{class: ClassPermanent, known: true}},
	{4040121, 4040145, classification{class: ClassPermanent, known: true}},
	{4040151, 4040163, classification{class: ClassPermanent, known: true}},
	{4050131, 4050141, classification{class: ClassPermanent, known: true}},
}

func classify(endpoint, code, msg string) classification {
	if code == "3040171" {
		if strings.Contains(msg, amegoReissuingMarker) {
			return classification{class: ClassRetryable, known: true}
		}
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
	_ = endpoint
	return classification{class: ClassReconcile}
}

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

func countsTowardBreaker(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return true
	}
	switch apiErr.Kind {
	case ErrorKindTimeout, ErrorKindConn, ErrorKindDecode:
		return true
	case ErrorKindProvider:
		c := classify(apiErr.Endpoint, apiErr.Code, apiErr.Msg)
		return c.breaker
	default:
		return false
	}
}

func isExcludedFromBreaker(err error) bool { return !countsTowardBreaker(err) }

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
