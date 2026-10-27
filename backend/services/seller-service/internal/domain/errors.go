package domain

import (
	"fmt"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

// ErrVersionConflict is what a delegated write gets when the row moved between
// the read that produced expectedVersion and the write (56 §5.4 / §6.6.4).
//
// Affected-rows = 0 is treated as a version conflict outright and never as "look
// it up again and overwrite": the whole point of the version column is that the
// seller's concurrent edit must not be silently lost. The client's recovery is
// to reload, merge, and retry with a fresh expectedVersion — and, per §6.5.1, a
// fresh Idempotency-Key.
var ErrVersionConflict = pkgerrors.NewBusiness(
	bizcode.DelegationResourceVersionConflict, "資料已被更新，請重新載入後再試")

// ErrMerchantProfileVersionConflict is what the PAYUni merchant-profile PATCH
// command (issue #2806 Phase 2) gets when the caller's If-Match version no
// longer matches payuni_registrations.merchant_profile_version.
var ErrMerchantProfileVersionConflict = pkgerrors.NewBusiness(
	bizcode.PayuniMerchantProfileVersionConflict, "資料已被更新，請重新整理後再試")

// ErrMerchantProfileIdempotencyKeyReused is what the same command gets when a
// client reuses an Idempotency-Key with a different payload hash than the
// first request that used it.
var ErrMerchantProfileIdempotencyKeyReused = pkgerrors.NewBusiness(
	bizcode.PayuniMerchantProfileIdempotencyKeyReused, "這個操作已經處理過，但內容不同，請重新操作")

// ErrMerchantProfileNotActive is what the PAYUni merchant-profile PATCH
// command (issue #2806 Phase 2) gets when the seller's PayUni registration
// is not yet active (never bound, still pending PayUni review, or a failed
// bind) — updating contact/address fields is only meaningful after a
// successful bind (spec §5.3 step 2).
var ErrMerchantProfileNotActive = pkgerrors.NewBusiness(
	bizcode.PayuniMerchantProfileNotActive, "商家尚未完成 PayUni 綁定或審核中，暫不可修改聯絡資訊")

// store_name／description 對應 PayUni 的 MerName／MerDesc，是 bind_merchant
// 建立後永久不可改的欄位（2026-09-10 以 update_merchant 逐參數實測確認）。
// 允許本地改只會造成永久分歧，而且兩邊都沒有修復路徑。
//
// 鎖從 submitted 起算而不是等資料真的送到 PayUni（forwarded）。理由不只是
// 一致性——admin 審核列表的店名是即時 JOIN sellers 取的，而 bind_merchant
// 送出的 MerName 也是核准當下現讀 seller.StoreName。若賣家在送審後還能改，
// admin 核准的值與實際送到 PayUni 的值可能不是同一個，等於送出一個沒有人
// 審過的店名。
//
// 兩個錯誤分開是因為賣家該做的事不同：審核中還能靠客服請 admin 退件，退件
// 後 cert 進 rejected 就解鎖；資料已到 PayUni 之後我們這邊誰都改不了。
//
// forward_failed 刻意不鎖：bind 失敗代表 PayUni 手上什麼都沒有，而且常常正是
// 資料有問題才失敗，鎖住會把賣家困在一個他無法自救的狀態。
//
// 用既有的 PreconditionFailed 而非新增專屬業務碼：這個錯誤在正常流程裡不該
// 出現——前端會把欄位設成唯讀，這裡是防線（防 DOM 改動、防直接呼叫 API、防
// 未來新增的呼叫端），不是常態路徑。為此新增 CodeMeta 碼要連帶同步四份前端
// 拷貝，代價與收益不成比例。

// ErrPayuniLockedFieldUnderReview：cert 在 submitted，資料還沒送到 PayUni。
var ErrPayuniLockedFieldUnderReview = pkgerrors.NewPreconditionFailed(
	"審核期間店名與商店介紹無法修改，如需變更請聯絡客服")

// ErrPayuniLockedField：cert 在 forwarded／pending_info／approved，PayUni
// 已經持有這家店的資料。
var ErrPayuniLockedField = pkgerrors.NewPreconditionFailed(
	"資料已送至 PayUni，店名與商店介紹無法修改，如需變更請聯繫 PayUni")

// ErrInvalidTransition represents an invalid state transition error.
// Implements DomainCoder interface (duck typing) so the error interceptor
// maps it to gRPC FailedPrecondition without importing go-pkg/errors.
type ErrInvalidTransition struct {
	From string
	To   string
}

// Error returns the user-facing Chinese message. This is what actually
// reaches Admin: grpc/handler.go's ApproveCertification/RejectCertification
// (and every other known-DomainCode branch in that file, ~55 call sites)
// embed err.Error() directly into a successful gRPC response's Message
// field — DomainMessage() is only read by the gRPC-status interceptor path,
// which a known-code domain error like this one never takes. Matches every
// other DomainError in this package, which already puts Chinese text
// straight in Error() (e.g. ErrMerchantProfileVersionConflict).
func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("目前狀態為「%s」，無法轉換到「%s」", certStateLabel(e.From), certStateLabel(e.To))
}

// DomainCode returns 1200 (CodeInvalidTransition).
func (e *ErrInvalidTransition) DomainCode() int { return 1200 }

// DomainMessage mirrors Error() — see its doc comment for why they must
// match here (unlike callers that only take the interceptor path).
func (e *ErrInvalidTransition) DomainMessage() string { return e.Error() }
