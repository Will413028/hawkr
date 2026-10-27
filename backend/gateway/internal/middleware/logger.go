package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	yaml "go.yaml.in/yaml/v2"
)

const maxBodyLogSize = 2000

const redactedValue = `"[REDACTED]"`

// sensitiveKeys／allowedKeys（見下方 shouldRedact）是這支 middleware 對已知
// JSON schema 欄位名做的 default-deny 判定，跟 pkg/logger/redact.go 的通用
// pattern backstop 是互補、刻意不合併的兩層：那邊不知道任何欄位名稱，是
// 所有 Go service 共用、對自由文字的最後一道防線；這裡看得到已知 DTO 的
// 欄位名，才能做「預設遮蔽、明確放行」而不是反過來猜。新增敏感欄位時，
// 屬於某個已知 HTTP DTO 才進這裡；跨 service 的通用密鑰命名慣例才進
// pkg/logger——不要把兩邊合併成一份清單。
var sensitiveKeys = map[string]struct{}{
	// 認證憑證
	"password":             {},
	"token":                {},
	"accesstoken":          {},
	"refreshtoken":         {},
	"refreshtokenenc":      {},
	"idtoken":              {},
	"firebasetoken":        {},
	"firebaseidtoken":      {},
	"firebasephoneidtoken": {},
	"identitytoken":        {},
	"tempbindtoken":        {},
	"authtoken":            {},
	"authorization":        {},
	"secret":               {},
	"apikey":               {},
	// 一次性驗證碼／OAuth authorization code。跟 password 同一類：拿到就能在有效
	// 期內冒充完成登入或換綁，不是「分類未完成的一般欄位」。auth.go 既有的
	// zap.Int("otpCodeLen", len(req.OtpCode)) 已經證明這條路徑上的維護者早就知道
	// 這個值不該整個印出來，只是這支 middleware 從沒補上對應的 key。
	"otpcode":           {},
	"authorizationcode": {},
	// 金流 token
	"applepaytoken":  {},
	"googlepaytoken": {},
	// 認證身分 / 銀行 PII
	"idcardno":       {},
	"idcardfronturl": {},
	"idcardbackurl":  {},
	"taxid":          {},
	"bankaccount":    {},
	"bankbookurl":    {},
	// KYC 個資（姓名 / 生日）——normalizeKey 去底線與大小寫後涵蓋
	// real_name / realName / RealName、birth_date / birthDate / BirthDate 各形態。
	"realname":  {},
	"birthdate": {},
	// 訪客購物車識別碼。它不是 PII，但它**是憑證**：後端對匿名購物車的讀 / 改 /
	// 刪 / merge 全部只認這一個值，沒有簽章也沒有擁有者檢查（#2295 ③）。落進
	// access log 等於把「誰能操作那台裝置的購物車」寫進任何有 Loki 讀取權的人
	// 眼前。normalizeKey 收斂後同時涵蓋 guest_id 與 guestId。
	"guestid": {},
	// 代行營運的兩把明文憑證（56 §8.1.6）。判準與 guestId 同一條：問「拿到它能做
	// 什麼」而不是「它含不含個資」——operationCredential 單獨持有即可對目標店家執行
	// 該工作階段的每一項能力，handoffCode 則能換出前者。它們唯二合法的出口是 S2
	// create 與 S6 redeem 的回應 body，而 access log 同時記 req_body 與 resp_body，
	// 兩支端點的請求與回應正好各洩漏一把。normalizeKey 收斂後同時涵蓋
	// operation_credential／operationCredential 與 handoff_code／handoffCode。
	"operationcredential": {},
	"handoffcode":         {},
	// 上傳用的預簽 URL（`/app/media/upload_sessions` 與代行版兩支的回應）。它比同一批
	// 端點回的 staged token 更該遮：預簽 URL 的定義就是「授權寫在 URL 裡」，拿到它的人
	// 不需要任何身分就能在有效期內覆寫那個暫存物件——而操作者接下來 complete 並掛上
	// 店面的，正是那個被覆寫後的內容。§10.3 的判準「單獨持有即可操作的識別碼一律視為
	// 憑證」講的就是這種東西。除錯要追的 upload session id 另有欄位，不靠這條 URL。
	"uploadurl": {},
	// 光貿 AMEGO 的賣家 AppKey（58 §8.1.2 層 2）。它是**寫入類憑證**：持有統編＋
	// AppKey 即可以該賣家名義向 AMEGO 開立、作廢、折讓真實電子發票，而發票是有法律
	// 效力的稅務憑證 —— 判準與 guestId／operationCredential 同一條（「拿到它能做
	// 什麼」），只是後果從「操作一台裝置的購物車」變成「以別人的稅籍開票」。
	//
	// **`apikey` 涵蓋不到它**：normalizeKey 去底線與大小寫之後 appKey／app_key／
	// AppKey 全部收斂成 "appkey"，與 "apikey" 是兩個不同的鍵。
	//
	// 唯一合法的入口是賣家 S03 串接精靈的請求 body 與 A04 的憑證測試端點；兩者都
	// 只寫入不回顯（§8.1.2 層 1），所以這條規則實際擋的是 req_body。
	//
	// ⚠️ **本表在 #2506 之後不再是「唯一擋住它的東西」**：body 與 query 的遮罩已改成
	// default-deny（`shouldRedact` ＝ 不在 `allowedKeys` 就遮），所以 appKey 今天就算
	// 不在本表也不會進 log。列在這裡買到的是 `shouldRedact` 註解寫的那件事——
	// forced-redact 勝過 `allowedKeys` 與 diagnostic escape hatch ⇒ 日後任何人把
	// appKey 加進 allowlist（或用診斷模式臨時放行）都覆蓋不掉它。
	// 58 §8.1.2 層 2 的原文是照舊的黑名單模型寫的，本註解是它在現行模型下的意思。
	"appkey": {},
	// 買家載具號碼（58 §8.1.2 層 2）。手機條碼與自然人憑證條碼**本身就是財政部
	// 端的識別碼**：拿到它可以在財政部平台查詢該載具下的全部發票與中獎狀態，跨平台、
	// 不限本站。它同時是 §8.1.3 分類下的 B 類（買受人識別資訊）。
	//
	// 它的進入路徑不只 body：結帳與發票中心都有以 `?carrier_id=` 帶查詢的形態，
	// 而 Logger() 是把 RawQuery 併進 path 欄位的 —— §10.3 的教訓（guestId 在 body
	// 有遮、在 query 沒遮）就是這個形狀，所以本條與 appkey 一樣走 isSensitiveKey
	// 這個唯一入口，redactBody 與 redactQuery 兩邊自動同時生效。
	// normalizeKey 收斂後同時涵蓋 carrier_id／carrierId／CarrierID。
	// 與 appkey 同理：default-deny 之下它今天本來就會被遮，列在這裡是為了讓
	// forced-redact 蓋過任何未來的 allowlist 或診斷放行。
	"carrierid": {},
	// staged token 家族不在本表逐條列舉，見 sensitiveKeySuffixes。
}

