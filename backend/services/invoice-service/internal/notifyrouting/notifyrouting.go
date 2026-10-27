// Package notifyrouting 持有「派送當下把 store_id 解析成 notify_seller_account_id」
// 這一段（58 §6.3.3.2）。
//
// 為什麼是獨立的葉 package 而不是留在 internal/worker：出向事件的**生產者**住在
// internal/usecase，而 worker import usecase ⇒ usecase 的測試不可能反向 import worker
// ⇒ 生產者永遠只能對著「自己抄一份的 dispatcher 邏輯」驗證。而「兩端都是自己寫的、
// 自己跟自己對必然全綠」正是 2026-08-30 那四個 P0 的共同結構性成因
// （見 .claude/skills/mq-wire-contract）。
//
// 把這 30 行搬成葉 package 之後，usecase 的契約測試打的是**真正會在線上跑的那一份**。
package notifyrouting

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"go.uber.org/zap"
)

// KeyStoreID 是賣家路由鍵在 wire 上的欄名。
//
// 這是**跨服務契約**：生產者（internal/eventwire.StoreID 的 json tag）與本檔必須
// 是同一個字串，兩邊各寫一次就是漂移的來源。
const KeyStoreID = "store_id"

// KeyNotifySellerAccountID 是本 package 補上去的那一欄。
const KeyNotifySellerAccountID = "notify_seller_account_id"

// UnresolvedAccountID 是解析失敗時填的值；下游看到它會 skip 該則賣家向通知。
const UnresolvedAccountID = "0"

// SellerAccountResolver 把賣家業務鍵（store_id）解析成登入 principal（account_id）。
type SellerAccountResolver interface {
	ResolveSellerAccountID(ctx context.Context, storeID int64) (int64, error)
}

// Apply 在 payload 上補 notify_seller_account_id 並回新的 payload。
//
// 🔴 解析失敗填 "0" 並**繼續派送**，不是回 error：買家向的通知靠 buyer_id 路由、
// 完全不依賴這個值，為了賣家那一則把整批事件卡住是把小缺口換成大缺口。
//
// 🔴 欄位是**字串**：TSID 以 int64 進 generic-map consumer（notification-service）
// 會被解成 float64 而截斷。
//
// 回 error 只有一種情況：payload 本身解不開／組不回去。那是我方自己 marshal 進去的，
// 解不開代表落庫時就壞了，讓它重試到 FAILED 至少留在表上看得到。
func Apply(ctx context.Context, raw []byte, sellers SellerAccountResolver, log *zap.Logger) ([]byte, error) {
	if log == nil {
		log = zap.NewNop()
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("notifyrouting: unmarshal payload: %w", err)
	}
	if _, already := payload[KeyNotifySellerAccountID]; already {
		return raw, nil
	}

	accountID := UnresolvedAccountID
	raw2, present := payload[KeyStoreID]
	switch {
	case !present:
		// 🔴 這一支曾經是完全靜默的：沒有這個鍵就直接跳過，連一行 warn 都沒有。
		// 而「payload 根本沒有 store_id」正是 P0-4（全 camelCase）與「新增事件型別忘了
		// 帶 store_id」兩種缺陷在**執行期**的樣子 —— 兩者都會讓賣家向通知 100% 消失。
		// 本 repo 的六個出向事件全部都有 store_id ⇒ 缺席一律是生產者的 bug。
		log.Warn("invoice.ob.store_id_absent",
			zap.String("impact", "賣家向通知 100% 不會送達；買家向不受影響"),
			zap.String("likely_cause", "payload 欄名不是 snake_case，或新增的事件型別漏了 store_id"))
	default:
		storeID, perr := ParseStringID(raw2)
		switch {
		case perr != nil:
			log.Warn("invoice.ob.store_id_unparseable",
				zap.Any(KeyStoreID, raw2), zap.Error(perr))
		default:
			resolved, rerr := sellers.ResolveSellerAccountID(ctx, storeID)
			if rerr != nil {
				log.Warn("invoice.ob.seller_account_unresolved",
					zap.Int64(KeyStoreID, storeID),
					zap.String("impact", "賣家向通知不會送達；買家向不受影響"), zap.Error(rerr))
			} else {
				accountID = FormatID(resolved)
			}
		}
	}
	payload[KeyNotifySellerAccountID] = accountID

	out, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("notifyrouting: remarshal payload: %w", err)
	}
	return out, nil
}

// ParseStringID 把 payload 上的 ID 還原成 int64。
//
// 出向 payload 的 ID 一律是 **string**（§6.3.4：int64 進 generic-map consumer 會被
// float64 截斷），但這裡走 `map[string]any` 解回來 ⇒ 若哪天有人在 payload 塞了裸數字，
// encoding/json 會給 float64。兩種都收，但 float64 只在能無損還原時才接受：
// 一個被截斷過的 TSID 解回來會指向另一筆資料，靜默用它比 fail 更糟。
func ParseStringID(raw any) (int64, error) {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return 0, fmt.Errorf("empty id string")
		}
		return strconv.ParseInt(v, 10, 64)
	case float64:
		id := int64(v)
		if float64(id) != v {
			return 0, fmt.Errorf("float64 %v is not an exact int64 (TSID precision already lost)", v)
		}
		return id, nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", raw)
	}
}

// FormatID 把 ID 轉回對外的字串型別。
func FormatID(v int64) string { return strconv.FormatInt(v, 10) }
