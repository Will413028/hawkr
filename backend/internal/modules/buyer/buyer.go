// Package buyer 提供買家模組公開 API。
package buyer

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/address"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/block"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/deletion"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/favorite"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/follow"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/membership"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/profile"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/restock"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer/internal/visit"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生，編譯後的 binary 不可能遇到
	}
	return sub
}

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	// Media 是 media 模組（*media.Module）：頭像在寫入的 transaction 內 Confirm，被換掉或終態的資產 MarkDeleted。
	Media          model.Media
	Now            func() time.Time
	NicknameNumber func() int
	Logger         *slog.Logger
}

// Module 買家模組的公開 use case 容器。
type Module struct {
	profile    *profile.Service
	address    *address.Service
	favorite   *favorite.Service
	follow     *follow.Service
	block      *block.Service
	visit      *visit.Service
	restock    *restock.Service
	membership *membership.Service
	deletion   *deletion.Service
}

// New 建立服務，外部依賴由組裝點提供。
func New(deps Deps) *Module {
	if deps.Now == nil {
		panic("buyer: Now is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &Module{
		profile: profile.New(profile.Deps{
			DB:             deps.DB,
			Media:          deps.Media,
			Now:            deps.Now,
			NicknameNumber: deps.NicknameNumber,
			Logger:         deps.Logger,
		}),
		address:    address.New(address.Deps{DB: deps.DB, Now: deps.Now}),
		membership: membership.New(membership.Deps{DB: deps.DB, Now: deps.Now}),
		deletion: deletion.New(deletion.Deps{
			DB:     deps.DB,
			Events: deps.Events,
			Media:  deps.Media,
			Now:    deps.Now,
		}),
		favorite: favorite.New(favorite.Deps{DB: deps.DB, Events: deps.Events, Now: deps.Now}),
		follow:   follow.New(follow.Deps{DB: deps.DB, Events: deps.Events, Now: deps.Now}),
		block:    block.New(block.Deps{DB: deps.DB, Now: deps.Now}),
		visit:    visit.New(visit.Deps{DB: deps.DB, Events: deps.Events, Now: deps.Now}),
		restock:  restock.New(restock.Deps{DB: deps.DB, Events: deps.Events, Now: deps.Now}),
	}
}

// Status stint 的生命週期狀態。
type Status = model.Status

// AccountRef 是買家與所屬帳號的對應（ListLiveAccounts 的元素）。
type AccountRef = model.AccountRef

// MaxLiveAccountsPage 是 ListLiveAccounts 一頁的上限。
const MaxLiveAccountsPage = model.MaxLiveAccountsPage

// Gender 買家自行提供的性別。
type Gender = model.Gender

// Profile 買家 stint 的個人資料。
type Profile = model.Profile

// UpdateProfileInput 個人資料的部分更新輸入；nil 欄位保留原值。
// DisplayName 空字串不合法；Phone/Gender/City/Job 空字串、BirthDate 零值清除該欄。
type UpdateProfileInput = model.UpdateProfileInput

// Display 顯示資料；CLOSED stint 回 Deleted 且名稱、頭像與頭像資產 ID 為 nil。
type Display = model.Display

// Address 買家擁有的收貨地址。
type Address = model.Address

// AddressInput 收貨地址新增或修改輸入。
type AddressInput = model.AddressInput

// AddressListInput 地址 keyset 清單的輸入。
type AddressListInput = model.AddressListInput

// AddressPage 一頁地址及下一頁位置。
type AddressPage = model.AddressPage

// AddressCursor 地址清單的排序位置。
type AddressCursor = model.AddressCursor

// Cursor 時間與業務 ID 組成的降冪分頁位置。
type Cursor = model.Cursor

// ListInput 買家關聯清單的 keyset 輸入，預設 20、最多 100 筆。
type ListInput = model.ListInput

// Item 關聯對象的 ID 與業務時間。
type Item = model.Item

// Page 一頁關聯資料及下一頁位置。
type Page = model.Page

// RestockCursor 補貨訂閱的時間與商品 ID 分頁位置。
type RestockCursor = model.RestockCursor

// RestockListInput 待通知補貨訂閱的分頁輸入。
type RestockListInput = model.RestockListInput

// Restock 待通知的買家與商品關聯。
type Restock = model.Restock

// RestockPage 一頁補貨訂閱及下一頁位置。
type RestockPage = model.RestockPage

// SubscribersInput 店家粉絲的 keyset 分頁輸入。
type SubscribersInput = model.SubscribersInput

// TimeWindow 是統計的時間窗 [Start, End)。
type TimeWindow = model.TimeWindow

// SubscriberStats 是粉絲總數及各時間窗的新增數。
type SubscriberStats = model.SubscriberStats

// MaxStatsWindows 是 SubscriberStats 一次最多接受的時間窗數。
const MaxStatsWindows = model.MaxStatsWindows

// AdminStats 後台買家統計。
type AdminStats = model.AdminStats

// Entitlement 有效方案、額度與期間。
type Entitlement = model.Entitlement

const (
	// Active 存活中的買家身分。
	Active = model.Active
	// PendingDeletion 等待未結訂單完成的買家身分。
	PendingDeletion = model.PendingDeletion
	// Closed 已關閉的歷史買家身分。
	Closed = model.Closed
	// Male 男性。
	Male = model.Male
	// Female 女性。
	Female = model.Female
	// Other 其他性別。
	Other = model.Other
)

// OnAccountCreated 在 identity 建帳的 transaction 內建立帳號唯一的買家（買家不是 identity 的角色，不授予角色）；失敗時建帳整筆回滾。
func (module *Module) OnAccountCreated(ctx context.Context, e identity.AccountCreated) error {
	return module.profile.Create(ctx, e.AccountID, e.Name)
}

// Profile 讀取帳號目前存活的 stint。
func (module *Module) Profile(ctx context.Context, accountID int64) (Profile, error) {
	return module.profile.Profile(ctx, accountID)
}

// CurrentBuyerID 只回傳現任 buyer_id，不讀取 profile 個資。
func (module *Module) CurrentBuyerID(ctx context.Context, accountID int64) (int64, error) {
	return module.profile.CurrentBuyerID(ctx, accountID)
}

// ProfileByBuyerID 讀取存活 profile；closed stint 回 NotFound。
func (module *Module) ProfileByBuyerID(ctx context.Context, buyerID int64) (Profile, error) {
	return module.profile.ProfileByBuyerID(ctx, buyerID)
}

// LockActive 在呼叫方的 transaction 內以 FOR KEY SHARE 確認買家是營運中（ACTIVE），鎖持有到 transaction 結束：BeginDeletion 的排他鎖要等
// 這個 transaction 提交，所以買家的社群寫入（讚、分享、留言）若在刪帳申請之前成立，一定在清除 job 開始之前提交；申請之後才開始的
// 寫入回 FailedPrecondition。已關閉回 NotFound。鎖順序：帳號列、買家列，然後才是呼叫方自己的表。必須在 transaction 內呼叫
// （沒有 transaction 時鎖在語句結束就放掉，沒有意義）。
func (module *Module) LockActive(ctx context.Context, buyerID int64) error {
	return module.profile.LockActive(ctx, buyerID)
}

// LockNotClosed 在呼叫方的 transaction 內以 FOR KEY SHARE 確認買家尚未關閉（ACTIVE 與 PENDING_DELETION 都放行），鎖持有到 transaction 結束：
// BeginDeletion 與 CloseForDeletion 的 FOR UPDATE 要等這個 transaction 提交。已關閉與不存在回 NotFound。必須在 transaction 內呼叫
// （沒有 transaction 時鎖在語句結束就放掉，沒有意義）。鎖順序同 LockActive：帳號列、買家列，然後才是呼叫方自己的表。
func (module *Module) LockNotClosed(ctx context.Context, buyerID int64) error {
	return module.profile.LockNotClosed(ctx, buyerID)
}

// ShareLockByAccount 在呼叫方的 transaction 內以共享鎖（FOR KEY SHARE）鎖住帳號唯一的買家列，讓買家列在 commit 之前不被關閉或改變狀態；
// 給只需要「買家在寫入完成前沒有關閉」而不改買家的讀者（例如把通知寫給收件買家）。買家已關閉時回 deleted=true，刪除中放行，帳號沒有買家回
// NotFound。必須在 transaction 內呼叫。鎖順序：帳號列、買家列，然後才是呼叫方自己的表。
func (module *Module) ShareLockByAccount(ctx context.Context, accountID int64) (deleted bool, err error) {
	return module.profile.ShareLockByAccount(ctx, accountID)
}

// ListLiveAccounts 列出 buyer_id 大於 afterBuyerID、尚未關閉（營運中與刪除中）的買家與其帳號，依 buyer_id 由小到大，最多 limit 筆
// （1 到 MaxLiveAccountsPage）；不足 limit 筆表示沒有更多。給通知公告這類對全體買家的大量派送：不加鎖，寫入端以 ShareLockByAccount 守門。
func (module *Module) ListLiveAccounts(ctx context.Context, afterBuyerID int64, limit int) ([]AccountRef, error) {
	return module.profile.ListLiveAccounts(ctx, afterBuyerID, limit)
}

// ListLiveFollowerAccounts 列出追蹤店家 storeID、buyer_id 大於 afterBuyerID、尚未關閉（營運中與刪除中）的買家與其帳號，依 buyer_id 由小到大，
// 最多 limit 筆（1 到 MaxLiveAccountsPage）；不足 limit 筆表示沒有更多。給通知對追蹤者的 fan-out：不加鎖，寫入端以 ShareLockByAccount 守門。
func (module *Module) ListLiveFollowerAccounts(ctx context.Context, storeID, afterBuyerID int64, limit int) ([]AccountRef, error) {
	return module.follow.ListLiveFollowerAccounts(ctx, storeID, afterBuyerID, limit)
}

// AccountIDByBuyerID 讀取歷史或現任 stint 所屬的帳號鍵，不曝露個資。
func (module *Module) AccountIDByBuyerID(ctx context.Context, buyerID int64) (int64, error) {
	return module.profile.AccountIDByBuyerID(ctx, buyerID)
}

// BatchDisplay 批次讀取顯示資料，CLOSED stint 回 Deleted 且不回個資；最多 MaxBatchSize 個 ID。
func (module *Module) BatchDisplay(ctx context.Context, ids []int64) ([]Display, error) {
	return module.profile.BatchDisplay(ctx, ids)
}

// BatchMeta 批次讀取存活 profile；未找到的 ID 略過，最多 MaxBatchSize 個 ID。
func (module *Module) BatchMeta(ctx context.Context, ids []int64) ([]Profile, error) {
	return module.profile.BatchMeta(ctx, ids)
}

// ResolvePrincipals 回傳存活 buyer_id 到 account_id 的映射；未回傳的 ID 為 unresolved，最多 MaxBatchSize 個 ID。
func (module *Module) ResolvePrincipals(ctx context.Context, ids []int64) (map[int64]int64, error) {
	return module.profile.ResolvePrincipals(ctx, ids)
}

// BuyerIDsByAccount 列出帳號所有目前與歷史 stint。
func (module *Module) BuyerIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	return module.profile.BuyerIDsByAccount(ctx, accountID)
}