// sensitiveKeySuffixes 補上「同一種憑證有多個欄名前綴」的情況，目前唯一成員是
// staged token 家族。
//
// 為什麼是後綴而不是把欄名一條條寫進 sensitiveKeys：同一個東西在本 repo 至少有五個
// 欄名——媒體上傳的 stagedToken／stagedTokens，以及 KYC 的 id_card_front_staged_token
// ／id_card_back_staged_token／bank_book_staged_token／document_staged_tokens。
// normalizeKey 收斂之後這些鍵彼此並不相等，逐條列舉必然漏掉下一個新增的。
//
// 為什麼要遮：它是 media-service 簽出來的 JWT（`StagedTokenService.generateToken`，
// claims 帶 session_id／account_id／scope），而 `admin_delegated_media.go` 與
// `validateStagedToken` 都已經把「never logged」寫成不變式——在此之前那個不變式沒有
// 任何機械保證，兩支代行媒體端點的 req_body 與 complete 端點的 resp_body 各洩漏一半。
// 它單獨持有不足以轉正資產（`MediaAssetService.prepareFinalize` 比對 caller 的
// account_id），所以它不像 guestId 那樣是全權憑證；遮它是因為簽章憑證不該以明文停在
// Loki 裡，而 log 讀者要追的是 upload session id（在 URL path 上），從來不是 token 值。
//
// 複數形要單獨列：HasSuffix("documentstagedtokens", "stagedtoken") 為 false。
var sensitiveKeySuffixes = []string{"stagedtoken", "stagedtokens"}

