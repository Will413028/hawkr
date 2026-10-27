package notifyrouting

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type onlyStore struct {
	storeID   int64
	accountID int64
	asked     []int64
}

func (r *onlyStore) ResolveSellerAccountID(_ context.Context, storeID int64) (int64, error) {
	r.asked = append(r.asked, storeID)
	if storeID != r.storeID {
		return 0, errors.New("no seller for that store_id")
	}
	return r.accountID, nil
}

func apply(t *testing.T, payload map[string]any, r SellerAccountResolver, log *zap.Logger) map[string]any {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out, err := Apply(context.Background(), raw, r, log)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return got
}

func TestResolvesStoreIDToSellerAccount(t *testing.T) {
	r := &onlyStore{storeID: 2685914682, accountID: 4242424242}
	got := apply(t, map[string]any{"store_id": "2685914682"}, r, nil)
	if got[KeyNotifySellerAccountID] != "4242424242" {
		t.Fatalf("%s = %#v，want \"4242424242\"", KeyNotifySellerAccountID, got[KeyNotifySellerAccountID])
	}
}

// TestUnresolvableStoreIDDoesNotBlockDispatch 守「解析失敗不阻斷派送」。
func TestUnresolvableStoreIDDoesNotBlockDispatch(t *testing.T) {
	r := &onlyStore{storeID: 2685914682, accountID: 1}
	got := apply(t, map[string]any{"store_id": "7304857280000000001", "buyer_id": "2623900520"}, r, nil)
	if got[KeyNotifySellerAccountID] != UnresolvedAccountID {
		t.Fatalf("解析失敗必須填 %q，實得 %#v", UnresolvedAccountID, got[KeyNotifySellerAccountID])
	}
	if got["buyer_id"] != "2623900520" {
		t.Fatalf("買家路由欄位被動到了：%#v", got["buyer_id"])
	}
}

// TestAbsentStoreIDIsWarned 是本次新增的訊號。
//
// 🔴 這一支之前是**完全靜默**的：payload 沒有 store_id 鍵就直接跳過，
// notify_seller_account_id 填 "0"，沒有 warn、沒有 metric、沒有 DLQ。而
// 「payload 根本沒有 store_id」正是 P0-4（全 camelCase）與「新增事件型別忘了帶
// store_id」在執行期的樣子——兩者都讓賣家向通知 100% 消失。
//
// 鑑別力：resolver **一次都不該被問**（缺席不是解析失敗），而 warn 必須出現。
func TestAbsentStoreIDIsWarned(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	r := &onlyStore{storeID: 2685914682, accountID: 1}
	got := apply(t, map[string]any{"storeId": "2685914682"}, r, zap.New(core))

	if got[KeyNotifySellerAccountID] != UnresolvedAccountID {
		t.Fatalf("%s = %#v，want %q", KeyNotifySellerAccountID, got[KeyNotifySellerAccountID], UnresolvedAccountID)
	}
	if len(r.asked) != 0 {
		t.Fatalf("store_id 缺席時不該去問 resolver，實際問了 %v", r.asked)
	}
	if n := logs.FilterMessage("invoice.ob.store_id_absent").Len(); n != 1 {
		t.Fatalf("store_id 缺席必須留下一行 warn，實得 %d 行（全部 warn：%v）",
			n, logs.All())
	}
}

// TestExistingNotifyFieldIsNotOverwritten 守重送時不重算（既有契約）。
func TestExistingNotifyFieldIsNotOverwritten(t *testing.T) {
	r := &onlyStore{storeID: 1, accountID: 999}
	got := apply(t, map[string]any{"store_id": "1", KeyNotifySellerAccountID: "555"}, r, nil)
	if got[KeyNotifySellerAccountID] != "555" {
		t.Fatalf("既有值被覆寫成 %#v", got[KeyNotifySellerAccountID])
	}
	if len(r.asked) != 0 {
		t.Fatalf("已有值時不該再問 resolver，實際問了 %v", r.asked)
	}
}

// TestParseStringIDRejectsTruncatedTSID 守「已經被 float64 截斷過的 TSID 不得靜默使用」
// ——它會指向另一筆資料。
func TestParseStringIDRejectsTruncatedTSID(t *testing.T) {
	if _, err := ParseStringID(float64(1) + 0.5); err == nil {
		t.Error("非整數 float64 必須被拒絕")
	}
	if got, err := ParseStringID("2685914682"); err != nil || got != 2685914682 {
		t.Errorf("ParseStringID(string) = %d, %v", got, err)
	}
	if _, err := ParseStringID(nil); err == nil {
		t.Error("nil 必須被拒絕")
	}
	if _, err := ParseStringID(""); err == nil {
		t.Error("空字串必須被拒絕（那是「有這個鍵但沒有值」，不是 0）")
	}
}