// UpdateProfile 在列鎖內合併提供的欄位；未提供的欄位保留原值。
func (module *Module) UpdateProfile(ctx context.Context, input UpdateProfileInput) (Profile, error) {
	return module.profile.UpdateProfile(ctx, input)
}

// StatsForAdmin 計算目前存活買家數量。
func (module *Module) StatsForAdmin(ctx context.Context) (AdminStats, error) {
	return module.profile.StatsForAdmin(ctx)
}

// ListAddresses 列出買家的全部地址，預設地址優先。
func (module *Module) ListAddresses(ctx context.Context, buyerID int64) ([]Address, error) {
	return module.address.List(ctx, buyerID)
}

// PageAddresses 依預設標記、更新時間及地址 ID 分頁。
func (module *Module) PageAddresses(ctx context.Context, input AddressListInput) (AddressPage, error) {
	return module.address.Page(ctx, input)
}

// GetAddress 讀取買家擁有的地址；不存在或不屬於該買家均回 NotFound。
func (module *Module) GetAddress(ctx context.Context, buyerID, addressID int64) (Address, error) {
	return module.address.Get(ctx, buyerID, addressID)
}

// AddAddress 新增地址，第一筆自動成為預設。
func (module *Module) AddAddress(ctx context.Context, input AddressInput) (Address, error) {
	return module.address.Add(ctx, input)
}