// isSensitiveKey 是遮罩判定的唯一入口。redactBody 與 redactQuery 都走它，一個值的
// 每條進入路徑才會套到同一份清單（§10.3）。傳入值必須先經 normalizeKey。
func isSensitiveKey(normalized string) bool {
	if _, ok := sensitiveKeys[normalized]; ok {
		return true
	}
	for _, suffix := range sensitiveKeySuffixes {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	return false
}

// allowedKeys is a default-deny allowlist: only keys normalized to one of
// these survive redaction. It seeds from two sources — (a) every key the
// pre-#2506 test suite already asserted must survive (logger_test.go's
// TestRedactBody_BenignTokenFamily / TestRedactBody_BenignScalarArraySurvives
// / TestRedactBody_PresignedUploadURL's sessionId, plus
// logger_query_redact_test.go's page/size), and (b) a field-by-field pass
// over every JSON tag in the handlers issue #2506 named as evidence
// (auth.go, buyer.go, checkout.go, shipping.go, seller_crm.go,
// aftersales.go, address.go), keeping only business/structural data — IDs,
// enums, money, counts, timestamps, business entity names — and excluding
// anything shaped like a person's name, contact detail, free-text note, or
// address component (those default to redacted — see
// logger_allowlist_test.go's TestRedactBody_GeneralPIIDefaultsRedacted).
// Whole-branch review for #2506 (fix wave, 2026-08-21) removed "name" and
// "message" from this map after that pass caught two real leaks the
// per-task reviews couldn't see: admin_delegated_order.go's
// DelegatedRecipientContact.Name (a real recipient's name) and ai.go's
// buyer/seller AI-assistant request bodies (free-text user prompts under
// json:"message") — both are exactly the shape this allowlist's own
// construction rule excludes.
//
// Anything not here defaults to redacted. Extend it when a real debugging
// session needs a specific field left out, not preemptively — a redaction
// list that tries to guess every future field in advance is the same
// structural mistake #2506 diagnosed in the old blacklist, just aimed the
// other direction.
var allowedKeys = map[string]struct{}{
	// 回應信封（API 設計規範統一 envelope）
	"code": {}, "errors": {}, "errormessage": {},
	"results": {}, "providermessage": {}, "providercontext": {},
	"providerrefundno": {}, "redirecturl": {}, "admincontext": {},

	// 業務識別碼（TSID / UUID / enum 代碼，非個資）
	"accountid": {}, "buyerid": {}, "assetid": {}, "appealid": {},
	"caseno": {}, "checkoutid": {}, "coupontemplateid": {}, "eshipid": {},
	"id": {}, "orderno": {}, "ordernos": {}, "orderitemid": {},
	"partnerid": {}, "productid": {}, "productids": {}, "shipmentno": {},
	"shiptradeno": {}, "skuid": {}, "storeid": {}, "storecode": {},
	"tagids": {}, "uid": {}, "sessionid": {}, "groupindex": {},
	"onboardingid": {}, "decidedbyadminid": {}, "traceid": {},
	"validationno": {},

	// 狀態／列舉／布林旗標
	"state": {}, "status": {}, "statuslabel": {}, "eventtype": {},
	"evidencetype": {}, "appealstate": {}, "decision": {}, "outcome": {},
	"resolutiontype": {}, "goodstype": {}, "temperature": {},
	"temperaturecode": {}, "shippingtype": {}, "shippingmethod": {},
	"shippingroute": {}, "paymenttype": {}, "providersource": {},
	"providerstatus": {}, "returnproviderstatus": {}, "refundtaskstatus": {},
	"payunisubmitstatus": {}, "clienttype": {}, "customertype": {},
	"usertype": {}, "platform": {}, "deliverytemp": {},
	"deliverytempintersection": {}, "onboardingstatus": {}, "errorcode": {},
	"valid": {}, "eligible": {}, "enabled": {}, "available": {},
	"canappeal": {}, "autoescalated": {}, "isdefault": {}, "iscontact": {},
	"istrolley": {}, "needrefundpage": {}, "onboardingrequired": {},
	"freeshippingeligible": {}, "tentativefreeshippingeligible": {},
	"webhookreceived": {}, "success": {}, "tags": {},

	// 金額／數量／計數（非個資）
	"actualpayamount": {}, "actualrefundamount": {}, "availableqty": {},
	"couponamount": {}, "couponcount": {}, "couponminspend": {},
	"discountamount": {}, "estimatedtotal": {}, "fee": {},
	"freeshippingthreshold": {}, "importedcount": {}, "listprice": {},
	"newdiscount": {}, "newshippingfee": {}, "newsubtotal": {},
	"newtotalamount": {}, "oldtotalamount": {}, "originalprice": {},
	"price": {}, "quantity": {}, "refundableamount": {}, "refundamount": {},
	"requestedqty": {}, "saleprice": {}, "sharediscount": {},
	"shippingfee": {}, "stockqty": {}, "subtotal": {},
	"subtotaloriginalamount": {}, "total": {}, "totalactualamount": {},
	"totalcount": {}, "totaldiscount": {}, "totaloriginalamount": {},
	"totalpayamount": {}, "totalshippingfee": {}, "totalspent": {},
	"deletedcount": {}, "successcount": {}, "count": {},
	"completiontokens": {}, "prompttokens": {}, "maxtokens": {},

	// 時間戳／期限
	"appealsubmittedat": {}, "appealdeadline": {}, "autocompleteatstamp": {},
	"autoconfirmat": {}, "bankdate": {}, "businessdays": {}, "calltime": {},
	"closedat": {}, "coveragedays": {}, "createdat": {}, "date": {},
	"dayofweek": {}, "decidedat": {}, "deliverydate": {},
	"estimatedrefunddate": {}, "lastrejectedat": {}, "maxholidaydate": {},
	"processdeadline": {}, "processdeadlinehours": {}, "sellerpickedupat": {},
	"shipbackdeadlineat": {}, "shipdate": {}, "statustime": {},
	"storearrivedat": {}, "storepickupdeadlineat": {}, "timeline": {},
	"timestamp": {}, "updatedat": {}, "year": {}, "years": {},
	"returndeliveredat": {},

	// Business entity 名稱（店家／商品／公司，非個人姓名）
	"storename": {}, "skuname": {}, "sellername": {}, "returnstorename": {},
	"labelcompanyname": {},

	// 裝置 / 推播 / AI 附件識別碼（既有測試已判定為 benign）
	"fcmtoken": {}, "phototokens": {}, "evidencephototokens": {},
	"inspectionphototokens": {},

	// 分頁 / query 常見參數
	"page": {}, "size": {},

	// 銀行識別碼（哪一家銀行，非帳號本身——bank_account 仍在 sensitiveKeys 強制遮）
	"bankcode": {}, "bankname": {},

	// 帳號型別旗標（非帳號本身）
	"accounttype": {},
}

// isAllowedKey is the allowlist counterpart to isSensitiveKey.
func isAllowedKey(normalized string) bool {
	_, ok := allowedKeys[normalized]
	return ok
}

// shouldRedact is the single decision point redactBody/redactQuery call: a
// key is redacted if it's on the forced-redact list (isSensitiveKey — belt
// and suspenders, wins even if a key were mistakenly added to allowedKeys
// too) OR it is not explicitly allowed. extraAllowed is an optional
// per-request addition to the allowlist for Task 4's diagnostic escape
// hatch; every other caller passes zero arguments.
func shouldRedact(normalized string, extraAllowed ...map[string]struct{}) bool {
	if isSensitiveKey(normalized) {
		return true
	}
	if isAllowedKey(normalized) {
		return false
	}
	if len(extraAllowed) > 0 && extraAllowed[0] != nil {
		if _, ok := extraAllowed[0][normalized]; ok {
			return false
		}
	}
	return true
}

// scanLimit 是 redactBody 實際送進 regex 的位元組上限。超過上限的尾段稍後會被
// truncateBody（maxBodyLogSize）砍掉、不會進 log，所以無需 redact；設一個遠大於
// maxBodyLogSize 的上限，純粹是給超大 body（例如 AI completion / 媒體列表）的 regex
// 成本設天花板。上限與 maxBodyLogSize 的差額必須遠大於任何單一敏感值的長度（JWT 可
// 達 1~2KB），否則一個跨越 maxBodyLogSize 邊界的長 token 其閉合引號會落在 window 外，
// 害 quoted-string 分支匹配失敗而漏遮。16KB 對任何單一欄位值都綽綽有餘。
const scanLimit = 16 << 10

// kvPattern 抓取 JSON 風格的 "key": value 配對（value 可為字串、數字、bool、null
// 或其他純量）。以正規表示式逐對掃描，因此即使 body 被上游截斷成非法 JSON、
// 或巢狀於任意深度，仍能遮蔽到敏感欄位（fail-safe：寧可多遮，不可漏遮）。
// key 字元類含 '-'，與 normalizeKey 去除 '-' 的邏輯一致（涵蓋 dash-case 欄位）。
// 注意：unquoted catch-all 排除結構字元（{ [ " :），否則會貪婪吞掉巢狀
// 物件 / 陣列，連帶吃掉內層的敏感欄位而漏遮；排除後 regex 會自然遞迴進巢狀
// 結構逐對比對。物件值本身不被當成單一 value（交由內層配對處理）。
//
// 純量陣列（`[…]` 內不含 { 或 [）是唯一被當成單一 value 的結構值，因為陣列元素
// 沒有 key ⇒ 逐對比對永遠遮不到它們，而 stagedTokens／document_staged_tokens
// ／photo tokens 這類欄位正是以字串陣列送出的。只吃純量陣列是刻意的：含 { 或
// 巢狀 [ 的陣列仍然不匹配，regex 照舊遞迴進去逐對比對，既有行為一個字都沒變
// （`{"tokens":[{"access_token":"…"}]}` 仍然遮得到）。
var kvPattern = regexp.MustCompile(`"([A-Za-z0-9_-]+)"\s*:\s*("(?:[^"\\]|\\.)*"|\[[^\[\]{}]*\]|true|false|null|-?\d+(?:\.\d+)?|[^\s,:{}\[\]"]+)`)

// normalizeKey 把欄位名正規化成小寫並去除分隔符，讓 access_token 與 accessToken
// 收斂到同一個比對鍵。
func normalizeKey(k string) string {
	k = strings.ToLower(k)
	k = strings.ReplaceAll(k, "_", "")
	k = strings.ReplaceAll(k, "-", "")
	return k
}

// redactBody 遮蔽 body 中的敏感欄位值後再截斷，避免密碼 / token / 身分證 /
// 銀行帳號等隨 access log 落入 Loki。空 body 回空字串、非 UTF-8 視為二進位。
// 單趟掃描（FindAllStringSubmatchIndex）只就地替換命中 value 的位元組區間，
// 保留原始 key 大小寫與排版，且不對每個 match 重跑 regex。
func redactBody(b []byte, extraAllowed ...map[string]struct{}) string {
	if len(b) == 0 {
		return ""
	}
	if !utf8.Valid(b) {
		return "[binary]"
	}
	s := string(b)
	if len(s) > scanLimit {
		s = s[:scanLimit]
	}
	// 子配對索引：[matchStart,matchEnd, keyStart,keyEnd, valStart,valEnd]
	matches := kvPattern.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return truncateBody([]byte(s))
	}
	var sb strings.Builder
	sb.Grow(len(s))
	last := 0
	for _, m := range matches {
		key := s[m[2]:m[3]]
		if !shouldRedact(normalizeKey(key), extraAllowed...) {
			continue
		}
		valStart, valEnd := m[4], m[5]
		sb.WriteString(s[last:valStart])
		sb.WriteString(redactedValue)
		last = valEnd
	}
	sb.WriteString(s[last:])
	return truncateBody([]byte(sb.String()))
}

