package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/gin-gonic/gin"
)

// IdempotencyKeyHeader 是冪等鍵的 header 名（58 號 §6.1.0(10)）。
const IdempotencyKeyHeader = "Idempotency-Key"

// IdempotencyReplayHeader 標示這一則回應是回放的，不是重新執行出來的。
// 純觀測用（前端不需要讀它）：沒有它的話，「重放成功」與「又建了一筆」在
// 回應層長得一模一樣，而那正是這個 bug 當初躲過驗證的原因。
const IdempotencyReplayHeader = "Idempotency-Replay"

// IdempotencyKeyPattern 是 `Idempotency-Key` 的值域，全 gateway **唯一一份**
// （58 號 §6.1.0(10) 定版）。
//
// 匯出而不是留成私有，是因為代行營運那條路徑（`handler/admin_delegated_common.go`）
// 早於本 middleware 就在驗同一個 header，而它的回應碼不同（`DelegationInvalidCommand`
// 而非 400）⇒ 那一支不能直接改掛本 middleware，但**值域必須是同一份**。
// 兩份各自維護的正則不會有任何東西轉紅，而漂移的症狀是「同一個 header 在兩條路徑上
// 合法性不同」。
var IdempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)

// RequireIdempotencyKey 對 POST／PUT／DELETE **驗格式並執行重放語意**；其餘 method
// 直接放行。
//
// 🔴 **格式驗證與重放執行刻意綁在同一個 function**（issue #3015）。在此之前這裡只驗
// 格式、把去重「委派」給 invoice-service 的 usecase，而那一層從未實作 ——
// `invoice_documents` 連 `idempotency_key` 欄都沒有，9350 全 repo 零 emitter。
// 結果是 36 條寫入路由把這個 header 列為必填卻零作用：dev 實測同一把鍵送兩份不同
// 內容，建出兩筆資料、兩個不同 id。**一個必填但不生效的參數比沒有更危險**，因為
// 前端會據此不再自己做防重。
// ⇒ 只要有一個掛載點掛得上「只驗格式」的版本，同一個洞就會再長回來，所以不留那個版本。
//
// 三態（58 號 §6.1.0(10)）：
//
//	同鍵同內容且已完成 ⇒ 回放上一次的 status ＋ body（不重跑）
//	同鍵但內容不同     ⇒ 9350（409），且**不執行**
//	同鍵同內容進行中   ⇒ 9351（409）
//
// **鍵的作用域**＝`accountId | METHOD | 路由樣板 | Idempotency-Key`；
// **指紋**＝`sha256(實際路徑 + query + body)`。路徑參數進指紋不進作用域，所以
// 「同一把鍵刪兩個不同的 profile_id」會被判成 9350 而不是互相回放。
//
// 🔴 **它擋不住什麼**（存放層的邊界寫在 `idempotency_store.go` 檔頭，這裡只寫路由面）：
//
//	① 未帶可信 accountId 的呼叫（理論上不存在：36 條路由全在 JWTAuth 之後）不做冪等，
//	   直接放行。作用域少了帳號等於把不同人的請求併成同一把鍵，那比不做更糟。
//	② 回放會**略過本 middleware 之後的守門**（賣家 capability 檢查掛在群組 .Use 之後）。
//	   回放的對象是**同一個帳號自己上一次拿到的回應**、且不產生任何新的副作用，
//	   所以這是「拿回舊收據」不是「重新取得授權」；但若該帳號在兩次之間被停權，
//	   TTL 內仍會拿到那張舊收據。
//	③ 只有 2xx 會留紀錄。非 2xx 一律放掉執行權 —— 失敗沒有值得回放的結果，
//	   把一次 4xx 釘死在那把鍵上會讓使用者修正問題後重送反而拿到舊錯誤。
func RequireIdempotencyKey() gin.HandlerFunc {
	return requireIdempotencyKey(defaultIdempotencyStore)
}