// UpdateAddress 修改地址；原預設不因 IsDefault=false 而清空。
func (module *Module) UpdateAddress(ctx context.Context, input AddressInput) (Address, error) {
	return module.address.Update(ctx, input)
}

// DeleteAddress 刪除地址；刪除預設時以最近更新的剩餘地址遞補。
func (module *Module) DeleteAddress(ctx context.Context, buyerID, addressID, expectedVersion int64) error {
	return module.address.Delete(ctx, buyerID, addressID, expectedVersion)
}

// AddFavorite 新增收藏，pending 不允許；重複新增不發布事件。
func (module *Module) AddFavorite(ctx context.Context, buyerID, id int64) error {
	return module.favorite.Add(ctx, buyerID, id)
}

// RemoveFavorite 移除收藏，只有實際刪除才發布事件。
func (module *Module) RemoveFavorite(ctx context.Context, buyerID, id int64) error {
	return module.favorite.Remove(ctx, buyerID, id)
}

// CheckFavorite 查詢商品是否已收藏。
func (module *Module) CheckFavorite(ctx context.Context, buyerID, id int64) (bool, error) {
	return module.favorite.Check(ctx, buyerID, id)
}

// Follow 新增追蹤，pending 不允許；重複追蹤不發布事件。
func (module *Module) Follow(ctx context.Context, buyerID, id int64) error {
	return module.follow.Add(ctx, buyerID, id)
}

