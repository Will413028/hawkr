// Package model 提供買家資料型別與輸入規則。
package model

import (
	"errors"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Status stint 的生命週期狀態。
type Status string

const (
	// Active 存活中的買家身分。
	Active Status = "ACTIVE"
	// PendingDeletion 等待未結訂單完成的買家身分。
	PendingDeletion Status = "PENDING_DELETION"
	// Closed 已關閉的歷史買家身分。
	Closed Status = "CLOSED"
)

// Gender 買家自行提供的性別。
type Gender string

const (
	// Male 男性。
	Male Gender = "MALE"
	// Female 女性。
	Female Gender = "FEMALE"
	// Other 其他性別。
	Other Gender = "OTHER"
)

// MembershipSource 會員權益的來源。
type MembershipSource string

const (
	// MembershipAdmin 為實際儲存的管理員指派來源。
	MembershipAdmin MembershipSource = "ADMIN"
	// MembershipSystemDefault 僅用於記憶體中合成的免費權益，不寫入資料庫。
	MembershipSystemDefault MembershipSource = "SYSTEM_DEFAULT"
)

// MembershipStatus 會員指派現況或歷史狀態。
type MembershipStatus string

const (
	// MembershipActive 目前生效的指派。
	MembershipActive MembershipStatus = "ACTIVE"
	// MembershipSuperseded 已被替代的指派。
	MembershipSuperseded MembershipStatus = "SUPERSEDED"
)

// StatusValues 回傳實際儲存的全部買家狀態。
func StatusValues() []Status { return []Status{Active, PendingDeletion, Closed} }

// GenderValues 回傳實際儲存的全部性別。
func GenderValues() []Gender { return []Gender{Male, Female, Other} }

// MembershipSourceValues 回傳實際儲存的來源，供 CHECK 驗證。
func MembershipSourceValues() []MembershipSource {
	// SYSTEM_DEFAULT 僅為記憶體中的免費權益來源，不寫入 memberships。
	return []MembershipSource{MembershipAdmin}
}

// MembershipStatusValues 回傳全部合法值，供 CHECK 與契約驗證。
func MembershipStatusValues() []MembershipStatus {
	return []MembershipStatus{MembershipActive, MembershipSuperseded}
}

// Profile 買家 stint 的個人資料。
type Profile struct {
	BuyerID, AccountID int64
	Version            int64

	Status                   Status
	DisplayName              string
	Phone, Avatar, City, Job *string
	Gender                   *Gender
	BirthDate                *time.Time
	CreatedAt, UpdatedAt     time.Time
	DeletedAt, PIIClearedAt  *time.Time
}

// UpdateProfileInput 是部分更新；nil 保留原值。DisplayName 不接受空白；Phone、Gender、City、Job
// 的空字串清成 NULL，BirthDate 指向零時間時清成 NULL。AvatarStagedToken 帶上傳完成的 staged token。
type UpdateProfileInput struct {
	BuyerID           int64
	ExpectedVersion   int64
	DisplayName       *string
	Phone             *string
	Gender            *Gender
	BirthDate         *time.Time
	City, Job         *string
	AvatarStagedToken *string
}

// Display 顯示資料；CLOSED stint 回 Deleted 且名稱與頭像為 nil。
type Display struct {
	BuyerID             int64
	DisplayName, Avatar *string
	Deleted             bool
}

// Address 買家擁有的收貨地址。
type Address struct {
	AddressID, BuyerID                                   int64
	Version                                              int64
	ReceiverName, ReceiverPhone, TownCode, AddressDetail string
	IsDefault                                            bool
	CreatedAt, UpdatedAt                                 time.Time
}

// AddressInput 收貨地址新增或修改輸入。
type AddressInput struct {
	BuyerID, AddressID                                   int64
	ExpectedVersion                                      int64
	ReceiverName, ReceiverPhone, TownCode, AddressDetail string
	IsDefault                                            bool
}

// Cursor 時間與業務 ID 組成的降冪分頁位置。
type Cursor struct {
	Time time.Time
	ID   int64
}

// Valid 是 cursor 的內容合法（解自 client 回傳的 page token，不可信）。
func (c Cursor) Valid() bool { return c.ID > 0 && !c.Time.IsZero() }

// AddressCursor 地址排序位置，預設地址在普通地址之前。
type AddressCursor struct {
	Default bool
	Time    time.Time
	ID      int64
}

// AddressListInput 地址 keyset 清單的輸入。
type AddressListInput struct {
	BuyerID  int64
	PageSize int32
	After    *AddressCursor
}

// AddressPage 一頁地址及下一頁位置。
type AddressPage struct {
	Items []Address
	Next  *AddressCursor
}

// ListInput 買家關聯清單的 keyset 輸入，預設 20、最多 100 筆。
type ListInput struct {
	BuyerID  int64
	PageSize int32
	After    *Cursor
}

// Item 關聯對象的 ID 與業務時間。
type Item struct {
	ID   int64
	Time time.Time
}

// Page 一頁關聯資料及下一頁位置。
type Page struct {
	Items []Item
	Next  *Cursor
}

// RestockCursor 補貨訂閱的時間與商品 ID 分頁位置。
type RestockCursor struct {
	Time time.Time
	ID   int64
}

// RestockListInput 待通知補貨訂閱的分頁輸入。
type RestockListInput struct {
	BuyerID  int64
	PageSize int32
	After    *RestockCursor
}

// Restock 待通知的買家與商品關聯。
type Restock struct {
	BuyerID, ProductID int64
	CreatedAt          time.Time
}

// RestockPage 一頁補貨訂閱及下一頁位置。
type RestockPage struct {
	Items []Restock
	Next  *RestockCursor
}

// SubscribersInput 店家粉絲的 keyset 分頁輸入。
type SubscribersInput struct {
	StoreID  int64
	PageSize int32
	After    *Cursor
}

// TimeWindow 是時間窗 [Start, End)。
type TimeWindow struct{ Start, End time.Time }

// SubscriberStats 是店家的粉絲總數，以及每個時間窗內新增的追蹤數（New 與請求的窗同長、同順序）。
type SubscriberStats struct {
	Total int64
	New   []int64
}

// MaxStatsWindows 是一次查詢粉絲統計最多接受的時間窗數。
const MaxStatsWindows = 4

// AdminStats 後台買家統計。
type AdminStats struct{ Total int64 }

// Plan 會員方案與額度設定。
type Plan struct {
	PlanCode, DisplayName string
	Version               int64
	MonthlyAnswerLimit    int32
	MonthlyPriceTWD       int64
	IsPurchasable         bool
	CreatedAt, UpdatedAt  time.Time
}

// UpdatePlanInput 會員方案的部分更新；nil 欄位保留原值。
type UpdatePlanInput struct {
	PlanCode           string
	ExpectedVersion    int64
	MonthlyAnswerLimit *int32
	MonthlyPriceTWD    *int64
	IsPurchasable      *bool
}

// Entitlement 有效方案、額度與期間。
type Entitlement struct {
	PlanCode               string
	MonthlyAnswerLimit     int32
	PeriodStart, PeriodEnd time.Time
	Source                 MembershipSource
}

// AssignMembershipInput 帶 request_id 的人工會員指派輸入。
type AssignMembershipInput struct {
	BuyerID    int64
	PlanCode   string
	PeriodDays int
	RequestID  string
	GrantedBy  *int64
	Note       string
}

// FavoriteAdded 帶 UUIDv7、OccurredAt 與業務 ID 的 FavoriteAdded 事件。
type FavoriteAdded struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	BuyerID, ProductID int64
}

