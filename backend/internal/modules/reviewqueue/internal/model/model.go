// Package model 是 reviewqueue 的型別與不碰 I/O 的規則：待辦的種類、一列待辦、keyset 的游標。
package model

import (
	"slices"
	"time"
	"uuid"
)

// Kind 是待辦的種類（step10e-3d-design.md §1.3）。值是 UPPER_SNAKE，與來源發布的 pub_*_review_items 的 kind 欄相同。
// 空字串與不在 Kinds 裡的值都不是有效的種類。
type Kind string

// seller 發布的 12 種（pub_review_items）。A7-1e 加 moderation 的種類時加在這裡與 Kinds。
const (
	ReportTriage          Kind = "REPORT_TRIAGE"
	CaseDecision          Kind = "CASE_DECISION"
	CaseInquiryOverdue    Kind = "CASE_INQUIRY_OVERDUE"
	CaseFDAReferral       Kind = "CASE_FDA_REFERRAL"
	AuthorityOrder        Kind = "AUTHORITY_ORDER"
	AuthorityTargetNotice Kind = "AUTHORITY_TARGET_NOTICE"
	MeasureAppeal         Kind = "MEASURE_APPEAL"
	MeasureLiftRequest    Kind = "MEASURE_LIFT_REQUEST"
	MeasureReview         Kind = "MEASURE_REVIEW"
	FoodRegistration      Kind = "FOOD_REGISTRATION"
	KYCApplication        Kind = "KYC_APPLICATION"
	PayUniBindRetry       Kind = "PAYUNI_BIND_RETRY"
)

// Kinds 是全部有效的種類。
var Kinds = []Kind{
	ReportTriage, CaseDecision, CaseInquiryOverdue, CaseFDAReferral, AuthorityOrder, AuthorityTargetNotice,
	MeasureAppeal, MeasureLiftRequest, MeasureReview, FoodRegistration, KYCApplication, PayUniBindRetry,
}

// Valid 回傳 k 是不是有效的種類。
func (k Kind) Valid() bool { return slices.Contains(Kinds, k) }

// Item 是一件待辦，欄位與來源列一一對應（seller.pub_review_item）。ItemID 是同一種類內唯一的鍵；對象的鍵依種類只有相關的有值，
// 其餘是 nil（哪一種用哪幾個，由 handler 依種類組成 subject）。StoreID 是 nil 表示沒有單一店家；StoreName 在沒有店家或店名已清理時是空字串。
type Item struct {
	Kind               Kind
	ItemID             string
	ReportID           *uuid.UUID
	CaseID             *uuid.UUID
	InquiryID          *uuid.UUID
	AuthorityOrderID   *uuid.UUID
	TargetPosition     *int16
	AccountID          *int64
	MeasureID          *uuid.UUID
	RequestID          *uuid.UUID
	FoodRegistrationID *uuid.UUID
	KYCApplicationID   *uuid.UUID
	StoreID            *int64
	StoreName          string
	Start              time.Time
	Due                time.Time
}

// Cursor 是 keyset 的位置：全序 (Due, Start, Kind, ItemID) 裡上一頁最後一列的鍵。時間精確到 µs（資料庫的精度）。
type Cursor struct {
	Due   time.Time
	Start time.Time
	Kind  Kind
	ID    string
}

// CursorOf 回傳 item 在全序裡的位置。
func CursorOf(item Item) Cursor {
	return Cursor{Due: item.Due, Start: item.Start, Kind: item.Kind, ID: item.ItemID}
}

// Valid 回傳游標是不是一個真的位置：兩個時間有值且精確到 µs、種類有效、ID 非空。解出來的 token 要先過它才能用。
func (c Cursor) Valid() bool {
	return !c.Due.IsZero() && c.Due.Equal(c.Due.Truncate(time.Microsecond)) &&
		!c.Start.IsZero() && c.Start.Equal(c.Start.Truncate(time.Microsecond)) &&
		c.Kind.Valid() && c.ID != ""
}