// Unfollow 移除追蹤，重複操作沒有副作用。
func (module *Module) Unfollow(ctx context.Context, buyerID, id int64) error {
	return module.follow.Remove(ctx, buyerID, id)
}

// CheckFollow 查詢是否追蹤店家。
func (module *Module) CheckFollow(ctx context.Context, buyerID, id int64) (bool, error) {
	return module.follow.Check(ctx, buyerID, id)
}

// RecordVisit 更新最近瀏覽時間並發布 ProductViewed。
func (module *Module) RecordVisit(ctx context.Context, buyerID, id int64) error {
	return module.visit.Add(ctx, buyerID, id)
}

// SubscribeRestock 建立 待通知訂閱，同組重複訂閱冪等。
func (module *Module) SubscribeRestock(ctx context.Context, buyerID, id int64) error {
	return module.restock.Subscribe(ctx, buyerID, id)
}

// UnsubscribeRestock 取消指定商品的 待通知訂閱。
func (module *Module) UnsubscribeRestock(ctx context.Context, buyerID, id int64) error {
	return module.restock.Unsubscribe(ctx, buyerID, id)
}

// ListFavorites 以時間與商品 ID 分頁列出收藏。
func (module *Module) ListFavorites(ctx context.Context, input ListInput) (Page, error) {
	return module.favorite.List(ctx, input)
}

// ListFollows 以時間與店家 ID 分頁列出追蹤。
func (module *Module) ListFollows(ctx context.Context, input ListInput) (Page, error) {
	return module.follow.List(ctx, input)
}

// ListVisits 以瀏覽時間與商品 ID 分頁。
func (module *Module) ListVisits(ctx context.Context, input ListInput) (Page, error) {
	return module.visit.List(ctx, input)
}

// FavoriteCount 計算買家收藏總數。
func (module *Module) FavoriteCount(ctx context.Context, buyerID int64) (int64, error) {
	return module.favorite.Count(ctx, buyerID)
}

// FollowCount 計算買家追蹤的店家數。
func (module *Module) FollowCount(ctx context.Context, buyerID int64) (int64, error) {
	return module.follow.Count(ctx, buyerID)
}

// FavoritedProductIDs 列出全部收藏商品 ID。
func (module *Module) FavoritedProductIDs(ctx context.Context, buyerID int64) ([]int64, error) {
	return module.favorite.IDs(ctx, buyerID)
}

// FollowedStoreIDs 列出全部追蹤店家 ID。
func (module *Module) FollowedStoreIDs(ctx context.Context, buyerID int64) ([]int64, error) {
	return module.follow.IDs(ctx, buyerID)
}

// ClearVisits 清空買家的瀏覽紀錄。
func (module *Module) ClearVisits(ctx context.Context, buyerID int64) error {
	return module.visit.Clear(ctx, buyerID)
}

// BatchCheckFavorites 批次查詢收藏狀態，輸入最多 100 個 ID。
func (module *Module) BatchCheckFavorites(ctx context.Context, buyerID int64, ids []int64) (map[int64]bool, error) {
	return module.favorite.BatchCheck(ctx, buyerID, ids)
}

