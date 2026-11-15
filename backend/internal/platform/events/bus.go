// Package events 是同一個 transaction 內同步派送的 in-process 事件（ADR D4）。
//
// 用途只有一種：下層模組通知上層模組一個需要原子性的狀態變更。發布方在 events.go
// 定義事件型別，訂閱在組裝點（cmd）註冊，handler 在發布方的 transaction 裡同步執行，
// 任何一個 handler 失敗，整個 transaction 都會 rollback。
// 不需要原子性的副作用（通知、外部呼叫）改用 jobs，不要用這裡。
package events

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// ErrNoSubscriber 表示 PublishRequired 指名的 handler 沒有註冊。
var ErrNoSubscriber = errors.New("events：這個事件必須有訂閱者，卻沒有")

// ErrNoTransaction 表示在 transaction 外發布事件。
var ErrNoTransaction = errors.New("events：Publish 必須在 db.InTx 內呼叫")

type handler func(context.Context, any) error

// named 是一個 handler 與它的註冊名稱；一般的 Subscribe 名稱是空字串。
type named struct {
	name string
	fn   handler
}

// Bus 保存事件型別到 handler 的對應。訂閱在啟動時完成，之後只讀。
type Bus struct {
	mu       sync.RWMutex
	handlers map[reflect.Type][]named
}

// NewBus 回傳空的 Bus。
func NewBus() *Bus {
	return &Bus{handlers: map[reflect.Type][]named{}}
}

// Subscribe 註冊事件 E 的 handler。同一個事件的 handler 依註冊順序執行。
// E 必須是具體型別，不能是 interface：事件是發布方公開 API 的一部分，要有明確的名字。
func Subscribe[E any](b *Bus, h func(context.Context, E) error) {
	subscribe(b, "", h)
}

// SubscribeNamed 與 Subscribe 相同，但以 name 註冊，讓發布方用 PublishRequired 指名要求這個 handler 存在（例如代行寫入的稽核）。name 不可為空。
func SubscribeNamed[E any](b *Bus, name string, h func(context.Context, E) error) {
	if name == "" {
		panic("events：SubscribeNamed 的名稱不可為空")
	}
	subscribe(b, name, h)
}

func subscribe[E any](b *Bus, name string, h func(context.Context, E) error) {
	t := reflect.TypeFor[E]()
	if t.Kind() == reflect.Interface {
		panic(fmt.Sprintf("events：事件型別 %s 必須是具體型別", t))
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[t] = append(b.handlers[t], named{name: name, fn: func(ctx context.Context, e any) error {
		return h(ctx, e.(E))
	}})
}

// Publish 在 ctx 的 transaction 內依序執行事件 e 的所有 handler，回傳第一個錯誤。
func Publish[E any](ctx context.Context, b *Bus, e E) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return ErrNoTransaction
	}
	b.mu.RLock()
	hs := b.handlers[reflect.TypeFor[E]()]
	b.mu.RUnlock()
	for _, h := range hs {
		if err := h.fn(ctx, e); err != nil {
			return fmt.Errorf("events：處理 %T：%w", e, err)
		}
	}
	return nil
}

// PublishRequired 與 Publish 相同，但事件 E 沒有以 name 註冊的 handler 時回 ErrNoSubscriber 而不發布：用在「指名的 handler 一定處理」才成立的寫入
// （例如代行寫入的稽核）。別的訂閱者存在不算數，所以事件可以是一般的領域事件；這個保證只在 bus 這一處，不在每個發布方各寫一份，也不靠啟動檢查（測試與 CLI 的組裝同樣受保護）。
func PublishRequired[E any](ctx context.Context, b *Bus, e E, name string) error {
	if !hasNamed[E](b, name) {
		return fmt.Errorf("%w：%T 沒有名為 %q 的 handler", ErrNoSubscriber, e, name)
	}
	return Publish(ctx, b, e)
}

// HasNamed 回傳事件 E 是否有以 name 註冊的 handler（組裝測試用）。
func HasNamed[E any](b *Bus, name string) bool { return hasNamed[E](b, name) }

func hasNamed[E any](b *Bus, name string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, h := range b.handlers[reflect.TypeFor[E]()] {
		if h.name == name {
			return true
		}
	}
	return false
}
