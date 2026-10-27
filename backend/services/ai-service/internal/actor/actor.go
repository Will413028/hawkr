package actor

import (
	"context"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/httputil"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"google.golang.org/grpc/metadata"
)

const (
	StateKeyStore = "store_id"
	StateKeyBuyer = "buyer_id"
)

// grpcMetadataKey 是 stateKey → gRPC metadata key 的對照表。metadata key 一律小寫
// （gRPC 慣例，metadata.MD 讀取時 key 本身也會被正規化成小寫），語意跟 httpHeader
// 對稱但不是同一張表——gRPC 版是本次新增，尚未有 Gateway 端寫入者（Phase B 才會有）。
var grpcMetadataKey = map[string]string{
	StateKeyStore: "x-store-id",
	StateKeyBuyer: "x-buyer-id",
}

// AccountTypeAdmin 是 accounts.account_type 的 admin 值，與 gateway
// requireAdmin() 的判準（JWT claim accountType == 4）逐字對齊。⚠️ account_type
// 在買賣家語意上是 bitmask（1=買家、2=賣家、3=兩者），但 admin 是獨立的值 4
// 而不是一個 bit——所以這裡是等值比較，不是 & 運算。改成 bitmask 判定會讓
// account_type==4 以外的組合意外通過。
//
// ⚠️ 這個值隱性耦合到 gateway 代行營運：`gateway/internal/middleware/
// delegation_context.go` 的 SetDelegationSession 對代行 session 也注入
// accountType=4（同一個 gateway/internal/middleware/account_type.go 的
// AccountTypeAdmin 常數），所以走到這裡的 ActorAdmin 判準目前**無法區分
// 「真 admin」與「能力受限的代行 session」**。今天不可達——admin AI 路由都在
// requireAdmin() 之後，代行走 adminNoAuth 的 sibling group——但這個常數本身
// 不足以守住這個不變式，未來新增可達路徑前要重新確認這件事。
const AccountTypeAdmin = "4"

// grpcMetadataKeyAccountType 由 gateway 的 metadata interceptor 對每一次出站
// gRPC 注入（gateway/internal/grpc/metadata_interceptor.go），服務間轉發時由
// pkg/middleware/grpc/interceptor/internal_auth.go 保留。
const grpcMetadataKeyAccountType = "x-account-type"

// AdminMetadataState 是 x-account-type gRPC metadata 的三態判讀結果，讓呼叫端
// 能區分「完全沒帶這個 key」（client 沒帶 header）與「帶了但值不是 admin」
// （身分本身有問題）——這兩種在 15 號設計書 §8.1／§6.2 是刻意分開的錯誤碼
// （分別對應 Unauthenticated 與 PermissionDenied），合併判讀會讓 on-call
// 分不出是 client 沒帶 header 還是新方法忘了註冊。
type AdminMetadataState int

const (
	// AdminMetadataAbsent 是完全沒有 x-account-type key（或沒有 incoming
	// metadata）：client 沒帶身分，不是「身分不是 admin」。
	AdminMetadataAbsent AdminMetadataState = iota
	// AdminMetadataNotAdmin 是有 key，但值不等於 AccountTypeAdmin。
	AdminMetadataNotAdmin
	// AdminMetadataAdmin 是有 key，且值等於 AccountTypeAdmin。
	AdminMetadataAdmin
)

// ResolveAdminGRPCMetadata 判讀這次呼叫的 x-account-type metadata 狀態。
//
// 刻意不回 Actor：Actor 的三個欄位都是 ADK session 分區鍵，admin 操作沒有
// 對應的 session；造一個假 Actor 會讓 admin 動作被寫進某個使用者的分區。
func ResolveAdminGRPCMetadata(ctx context.Context) AdminMetadataState {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return AdminMetadataAbsent
	}
	vals := md.Get(grpcMetadataKeyAccountType)
	if len(vals) == 0 {
		return AdminMetadataAbsent
	}
	if vals[0] == AccountTypeAdmin {
		return AdminMetadataAdmin
	}
	return AdminMetadataNotAdmin
}

// Actor 是一次 AI 助手請求的行為者：賣家路由是現任 store_id，買家路由是現任
// buyer_id（都由 Gateway 解析完才送進來，不是原始登入 account_id）。
type Actor struct {
	StateKey string // StateKeyStore | StateKeyBuyer
	UserID   string // ADK session 分區鍵
	// StateVal 寫進 ADK session state（map[string]any）。必須是預先格式化好的字串，
	// 不能是裸 int64：adk-go 的 database session service 把 state 序列化進 Postgres
	// JSONB 後，讀回時用 encoding/json.Unmarshal 進 map[string]any，JSON 數字一律
	// 變成 float64，19 位 TSID 遠超 float64 的 53-bit 精確整數上限，會被靜默改寫成
	// 鄰近值（2026-08-07 PR #2281 修的那個 bug）。
	StateVal string
}

func NewStoreActor(id identity.StoreID) Actor {
	return Actor{
		StateKey: StateKeyStore,
		UserID:   strconv.FormatInt(int64(id), 10),
		StateVal: strconv.FormatInt(int64(id), 10),
	}
}

func NewBuyerActor(id identity.BuyerID) Actor {
	return Actor{
		StateKey: StateKeyBuyer,
		UserID:   strconv.FormatInt(int64(id), 10),
		StateVal: strconv.FormatInt(int64(id), 10),
	}
}

// StoreID 回傳這個 Actor 的 store domain ID，僅在 Actor 是經 ActorSeller
// method-policy（StateKeyStore）解析出來時有效。UserID 對 store actor 就是
// NewStoreActor 寫入的十進位字串（見上），用 httputil.ParseStoreID 轉回具名
// 型別，讓呼叫端（例如 gRPC handler 從 ActorFromContext 取 storeID）不必自己
// 重寫一次「先檢查 StateKey 再轉型別」的樣板。
//
// StateKey 不是 StateKeyStore，或字串解析失敗（理論上不會發生——
// ParseGRPCMetadata 寫入前已經驗證過），一律回 (0, false)。
func (a Actor) StoreID() (identity.StoreID, bool) {
	if a.StateKey != StateKeyStore {
		return 0, false
	}
	return httputil.ParseStoreID(a.UserID)
}

// ParseGRPCMetadata 依 stateKey 取對應 gRPC incoming metadata key，解析成該語意的
// 具名型別 ID。缺 metadata / 值非法 / 值為 0 → (zero, false)。
func ParseGRPCMetadata(ctx context.Context, stateKey string) (Actor, bool) {
	key, ok := grpcMetadataKey[stateKey]
	if !ok {
		return Actor{}, false
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return Actor{}, false
	}
	vals := md.Get(key)
	if len(vals) == 0 {
		return Actor{}, false
	}
	switch stateKey {
	case StateKeyStore:
		id, ok := httputil.ParseStoreID(vals[0])
		if !ok {
			return Actor{}, false
		}
		return NewStoreActor(id), true
	case StateKeyBuyer:
		id, ok := httputil.ParseBuyerID(vals[0])
		if !ok {
			return Actor{}, false
		}
		return NewBuyerActor(id), true
	}
	return Actor{}, false
}
