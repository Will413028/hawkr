// conversation.go 是「Hawkr 幫找」對話容器（列表／transcript／改名／軟刪）的
// 業務邏輯層。四個端點各自很薄——page_size clamp 與 limit+1 trim（List）、把
// answer_patch 展成 buyer/assistant 兩筆訊息（Messages）、直接透傳 repository
// 的冪等結果（Rename／Delete）——所以收在同一個檔案，不比照 turn.go／
// candidate_search.go 各自成檔的切法（那兩個各自有明顯更重的編排邏輯）。
package usecase

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

const (
	defaultConversationPageSize = 20
	maxConversationPageSize     = 50
)

// ConversationStore 是 ConversationUseCase 需要的持久層能力，介面收窄到剛好
// 這四個端點用得到的方法——跟 RevisionStore／CandidateSearchStore 同一個切法。
type ConversationStore interface {
	ListConversations(ctx context.Context, buyerID identity.BuyerID, cursor *repository.ConversationCursor, limitPlusOne int) ([]repository.Conversation, error)
	GetConversationForMessages(ctx context.Context, conversationID int64, buyerID identity.BuyerID) (repository.Conversation, string, error)
	ListAnswerPatchesByRequestID(ctx context.Context, requestID int64) ([]gen.ProcurementAnswerPatch, error)
	RenameConversation(ctx context.Context, conversationID int64, buyerID identity.BuyerID, title, clientRequestID string) (found, hit bool, record repository.RenameRecord, err error)
	DeleteConversation(ctx context.Context, conversationID int64, buyerID identity.BuyerID, clientRequestID string) (found, alreadyDeleted bool, err error)
}

type ConversationUseCase struct {
	store ConversationStore
}

func NewConversationUseCase(store ConversationStore) *ConversationUseCase {
	return &ConversationUseCase{store: store}
}

type ConversationListItem struct {
	ConversationID     int64
	Title              string
	LastMessagePreview string
	UpdatedAt          time.Time
}

type ConversationListResult struct {
	Records []ConversationListItem
	HasMore bool
	// NextCursor 是不透明 token（見 encodeConversationCursor），不是 conversation
	// id——列表依 (updated_at, id) 排序，游標必須是複合值。
	NextCursor string
}

// encodeConversationCursor 把 (updated_at, id) 複合游標編成一個不透明字串。
// 用 base64(RawURL) 是為了讓它能安全放進 query string，並且明確表達「client
// 不該解讀或自行構造這個值」——它只該原封不動把上一頁的 nextCursorId 傳回來。
// 時間用 UnixMicro：Postgres timestamptz 的精度就是微秒，不會因為 round-trip
// 而落在錯誤的那一側。
func encodeConversationCursor(updatedAt time.Time, id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d.%d", updatedAt.UnixMicro(), id)))
}

// decodeConversationCursor returns (nil, true) for an empty token (first page) and
// (nil, false) for a malformed token — callers treat a malformed token as "start over
// from page 1" rather than an error, matching this endpoint's existing tolerant-cursor
// behavior (an invalid page_size already clamps rather than 400s).
func decodeConversationCursor(token string) (*repository.ConversationCursor, bool) {
	if token == "" {
		return nil, true
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, false
	}
	microsStr, idStr, found := strings.Cut(string(raw), ".")
	if !found {
		return nil, false
	}
	micros, err := strconv.ParseInt(microsStr, 10, 64)
	if err != nil {
		return nil, false
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, false
	}
	return &repository.ConversationCursor{UpdatedAt: time.UnixMicro(micros), ID: id}, true
}

