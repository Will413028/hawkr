package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ---------------------------------------------------------------------------
// 冪等鍵（#3015 裁決 A，DEC-20260902-163404）
//
// 🔴 **本檔的每一支都必須在業務寫入的同一個 transaction 內呼叫。**
// 那是整個機制唯一的正確性來源：分開寫等於在兩次寫入之間留一個窗口，
// 而那正是它要消滅的東西。
//
// 與 gateway 那一層（PR #3025）的分工：
//   gateway  —— 單一 process 的重放快取，跨 pod 不生效（prod replicas: 2 ⇒ 約一半）
//   本層     —— 耐久保證，「同一把鍵絕不產生第二筆」
// 兩者不是二選一：gateway 那層讓多數重送連下游都不用打，本層負責它漏掉的那些。
// ---------------------------------------------------------------------------

// 冪等作用域。**一個端點一個常數**，不要共用字串：
// 共用會讓前端對整個頁面用同一把鍵時，A 端點的請求擋住 B 端點的。
const (
	IdemScopeBuyerProfileCreate = "BUYER_INVOICE_PROFILE_CREATE"
)

// ErrIdempotencyKeyReused ＝ 同一把鍵配上不同的請求內容。
//
// 🔴 呼叫端回 **9350**，而且**不得執行**。這一格就是 #3015 的實測缺陷：
// 舊行為是「body 不同就當成全新請求」⇒ 建立了第二筆資料。
//
// 為什麼不是靜默回放第一次的結果：那會讓呼叫端以為它送出的**新**內容被接受了，
// 而實際上什麼都沒改。RFC 慣例也是明確拒絕。
var ErrIdempotencyKeyReused = errors.New("invoice: idempotency key reused with a different request body")

// IdempotencyOutcome 是認領的結果。
type IdempotencyOutcome struct {
	// FirstTime ＝ 這一次是第一次，呼叫端往下做業務寫入，完成後呼叫 RecordResult。
	FirstTime bool
	// ResultRef 只在 FirstTime=false 時有意義：第一次成功的業務主鍵，用來回放。
	ResultRef int64
}

// RequestFingerprint 把請求的**語意內容**折成一個雜湊。
//
// 🔴 傳入的欄位順序與內容決定它的鑑別力：漏掉一個使用者改得動的欄位，
// 「同鍵不同內容」就抓不到那一種改法。用 \x1f（unit separator）分隔而不是逗號，
// 是因為欄位值本身可能含逗號（例如買受人抬頭）⇒ 不同的輸入會折成同一個字串。
func RequestFingerprint(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}

// ClaimIdempotency 在交易內認領一把鍵。
//
// 三態：
//   - FirstTime=true            ⇒ 往下做業務寫入
//   - FirstTime=false, ResultRef ⇒ 回放（呼叫端讀那一筆回傳，不重跑）
//   - ErrIdempotencyKeyReused    ⇒ 同鍵不同內容，回 9350 且不執行
//
// key 為空字串時**直接放行**（FirstTime=true）而不是報錯：本服務有內部呼叫端
// （MQ consumer、worker）不帶冪等鍵，它們的重複由各自的 dedup 機制處理。
// gateway 那一層已經把「對外端點必填」擋在前面。
func ClaimIdempotency(ctx context.Context, repo domain.IdempotencyRepository,
	in domain.ClaimIdempotency) (IdempotencyOutcome, error) {
	if repo == nil || strings.TrimSpace(in.Key) == "" {
		return IdempotencyOutcome{FirstTime: true}, nil
	}
	rec, claimed, err := repo.Claim(ctx, in)
	if err != nil {
		return IdempotencyOutcome{}, err
	}
	if claimed {
		return IdempotencyOutcome{FirstTime: true}, nil
	}
	if rec.RequestHash != in.RequestHash {
		return IdempotencyOutcome{}, ErrIdempotencyKeyReused
	}
	if rec.ResultRef == nil {
		// 已認領但結果還沒補上。同交易的中間狀態正常讀不到這一格；
		// 讀到代表第一次的交易失敗後留下了紀錄（不該發生）或有人在交易外呼叫。
		// 回「第一次」讓業務寫入自己的唯一鍵去擋，比回放一個不存在的結果安全。
		return IdempotencyOutcome{FirstTime: true}, nil
	}
	return IdempotencyOutcome{ResultRef: *rec.ResultRef}, nil
}
