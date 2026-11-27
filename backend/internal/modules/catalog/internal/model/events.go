package model

import (
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)

// 模組事件只帶領域鍵與標題（以 rune 截斷的預覽），在寫入的 transaction 內發布，沒有訂閱者也照發。
// 每個成功的寫入呼叫發一次，店主自己的寫入也發（Actor 為 nil）；Actor 是呼叫方明確傳入的代行者（A7-D27），
// 帶代行者的事件要有名為 delegatedactor.AuditHandler 的 handler 才發得出去（delegatedactor.Publish）。

// ProductWritten 是商品寫入事件共用的欄位：寫入後商品的標題（稽核的資源名稱）與版本（Product.Version）。
type ProductWritten struct {
	EventID    uuid.UUID
	OccurredAt time.Time
	ProductID  uuid.UUID
	StoreID    int64
	Title      string
	Version    int64
	Actor      *delegatedactor.Actor
}

// ProductCreated 商品建立（一律是草稿）。重送的建立沒有寫入，不發。
type ProductCreated struct{ ProductWritten }

// ProductUpdated 商品內容修改（狀態不變）。
type ProductUpdated struct{ ProductWritten }

// ProductPublished 商品切成 ACTIVE（狀態改變）。
type ProductPublished struct{ ProductWritten }

// ProductUnpublished 商品切成 ARCHIVED（狀態改變）。
type ProductUnpublished struct{ ProductWritten }

// ProductPublishRepeated 商品已經是 ACTIVE 時再切成 ACTIVE（冪等的 no-op，版本不變）：狀態沒有改變，但請求本身是一次成功的寫入，要能被稽核。
type ProductPublishRepeated struct{ ProductWritten }

// ProductUnpublishRepeated 商品已經是 ARCHIVED 時再切成 ARCHIVED（冪等的 no-op），理由同 ProductPublishRepeated。
type ProductUnpublishRepeated struct{ ProductWritten }

// ProductsUnpublished 一次批次下架（UnpublishProducts，原子）：一個事件，Items 依商品 ID 由小到大，已經下架的項目也在其中（Changed 為 false）。
type ProductsUnpublished struct {
	EventID    uuid.UUID
	OccurredAt time.Time
	StoreID    int64
	Items      []UnpublishedItem
	Actor      *delegatedactor.Actor
}

// UnpublishedItem 是批次下架的一個商品：標題、請求帶的版本與寫入後的版本；Changed 為 false 表示它原本就是 ARCHIVED（沒有寫入，兩個版本相同）。
type UnpublishedItem struct {
	ProductID     uuid.UUID
	Title         string
	VersionBefore int64
	VersionAfter  int64
	Changed       bool
}

// VersionedProduct 是批次操作的一項：商品與呼叫方讀取時拿到的版本。
type VersionedProduct struct {
	ProductID uuid.UUID
	Version   int64
}

// BatchVersionConflictError 表示批次裡有商品的版本與目前的不符，整批沒有寫入；errors.Is(err, ErrVersionConflict) 成立。ProductIDs 是全部衝突的商品，由小到大。
type BatchVersionConflictError struct{ ProductIDs []uuid.UUID }

func (e *BatchVersionConflictError) Error() string { return "catalog: batch product version conflict" }

// Is 讓 errors.Is(err, ErrVersionConflict) 成立。
func (e *BatchVersionConflictError) Is(target error) bool { return target == ErrVersionConflict }

// SortedIDs 回傳由小到大的衝突商品 ID（UUID 的位元組序）。
func (e *BatchVersionConflictError) SortedIDs() []uuid.UUID {
	out := slices.Clone(e.ProductIDs)
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}
