// Package exposure 是「店家可以公開曝光」的讀取與公告：規則本身只在 SQL（seller.pub_exposable_stores(at)，db/objects.sql），
// Classify 是 seller 的 Go 讀它的唯一一處（讀取路徑與公告共用），Announce 把
// 現在讀出來的可曝光與最後公告的值比對，不同時更新 store_exposures 並在呼叫方的 transaction 內發 StoreExposureChanged
// （ADR 2026-10-05-store-exposure-to-legacy 決策 1）。
//
// 它在 payuni 之下（payuni 的寫入也要公告），KYC 流程的推導（stint.FlowStatusesOf）放在兩者共用的 stint。
package exposure

import (
	"context"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	Events *events.Bus
}

// Service 套用可曝光的規則並公告。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service {
	if d.Events == nil {
		panic("seller/exposure: missing dependency")
	}
	return &Service{d: d}
}

// KYCFlows 讀取店家的 KYC 流程（沒有申請的店家不在結果裡：零值的流程不是 APPROVED），給後台清單顯示流程狀態；曝光與能力的
// KYC 條件不讀它（model.Conditions.KYCApproved 讀自 seller.store_requirement_flags，兩者的 APPROVED 由資料集測試鎖住一致）。
// 固定兩次查詢（KYC 申請、PayUni 註冊），與 ID 數量無關。
func (s *Service) KYCFlows(ctx context.Context, q *store.Queries, storeIDs []int64) (map[int64]model.KYCFlowStatus, error) {
	applications, err := q.CurrentKYCStatuses(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	statuses := make(map[int64]model.KYCStatus, len(applications))
	for _, a := range applications {
		statuses[a.StoreID] = model.KYCStatus(a.Status)
	}
	return stint.FlowStatusesOf(ctx, q, statuses)
}

// Verdict 是一個店家（帶同一刻的停權與條件）與 seller.pub_exposable_stores 對它的判定。
type Verdict struct {
	Store     model.Store
	Exposable bool
}

// Classify 是 seller 的 Go 讀可曝光規則的唯一一處（at 是判斷停權的時間點，呼叫方自己的時鐘）：可不可以曝光讀自
// seller.pub_exposable_stores(at)（上層模組在 SQL 裡 join 的同一個函式，所以所有消費端在同一刻得到同一個答案），回傳的店家帶著
// 同一刻的停權（stint.EffectiveMany）與條件（stint.WithConditions），給呼叫方說明缺什麼（model.MissingExposure）。回傳每一列（依輸入
// 順序）的判定；固定三次查詢。
func (s *Service) Classify(ctx context.Context, q *store.Queries, at time.Time, rows []store.SellerStore) ([]Verdict, error) {
	if len(rows) == 0 {
		return []Verdict{}, nil
	}
	at = at.UTC()
	storeIDs := make([]int64, len(rows))
	accountIDs := make([]int64, len(rows))
	for i, row := range rows {
		storeIDs[i], accountIDs[i] = row.StoreID, row.AccountID
	}
	suspensions, err := stint.EffectiveMany(ctx, q, accountIDs, at)
	if err != nil {
		return nil, err
	}
	stores := make([]model.Store, len(rows))
	for i, row := range rows {
		stores[i] = stint.ToStore(row)
		stores[i].Sanctions = suspensions[row.AccountID]
	}
	if err := stint.WithConditions(ctx, q, stores); err != nil {
		return nil, err
	}
	exposable, err := q.ExposableStoreIDs(ctx, store.ExposableStoreIDsParams{StoreIds: storeIDs, At: at})
	if err != nil {
		return nil, err
	}
	out := make([]Verdict, len(rows))
	for i := range rows {
		out[i] = Verdict{Store: stores[i], Exposable: slices.Contains(exposable, rows[i].StoreID)}
	}
	return out, nil
}

// Announce 讓這些店家最後公告的可曝光與 at 這一刻一致（at 是呼叫方在這個 transaction 裡判斷停權用的時間點，不在這裡另讀時鐘；
// 公告與事件的時間也是 at），必須在呼叫方的寫入 transaction 內呼叫（q 綁著那個 transaction），而且是
// transaction 裡最後一個鎖：鎖順序是店家列、（申請、註冊）、公告列。
//
// 順序是安全性質：先鎖住這些店家的公告列（開店時與店家列一起建立，這裡不插入），鎖住之後才讀店家與算現在的可曝光。先算再鎖，
// 兩個同時進行的寫入各拿著對方還沒 commit 之前的輸入，後鎖到的會以舊的輸入覆蓋先 commit 的公告。算出來與最後公告的值不同
// 時，更新那一列（version 加一）並在同一個 transaction 發 StoreExposureChanged；相同時什麼都不寫、不發。算出來是可曝光的店
// 順便記下公開時間（first_exposed_at 只在這裡寫，已有的不覆寫）。不存在的店家略過，重複的 ID 只算一次；店家存在卻沒有公告列是不變量被破壞，回錯誤。
func (s *Service) Announce(ctx context.Context, q *store.Queries, at time.Time, storeIDs ...int64) error {
	return s.announce(ctx, q, at, false, storeIDs)
}

// Reannounce 與 Announce 是同一個計算與同一條寫入（鎖順序、規則、版本加一、事件、first_exposed_at），差別只有一點：不比對最後公告的值，
// 每一家存在的店都更新公告列（version 加一）並發 StoreExposureChanged。用在下游的 store_exposure 要從 seller 重建的時候
// （全量重新公告，ADR 2026-10-05-store-exposure-to-legacy）：版本照常遞增，下游依版本排序，所以重發的事件一定新於它已有的值。
func (s *Service) Reannounce(ctx context.Context, q *store.Queries, at time.Time, storeIDs ...int64) error {
	return s.announce(ctx, q, at, true, storeIDs)
}

func (s *Service) announce(ctx context.Context, q *store.Queries, at time.Time, force bool, storeIDs []int64) error {
	ids := unique(storeIDs)
	if len(ids) == 0 {
		return nil
	}
	announced, err := q.LockStoreExposures(ctx, ids)
	if err != nil {
		return err
	}
	rows, err := q.StoresByIDs(ctx, ids)
	if err != nil {
		return err
	}
	verdicts, err := s.Classify(ctx, q, at, rows)
	if err != nil {
		return err
	}
	slices.SortFunc(verdicts, func(a, b Verdict) int { return int(a.Store.StoreID - b.Store.StoreID) })
	last := make(map[int64]store.SellerStoreExposure, len(announced))
	for _, e := range announced {
		last[e.StoreID] = e
	}
	now := at.UTC()
	var exposable []int64
	for _, v := range verdicts {
		id := v.Store.StoreID
		if v.Exposable {
			exposable = append(exposable, id)
		}
		prev, ok := last[id]
		if !ok {
			return fmt.Errorf("seller：店家 %d 沒有公告列（開店時與店家列一起建立）", id)
		}
		if !force && prev.Exposable == v.Exposable {
			continue
		}
		next, err := q.AnnounceStoreExposure(ctx, store.AnnounceStoreExposureParams{StoreID: id, Exposable: v.Exposable, Now: &now})
		if err != nil {
			return err
		}
		err = events.Publish(ctx, s.d.Events, model.StoreExposureChanged{
			OccurredAt: now, EventID: uuid.NewV7(), StoreID: id, Exposable: next.Exposable, Version: next.Version,
		})
		if err != nil {
			return err
		}
	}
	if len(exposable) == 0 {
		return nil
	}
	_, err = q.StampFirstExposed(ctx, store.StampFirstExposedParams{Now: &now, StoreIds: exposable})
	return err
}

func unique(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
