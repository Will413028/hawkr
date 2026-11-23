// Package model 放 imageai 的型別與常數。
package model

import (
	"errors"
	"time"
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

// Usage 是一次扣額度之後的本週狀態；扣失敗時 Used 等於 Limit。
type Usage struct {
	Used, Limit int32
	// PeriodStart 是本週的起點（週一 00:00 Asia/Taipei），Refund 以它找回同一列。
	PeriodStart time.Time
	// ResetTime 是下週的起點，額度恢復的時刻。
	ResetTime time.Time
}