// redactQuery masks the same sensitive keys in a query string that redactBody
// masks in a JSON body.
//
// It exists because the two halves of one request were held to different
// standards: a value posted as {"guestId": "..."} was redacted, while the same
// value sent as ?guest_id=... went into the log verbatim (#2295 ③ — the guest
// cart identifier is a bearer credential, and it travels by query string on the
// GET and DELETE routes). Any future sensitive key added to sensitiveKeys now
// gets both halves at once, which is the point: a redaction list that only
// covers one transport is a list that will be wrong again.
//
// Parses manually rather than via url.ParseQuery so that a malformed query is
// still redacted (ParseQuery discards what it cannot parse, and a log line is
// exactly where malformed input shows up) and so the original encoding and
// parameter order survive.
func redactQuery(raw string, extraAllowed ...map[string]struct{}) string {
	var sb strings.Builder
	sb.Grow(len(raw))
	for rest := raw; rest != ""; {
		pair := rest
		sep := byte(0)
		if n := strings.IndexAny(rest, "&;"); n >= 0 {
			pair, sep, rest = rest[:n], rest[n], rest[n+1:]
		} else {
			rest = ""
		}
		if eq := strings.IndexByte(pair, '='); eq < 0 {
			sb.WriteString(pair) // bare flag, no value to hide
		} else {
			key := pair[:eq]
			// Keys arrive percent-encoded in principle. A key we cannot decode
			// falls back to its raw form rather than skipping the check — that
			// is precisely the case where assuming "harmless" is wrong.
			decoded := key
			if u, err := url.QueryUnescape(key); err == nil {
				decoded = u
			}
			sb.WriteString(key)
			sb.WriteByte('=')
			if shouldRedact(normalizeKey(decoded), extraAllowed...) {
				sb.WriteString(redactedValue)
			} else {
				sb.WriteString(pair[eq+1:])
			}
		}
		if sep != 0 {
			sb.WriteByte(sep)
		}
	}
	return sb.String()
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body               *bytes.Buffer
	contentTypeChecked bool
	streaming          bool
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	if !w.contentTypeChecked {
		w.contentTypeChecked = true
		w.streaming = isStreamingContentType(w.Header().Get("Content-Type"))
	}
	if !w.streaming && w.body.Len() < scanLimit+1 {
		remaining := scanLimit + 1 - w.body.Len()
		chunk := b
		if len(chunk) > remaining {
			chunk = chunk[:remaining]
		}
		w.body.Write(chunk)
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying connection.
// gin.ResponseWriter (the embedded interface field's type) does not declare
// Unwrap() even though gin's concrete implementation has it — embedding an
// interface only promotes methods the interface itself declares. Without
// this, SetReadDeadline/SetWriteDeadline calls against a Logger-wrapped
// c.Writer silently no-op instead of reaching the real connection.
func (w *responseBodyWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func isStreamingContentType(ct string) bool {
	switch {
	case strings.HasPrefix(ct, "text/event-stream"),
		// Connect 的串流（application/connect+json、application/connect+proto）；unary 是 application/json。
		strings.HasPrefix(ct, "application/connect+"),
		strings.HasPrefix(ct, "application/octet-stream"),
		strings.HasPrefix(ct, "image/"),
		strings.HasPrefix(ct, "video/"),
		strings.HasPrefix(ct, "audio/"):
		return true
	default:
		return false
	}
}

// appendTraceFields appends trace_id/span_id fields when the request's
// context carries a sampled OTel span, letting Loki correlate to Tempo.
// Shared by loggerMetadataOnly and loggerWithBody so the two log paths can
// never drift out of sync on this logic.
func appendTraceFields(fields []zap.Field, ctx context.Context) []zap.Field {
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.HasTraceID() {
		fields = append(fields,
			zap.String("trace_id", spanCtx.TraceID().String()),
			zap.String("span_id", spanCtx.SpanID().String()),
		)
	}
	return fields
}

// Logger logs request and response details for every API call. Prod takes a
// metadata-only path that never reads request/response bodies (architectural
// elimination, not a config toggle that can be left on by mistake — see
// #2506's design spec §2 for why). Dev/staging keep body capture but redact
// by allowlist (Task 2). diagnosticOverrideActive lets a specific prod route
// borrow the dev path for a config-scoped, expiring window (Task 4).
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if pkgconfig.IsProduction() && !diagnosticOverrideActive(c) {
			loggerMetadataOnly(c)
			return
		}
		loggerWithBody(c)
	}
}

// loggerMetadataOnly is the prod default: no body buffering, no Authorization
// value at all (actor_id already answers "who" without touching a
// credential). This function must never call c.Request.Body's Read, nor wrap
// c.Writer to capture response bytes — that absence is the actual security
// boundary, not a runtime check.
func loggerMetadataOnly(c *gin.Context) {
	start := time.Now()
	method := c.Request.Method

	c.Next()

	latency := time.Since(start)
	status := c.Writer.Status()

	fields := []zap.Field{
		zap.String("method", method),
		zap.String("route", c.FullPath()),
		zap.Int("status", status),
		zap.Duration("latency", latency),
		zap.String("ip", c.ClientIP()),
		zap.String("xff", truncateHeaderValue(c.GetHeader("X-Forwarded-For"))),
		zap.String("app_version", truncateHeaderValue(c.GetHeader("X-App-Version"))),
		zap.String("app_type", truncateHeaderValue(c.GetHeader("X-App-Type"))),
	}

	if actorID, ok := c.Get("accountId"); ok {
		if id, ok := actorID.(int64); ok {
			fields = append(fields, zap.Int64("actor_id", id))
		}
	}

	if status >= http.StatusBadRequest {
		fields = append(fields, zap.String("error", truncateHeaderValue(c.Errors.String())))
	}

	fields = appendTraceFields(fields, c.Request.Context())

	zap.L().Info("gateway request", fields...)
}

// loggerWithBody is the dev/staging path (and prod's diagnostic-override
// path, Task 4): buffers request/response bodies and redacts by allowlist
// (Task 2). This is the pre-#2506-redesign Logger() body, otherwise
// unchanged.
func loggerWithBody(c *gin.Context) {
	extra := diagnosticExtraAllowed(c)
	start := time.Now()
	path := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		path += "?" + redactQuery(c.Request.URL.RawQuery, extra)
	}
	method := c.Request.Method

	var reqBody string
	if c.Request.Body != nil {
		limited := io.LimitReader(c.Request.Body, scanLimit+1)
		bodyBytes, err := io.ReadAll(limited)
		c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(bodyBytes), c.Request.Body))
		if err == nil {
			reqBody = redactBody(bodyBytes, extra)
		}
	}

	auth := ""
	if c.GetHeader("Authorization") != "" {
		auth = "[REDACTED]"
	}

	rbw := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
	c.Writer = rbw

	c.Next()

	latency := time.Since(start)
	status := c.Writer.Status()
	respBody := redactBody(rbw.body.Bytes(), extra)

	fields := []zap.Field{
		zap.Int("status", status),
		zap.Duration("latency", latency),
		zap.String("ip", c.ClientIP()),
		zap.String("xff", truncateHeaderValue(c.GetHeader("X-Forwarded-For"))),
		zap.String("method", method),
		zap.String("path", path),
		zap.String("auth", auth),
		zap.String("req_body", reqBody),
		zap.String("resp_body", respBody),
		zap.String("app_version", truncateHeaderValue(c.GetHeader("X-App-Version"))),
		zap.String("app_type", truncateHeaderValue(c.GetHeader("X-App-Type"))),
	}

	fields = appendTraceFields(fields, c.Request.Context())

	zap.L().Info("gateway request", fields...)
}

