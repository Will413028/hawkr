// Package characterize 是模組搬遷時對外 API 特性測試的共用部分（ADR 2026-09-24-go-modular-monolith D8 的
// 第一層安全網、ADR 2026-09-26-external-contract-protobuf-connect D4）。只給測試 import。
//
// 每個模組在 internal/httpapi/<模組>/characterization 寫自己的情境與 driver（make new-module 產生骨架）：
//   - 情境只寫一次（Scenario[D]，D 是模組的 driver 介面），只看業務上的結果：ID 當成不透明的字串、錯誤只比
//     這裡的分類。
//   - REST driver 經單體的入口打舊服務（/api/v1 由單體轉給 gateway，REST 基底），Connect driver 打單體；錯誤各自
//     以 RESTKind、ConnectKind 對應到同一組分類。
//   - 身分一律是本機金鑰簽的 dev token（Clerk 形式），以 Bearer 帶（identity.go）：直接打單體的一份（build tag
//     integration，CI 跑）以 DevAuth 產生；經入口的一份（build tag gateway，scripts/characterize.sh 起服務後執行）讀
//     腳本以 hawkr dev-token 簽的 token（EnvTokens）。
//
// 舊服務刪除後，該模組的 REST driver 與經入口的一份隨之刪除。
package characterize

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Caller 是呼叫者。需要別的身分（例如賣家）的模組在這裡加上，連同 DevAuth 與 EnvTokens。
type Caller int

const (
	Anonymous Caller = iota
	Buyer
	Admin
	// Seller、Seller2 是兩個不同店家的賣家，Buyer2 是第二位買家：需要多個身分的模組才設定它們的 token。
	Seller
	Seller2
	Buyer2
)

// 錯誤的分類。driver 把各自的錯誤對應到這幾種（errors.Is）。
var (
	ErrUnauthenticated = errors.New("未登入")
	ErrPermission      = errors.New("權限不足")
	ErrNotFound        = errors.New("不存在")
	ErrInvalid         = errors.New("輸入不合法")
	ErrConflict        = errors.New("版本衝突")
)

// T 是情境用到的 testing.T 子集：突變測試以一個記錄失敗的 T 執行情境（Probe），確認情境真的抓得到錯的資料。
type T interface {
	Helper()
	Error(args ...any)
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// Scenario 是一個情境，D 是模組的 driver 介面。
type Scenario[D any] struct {
	Name string
	Run  func(ctx context.Context, t T, d D)
}

// scenarioTimeout 是一個情境的時間上限。
const scenarioTimeout = 30 * time.Second

// Run 以 d 跑每個情境。沒有情境時失敗：空的清單會讓安全網看起來通過。
func Run[D any](t *testing.T, d D, list []Scenario[D]) {
	t.Helper()
	if len(list) == 0 {
		t.Fatal("沒有任何情境")
	}
	for _, s := range list {
		t.Run(s.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), scenarioTimeout)
			defer cancel()
			s.Run(ctx, t, d)
		})
	}
}

// Probe 以記錄失敗的 T 跑一個情境，回傳它記錄的失敗。突變測試用它確認單體回錯資料時情境會失敗。
func Probe[D any](d D, s Scenario[D]) (failures []string) {
	p := &probe{}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(probeStop); !ok {
				panic(r)
			}
		}
		failures = p.failures
	}()
	ctx, cancel := context.WithTimeout(context.Background(), scenarioTimeout)
	defer cancel()
	s.Run(ctx, p, d)
	return p.failures
}

// probe 是記錄失敗的 T：Fatalf 以 panic 中止情境，由 Probe 接住。
type probe struct{ failures []string }

type probeStop struct{}

func (p *probe) Helper() {}

func (p *probe) Error(args ...any) { p.failures = append(p.failures, fmt.Sprint(args...)) }

func (p *probe) Errorf(format string, args ...any) {
	p.failures = append(p.failures, fmt.Sprintf(format, args...))
}

func (p *probe) Fatalf(format string, args ...any) {
	p.Errorf(format, args...)
	panic(probeStop{})
}

// WantKind 確認 err 屬於 kind 這個分類。
func WantKind(t T, err, kind error) {
	t.Helper()
	if !errors.Is(err, kind) {
		t.Fatalf("err = %v，預期 %v", err, kind)
	}
}

// Nonce 是情境標在自己建立的資料上的隨機字串：資料庫可能已經有別的資料，情境只看帶著它的那幾筆。
func Nonce() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Result 把 (值, error) 包成一個參數：Go 不允許 Must(t, d.X()) 這種混用多回傳值的呼叫，寫成
// Must(t, Ret(d.X()))。
type Result[V any] struct {
	v   V
	err error
}

// Ret 包起 (值, error)。
func Ret[V any](v V, err error) Result[V] { return Result[V]{v, err} }

// Must 回傳值；有 error 時情境失敗。
func Must[V any](t T, res Result[V]) V {
	t.Helper()
	if res.err != nil {
		t.Fatalf("%v", res.err)
	}
	return res.v
}

// ConnectKind 把 Connect 的錯誤碼對應到錯誤的分類；對應不到的原樣回傳。
func ConnectKind(err error) error {
	if err == nil {
		return nil
	}
	var k error
	switch connect.CodeOf(err) {
	case connect.CodeUnauthenticated:
		k = ErrUnauthenticated
	case connect.CodePermissionDenied:
		k = ErrPermission
	case connect.CodeNotFound:
		k = ErrNotFound
	case connect.CodeInvalidArgument:
		k = ErrInvalid
	case connect.CodeAborted, connect.CodeAlreadyExists:
		k = ErrConflict
	case connect.CodeCanceled, connect.CodeUnknown, connect.CodeDeadlineExceeded,
		connect.CodeResourceExhausted, connect.CodeFailedPrecondition, connect.CodeOutOfRange,
		connect.CodeUnimplemented, connect.CodeInternal, connect.CodeUnavailable, connect.CodeDataLoss:
		return err
	default:
		return err
	}
	return apperr.WithCause(k, err)
}
