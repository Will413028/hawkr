package usecase

import (
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	bizcode "github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

var (
	ErrNotFound           = pkgerrors.NewNotFound("not found")
	ErrForbidden          = pkgerrors.NewForbidden("forbidden")
	ErrBadRequest         = pkgerrors.NewInvalidInput("bad request")
	ErrModerationRejected = pkgerrors.NewBusiness(bizcode.ModerationRejected, "內容不符社群守則")
	ErrReportRateLimited  = pkgerrors.NewBusiness(bizcode.ReportRateLimited, "檢舉過於頻繁，請稍後再試")
	ErrInvalidTargetType  = pkgerrors.NewInvalidInput("invalid target type")
	// ErrPostAlreadyPublished rejects a schedule on a post that is already
	// published (56 §7.6). The comment here used to say §6.5.1 had no code for
	// "the resource is already in the state you asked to schedule it into" and
	// settled for 9260 — that gap is now filled by 9291's sibling 9290 (#2744).
	// 9260 was a lie in the direction that matters: it says the operator sent a
	// malformed command, so the audit row blames them for a request that was
	// entirely well-formed. Same argument 錯誤碼.md already made for 9283/9284.
	ErrPostAlreadyPublished = pkgerrors.NewBusiness(bizcode.DelegationResourceAlreadyInState, "貼文已發布，無法設定排程發布")
	ErrInvalidReason        = pkgerrors.NewInvalidInput("invalid reason code")
	// 公開中的貼文恰好 1 張照片、內文必填（10d 第 3a 刀 B1）。legacy 沿用 InvalidInput；
	// 訊息只有繞過 client 才看得到。
	ErrPostImageRequired = pkgerrors.NewInvalidInput("每則貼文需要 1 張照片才能發布")
	ErrPostContentBlank  = pkgerrors.NewInvalidInput("貼文內容不能空白")
	ErrPostTooManyImages = pkgerrors.NewInvalidInput("每則貼文只能有 1 張照片")
	// ErrSellerSuspendedOperationBlocked：46 號 §4.4 self-vs-other 細分——停權賣家對
	// 「自己貼文/論壇」的留言寫入（管理性質，06 §四.10「客服/經營」情境）擋；買家或
	// 對他人商店的留言不受影響（ALLOW-ALWAYS，見 comment.go/forum.go 呼叫點）。
	ErrSellerSuspendedOperationBlocked = pkgerrors.NewBusiness(bizcode.SellerSuspendedOperationBlocked, "賣家帳號停權中，無法執行此操作")
)
