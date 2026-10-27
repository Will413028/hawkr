package response

import (
	"net/http"

	"github.com/Will413028/hawkr/go-pkg/envelope"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/gin-gonic/gin"
)

// bizHTTPStatus 是「業務碼 → HTTP status」的唯一登錄表（56 §6.5.2）。
// 未登錄的碼沿用既有塌縮行為（409），與 BizDeny 一致。
//
// 收錄範圍：代行營運（92xx）＋ 商品認證申請與審核（3025–3030，19 號 §11.1／§9.3）。
// 不是只有代行——新增其他領域的碼時照樣登錄在這裡，不要另建第二張表。
//
// 這張表存在的理由：Gateway 對未登錄的業務碼一律塌縮成 409，而代行營運需要
// 400／401／403／409／410／422／424／429 分流——前端對 401 清憑證、對 410 清 cookie 並
// 切阻擋畫面、對 424 顯示「暫時不可用」而**不**隱藏入口（503／404 會被讀成「此環境
// 尚未部署代行營運服務」並讓入口永久消失，C12）。沒有這張表，前端拿到的全是 409。
//
// ⚠️ **本表的值一律必須是 4xx**，這是可用性的硬前提而不是風格偏好：Cloudflare 站在
// Gateway 前面，origin 回 5xx 時它會把整個 body 換成自己的錯誤頁（實測：叢集內直打回
// `{"code":9280,…}` ＋ `application/json`，同一支端點經 CF 變成 `error code: 502` ＋
// `text/plain`）⇒ 前端的 `response.json()` 直接拋錯，**永遠拿不到業務碼**。9280 原本
// 登錄成 502 就是這個形狀；改 424 Failed Dependency 之後語意不變（依賴的上游失敗）而
// body 送得到。C12 禁的是 404／503——它們會被讀成「此環境未部署代行」並永久隱藏入口，
// 那個顧慮對 4xx 不成立：4xx 說的是「這一次請求出了事」，不是「這個服務不存在」。
// TestBizHTTPStatus_NoServerErrorStatuses 是守住這一條的機械閘門。
var bizHTTPStatus = map[int32]int{
	bizcode.DelegationInvalidCommand:       http.StatusBadRequest,          // 400
	bizcode.DelegationAdminSessionRequired: http.StatusUnauthorized,        // 401
	bizcode.DelegationCredentialInvalid:    http.StatusUnauthorized,        // 401
	bizcode.DelegationCapabilityDenied:     http.StatusForbidden,           // 403
	bizcode.DelegationTargetMismatch:       http.StatusForbidden,           // 403
	bizcode.DelegationAuthorizationRevoked: http.StatusForbidden,           // 403
	bizcode.DelegationStoreNotDelegable:    http.StatusForbidden,           // 403
	bizcode.DelegationResourceNotOwned:     http.StatusForbidden,           // 403
	bizcode.DelegationActorMismatch:        http.StatusForbidden,           // 403
	bizcode.DelegationSessionExpired:       http.StatusGone,                // 410
	bizcode.DelegationSessionRevoked:       http.StatusGone,                // 410
	bizcode.DelegationSessionNotFound:      http.StatusGone,                // 410
	bizcode.DelegationHandoffInvalid:       http.StatusGone,                // 410
	bizcode.DelegationStoreNotOperable:     http.StatusUnprocessableEntity, // 422
	bizcode.DelegationRateLimited:          http.StatusTooManyRequests,     // 429
	bizcode.DelegationUpstreamUnavailable:  http.StatusFailedDependency,    // 424

	// 授權列治理（#2444）。四個都是全新的碼、零既有消費者，登錄它們不是契約變更。
	// 9285 的 409 與未登錄時的塌縮值相同，仍明寫：四個碼是同一組治理語意，讓其中一個
	// 靠塌縮拿到正確答案，等於把它的 status 交給一個與它無關的預設值保管。
	// 9285 是 409 而不是 400：重複開通不是「請求寫錯了」，而是「你看到的畫面過時了」，
	// 前端據此重抓詳情再讓人決定要不要改成調整範圍。
	// 9286 是 404 —— 本表唯一的 404。C12 禁的是把「這個 endpoint 沒部署」講成 404，
	// 而這裡回答的是一個具名資源（某家店的授權列）不存在，路由本身仍在；治理面也不隱藏
	// 入口（隱藏的是「代行工作區」那條路徑，與本組端點無關）。
	bizcode.DelegationAuthorizationAlreadyActive:     http.StatusConflict,   // 409
	bizcode.DelegationAuthorizationNotFound:          http.StatusNotFound,   // 404
	bizcode.DelegationCapabilityDependencyIncomplete: http.StatusBadRequest, // 400
	bizcode.DelegationAdminNotAllowlisted:            http.StatusForbidden,  // 403

	// 一碼多義拆分（#2744）。三個碼都是全新的、零既有消費者。
	// 9289 與 9280 同為 424 是刻意的：兩者的 HTTP 語意確實相同（這次請求因為它依賴的
	// 東西壞了而失敗），差別在「重試會不會好」，而那個差別由**碼**承擔——本表的 doc
	// comment 已經寫明前端一律依 code 決定文案。把 9289 塞進 5xx 才是錯的（CF 會換掉
	// body），塞進 403 更錯（那正是它要修的謊：說成操作者權限不足）。
	bizcode.DelegationInternalContractMismatch: http.StatusFailedDependency, // 424
	// 9290 的 409 與未登錄時的塌縮值相同，仍明寫——理由同 9285：讓一個碼靠塌縮拿到
	// 正確答案，等於把它的 status 交給一個與它無關的預設值保管。
	bizcode.DelegationResourceAlreadyInState: http.StatusConflict, // 409
	// 9291 沿用 9265 的 403：它取代的就是 9265 在 flag 關閉時那一支，換碼不換行為。
	// ⚠️ 不得改成 404／503（C12：ADM 會讀成「此環境尚未部署代行營運服務」並永久隱藏入口）。
	bizcode.DelegationFeatureDisabled: http.StatusForbidden, // 403

	// 商品認證申請與審核（19 號設計書 §11.1，#2755）。五個碼都是全新的、零既有消費者，
	// 登錄它們不是契約變更。**不登錄就全部塌縮成 409**——那會讓「這筆申請不存在」與
	// 「版本衝突」在 HTTP 層長得一模一樣，ADM 分不出該重抓詳情還是該回列表。
	//
	// 3025 是 404：它回答的是一個具名資源（某一筆認證申請）不存在，路由本身仍在。
	// 賣家路徑的「非本人商品」也走這個碼——**刻意讓兩者無法區分**，否則以商品 ID 掃一遍
	// 就能靠 404 與 403 的差別枚舉出別人有哪些商品（19 §9.1 的防 IDOR 要求）。
	// 3028 是 400：駁回／撤銷少帶 reasonCode 是請求本身寫錯了，重送同一份不會變好。
	// 3026 / 3027 / 3029 是 409：三者都是「你看到的畫面過時了」——版本被別人推進、
	// 狀態已經不在可轉移的位置、同類型已有另一筆待審。前端的恢復動作都是重抓再決定。
	bizcode.CertApplicationNotFound:         http.StatusNotFound,   // 404
	bizcode.CertApplicationVersionConflict:  http.StatusConflict,   // 409
	bizcode.CertApplicationStateInvalid:     http.StatusConflict,   // 409
	bizcode.CertReviewReasonRequired:        http.StatusBadRequest, // 400
	bizcode.CertApplicationDuplicatePending: http.StatusConflict,   // 409

	// 3030 是 429：「重新查驗」繞過結果快取之後每按一次都真的打農業部並消耗每日配額
	// （19 §9.3），冷卻窗內的請求會被拒。**不能讓它塌縮成 409** —— 409 的語意是
	// 「你看到的畫面過時了、重新載入即可」，而重新載入對冷卻中的申請完全沒有幫助，
	// 等於把管理員推去做一件不會有用的事。429 才說得出「等一下再來」。
	bizcode.CertReverifyCooldown: http.StatusTooManyRequests, // 429

	// 會員方案代碼無效（9400，59 號設計書 §10）：未知方案代碼映射為 400 Bad Request。
	bizcode.InvalidPlanCode: http.StatusBadRequest, // 400

	// PAYUni update_merchant 有限範圍寫入（issue #2806 Phase 2，
	// docs/superpowers/specs/2026-08-30-payuni-update-merchant-phase2-design.md
	// §6.2）。9234 是 412：樂觀鎖版本衝突，跟「同 Idempotency-Key、不同 payload」
	// 的 409 刻意分開語意——409 說「畫面過時了、重新整理即可」，412 才說得出「你手上
	// 這份草稿的版本號已經不是最新」，前端需要靠這個差異決定要不要清空使用者尚未送出
	// 的草稿。9235/9236 與塌縮值相同仍明寫——理由同 9285/9290：不能讓一個碼靠一個跟
	// 它無關的預設值苟活。
	bizcode.PayuniMerchantProfileVersionConflict:      http.StatusPreconditionFailed, // 412
	bizcode.PayuniMerchantProfileIdempotencyKeyReused: http.StatusConflict,           // 409
	bizcode.PayuniMerchantProfileNotActive:            http.StatusConflict,           // 409

	// 以下維持 409（不必登錄，列出僅為可讀性）：
	//   DelegationResourceVersionConflict / DelegationIdempotencyKeyReused
	//   DelegationConflictActorOtherStore / DelegationConflictStoreOtherActor
	//   DelegationAuditIntentFailed
	//
	// 9276 DelegationBatchPartialFailed 刻意不在表內：它的 HTTP 是 200，走既有的
	// response.Biz。把 200 登錄進一張名為「業務拒絕 → HTTP status」的表，會讓下一個
	// 讀表的人以為 9276 是一種 4xx。

	// ── 65 物流履約平台 Shared Foundation（9500~9699；65 骨幹佔 9501~9575）──────────
	//
	// 未登錄一律塌縮成 409，而下列四類的前端行為必須分流：404 要走「找不到」而不是
	// 「衝突」；403 是「資格不成立」；424 是「依賴讀不到、稍後再試」；400 是「你少填了欄位」。
	//
	// 🔴 403 這一組刻意**不用** 404／503：那兩個會被前端讀成「此環境尚未部署物流履約」
	// 而把入口永久隱藏起來——與代行營運 C12 逐字同源的顧慮。
	// 🔴 424（而非 502／503）：本表的值一律必須是 4xx，理由見檔頭（CF 會把 origin 的
	// 5xx body 整個換掉 ⇒ 前端永遠拿不到業務碼）。9504／9553 的語意是「撥款資格或商流
	// 判定的輸入暫時讀不到」，與 9280 同型 ⇒ 前端顯示「暫時不可用」但**不**隱藏入口。
	bizcode.SellerInvoiceObligationNotFound: http.StatusNotFound, // 404
	bizcode.SettlementNotFound:              http.StatusNotFound, // 404
	bizcode.SettlementHoldNotFound:          http.StatusNotFound, // 404
	bizcode.LedgerEntryNotFound:             http.StatusNotFound, // 404
	bizcode.ExceptionCaseNotFound:           http.StatusNotFound, // 404
	bizcode.ExceptionCaseResolutionNotFound: http.StatusNotFound, // 404

	bizcode.FulfillmentConsignmentAgreementRequired: http.StatusForbidden, // 403
	bizcode.FulfillmentProviderNotEnabled:           http.StatusForbidden, // 403
	bizcode.SettlementPayoutNotApplicable:           http.StatusForbidden, // 403
	bizcode.ExceptionCaseReviewSelfApprovalDenied:   http.StatusForbidden, // 403

	bizcode.FulfillmentCommerceModelUnknown:  http.StatusFailedDependency, // 424
	bizcode.SettlementPayoutInputUnavailable: http.StatusFailedDependency, // 424

	bizcode.SellerInvoiceObligationFileRequired:         http.StatusBadRequest, // 400
	bizcode.SellerInvoiceObligationRejectReasonRequired: http.StatusBadRequest, // 400
	bizcode.SettlementHoldReasonRequired:                http.StatusBadRequest, // 400
	bizcode.LedgerAdjustmentReasonRequired:              http.StatusBadRequest, // 400

	// 顯式登錄「就是 409」而不是靠預設：這一格的語意是**可重試的併發**，與其他未登錄碼
	// 「狀態不允許」的 409 不同義。寫出來是為了讓下一個人不必回頭猜它是漏登還是刻意。
	bizcode.FulfillmentOperationInProgress: http.StatusConflict, // 409

	// ── 平台功能開通（9700~9709；backend#3329）────────────────────────────────
	//
	// 語意是「**這個環境還沒開通這項功能**」——我方配置缺失，不是使用者資格不成立。
	// 這幾個碼取代的是「憑證沒設就拒絕啟動整個服務」的舊行為（#3230：invoice-service
	// 因此被 replicas: 0，76 條無關路由陪葬三天）。
	//
	// 🔴 424 而非 403：403 會被前端讀成「你沒有權限」而引導使用者去申請權限，但使用者
	// 什麼都做不了——要動的是維運。424 的既定前端處置是「暫時不可用、保留入口」，正確。
	// 🔴 424 而非 503／404：那兩個會被讀成「此環境沒有這個端點」而把入口**永久隱藏**，
	// 等憑證補上之後入口也不會自己回來（與 9503 那一組逐字同源的顧慮）。
	// 🔴 424 而非 5xx：本表的值一律必須是 4xx，理由見檔頭（CF 會換掉 origin 的 5xx body）。
	bizcode.FeatureAddressLookupDisabled: http.StatusFailedDependency, // 424
	bizcode.FeatureAiAssistantDisabled:   http.StatusFailedDependency, // 424

	// 🔴 9705 與 9503 FulfillmentProviderNotEnabled 刻意不同 status：9503 是 403
	// 「這個賣家沒開通這個配送方式」（賣家自己建店就能解），9705 是 424「我方的 PayUni
	// 代理商憑證沒設定」（賣家做什麼都沒用）。都寫成 403 會把後者顯示成前者，
	// 賣家被導去重複申請建店，而真正要動的是維運。
	bizcode.FeaturePayuniMerchantDisabled:   http.StatusFailedDependency, // 424
	bizcode.FeatureShippingProviderDisabled: http.StatusFailedDependency, // 424
}

