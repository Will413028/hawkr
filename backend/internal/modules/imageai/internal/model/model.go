// Package model 放 imageai 的型別與常數。
package model

import (
	"errors"
	"time"

	"uuid"
)

const (
	// DefaultWeeklyLimit 是沒有個別設定的店每週可修圖的次數。
	DefaultWeeklyLimit int32 = 10
	// MaxWeeklyLimit 是後台能為一家店設定的每週次數上限。
	MaxWeeklyLimit int32 = 1000
	// DefaultMaxConcurrent 是每個 pod 同時進行的修圖數預設值（未量測）。
	DefaultMaxConcurrent = 21
)

// ErrQuotaExhausted 是本週額度用完。
var ErrQuotaExhausted = errors.New("imageai：本週修圖額度已用完")

// ErrNothingToRefund 是退額度時找不到可退的次數：那一週的列已不在（店家退場或已進入新的一週）或 used 已是 0。
var ErrNothingToRefund = errors.New("imageai：沒有可退的修圖額度")

// Usage 是一次扣額度之後的本週狀態；扣失敗時 Used 等於 Limit。
type Usage struct {
	Used, Limit int32
	// PeriodStart 是本週的起點（週一 00:00 Asia/Taipei），Refund 以它找回同一列。
	PeriodStart time.Time
	// ResetTime 是下週的起點，額度恢復的時刻。
	ResetTime time.Time
}

// StyleCategory 是風格適用的用途，儲存碼（styles.category 的 CHECK 與這裡相同）。
type StyleCategory string

const (
	// CategoryProduct 是商品圖。
	CategoryProduct StyleCategory = "PRODUCT"
	// CategoryPost 是貼文圖。
	CategoryPost StyleCategory = "POST"
)

// Style 是賣家看到的一個風格。
type Style struct {
	ID           uuid.UUID
	Name, Label  string
	Prompt       string
	ThumbnailKey string
	DisplayOrder int32
}

// StyleList 是一個用途的風格：DefaultPrompt 取自預設風格，預設風格本身不在 Styles 裡。
type StyleList struct {
	DefaultPrompt string
	Styles        []Style
}

const (
	// MaxImageBytes 是修圖原圖的大小上限（A9-D37）。
	MaxImageBytes = 2 << 20
	// MaxPromptRunes 是修圖提示詞去掉頭尾空白後的字元數上限（A9-D40）。
	MaxPromptRunes = 500
)

// TransformInput 是一次修圖：StoreID 是呼叫者的店家（入口層從 session 推導）。
type TransformInput struct {
	StoreID  int64
	Image    []byte
	MimeType string
	Prompt   string
}

// TransformOutput 是修好的圖與這次之後本週的用量。
type TransformOutput struct {
	Image    []byte
	MimeType string
	Usage    Usage
}

// AdminStyle 是後台看到的風格（含停用的）。
type AdminStyle struct {
	ID                 uuid.UUID
	Name, Label        string
	Prompt             string
	Category           StyleCategory
	ThumbnailKey       string
	DisplayOrder       int32
	IsDefault, Enabled bool
	// Version 是樂觀鎖的版本，對外是 etag。
	Version              int64
	CreatedAt, UpdatedAt time.Time
}

// Actor 是做後台寫入的管理員與後台 RPC 的 procedure 名稱（handler 組成，寫進 adminaudit）。
type Actor struct {
	AdminID int64
	Action  string
}

// CreateStyleInput 是新增風格：ThumbnailToken 必填。
type CreateStyleInput struct {
	Actor               Actor
	Name, Label, Prompt string
	Category            StyleCategory
	ThumbnailToken      string
	DisplayOrder        int32
	IsDefault, Enabled  bool
}

// UpdateStyleInput 是整筆覆寫風格可改的欄位（name 與 category 建立後不能改，所以不在這裡）。Version 必填；ThumbnailToken 空字串表示沿用目前的圖。
type UpdateStyleInput struct {
	Actor              Actor
	ID                 uuid.UUID
	Version            int64
	Label, Prompt      string
	ThumbnailToken     string
	DisplayOrder       int32
	IsDefault, Enabled bool
}

// AdminQuota 是一家店的每週額度（後台）：Custom 為 false 時是預設額度（沒有 Version 與 UpdatedAt）。
type AdminQuota struct {
	StoreID     int64
	WeeklyLimit int32
	WeeklyUsage int32
	Memo        string
	Custom      bool
	Version     int64
	UpdatedAt   time.Time
}

// UpdateQuotaInput 是為一家店個別設定額度：Version 為 0 表示沒帶 etag（只能建立）。
type UpdateQuotaInput struct {
	Actor       Actor
	StoreID     int64
	WeeklyLimit int32
	Memo        string
	Version     int64
}
