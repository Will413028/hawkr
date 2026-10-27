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

// ErrNoTransaction 表示在 transaction 外發布事件。
var ErrNoTransaction = errors.New("events：Publish 必須在 db.InTx 內呼叫")

type handler func(context.Context, any) error

// Bus 保存事件型別到 handler 的對應。訂閱在啟動時完成，之後只讀。
type Bus struct {
	mu       sync.RWMutex
	handlers map[reflect.Type][]handler
}

// NewBus 回傳空的 Bus。
func NewBus() *Bus {
	return &Bus{handlers: map[reflect.Type][]handler{}}
}

// Subscribe 註冊事件 E 的 handler。同一個事件的 handler 依註冊順序執行。
// E 必須是具體型別，不能是 interface：事件是發布方公開 API 的一部分，要有明確的名字。
func Subscribe[E any](b *Bus, h func(context.Context, E) error) {
	t := reflect.TypeFor[E]()
	if t.Kind() == reflect.Interface {
		panic(fmt.Sprintf("events：事件型別 %s 必須是具體型別", t))
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[t] = append(b.handlers[t], func(ctx context.Context, e any) error {
		return h(ctx, e.(E))
	})
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
		if err := h(ctx, e); err != nil {
			return fmt.Errorf("events：處理 %T：%w", e, err)
		}
	}
	return nil
}
