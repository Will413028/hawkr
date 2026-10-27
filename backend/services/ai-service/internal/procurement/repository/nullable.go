package repository

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

func pointerInt64(value int64) *int64 {
	return &value
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

// timeValue 是 nullableTime 的反向：SQL NULL 讀回 domain 端的零值時間。
func timeValue(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func nullableTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

// timestamptzParam is the explicit driver boundary for generated non-null
// timestamptz parameters. sqlc keeps these parameters as pgtype values because
// their query contract is non-null; nullable columns use native pointers.
func timestamptzParam(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: !value.IsZero()}
}

// TimestamptzParam 是 timestamptzParam 對同域 sibling package 的出口。
//
// procurement/publicationoutbox 直接包本 package 的 gen 查詢（同
// internal/procurement/outbox 的做法），需要的是**同一個** driver 邊界轉換，
// 不是第三份手抄的 pgtype.Timestamptz{...} 字面值（本 repo 規範 §15）。
func TimestamptzParam(value time.Time) pgtype.Timestamptz {
	return timestamptzParam(value)
}

// dateParam 把 domain 端的 *string（YYYY-MM-DD）轉成 sqlc 對 DATE 欄位的參數型別。
// nil 或格式不合都寫入 SQL NULL——DATE 欄位可為空，格式錯誤在到這裡之前就該被
// 驗證層擋下，這裡不吞掉錯誤也不臆測，只是不寫壞資料。
func dateParam(value *string) pgtype.Date {
	if value == nil {
		return pgtype.Date{}
	}
	t, err := time.Parse(domain.ResponseDeadlineLayout, *value)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// dateValue 是 dateParam 的反向：SQL NULL 讀回 nil。
func dateValue(value pgtype.Date) *string {
	if !value.Valid {
		return nil
	}
	s := value.Time.Format(domain.ResponseDeadlineLayout)
	return &s
}

// int32PtrToIntPtr／intPtrToInt32Ptr 是 sqlc 對 INT 欄位產出的 *int32 與 domain 端
// *int 之間的轉換。nil 必須原樣穿透——candidate set 兩欄的 nil 代表「WISH_POOL 沒有
// 這個概念」，轉成 0 會讓讀取端分不出它與「值真的是 0」。
func int32PtrToIntPtr(value *int32) *int {
	if value == nil {
		return nil
	}
	v := int(*value)
	return &v
}

func intPtrToInt32Ptr(value *int) *int32 {
	if value == nil {
		return nil
	}
	v := int32(*value)
	return &v
}