func requireIdempotencyKey(store *IdempotencyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete:
		default:
			c.Next()
			return
		}
		key := c.GetHeader(IdempotencyKeyHeader)
		if !IdempotencyKeyPattern.MatchString(key) {
			response.BadRequest(c, "Idempotency-Key 格式不符 ^[A-Za-z0-9._:-]{8,128}$")
			c.Abort()
			return
		}

		accountID, ok := accountIDFromContext(c)
		if !ok {
			// 見上方「擋不住什麼」①。
			c.Next()
			return
		}

		body, ok := readAndRestoreBody(c)
		if !ok {
			// body 讀不完整就沒有可信的指紋 ⇒ 不冒充做過冪等，交給 handler 自己失敗。
			c.Next()
			return
		}

		scope := idempotencyScope(accountID, c.Request.Method, c.FullPath(), key)
		fingerprint := idempotencyFingerprint(c.Request.URL.Path, c.Request.URL.RawQuery, body)

		switch outcome, rec := store.Begin(scope, fingerprint); outcome {
		case idemReplay:
			replayIdempotentResponse(c, rec)
			return
		case idemMismatch:
			response.AbortBizStatus(c, bizcode.InvoiceIdempotencyPayloadMismatch,
				bizcode.Message(bizcode.InvoiceIdempotencyPayloadMismatch))
			return
		case idemInFlight:
			response.AbortBizStatus(c, bizcode.InvoiceOperationInProgress,
				bizcode.Message(bizcode.InvoiceOperationInProgress))
			return
		}

		capture := &idempotencyCapture{ResponseWriter: c.Writer}
		c.Writer = capture
		// defer 而不是 c.Next() 之後直接寫：panic 會被更外層的 Recovery 接住，
		// 而執行權必須在那條路徑上一樣被歸還，否則那把鍵在 TTL 內永遠回 9351。
		defer func() {
			status := capture.Status()
			// Written() 一起看：gin 的 Status() 在什麼都沒寫時預設回 200，只看它會把
			// 「handler 什麼都沒做」存成一則可回放的空 200。
			if capture.Written() && status >= http.StatusOK &&
				status < http.StatusMultipleChoices && !capture.truncated {
				store.Commit(scope, status, capture.Header().Get("Content-Type"), capture.body.Bytes())
				return
			}
			store.Release(scope)
		}()
		c.Next()
	}
}

// readAndRestoreBody 讀完 request body 並塞回去讓 handler 還讀得到。
// 大小由 BodyGuard 先封頂（一般路由 2 MiB），這裡不再自訂上限——自訂第二個上限
// 只會產生兩個會漂移的數字。
func readAndRestoreBody(c *gin.Context) ([]byte, bool) {
	if c.Request.Body == nil {
		return nil, true
	}
	body, err := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return nil, false
	}
	return body, true
}

func idempotencyScope(accountID int64, method, routeTemplate, key string) string {
	return strconv.FormatInt(accountID, 10) + "|" + method + "|" + routeTemplate + "|" + key
}

func idempotencyFingerprint(path, rawQuery string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write([]byte(rawQuery))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func replayIdempotentResponse(c *gin.Context, rec *idempotencyRecord) {
	c.Abort()
	if rec.contentType != "" {
		c.Header("Content-Type", rec.contentType)
	}
	c.Header(IdempotencyReplayHeader, "true")
	c.Status(rec.status)
	_, _ = c.Writer.Write(rec.body)
}

// idempotencyCapture 抄下回應以便日後回放。
// 超過 idempotencyMaxBodyBytes 就標記 truncated 而**不**留紀錄——回放半截 JSON
// 比不回放危險得多。
type idempotencyCapture struct {
	gin.ResponseWriter
	body      bytes.Buffer
	truncated bool
}

func (w *idempotencyCapture) Write(b []byte) (int, error) {
	if !w.truncated {
		if w.body.Len()+len(b) > idempotencyMaxBodyBytes {
			w.truncated = true
			w.body.Reset()
		} else {
			w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *idempotencyCapture) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// Unwrap 讓 http.ResponseController 能穿到底層連線。理由與 logger.go 的
// responseBodyWriter.Unwrap 相同：嵌入 interface 只會提升該 interface 自己宣告的
// method，gin.ResponseWriter 沒有宣告 Unwrap ⇒ 不補這一支，BodyGuard 的
// SetReadDeadline 會靜默 no-op。
func (w *idempotencyCapture) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
