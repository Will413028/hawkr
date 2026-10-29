// Package profile 是店家 profile、logo 與封面的讀寫；寫入以版本做樂觀鎖（ADR 2026-09-25-optimistic-concurrency-etag）。
package profile

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/payuni"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
)

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	Media  media.Media
	// Exposure 是可曝光規則的套用與公告（classify 與每個寫入的公告都經它）；必填。
	Exposure *exposure.Service
	// StorefrontBaseURL 是店家頁的網站根網址（PayUni 綁定的資料完整性要用它；空字串表示沒有）。
	StorefrontBaseURL string
	Now               func() time.Time
	Logger            *slog.Logger
}

// Service 是 profile 的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

func errVersionConflict() error { return apperr.VersionConflict("店家資料已被修改") }

// CurrentStore 讀取帳號目前的 stint；沒有時回 NotFound。
func (s *Service) CurrentStore(ctx context.Context, accountID int64) (model.Store, error) {
	if err := model.Positive(accountID); err != nil {
		return model.Store{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.CurrentStore(ctx, accountID)
	if err != nil {
		return model.Store{}, model.Error(err)
	}
	return stint.Load(ctx, q, row, s.d.Now().UTC())
}

// Store 讀取指定的 stint，包含已關閉的。
func (s *Service) Store(ctx context.Context, storeID int64) (model.Store, error) {
	if err := model.Positive(storeID); err != nil {
		return model.Store{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.StoreByID(ctx, storeID)
	if err != nil {
		return model.Store{}, model.Error(err)
	}
	return stint.Load(ctx, q, row, s.d.Now().UTC())
}

// ExposableStores 從 storeIDs 中挑出可以公開曝光的店家（規則只有 seller.pub_exposable_stores，經 exposure.Classify），依輸入順序、重複的 ID 只算一次，
// 不存在或不可曝光的略過。回傳的 PublishedAt 是公開時間：stores.first_exposed_at；還沒被記下的（公告函式與對帳 job 記下，空窗最多一個對帳週期）
// 是讀取當下，視為剛公開（這個規則只在 classify 寫一次）。固定四次查詢（店家、處分、條件、pub_exposable_stores），與 ID 數量無關；停權經 stint.EffectiveMany
// （與 Derive 同一支查詢），條件經 stint.WithConditions。任何一次讀取失敗都回錯誤，
// 不回部分結果（fail-closed）。
func (s *Service) ExposableStores(ctx context.Context, storeIDs []int64) ([]model.ExposableStore, error) {
	exposed, err := s.exposed(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	out := make([]model.ExposableStore, len(exposed))
	for i, e := range exposed {
		out[i] = model.ExposableStore{StoreID: e.store.StoreID, PublishedAt: e.publishedAt}
	}
	return out, nil
}

// PublicStores 從 storeIDs 中挑出可以公開曝光的店家並回傳完整的店家資料；篩選、順序與讀取次數與 ExposableStores 相同
// （同一個 exposed）。
func (s *Service) PublicStores(ctx context.Context, storeIDs []int64) ([]model.Store, error) {
	exposed, err := s.exposed(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	out := make([]model.Store, len(exposed))
	for i, e := range exposed {
		out[i] = e.store
	}
	return out, nil
}

// PublicStoreByCode 以 store_code 讀取可以公開曝光的店家；不存在與不可曝光一律是同一個 NotFound（不洩漏哪些店存在但
// 不公開）。key 只有 store_code：store_id 是連號、可以逐號列舉，不當成這裡的鍵。
func (s *Service) PublicStoreByCode(ctx context.Context, storeCode int64) (model.PublicStore, error) {
	if err := model.Positive(storeCode); err != nil {
		return model.PublicStore{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.StoreByCode(ctx, storeCode)
	if err != nil {
		return model.PublicStore{}, model.Error(err)
	}
	exposed, err := s.exposedRows(ctx, q, []store.SellerStore{row})
	if err != nil {
		return model.PublicStore{}, err
	}
	if len(exposed) == 0 {
		return model.PublicStore{}, apperr.NotFound("找不到店家")
	}
	announced, err := q.StoreExposureChangedAt(ctx, []int64{row.StoreID})
	if err != nil {
		return model.PublicStore{}, err
	}
	var announcedAt *time.Time
	if len(announced) == 1 {
		announcedAt = announced[0].ExposableChangedAt
	}
	return publicStore(exposed[0], row.UpdatedAt, row.FirstExposedAt, announcedAt), nil
}

// ListPublicStores 依 store_id 由小到大列出最後公告為可曝光的店家（store_exposures，keyset），取 PageSize+1 家候選判斷有沒有下一頁，
// 再對這一頁經 classify（與 PublicStoreByCode 同一個規則）濾掉公告落後現況、此刻其實不可曝光的店：一頁可以少於 PageSize，
// 結束只看 Next 是不是 nil。cursor 是這一頁最後一個候選的 store_id（被濾掉的也算），下一頁從它之後讀；翻頁途中店家變得可曝光或不可曝光
// 只影響還沒讀到的部分，不會讓已讀過的店重複。剛變得可曝光、公告還沒追上的店，在公告之後才出現。
func (s *Service) ListPublicStores(ctx context.Context, in model.ListPublicStoresInput) (model.PublicStorePage, error) {
	pageSize, err := pagination.PageSize(in.PageSize, 100, 1000)
	if err != nil {
		return model.PublicStorePage{}, err
	}
	size := int(pageSize)
	var after int64
	if in.After != nil {
		if !in.After.Valid() {
			return model.PublicStorePage{}, apperr.InvalidInput("page_token 不合法")
		}
		after = in.After.StoreID
	}
	q := store.New(s.d.DB.Conn(ctx))
	candidates, err := q.ExposedStoresAfter(ctx, store.ExposedStoresAfterParams{AfterStoreID: after, BatchSize: pageSize + 1})
	if err != nil {
		return model.PublicStorePage{}, err
	}
	page := model.PublicStorePage{Items: make([]model.PublicStore, 0, size)}
	if len(candidates) > size {
		candidates = candidates[:size]
		page.Next = &model.PublicStoreCursor{StoreID: candidates[size-1].SellerStore.StoreID}
	}
	rows := make([]store.SellerStore, len(candidates))
	for i, c := range candidates {
		rows[i] = c.SellerStore
	}
	verdicts, err := s.classify(ctx, q, rows)
	if err != nil {
		return model.PublicStorePage{}, err
	}
	for i, v := range verdicts {
		if v.exposable {
			c := candidates[i]
			page.Items = append(page.Items, publicStore(v.exposedStore, c.SellerStore.UpdatedAt, c.SellerStore.FirstExposedAt, c.ExposureChangedAt))
		}
	}
	return page, nil
}

func publicStore(e exposedStore, updatedAt time.Time, firstExposedAt, announcedAt *time.Time) model.PublicStore {
	return model.PublicStore{Store: e.store, UpdateTime: model.PublicStoreUpdateTime(updatedAt, firstExposedAt, announcedAt)}
}

type exposedStore struct {
	store       model.Store
	publishedAt time.Time
}

// exposed 讀取 storeIDs（依輸入順序、重複只算一次、不存在的略過）並套用曝光規則。
func (s *Service) exposed(ctx context.Context, storeIDs []int64) ([]exposedStore, error) {
	if err := model.BatchIDs(storeIDs); err != nil {
		return nil, err
	}
	if len(storeIDs) == 0 {
		return []exposedStore{}, nil
	}
	q := store.New(s.d.DB.Conn(ctx))
	rows, err := q.StoresByIDs(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	return s.exposedRows(ctx, q, inInputOrder(storeIDs, rows))
}

// inInputOrder 把讀到的列排成輸入的順序，重複的 ID 只留第一次，不存在的略過。
func inInputOrder(storeIDs []int64, rows []store.SellerStore) []store.SellerStore {
	byID := make(map[int64]store.SellerStore, len(rows))
	for _, row := range rows {
		byID[row.StoreID] = row
	}
	out := make([]store.SellerStore, 0, len(rows))
	for _, id := range storeIDs {
		if row, ok := byID[id]; ok {
			out = append(out, row)
			delete(byID, id)
		}
	}
	return out
}

// classified 是一個店家與曝光規則對它的判定。
type classified struct {
	exposedStore
	exposable     bool
	hasPublicCard bool
}

// classify 是 exposure.Classify 加上公開時間：規則只在 SQL（seller.pub_exposable_stores），Go 只在 exposure.Classify 讀它一次；
// 公開時間是 first_exposed_at，沒有時是讀取當下。回傳每一列（依輸入順序）與它是否可曝光；固定三次查詢。
func (s *Service) classify(ctx context.Context, q *store.Queries, rows []store.SellerStore) ([]classified, error) {
	verdicts, err := s.d.Exposure.Classify(ctx, q, s.d.Now().UTC(), rows)
	if err != nil {
		return nil, err
	}
	now := s.d.Now().UTC()
	out := make([]classified, 0, len(rows))
	for i, row := range rows {
		published := now
		if row.FirstExposedAt != nil {
			published = *row.FirstExposedAt
		}
		out = append(out, classified{
			exposedStore:  exposedStore{store: verdicts[i].Store, publishedAt: published},
			exposable:     verdicts[i].Exposable,
			hasPublicCard: model.HasPublicCard(row.FirstExposedAt, verdicts[i].Exposable),
		})
	}
	return out, nil
}

// exposedRows 只留下 classify 判定可曝光的列（探索、店面頁、卡片與批次的可公開讀取都經它）。
func (s *Service) exposedRows(ctx context.Context, q *store.Queries, rows []store.SellerStore) ([]exposedStore, error) {
	all, err := s.classify(ctx, q, rows)
	if err != nil {
		return nil, err
	}
	out := make([]exposedStore, 0, len(all))
	for _, c := range all {
		if c.exposable {
			out = append(out, c.exposedStore)
		}
	}
	return out, nil
}

// StoreCards 從 storeIDs 中挑出「曾經公開過」的店家，回傳卡片資料（店名、logo、封面、營運狀態），依輸入順序、重複的 ID
// 只算一次；不存在與從沒公開過的略過（store_id 在過渡期連號、可列舉，沒公開過的店名不能被逐號讀出）。曾經公開過是
// first_exposed_at 有值（公告函式寫入），或此刻可以公開曝光（涵蓋 job 還沒追上的空窗）；曝光規則仍只有 exposedRows。
// 營運狀態由 model.CardStatusOf 推導：OPEN 是此刻可以公開曝光，已關閉是 CLOSED，其餘（停權、等待刪除、PayUni 被拒絕、身分認證
// 改回草稿、缺店名／logo／所在地）是 PAUSED。任何一次讀取失敗都回錯誤，不回部分結果。
func (s *Service) StoreCards(ctx context.Context, storeIDs []int64) ([]model.StoreCard, error) {
	if err := model.BatchIDs(storeIDs); err != nil {
		return nil, err
	}
	if len(storeIDs) == 0 {
		return []model.StoreCard{}, nil
	}
	q := store.New(s.d.DB.Conn(ctx))
	rows, err := q.StoresByIDs(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	ordered := inInputOrder(storeIDs, rows)
	verdicts, err := s.classify(ctx, q, ordered)
	if err != nil {
		return nil, err
	}
	out := make([]model.StoreCard, 0, len(ordered))
	for i, row := range ordered {
		if !verdicts[i].hasPublicCard {
			continue
		}
		card := model.StoreCard{
			StoreID: row.StoreID, StoreCode: row.StoreCode, LogoKey: row.LogoKey, CoverKey: row.CoverKey, TownCode: row.TownCode,
			Status: model.CardStatusOf(model.Status(row.Status), verdicts[i].exposable),
		}
		if card.Status == model.CardClosed {
			// 關店可逆，logo 與封面留在店家列直到個資清理才釋放；卡片的契約仍是「關閉的店沒有 logo 與封面」。
			card.LogoKey, card.CoverKey = nil, nil
		}
		if row.StoreName != nil && *row.StoreName != "" {
			card.Name = row.StoreName
		}
		out = append(out, card)
	}
	return out, nil
}

// UpdateProfile 在店家列鎖內合併提供的欄位、驗證合併後的結果，再以版本條件寫入。店家要有 CapStorefrontWrite（停權、
// 等待刪除時拒絕）。合併後沒有任何欄位改變、也沒有換圖時不寫入：回傳目前的店家，版本與 etag 不變、不發事件（重送同樣的
// 修改是冪等的；版本仍要相符，過期的版本照樣回 VersionConflict）。
//
// LogoToken、CoverToken 非空時換圖：先在 transaction 外轉正（不建立引用；轉正前先做所有不需寫入的判斷：能力、版本、合併後的
// 驗證、店名鎖，被拒絕的請求不消耗 token），再於同一個版本條件的 transaction 內寫入文字欄位與圖片的 key 與 asset ID、把被換掉的
// 圖記成退場並排入引用的同步（mediaref）；已退場的圖不能寫回。換圖時必須在 transaction 外呼叫。
func (s *Service) UpdateProfile(ctx context.Context, in model.UpdateProfileInput) (model.Store, error) {
	if err := model.Positive(in.StoreID); err != nil {
		return model.Store{}, err
	}
	if in.ExpectedVersion <= 0 {
		return model.Store{}, apperr.InvalidInputOn([]string{"ExpectedVersion"}, "必須提供讀取時的版本")
	}
	changes := []imageChange{{kind: logo, field: "LogoToken", token: in.LogoToken}, {kind: cover, field: "CoverToken", token: in.CoverToken}}
	changes = slices.DeleteFunc(changes, func(c imageChange) bool { return c.token == "" })
	for _, c := range changes {
		if strings.TrimSpace(c.token) == "" {
			return model.Store{}, apperr.InvalidInputOn([]string{c.field}, "必須提供圖片")
		}
	}
	if len(changes) > 0 {
		if err := s.finalizeImages(ctx, in, changes); err != nil {
			return model.Store{}, err
		}
	}
	updated, err := s.writeProfile(ctx, in, changes)
	if err != nil {
		return model.Store{}, err
	}
	return updated, nil
}

// imageChange 是一次換圖：轉正之後 asset 才有值。
type imageChange struct {
	kind  image
	field string
	token string
	asset media.Asset
}

// preflight 在轉正圖片之前做所有不需要寫入就能判斷的拒絕（能力、版本、合併後的文字驗證、PayUni 的店名鎖）：被拒絕的請求
// 不消耗 staged token，client 修正之後可以用同一個 token 重送。transaction 內會再判斷一次（以列鎖內的資料為準）。
func (s *Service) preflight(ctx context.Context, in model.UpdateProfileInput) error {
	current, err := s.Store(ctx, in.StoreID)
	if err != nil {
		return err
	}
	if err := model.Authorize(current, model.CapStorefrontWrite); err != nil {
		return err
	}
	if current.Version != in.ExpectedVersion {
		return errVersionConflict()
	}
	_, err = s.mergeChecked(ctx, store.New(s.d.DB.Conn(ctx)), current, in)
	return err
}

// mergeChecked 把輸入疊到 before 的 profile 並驗證；店名或介紹改變時要 PayUni 沒有持有它們（盤點 D17）。
func (s *Service) mergeChecked(ctx context.Context, q *store.Queries, before model.Store, in model.UpdateProfileInput) (model.Profile, error) {
	merged := merge(before.Profile, in)
	if err := model.Normalize(&merged); err != nil {
		return model.Profile{}, err
	}
	if err := checkRequiredNotCleared(ctx, q, before, merged); err != nil {
		return model.Profile{}, err
	}
	// PayUni 持有的店名與介紹不會跟著 update_merchant 改變：KYC 審核中或 PayUni 已收下註冊時鎖住（盤點 D17）。
	if merged.StoreName != before.StoreName || !equalText(merged.Description, before.Description) {
		if err := payuni.CheckNameUnlocked(ctx, q, in.StoreID); err != nil {
			return model.Profile{}, err
		}
	}
	return merged, nil
}

// requiredFieldNames 是 PayUni 必填的店家欄位被清掉時回報的輸入欄位名與說明（地址依缺的是鄉鎮市區還是明細）。
var requiredFieldMessages = map[string]string{
	model.StoreFieldContactName:   "聯絡人姓名必填",
	model.StoreFieldContactMobile: "聯絡人手機必填",
	model.StoreFieldSupportEmail:  "客服電子信箱必填",
	model.StoreFieldAddress:       "店家地址必填",
}

// checkRequiredNotCleared：店家有 PayUni 註冊之後，PayUni 必填的店家欄位（地址、聯絡人姓名與手機、客服信箱）不能清掉：清掉之後 PayUni
// 仍是舊值，卻會被記成同步成功。還沒有註冊時可以清（送出 KYC 之前會再檢查完整性）。原本就缺的欄位不算，不相干的修改照常通過。
func checkRequiredNotCleared(ctx context.Context, q *store.Queries, before model.Store, merged model.Profile) error {
	cleared := payuni.NewlyMissing(before.Profile, merged)
	if len(cleared) == 0 {
		return nil
	}
	if _, err := q.PayUniRegistrationByStore(ctx, before.StoreID); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	field := map[string]string{model.StoreFieldContactName: "ContactName", model.StoreFieldContactMobile: "ContactMobile", model.StoreFieldSupportEmail: "SupportEmail"}[cleared[0]]
	if cleared[0] == model.StoreFieldAddress {
		field = "AddressDetail"
		if merged.TownCode == nil {
			field = "TownCode"
		}
	}
	return apperr.InvalidInputOn([]string{field}, "%s", requiredFieldMessages[cleared[0]])
}

// finalizeImages 轉正每一張新圖，結果寫回 changes（呼叫前先 preflight）。轉正不建立引用，所以中途失敗不需要補償。
func (s *Service) finalizeImages(ctx context.Context, in model.UpdateProfileInput, changes []imageChange) error {
	if _, ok := db.TxFrom(ctx); ok {
		return apperr.FailedPrecondition("換圖必須在 transaction 外開始")
	}
	if err := model.Positive(in.UploaderID); err != nil {
		return err
	}
	if err := s.preflight(ctx, in); err != nil {
		return err
	}
	for i := range changes {
		asset, err := media.FinalizeField(ctx, s.d.Media, media.Uploader{ID: in.UploaderID}, media.Field{Name: changes[i].field}, changes[i].token, changes[i].kind.refType)
		if err != nil {
			return err
		}
		changes[i].asset = asset
	}
	return nil
}

// writeProfile 是 UpdateProfile 的資料庫部分：文字欄位與圖片在同一個 transaction，每一次寫入以前一次寫入的版本為條件。
func (s *Service) writeProfile(ctx context.Context, in model.UpdateProfileInput, changes []imageChange) (model.Store, error) {
	var updated model.Store
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		current, err := stint.Lock(ctx, q, in.StoreID)
		if err != nil {
			return err
		}
		if current.Version != in.ExpectedVersion {
			return errVersionConflict()
		}
		now := s.d.Now().UTC()
		before, err := stint.Load(ctx, q, current, now)
		if err != nil {
			return err
		}
		if err := model.Authorize(before, model.CapStorefrontWrite); err != nil {
			return err
		}
		merged, err := s.mergeChecked(ctx, q, before, in)
		if err != nil {
			return err
		}
		textChanged := !reflect.DeepEqual(merged, before.Profile)
		if !textChanged && len(changes) == 0 {
			updated = before
			return nil
		}
		row, version := current, in.ExpectedVersion
		if textChanged {
			row, err = q.UpdateProfile(ctx, store.UpdateProfileParams{
				StoreID: in.StoreID, ExpectedVersion: version,
				StoreName: merged.StoreName, Description: merged.Description, ContactPhone: merged.ContactPhone,
				ContactName: merged.ContactName, ContactMobile: merged.ContactMobile, SupportEmail: merged.SupportEmail,
				TownCode: merged.TownCode, AddressDetail: merged.AddressDetail, Prize: merged.Prize,
				WebsiteUrl:          merged.WebsiteURL,
				OnboardingCompleted: merged.OnboardingCompleted, Now: now,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				return errVersionConflict()
			}
			if err != nil {
				return err
			}
			version = row.Version
			// 地址與聯絡資料是送給 PayUni 的資料：改了就登記同步（同一個 transaction 排入以 store_id 為唯一鍵的 job，PENDING 期間也送）。
			if payuni.SyncedFieldsChanged(before.Profile, merged) {
				if err := payuni.RequestSync(ctx, q, s.d.Jobs, s.d.StorefrontBaseURL, in.StoreID, now); err != nil {
					return err
				}
			}
		}
		for _, c := range changes {
			ref := mediaref.StoreRef(c.kind.refType, in.StoreID)
			if err := mediaref.Reject(ctx, q, ref, media.SyncCandidate{Field: c.field, AssetID: c.asset.ID}); err != nil {
				return err
			}
			row, err = c.kind.write(ctx, q, in.StoreID, version, c.asset, now)
			if errors.Is(err, pgx.ErrNoRows) {
				return errVersionConflict()
			}
			if err != nil {
				return err
			}
			version = row.Version
			var retired []string
			if old := c.kind.assetOf(current); old != nil && *old != c.asset.ID {
				retired = append(retired, *old)
			}
			if err := mediaref.Changed(ctx, q, s.d.Jobs, now, ref, retired...); err != nil {
				return err
			}
		}
		if err := s.changed(ctx, row); err != nil {
			return err
		}
		// 店名、鄉鎮市區代碼與 logo 是可曝光的輸入；公告是這個 transaction 的最後一個鎖。
		if err := s.d.Exposure.Announce(ctx, q, now, row.StoreID); err != nil {
			return err
		}
		updated, err = stint.Load(ctx, q, row, now)
		return err
	})
	return updated, err
}

func equalText(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// merge 把輸入疊到目前的 profile：nil 保留原值，空字串清成沒有值。
func merge(p model.Profile, in model.UpdateProfileInput) model.Profile {
	if in.StoreName != nil {
		p.StoreName = *in.StoreName
	}
	for _, f := range []struct{ from, to **string }{
		{&in.Description, &p.Description}, {&in.ContactPhone, &p.ContactPhone}, {&in.TownCode, &p.TownCode},
		{&in.ContactName, &p.ContactName}, {&in.ContactMobile, &p.ContactMobile}, {&in.SupportEmail, &p.SupportEmail},
		{&in.AddressDetail, &p.AddressDetail}, {&in.Prize, &p.Prize},
		{&in.WebsiteURL, &p.WebsiteURL},
	} {
		if *f.from != nil {
			*f.to = model.Text(**f.from)
		}
	}
	return p
}

// image 描述 logo 或封面：media 的引用類型與寫入的查詢。
type image struct {
	refType string
	assetOf func(store.SellerStore) *string
	write   func(ctx context.Context, q *store.Queries, storeID, version int64, asset media.Asset, now time.Time) (store.SellerStore, error)
}

var (
	logo = image{refType: media.RefTypeSellerLogo, assetOf: func(r store.SellerStore) *string { return r.LogoAssetID }, write: func(ctx context.Context, q *store.Queries, storeID, version int64, asset media.Asset, now time.Time) (store.SellerStore, error) {
		return q.UpdateLogo(ctx, store.UpdateLogoParams{StoreID: storeID, ExpectedVersion: version, LogoKey: &asset.ObjectKey, LogoAssetID: &asset.ID, Now: now})
	}}
	cover = image{refType: media.RefTypeSellerCover, assetOf: func(r store.SellerStore) *string { return r.CoverAssetID }, write: func(ctx context.Context, q *store.Queries, storeID, version int64, asset media.Asset, now time.Time) (store.SellerStore, error) {
		return q.UpdateCover(ctx, store.UpdateCoverParams{StoreID: storeID, ExpectedVersion: version, CoverKey: &asset.ObjectKey, CoverAssetID: &asset.ID, Now: now})
	}}
)

// changed 在寫入的 transaction 內發出 StoreProfileChanged 並排入快取失效。
func (s *Service) changed(ctx context.Context, row store.SellerStore) error {
	if err := events.Publish(ctx, s.d.Events, model.StoreProfileChanged{
		EventID: uuid.NewV7(), OccurredAt: s.d.Now().UTC(), StoreID: row.StoreID,
	}); err != nil {
		return err
	}
	return stint.Invalidate(ctx, s.d.Jobs, row)
}

// CompleteOnboarding 標記新手引導完成：只設這個旗標，不重新驗證其他欄位（已存的值可能不再通過現行的驗證，例如退役的
// 鄉鎮市區代碼，這不該擋住完成引導）。要 CapStorefrontWrite，版本不符回 VersionConflict；已完成時回傳目前的店家、
// 不寫入（冪等，版本仍要相符）。
func (s *Service) CompleteOnboarding(ctx context.Context, storeID, expectedVersion int64) (model.Store, error) {
	if err := model.Positive(storeID); err != nil {
		return model.Store{}, err
	}
	if expectedVersion <= 0 {
		return model.Store{}, apperr.InvalidInputOn([]string{"ExpectedVersion"}, "必須提供讀取時的版本")
	}
	var updated model.Store
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		current, err := stint.Lock(ctx, q, storeID)
		if err != nil {
			return err
		}
		if current.Version != expectedVersion {
			return errVersionConflict()
		}
		now := s.d.Now().UTC()
		before, err := stint.Load(ctx, q, current, now)
		if err != nil {
			return err
		}
		if err := model.Authorize(before, model.CapStorefrontWrite); err != nil {
			return err
		}
		if before.OnboardingCompleted {
			updated = before
			return nil
		}
		p := before.Profile
		row, err := q.UpdateProfile(ctx, store.UpdateProfileParams{
			StoreID: storeID, ExpectedVersion: expectedVersion,
			StoreName: p.StoreName, Description: p.Description, ContactPhone: p.ContactPhone,
			ContactName: p.ContactName, ContactMobile: p.ContactMobile, SupportEmail: p.SupportEmail,
			TownCode: p.TownCode, AddressDetail: p.AddressDetail, Prize: p.Prize,
			WebsiteUrl:          p.WebsiteURL,
			OnboardingCompleted: true, Now: now,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errVersionConflict()
		}
		if err != nil {
			return err
		}
		if err := s.changed(ctx, row); err != nil {
			return err
		}
		updated, err = stint.Load(ctx, q, row, now)
		return err
	})
	return updated, err
}

// BatchStores 批次讀取店家（含已關閉的 stint，每個帶推導的停權），依輸入順序、重複的 ID 只算一次，不存在的略過；不套用曝光規則。
// 固定兩次查詢（店家、處分），與 ID 數量無關；任何讀取失敗都回錯誤。
func (s *Service) BatchStores(ctx context.Context, storeIDs []int64) ([]model.Store, error) {
	if err := model.BatchIDs(storeIDs); err != nil {
		return nil, err
	}
	if len(storeIDs) == 0 {
		return []model.Store{}, nil
	}
	q := store.New(s.d.DB.Conn(ctx))
	rows, err := q.StoresByIDs(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	stores, err := s.loadMany(ctx, q, inInputOrder(storeIDs, rows))
	if err != nil {
		return nil, err
	}
	return stores, nil
}

// loadMany 把列轉成公開型別並帶上推導的停權狀態（一次 stint.EffectiveMany）、能力與曝光的條件（一次 stint.WithConditions）與關店紀錄。
func (s *Service) loadMany(ctx context.Context, q *store.Queries, rows []store.SellerStore) ([]model.Store, error) {
	accountIDs := make([]int64, len(rows))
	for i, row := range rows {
		accountIDs[i] = row.AccountID
	}
	suspensions, err := stint.EffectiveMany(ctx, q, accountIDs, s.d.Now().UTC())
	if err != nil {
		return nil, err
	}
	out := make([]model.Store, len(rows))
	for i, row := range rows {
		out[i] = stint.ToStore(row)
		out[i].Sanctions = suspensions[row.AccountID]
	}
	if err := stint.WithConditions(ctx, q, out); err != nil {
		return nil, err
	}
	if err := stint.WithClosures(ctx, q, out); err != nil {
		return nil, err
	}
	return out, nil
}

// StoresWithExposure 是 BatchStores 加上每個店家此刻是否可以公開曝光：與 ExposableStores 同一個 classify（規則只有
// seller.pub_exposable_stores）。固定四次查詢（店家、處分、條件、pub_exposable_stores）。
func (s *Service) StoresWithExposure(ctx context.Context, storeIDs []int64) ([]model.StoreExposure, error) {
	if err := model.BatchIDs(storeIDs); err != nil {
		return nil, err
	}
	if len(storeIDs) == 0 {
		return []model.StoreExposure{}, nil
	}
	q := store.New(s.d.DB.Conn(ctx))
	rows, err := q.StoresByIDs(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	all, err := s.classify(ctx, q, inInputOrder(storeIDs, rows))
	if err != nil {
		return nil, err
	}
	out := make([]model.StoreExposure, len(all))
	for i, c := range all {
		out[i] = model.StoreExposure{Store: c.store, Exposable: c.exposable, HasPublicCard: c.hasPublicCard}
	}
	return out, nil
}

// ResolvePrincipals 回傳存活（沒有關閉）的 stint 的 store_id 到所屬帳號的對照；已關閉與不存在的 ID 不在結果裡。一次查詢。
func (s *Service) ResolvePrincipals(ctx context.Context, storeIDs []int64) (map[int64]int64, error) {
	if err := model.BatchIDs(storeIDs); err != nil {
		return nil, err
	}
	out := make(map[int64]int64, len(storeIDs))
	if len(storeIDs) == 0 {
		return out, nil
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).StoresByIDs(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		// 已關閉的 stint 不解析（舊契約：unresolved 是「沒有存活的店家列」；舊 seller-service 關店一定同時寫 deleted_at）。
		if row.Status == string(model.Closed) {
			continue
		}
		out[row.StoreID] = row.AccountID
	}
	return out, nil
}

// StoreIDsByAccount 列出帳號全部的 stint（含已關閉的），依 store_id；沒有時是空清單。
func (s *Service) StoreIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	if err := model.Positive(accountID); err != nil {
		return nil, err
	}
	ids, err := store.New(s.d.DB.Conn(ctx)).StoreIDsByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []int64{}
	}
	return ids, nil
}

// ExposableStoreIDsByCounty 列出每個縣市（內政部 5 碼縣市代碼）裡最後公告為可曝光的店家 ID，依 store_id；結果對每個要求的縣市
// 都有一個鍵（沒有店家是空清單）。讀的是 store_exposures（公告函式與對帳 job 維護），不是現算的規則：Java 的過濾依同一份公告
// 的事件，所以兩邊對「這家店可不可曝光」只有一個答案，落後最多一個對帳週期。不是現行縣市代碼的輸入回 InvalidInput。
func (s *Service) ExposableStoreIDsByCounty(ctx context.Context, countyCodes []string) (map[string][]int64, error) {
	known := make(map[string]bool)
	for _, c := range twaddr.Counties() {
		known[c.Code] = true
	}
	out := make(map[string][]int64, len(countyCodes))
	for _, code := range countyCodes {
		if !known[code] {
			return nil, apperr.InvalidInput("不是現行的縣市代碼：%s", code)
		}
		out[code] = []int64{}
	}
	if len(countyCodes) == 0 {
		return out, nil
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).ExposableStoresByCounty(ctx, countyCodes)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.CountyCode] = append(out[r.CountyCode], r.StoreID)
	}
	return out, nil
}