// BatchCheckFollows 批次查詢追蹤狀態，輸入最多 100 個 ID。
func (module *Module) BatchCheckFollows(ctx context.Context, buyerID int64, ids []int64) (map[int64]bool, error) {
	return module.follow.BatchCheck(ctx, buyerID, ids)
}

// BatchCheckRestock 批次查詢 待通知訂閱，輸入最多 100 個 ID。
func (module *Module) BatchCheckRestock(ctx context.Context, buyerID int64, ids []int64) (map[int64]bool, error) {
	return module.restock.BatchCheck(ctx, buyerID, ids)
}

// Subscribers 以追蹤時間與 buyer_id 分頁列出粉絲。
func (module *Module) Subscribers(ctx context.Context, input SubscribersInput) (Page, error) {
	return module.follow.Subscribers(ctx, input)
}

// SubscriberCounts 批次回傳粉絲數，沒有粉絲的店家為零；最多 MaxBatchSize 個 ID。
func (module *Module) SubscriberCounts(ctx context.Context, storeIDs []int64) (map[int64]int64, error) {
	return module.follow.SubscriberCounts(ctx, storeIDs)
}

// SubscriberStats 以一句查詢回傳店家的粉絲總數，以及每個時間窗 [Start, End)（以追蹤時間計）內的新增數，總數與各窗來自同一個
// snapshot；時區的界線由呼叫方決定。windows 必須是 1 到 MaxStatsWindows 個，每個起點早於終點，storeID 必須是正數，否則回
// InvalidInput。
func (module *Module) SubscriberStats(ctx context.Context, storeID int64, windows []TimeWindow) (SubscriberStats, error) {
	return module.follow.SubscriberStats(ctx, storeID, windows)
}

// ListRestock 分頁列出 待通知補貨訂閱。
func (module *Module) ListRestock(ctx context.Context, input RestockListInput) (RestockPage, error) {
	return module.restock.List(ctx, input)
}

// FanOut 在獨立 transaction 處理最多 Limit 筆（1..100），回傳是否仍有待通知訂閱。
func (module *Module) FanOut(ctx context.Context, input FanOutInput) (bool, error) {
	return module.restock.FanOut(ctx, input)
}

// Entitlement 讀取有效權益：免費方案，額度週期以 stint 錨點計算。
func (module *Module) Entitlement(ctx context.Context, buyerID int64) (Entitlement, error) {
	return module.membership.Entitlement(ctx, buyerID)
}

// LockForDeletion 在呼叫方的 transaction 內鎖住買家列（持有到 transaction 結束）。accountdeletion 在派發買家的刪除事件之前，先依單體規則 4
// 的順序（帳號列、買家列、店家列、上層模組）鎖完買家與店家：事件的同 transaction 訂閱者（social）會鎖上層的列，不能排在店家列之前。
// 必須在 transaction 內呼叫；只由 accountdeletion 在已鎖住帳號列之後呼叫。
func (module *Module) LockForDeletion(ctx context.Context, buyerID int64) error {
	return module.deletion.Lock(ctx, buyerID)
}

// BeginDeletion 將買家標記為待刪；重送不重複寫入。只由 accountdeletion 在已鎖住帳號列的 transaction 內呼叫。
func (module *Module) BeginDeletion(ctx context.Context, buyerID int64) error {
	return module.deletion.BeginDeletion(ctx, buyerID)
}

// CloseForDeletion 關閉買家；重送安全。關閉買家唯一的路徑：只由 accountdeletion 在已鎖住帳號列（鎖順序：帳號列先於
// 買家列）、確認沒有未結訂單的 transaction 內呼叫，並在同一個 transaction 刪除帳號。
func (module *Module) CloseForDeletion(ctx context.Context, buyerID int64) error {
	return module.deletion.CloseForDeletion(ctx, buyerID)
}

// CleanupPII 按 buyer_id 分批清除已關閉超過 30 天的個資。
func (module *Module) CleanupPII(ctx context.Context) error { return module.deletion.CleanupPII(ctx) }

// AddWorkers 登記 River workers。
func (module *Module) AddWorkers(workers *river.Workers) {
	module.deletion.AddWorkers(workers)
}

// PeriodicJobs 回傳啟動即執行、每小時補跑的 PII 清理排程。
func (module *Module) PeriodicJobs() []*river.PeriodicJob {
	return module.deletion.PeriodicJobs()
}

// StatusValues 回傳全部合法值，供 CHECK 與契約驗證。
func StatusValues() []Status { return model.StatusValues() }