// FavoriteRemoved 帶 UUIDv7、OccurredAt 與業務 ID 的 FavoriteRemoved 事件。
type FavoriteRemoved struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	BuyerID, ProductID int64
}

// SellerFollowed 帶 UUIDv7、OccurredAt 與業務 ID 的 SellerFollowed 事件。
type SellerFollowed struct {
	OccurredAt       time.Time
	EventID          uuid.UUID
	BuyerID, StoreID int64
}

// ProductViewed 帶 UUIDv7、OccurredAt 與業務 ID 的 ProductViewed 事件。
type ProductViewed struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	BuyerID, ProductID int64
}

// RestockTargeted 帶 UUIDv7、OccurredAt 與業務 ID 的 RestockTargeted 事件。
type RestockTargeted struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	BuyerID, ProductID int64
	ProductName        string
	ImageKey           string
}

// FanOutInput 是一批補貨通知的輸入：上游補貨事件的商品與顯示快照，以及事件發生的時間。全部來自事件本身，
// 分批與重試都沿用同一份（M4）；OccurredAt 必填，零值回 InvalidInput，不以處理當下的時間代替。
type FanOutInput struct {
	ProductID   int64
	Limit       int32
	ProductName string
	ImageKey    string
	OccurredAt  time.Time
}

// BuyerDeleted 帶 UUIDv7、OccurredAt 與業務 ID 的 BuyerDeleted 事件。
type BuyerDeleted struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	BuyerID, AccountID int64
}

// BuyerDeletionRequested 買家進入 PENDING_DELETION（刪帳申請，不能撤回）的事件，帶 UUIDv7、OccurredAt 與業務 ID；只在買家真的從
// ACTIVE 轉成 PENDING_DELETION 的 transaction 內發布一次，重送不重複。訂閱者只該排入自己的清除 job（單體規則 4）。
type BuyerDeletionRequested struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	BuyerID, AccountID int64
}

// Error 將買家查詢的無資料錯誤轉成 NotFound。
func Error(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("找不到買家資料")
	}
	return err
}

// Positive 驗證每個業務 ID 都是正數。
func Positive(ids ...int64) error {
	for _, id := range ids {
		if id <= 0 {
			return apperr.InvalidInput("ID 必須為正數")
		}
	}
	return nil
}

// PageOf 截取指定頁面大小，從最後一筆建立下一頁 cursor。
func PageOf(items []Item, size int32) Page {
	page := Page{Items: items}
	if len(items) > int(size) {
		page.Items = items[:size]
		last := page.Items[len(page.Items)-1]
		page.Next = &Cursor{Time: last.Time, ID: last.ID}
	}
	return page
}

// MaxBatchSize 是一次批次查詢最多接受的 ID 數量。
const MaxBatchSize = 100

// MaxAddresses 是每個買家的收貨地址上限（Will 2026-09-30 W4）；地址清單的預設頁大小等於它，一頁就是完整清單。
const MaxAddresses = 20

// BatchIDs 驗證批次大小及每個 ID，空清單是合法的空查詢。
func BatchIDs(ids []int64) error {
	if len(ids) > MaxBatchSize {
		return apperr.InvalidInput("批次最多接受 %d 個 ID", MaxBatchSize)
	}
	return Positive(ids...)
}