// StatusForBizCode 回傳業務碼對應的 HTTP status，未登錄者塌縮成 409。
func StatusForBizCode(code int32) int {
	if s, ok := bizHTTPStatus[code]; ok {
		return s
	}
	return http.StatusConflict
}

// BizStatus 送出「業務碼 + 登錄表決定的 HTTP status」的 envelope。
// 形狀與 BizDeny 完全相同（Code / Message / MessageZh / traceId），差別只在 status
// 由登錄表決定而不是恆為 409。
func BizStatus(c *gin.Context, code int32, message string) {
	// code 是呼叫端給的（9 個呼叫點寫的是 resp.GetCode()）⇒ 交出去讓 guard 判。
	// code==0 時 StatusForBizCode 會塌成 409 而 envelope 的 Code 仍是 0 —— 前端讀
	// code 判成功，那就是一個披著 409 外衣的假成功，與第十種繞道同形。
	if guard(c, code, nil) {
		return
	}
	c.JSON(StatusForBizCode(code), Envelope{
		Code:      code,
		Message:   message,
		MessageZh: message,
		TraceID:   envelope.ExtractTraceID(c),
	})
}

// AbortBizStatus 是 BizStatus 加上中止 handler chain，供 guard 使用：守門判定為拒絕
// 之後，後續 handler 一律不得執行。
func AbortBizStatus(c *gin.Context, code int32, message string) {
	// 守衛由 BizStatus 那一層做（它才是真正寫出 envelope 的地方）；這裡只負責中止。
	c.Abort()
	BizStatus(c, code, message)
}