// GenderValues 回傳全部合法值，供 CHECK 與契約驗證。
func GenderValues() []Gender { return model.GenderValues() }

// MaxBatchSize 是批次 ID 查詢的輸入上限。
const MaxBatchSize = model.MaxBatchSize

// MaxAddresses 是每個買家的收貨地址上限，也是地址清單的預設頁大小。
const MaxAddresses = model.MaxAddresses

// BlockedBuyerCursor 買家封鎖買家名單的降冪分頁位置。
type BlockedBuyerCursor = model.BlockedBuyerCursor

// BlockedBuyerListInput 買家封鎖買家名單的 keyset 輸入，預設 20、最多 100 筆。
type BlockedBuyerListInput = model.BlockedBuyerListInput

// BlockedBuyerItem 買家封鎖買家名單的一列。
type BlockedBuyerItem = model.BlockedBuyerItem

// BlockedBuyerPage 一頁買家封鎖買家名單及下一頁位置。
type BlockedBuyerPage = model.BlockedBuyerPage

// BlockedStoreCursor 買家封鎖店名單的降冪分頁位置。
type BlockedStoreCursor = model.BlockedStoreCursor

// BlockedStoreListInput 買家封鎖店名單的 keyset 輸入，預設 20、最多 100 筆。
type BlockedStoreListInput = model.BlockedStoreListInput

// BlockedStoreItem 買家封鎖店名單的一列。
type BlockedStoreItem = model.BlockedStoreItem

// BlockedStorePage 一頁買家封鎖店名單及下一頁位置。
type BlockedStorePage = model.BlockedStorePage

// BlockBuyer 買家封鎖買家；已經封鎖回 false。發起者必須是營運中，封鎖自己回 InvalidInput，對象不存在或已關閉回 NotFound。
// 冪等，不發布事件。
func (module *Module) BlockBuyer(ctx context.Context, buyerID, targetBuyerID int64) (bool, error) {
	return module.block.AddBuyer(ctx, buyerID, targetBuyerID)
}

// UnblockBuyer 解除買家封鎖買家；沒有封鎖回 false。冪等。
func (module *Module) UnblockBuyer(ctx context.Context, buyerID, targetBuyerID int64) (bool, error) {
	return module.block.RemoveBuyer(ctx, buyerID, targetBuyerID)
}

// BlockStore 買家封鎖店；已經封鎖回 false。發起者必須是營運中；店是否存在不在這裡檢查（buyer 不依賴 seller）。冪等，不發布事件。
func (module *Module) BlockStore(ctx context.Context, buyerID, storeID int64) (bool, error) {
	return module.block.AddStore(ctx, buyerID, storeID)
}

// UnblockStore 解除買家封鎖店；沒有封鎖回 false。冪等。
func (module *Module) UnblockStore(ctx context.Context, buyerID, storeID int64) (bool, error) {
	return module.block.RemoveStore(ctx, buyerID, storeID)
}

// ListBlockedBuyers 以建立時間與被封鎖的買家 ID 分頁列出買家封鎖的買家（新的在前）。對象刪帳後列就消失。
func (module *Module) ListBlockedBuyers(ctx context.Context, input BlockedBuyerListInput) (BlockedBuyerPage, error) {
	return module.block.ListBuyers(ctx, input)
}

// ListBlockedStores 以建立時間與店家 ID 分頁列出買家封鎖的店（新的在前）。店退場後的列照樣在名單裡，店是否已關閉由呼叫端組裝時顯示。
func (module *Module) ListBlockedStores(ctx context.Context, input BlockedStoreListInput) (BlockedStorePage, error) {
	return module.block.ListStores(ctx, input)
}

// IsBlockingStore 查詢買家是否封鎖了店。給 A6 的 im、review 與 A7-3 的通知過濾使用；加入呼叫方的 transaction。
func (module *Module) IsBlockingStore(ctx context.Context, buyerID, storeID int64) (bool, error) {
	return module.block.IsBlockingStore(ctx, buyerID, storeID)
}

// IsBlockedBetween 查詢兩位買家之間任一方向是否有封鎖（買家對買家是雙向互不可見）。加入呼叫方的 transaction。
func (module *Module) IsBlockedBetween(ctx context.Context, buyerA, buyerB int64) (bool, error) {
	return module.block.IsBlockedBetween(ctx, buyerA, buyerB)
}