// diagnosticBodyLoggingConfigPath is a var (not const) so tests can point it
// at a temp file instead of the real ConfigMap mount.
var diagnosticBodyLoggingConfigPath = "/etc/gateway/diagnostic-body-logging.yaml"

// diagnosticBodyLoggingRule is one entry in the prod diagnostic escape hatch
// (design spec §3.4). expires_at is checked against wall-clock time on every
// request — no cache, no background reload; this is a deliberately
// low-traffic diagnostic path where correctness beats micro-latency, and it
// avoids needing any new config-watch machinery in a gateway that has none
// today (config.Load() is startup-only).
type diagnosticBodyLoggingRule struct {
	Route     string    `yaml:"route"`
	Fields    []string  `yaml:"fields"`
	Owner     string    `yaml:"owner"`
	ExpiresAt time.Time `yaml:"expires_at"`
}

type diagnosticBodyLoggingConfig struct {
	Rules []diagnosticBodyLoggingRule `yaml:"diagnostic_body_logging"`
}

// loadDiagnosticBodyLoggingRules reads and parses the ConfigMap-mounted
// diagnostic config fresh on every call. A missing file (the common case: no
// ConfigMap mounted), malformed YAML, or an unknown/misspelled key (e.g.
// "field:" instead of "fields:") all yield nil — fail closed, prod stays on
// the metadata-only path. UnmarshalStrict (not Unmarshal) is what turns a
// typo'd key into a parse error instead of a silently-empty Fields slice —
// #2506 whole-branch review Fix D: a rule that parsed with Fields empty
// would otherwise still match its route and, per diagnosticExtraAllowed
// below, flip prod onto the full dev/staging body-capture path with zero
// fields actually scoped.
func loadDiagnosticBodyLoggingRules() []diagnosticBodyLoggingRule {
	data, err := os.ReadFile(diagnosticBodyLoggingConfigPath)
	if err != nil {
		return nil
	}
	var cfg diagnosticBodyLoggingConfig
	if err := yaml.UnmarshalStrict(data, &cfg); err != nil {
		zap.L().Warn("diagnostic body logging config malformed, ignoring", zap.Error(err))
		return nil
	}
	return cfg.Rules
}

