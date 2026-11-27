// Package conversation 是買家的對話管理：列表（keyset 分頁）、改名、刪除（A9-D31 Q7、Q8、Q9）。
package conversation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

const (
	// DefaultPageSize 是 page_size 為 0 時的每頁筆數。
	DefaultPageSize = 20
	// MaxPageSize 是每頁上限；超過以上限計。
	MaxPageSize = 50
	// MaxTitleRunes 是標題的長度上限（字元數），資料表的 CHECK 同值。
	MaxTitleRunes = 40
)

// Service 是對話管理的 use case。
type Service struct {
	DB        *db.DB
	Now       func() time.Time
	Canceller run.Canceller
}

type token struct {
	UpdatedAt int64     `json:"u"`
	ID        uuid.UUID `json:"i"`
}

func encodeToken(t time.Time, id uuid.UUID) string {
	b, _ := json.Marshal(token{UpdatedAt: t.UnixMicro(), ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeToken(raw string) (token, bool) {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return token{}, false
	}
	var t token
	if err := json.Unmarshal(b, &t); err != nil || t.ID == (uuid.UUID{}) {
		return token{}, false
	}
	if t.UpdatedAt < minTokenMicro {
		return token{}, false
	}
	return t, true
}

var minTokenMicro = time.Date(-4712, 1, 2, 0, 0, 0, 0, time.UTC).UnixMicro()

// List 回傳買家自己的對話，依 (updated_at, id) 由新到舊，已刪除的不列。pageSize 為 0 用預設，超過上限以上限計，負數回 INVALID_ARGUMENT（欄位 page_size）；
// pageToken 不是這個方法發出的 token 回 INVALID_ARGUMENT（欄位 page_token）。
func (s Service) List(ctx context.Context, buyerID int64, pageSize int32, pageToken string) (model.ConversationPage, error) {
	switch {
	case pageSize < 0:
		return model.ConversationPage{}, apperr.InvalidInputOn([]string{"page_size"}, "page_size 不能是負數")
	case pageSize == 0:
		pageSize = DefaultPageSize
	case pageSize > MaxPageSize:
		pageSize = MaxPageSize
	}
	q := store.New(s.DB.Conn(ctx))
	var rows []store.ProcurementConversation
	var err error
	if pageToken == "" {
		rows, err = q.ListConversations(ctx, store.ListConversationsParams{BuyerID: buyerID, PageSize: pageSize + 1})
	} else {
		t, ok := decodeToken(pageToken)
		if !ok {
			return model.ConversationPage{}, apperr.InvalidInputOn([]string{"page_token"}, "page_token 不是有效的分頁 token")
		}
		rows, err = q.ListConversationsAfter(ctx, store.ListConversationsAfterParams{
			BuyerID: buyerID, AfterUpdatedAt: time.UnixMicro(t.UpdatedAt).UTC(), AfterID: t.ID, PageSize: pageSize + 1,
		})
	}
	if err != nil {
		return model.ConversationPage{}, fmt.Errorf("conversation：列出對話：%w", err)
	}
	var out model.ConversationPage
	more := len(rows) > int(pageSize)
	if more {
		rows = rows[:pageSize]
	}
	for _, r := range rows {
		out.Conversations = append(out.Conversations, model.ConversationSummary{
			ID: r.ID, Title: r.Title, LastMessagePreview: r.LastMessagePreview, UpdateTime: r.UpdatedAt,
		})
	}
	if more {
		last := rows[len(rows)-1]
		out.NextPageToken = encodeToken(last.UpdatedAt, last.ID)
	}
	return out, nil
}

// ValidTitle 去頭尾空白後檢查標題：不能是空的、不超過 MaxTitleRunes 個字元、不含換行與其他控制字元（含 U+2028、U+2029）；回傳去空白後的標題。
func ValidTitle(raw string) (string, error) {
	t := trimtext.Trim(raw)
	if t == "" || utf8.RuneCountInString(t) > MaxTitleRunes || strings.IndexFunc(t, isLineBreakOrControl) >= 0 {
		return "", apperr.InvalidInputOn([]string{"title"}, "標題需為 1 到 %d 個字元，不含換行", MaxTitleRunes)
	}
	return t, nil
}

func isLineBreakOrControl(r rune) bool { return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' }

// Rename 改買家自己的對話的標題（目標狀態的方法：重送結果相同，不建收據）。標題不合規則回 INVALID_ARGUMENT（欄位 title）；
// 對話不存在、已刪除或不是買家的回 NOT_FOUND。
func (s Service) Rename(ctx context.Context, buyerID int64, id uuid.UUID, title string) error {
	t, err := ValidTitle(title)
	if err != nil {
		return err
	}
	n, err := store.New(s.DB.Conn(ctx)).RenameConversation(ctx, store.RenameConversationParams{ID: id, BuyerID: buyerID, Title: t, Now: s.Now()})
	if err != nil {
		return fmt.Errorf("conversation：改名：%w", err)
	}
	if n == 0 {
		return apperr.NotFound("找不到這個對話")
	}
	return nil
}

// Delete 刪除買家自己的對話（軟刪除），同一個 transaction 取消它需求還沒結束的 run；已發布的詢價不受影響。
// 目標狀態的方法：已刪除的重送回成功；不存在或不是買家的回 NOT_FOUND。鎖順序：買家送出鎖、run、對話列。
func (s Service) Delete(ctx context.Context, buyerID int64, id uuid.UUID) error {
	return s.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.DB.Conn(ctx))
		if err := q.LockBuyerSubmits(ctx, buyerID); err != nil {
			return fmt.Errorf("conversation：鎖買家的送出：%w", err)
		}
		conv, err := q.GetConversationIncludingDeleted(ctx, store.GetConversationIncludingDeletedParams{ID: id, BuyerID: buyerID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("找不到這個對話")
		}
		if err != nil {
			return fmt.Errorf("conversation：讀取對話：%w", err)
		}
		if conv.DeletedAt != nil {
			return nil
		}
		if err := s.Canceller.CancelAllOfRequest(ctx, buyerID, conv.RequestID); err != nil {
			return err
		}
		n, err := q.SoftDeleteConversation(ctx, store.SoftDeleteConversationParams{ID: id, BuyerID: buyerID, Now: ptr(s.Now())})
		if err != nil {
			return fmt.Errorf("conversation：刪除對話：%w", err)
		}
		if n != 1 {
			return fmt.Errorf("conversation：刪除對話 %s 影響 %d 列", id, n)
		}
		return nil
	})
}

func ptr[T any](v T) *T { return &v }
