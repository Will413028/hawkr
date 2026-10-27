package model

import (
	"context"
	"errors"
)

// ErrNotSecure 表示 ReadSecure 的資產不是機敏資產（media 回 400）：呼叫方把它當成設定或資料的錯誤，不是「找不到」。
var ErrNotSecure = errors.New("media：不是機敏資產")

// ErrStagedTokenRejected 表示 media 拒絕了轉正的 staged token（無效、過期、不屬於上傳者）；見 media.ErrStagedTokenRejected。
var ErrStagedTokenRejected = errors.New("media：staged token 不可用")

// Uploader 是上傳暫存圖片的人。轉正時 media 確認 staged token 屬於他；由呼叫方傳入，media 不自己反查
// 登入者（ADR 2026-09-24-go-modular-monolith D2）。帳號與管理員的 ID 各自發號、會同號（identity 的盤點 G8），
// 所以要帶上是哪一種。
type Uploader struct {
	ID    int64
	Admin bool
}

// Ref 是引用資產的業務列：Type 是本套件的 RefType 常數，ID 是該列的主鍵。
type Ref struct {
	Type string
	ID   string
}

// Asset 是轉正後的資產。業務列只存 ObjectKey，顯示用的 URL 在讀取時組裝。
type Asset struct {
	ID        string
	ObjectKey string
	// Secure 是 media 回報這個資產是機敏檔案（上傳時走機敏通道，內容在 media 加密落庫）；證件這類資產的擁有者要求它為 true。
	Secure bool
}

// Media 是其他模組對 media 的呼叫。
//
// 引用（asset 與業務列的關係）只有擁有該業務列的模組的同步 job 寫入（refsync.Sync）：寫入端在 transaction 外只做
// Finalize，transaction 內寫業務列、把被換掉的資產記成退場並排入同步 job；同步 job 依業務列目前的狀態 Link，再
// Unlink 精確列出的退場資產。呼叫都在 DB transaction 之外。
type Media interface {
	// Finalize 把 staged token 轉正成資產，不建立引用。轉正是冪等的：同一個 token 在有效期間內回傳同一個資產。
	// 轉正了卻沒有任何引用的資產，由 media 的 GC 在建立滿 7 天後回收。
	Finalize(ctx context.Context, uploader Uploader, stagedToken string, refType string) (Asset, error)

	// Link 登記 ref 對 assetIDs 的引用。冪等：已存在的引用不重複建立，所以可以重試。
	Link(ctx context.Context, ref Ref, assetIDs []string) error

	// Unlink 只移除 ref 對 assetIDs 的引用，不碰 ref 的其他引用；沒有的引用忽略。
	Unlink(ctx context.Context, ref Ref, assetIDs []string) error

	// ReadSecure 讀機敏資產（secure/ 前綴、已解密）的內容。授權述詞是 (objectKey, ref.Type, ref.ID) 三者與轉正當時全等：
	// 查無與越權無法分辨（都是 NotFound）。呼叫方必須先自己授權（擁有者端的規則，例如 KYC 的限時授權），media 只確認三元組。
	// 在 DB transaction 之外呼叫。資產不是機敏的（400）回 ErrNotSecure，與 NotFound 分開。B1 之後只有這個 port 後面的 adapter 換成單體內
	// 的 media（擁有者的端點仍然代理、每次讀取都先記錄；簽章網址記不到讀取，R2 存的又是 media 加密過的密文）。
	ReadSecure(ctx context.Context, objectKey string, ref Ref) (SecureAsset, error)
}

// SecureAsset 是機敏資產的內容。
type SecureAsset struct {
	Data        []byte
	ContentType string
	// Filename 是顯示用的檔名（media 儲存的原始檔名，舊資產是 object key 的尾段）。
	Filename string
}
