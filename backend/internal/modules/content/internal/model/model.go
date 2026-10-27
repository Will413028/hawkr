package model

import (
	"time"
	"uuid"
)

// BannerActionType 是 banner 被點擊後的行為（設計書 25）。
type BannerActionType string

const (
	// BannerActionPopup 顯示彈窗大圖，不帶其他值。
	BannerActionPopup BannerActionType = "popup"
	// BannerActionURL 開啟外部網頁，帶 URL。
	BannerActionURL BannerActionType = "url"
	// BannerActionProduct 開啟商品頁，帶商品 ID。
	BannerActionProduct BannerActionType = "product"
	// BannerActionPost 開啟貼文頁，帶貼文 ID。
	BannerActionPost BannerActionType = "post"
)

// BannerAction 是點擊行為。Target 是 url 的網址、product 與 post 的 ID；popup 沒有值。
// 存成 banners 的 action_type、action_target 兩欄，DB 的 CHECK 是第二道防線。
type BannerAction struct {
	Type   BannerActionType
	Target string
}

// Banner 是一則首頁 banner。
type Banner struct {
	ID              uuid.UUID
	Title           string
	ButtonText      string
	PreviewImageKey string
	PopupImageKey   string
	// CanvasJSON 是後台編輯器的畫布狀態，後端不解讀。
	CanvasJSON   string
	Action       BannerAction
	DisplayOrder int32
	IsActive     bool
	// StartsAt、EndsAt 是顯示期間；nil 表示該端不設限。
	StartsAt *time.Time
	EndsAt   *time.Time
	// Version 是樂觀鎖的版本，對外是 etag；修改時帶讀到的值。
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BannerCursor 是 banner 清單的排序鍵。清單依 (DisplayOrder, CreatedAt, ID) 排列，下一頁從它之後開始。
type BannerCursor struct {
	DisplayOrder int32
	CreatedAt    time.Time
	ID           uuid.UUID
}

// BannerPage 是一頁 banner。Next 為 nil 表示這是最後一頁。
type BannerPage struct {
	Banners []Banner
	Next    *BannerCursor
}

// BannerInput 是新增與修改 banner 的內容。兩張圖都是新上傳的圖的 staged token，空字串表示沿用現在的圖；
// 新增時兩張都必須上傳。
type BannerInput struct {
	Title             string
	ButtonText        string
	PreviewImageToken string
	PopupImageToken   string
	CanvasJSON        string
	Action            BannerAction
	DisplayOrder      int32
	IsActive          bool
	StartsAt          *time.Time
	EndsAt            *time.Time
}

// UpdateBannerInput 是覆寫 banner 的輸入。
type UpdateBannerInput struct {
	AdminID int64
	ID      uuid.UUID
	Version int64
	Banner  BannerInput
}

// AppType 是 App 的類型。值與對外契約 AppType 的值名去掉 APP_TYPE_ 前綴相同，也是資料庫存的值
// （ADR 2026-09-25-codemeta-retirement D3）。
type AppType string

const (
	AppTypeBuyer  AppType = "BUYER"
	AppTypeSeller AppType = "SELLER"
)

// AppTypes 是所有的 App 類型。資料庫的 CHECK 必須與它一致（integration 測試比對）。
func AppTypes() []AppType { return []AppType{AppTypeBuyer, AppTypeSeller} }

// AppVersionGate 是一支 App 的最低可用版本（設計書 54 §5.1），以 AppType 識別。App 只發 iOS，
// 所以沒有平台這個維度。
type AppVersionGate struct {
	AppType        AppType
	MinimumVersion string
	// StoreURL 為空表示還沒設定，App 會隱藏「前往更新」按鈕（§7.6 ④）。
	StoreURL    string
	NoticeTitle string
	NoticeBody  string
	// Version 是樂觀鎖的版本，對外是 etag；修改時帶讀到的值。
	Version   int64
	UpdatedAt time.Time
}

// VersionCheck 是版本檢查的結果。版本比較只在這裡做，App 只看 UpdateRequired（§4.1、§4.2）。
type VersionCheck struct {
	UpdateRequired bool
	MinimumVersion string
	StoreURL       string
	NoticeTitle    string
	NoticeBody     string
}