func (uc *ConversationUseCase) List(ctx context.Context, buyerID identity.BuyerID, cursorToken string, pageSize int) (ConversationListResult, error) {
	if pageSize < 1 {
		pageSize = defaultConversationPageSize
	}
	if pageSize > maxConversationPageSize {
		pageSize = maxConversationPageSize
	}

	cursor, _ := decodeConversationCursor(cursorToken) // malformed token silently treated as first page

	rows, err := uc.store.ListConversations(ctx, buyerID, cursor, pageSize+1)
	if err != nil {
		return ConversationListResult{}, err
	}

	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	records := make([]ConversationListItem, len(rows))
	for i, row := range rows {
		records[i] = ConversationListItem{
			ConversationID: row.ID, Title: row.Title,
			LastMessagePreview: row.LastMessagePreview, UpdatedAt: row.UpdatedAt,
		}
	}

	var nextCursor string
	if hasMore && len(records) > 0 {
		last := records[len(records)-1]
		nextCursor = encodeConversationCursor(last.UpdatedAt, last.ConversationID)
	}

	return ConversationListResult{Records: records, HasMore: hasMore, NextCursor: nextCursor}, nil
}

type MessageRecord struct {
	ID               string
	Role             string
	Text             string
	Steps            []domain.Step
	RecordedThisTurn []domain.RecordedField
	CreatedAt        time.Time
}

type MessagesResult struct {
	ConversationID int64
	RequestID      int64
	Status         string
	Title          string
	Records        []MessageRecord
}

// turnPayloadForReplay 只解析 GET .../messages 需要的欄位——刻意不重用
// turn.go 的 turnPayload（那是 unexported，跨檔案同 package 其實可以直接用，
// 但這裡刻意獨立定義，讓「這個 handler 讀取哪些欄位」的意圖在型別層面就看得
// 出來）。舊 payload 的 reasoning key（#2447 BE-01A 移除，BE-01C 清理前 DB 仍有）
// 不在此列，解碼時被 encoding/json 自動忽略，永不進 replay。
type turnPayloadForReplay struct {
	Message          string                 `json:"message"`
	Reply            string                 `json:"reply"`
	Steps            []domain.Step          `json:"steps"`
	RecordedThisTurn []domain.RecordedField `json:"recordedThisTurn"`
}

func (uc *ConversationUseCase) Messages(ctx context.Context, conversationID int64, buyerID identity.BuyerID) (MessagesResult, error) {
	conv, status, err := uc.store.GetConversationForMessages(ctx, conversationID, buyerID)
	if err != nil {
		return MessagesResult{}, err
	}

	patches, err := uc.store.ListAnswerPatchesByRequestID(ctx, conv.RequestID)
	if err != nil {
		return MessagesResult{}, err
	}

	records := make([]MessageRecord, 0, len(patches)*2)
	for _, p := range patches {
		var payload turnPayloadForReplay
		if err := json.Unmarshal(p.PayloadJson, &payload); err != nil {
			return MessagesResult{}, err
		}
		createdAt := p.CreatedAt.Time
		records = append(records,
			MessageRecord{ID: strconv.FormatInt(p.ID, 10) + "-buyer", Role: "buyer", Text: payload.Message, CreatedAt: createdAt},
			MessageRecord{ID: strconv.FormatInt(p.ID, 10) + "-assistant", Role: "assistant", Text: payload.Reply, Steps: payload.Steps, RecordedThisTurn: payload.RecordedThisTurn, CreatedAt: createdAt},
		)
	}

	return MessagesResult{
		ConversationID: conv.ID, RequestID: conv.RequestID, Status: status, Title: conv.Title,
		Records: records,
	}, nil
}

func (uc *ConversationUseCase) Rename(ctx context.Context, conversationID int64, buyerID identity.BuyerID, title, clientRequestID string) (bool, error) {
	found, _, _, err := uc.store.RenameConversation(ctx, conversationID, buyerID, title, clientRequestID)
	return found, err
}

func (uc *ConversationUseCase) Delete(ctx context.Context, conversationID int64, buyerID identity.BuyerID, clientRequestID string) (bool, error) {
	found, _, err := uc.store.DeleteConversation(ctx, conversationID, buyerID, clientRequestID)
	return found, err
}