// diagnosticOverrideActive reports whether the current request matches a
// live (non-expired) diagnostic rule. Delegates to diagnosticExtraAllowed so
// the route-match/expiry predicate exists in exactly one place — this
// function and diagnosticExtraAllowed are called from different points in
// the request lifecycle (Logger() vs. loggerWithBody), so each still does
// its own loadDiagnosticBodyLoggingRules() read, but neither duplicates the
// matching logic itself.
func diagnosticOverrideActive(c *gin.Context) bool {
	return diagnosticExtraAllowed(c) != nil
}

// diagnosticExtraAllowed returns the extraAllowed map for the currently
// matched rule (nil if none matches), normalized the same way allowedKeys
// is, for direct use as redactBody/redactQuery's extraAllowed argument.
//
// A rule with no fields scoped (len(r.Fields) == 0, e.g. "fields: []" or the
// key omitted entirely) is treated as not matching at all — #2506
// whole-branch review Fix D: without this guard, any route+expiry match
// would still return a non-nil (merely empty) map, and diagnosticOverrideActive
// (which only checks != nil) would flip prod onto the full dev/staging
// body-buffering path anyway, revealing the entire base allowedKeys set
// instead of the scoped subset "field scoping" is supposed to guarantee.
func diagnosticExtraAllowed(c *gin.Context) map[string]struct{} {
	rules := loadDiagnosticBodyLoggingRules()
	routeKey := c.Request.Method + " " + c.FullPath()
	now := time.Now()
	for _, r := range rules {
		if r.Route == routeKey && now.Before(r.ExpiresAt) && len(r.Fields) > 0 {
			extra := make(map[string]struct{}, len(r.Fields))
			for _, f := range r.Fields {
				extra[normalizeKey(f)] = struct{}{}
			}
			return extra
		}
	}
	return nil
}

// headerValueLogLimit 是 client 可控 header 寫進 access log 的長度上限（設計書 54 §8.2）。
const headerValueLogLimit = 32

// truncateHeaderValue 把 client 可控的 header 值截到 headerValueLogLimit 字元，
// 避免任意長字串灌進 Loki。缺值回空字串。
func truncateHeaderValue(v string) string {
	if len(v) <= headerValueLogLimit {
		return v
	}
	// 以 rune 為單位切，避免把多位元組字元切成無效 UTF-8。
	r := []rune(v)
	if len(r) <= headerValueLogLimit {
		return v
	}
	return string(r[:headerValueLogLimit])
}

func truncateBody(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if !utf8.Valid(b) {
		return "[binary]"
	}
	s := strings.TrimSpace(string(b))
	if len(s) > maxBodyLogSize {
		return s[:maxBodyLogSize] + "...(truncated)"
	}
	return s
}
