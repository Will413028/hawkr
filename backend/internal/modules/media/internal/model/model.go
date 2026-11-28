package model

import (
	"errors"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// ErrNotSecure 表示 ReadSecure 的資產不是機敏資產：呼叫方把它當成設定或資料的錯誤，不是「找不到」。
var ErrNotSecure = errors.New("media：不是機敏資產")

// ErrNoKeyring 表示模組沒有 KEK 金鑰環：BeginSecureUpload、PutSecureChunk、PutSecure、ReadSecure、ReadStagedSecure、RewrapDEKs 拒絕執行，也不寫任何物件（沒有主密鑰就不寫）。
var ErrNoKeyring = errors.New("media：沒有設定 KEK，不能處理機敏資產")

// Uploader 是上傳暫存圖片的人。轉正時 media 確認 staged token 屬於他；由呼叫方傳入，media 不自己反查
// 登入者（ADR 2026-09-24-go-modular-monolith D2）。帳號與管理員的 ID 各自發號、會同號（identity 的盤點 G8），
// 所以要帶上是哪一種。
type Uploader struct {
	ID    int64
	Admin bool
	// Store 是代行營運時被代行的店家 ID（B1-D16），非代行是 0。機敏分段上傳開會話時記下，之後 token 的每個動作（續傳、預覽、轉正）都要帶同一個值，
	// 所以代行店家 A 開的會話不能被店家 B 的代行確認；非代行的 token 也不能被帶 Store 的呼叫者使用。
	Store int64
}

// ErrUploadAlreadyConfirmed 表示 token 已被另一個已提交的 transaction 確認（B1-D23）：呼叫端用 errors.Is 辨認，不比對文字。
var ErrUploadAlreadyConfirmed = errors.New("media：上傳已在別的 transaction 確認")

type confirmedElsewhere struct{ err error }

func (e *confirmedElsewhere) Error() string        { return e.err.Error() }
func (e *confirmedElsewhere) Unwrap() error        { return e.err }
func (e *confirmedElsewhere) Is(target error) bool { return target == ErrUploadAlreadyConfirmed }

// MarkConfirmedElsewhere 讓 err（分類與訊息不變）也符合 errors.Is(err, ErrUploadAlreadyConfirmed)。
func MarkConfirmedElsewhere(err error) error { return &confirmedElsewhere{err} }

var ErrAssetGone = errors.New("media：資產已在刪除，來不及保全")

var ErrHoldContended = apperr.Unavailable("media：保全撞上資產閘門的整理，請重試整個 transaction")

var ErrAssetNotFound = errors.New("media：資產不存在")

var ErrHoldNotFound = errors.New("media：找不到該來源的保全")

const CopySource = "media.copy_to_secure"

const CopyHoldTTL = time.Hour