// BizStatusData 同 BizStatus 但帶 data。典型消費者是 9266（409 ＋ 版本衝突的 diff
// payload）。簽章刻意沒有 httpStatus 參數：多一個參數就等於多一條繞過登錄表的路徑。
func BizStatusData(c *gin.Context, code int32, message string, data interface{}) {
	if guard(c, code, data) {
		return
	}
	c.JSON(StatusForBizCode(code), Envelope{
		Code:      code,
		Message:   message,
		MessageZh: message,
		Data:      data,
		TraceID:   envelope.ExtractTraceID(c),
	})
}

// writeBizCodeEnvelopeMapped 是 writeBizCodeEnvelopeIfPrefixed 的登錄表版本：訊息帶
// "[NNNN] " 前綴時還原業務碼，HTTP status 優先取 bizHTTPStatus，沒登錄才用該 gRPC
// status 分支的預設值。
//
// 兩者並存不是重複：既有那支維持「同一分支內帶不帶業務碼都回同一個 status」的性質，
// 本支則是代行營運明確要求的 status 分流（§6.5.2）。
//
// ⚠️ 登錄表原本只收代行碼，2026-08 起也收商品認證的 3025–3029（19 號 §11.1）。所以
// 「非代行碼走到這裡與既有行為逐位元相同」這句話現在只對**未登錄**的碼成立——那正是
// 登錄的目的：讓 3025 從 InvalidArgument 分支的 400 變成它該有的 404。
func writeBizCodeEnvelopeMapped(c *gin.Context, fallbackStatus int, msg string) bool {
	// 🔴 解析在守衛之前：第一版在函式開頭無條件 MarkEmitted，但 code<=0 這條路
	// **什麼都沒寫**就回 false（由呼叫端 GrpcErr 續寫）——在還沒寫東西之前就蓋
	// 「回應是我寫的」的章，正是 A2 那個病的縮影。
	code, bizMsg := envelope.ParseBizMessage(msg)
	if code <= 0 {
		return false
	}
	if guard(c, code, nil) {
		return true
	}
	status := fallbackStatus
	if s, ok := bizHTTPStatus[code]; ok {
		status = s
	}
	c.JSON(status, Envelope{
		Code:      code,
		Message:   bizMsg,
		MessageZh: bizMsg,
		TraceID:   envelope.ExtractTraceID(c),
	})
	return true
}
