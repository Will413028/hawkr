package usecase

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository/gen"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// fakeConvRow is the minimal shape a test needs to control for one row
// ListConversations would hand back — just enough fields for the two List
// tests below (id-only ordering/trimming assertions). Not a mirror of
// repository.Conversation; extend it only when a test actually needs another
// field, mirroring fakeRepo's philosophy in fakerepo_test.go.
type fakeConvRow struct {
	id        int64
	title     string
	preview   string
	updatedAt time.Time
}

// fakePatchRow is the input shape for one answer_patch as the Messages tests
// need to control it — the fields that actually vary across the "excludes
// reasoning" assertion. fakeConversationRepo.ListAnswerPatchesByRequestID
// marshals these into the same JSON shape turn.go's turnPayload writes
// (message/reply/reasoning/steps), so the fake exercises the real decoding
// path in conversation.go's turnPayloadForReplay, not a shortcut.
type fakePatchRow struct {
	message          string
	reply            string
	reasoning        string
	steps            []domain.Step
	recordedThisTurn []domain.RecordedField
}

// fakeAnswerPatchPayload mirrors turn.go's turnPayload JSON shape (message/
// reply/reasoning/steps/recordedThisTurn) closely enough for these tests —
// it's what actually gets written to procurement_answer_patches.payload_json
// in production.
type fakeAnswerPatchPayload struct {
	Message          string                 `json:"message"`
	Reply            string                 `json:"reply"`
	Reasoning        string                 `json:"reasoning,omitempty"`
	Steps            []domain.Step          `json:"steps,omitempty"`
	RecordedThisTurn []domain.RecordedField `json:"recordedThisTurn,omitempty"`
}

// fakeConversationRepo is the in-memory stand-in for ConversationStore,
// following fakerepo_test.go's fakeRepo style — enough behavior to drive
// ConversationUseCase's four methods without touching a DB, plus a couple of
// spy fields (lastLimitPlusOne) so tests can assert what the usecase asked
// the store for, not just what it returned.
type fakeConversationRepo struct {
	records []fakeConvRow
	patches []fakePatchRow

	lastLimitPlusOne int
	lastCursor       *repository.ConversationCursor
	lastCursorSeen   bool
}

func newFakeConversationRepo() *fakeConversationRepo {
	return &fakeConversationRepo{}
}

func (f *fakeConversationRepo) ListConversations(_ context.Context, _ identity.BuyerID, cursor *repository.ConversationCursor, limitPlusOne int) ([]repository.Conversation, error) {
	f.lastLimitPlusOne = limitPlusOne
	f.lastCursor = cursor
	f.lastCursorSeen = true
	out := make([]repository.Conversation, len(f.records))
	for i, r := range f.records {
		out[i] = repository.Conversation{ID: r.id, Title: r.title, LastMessagePreview: r.preview, UpdatedAt: r.updatedAt}
	}
	return out, nil
}

func (f *fakeConversationRepo) GetConversationForMessages(_ context.Context, conversationID int64, _ identity.BuyerID) (repository.Conversation, string, error) {
	return repository.Conversation{ID: conversationID, RequestID: conversationID}, "CLARIFYING", nil
}

func (f *fakeConversationRepo) ListAnswerPatchesByRequestID(_ context.Context, _ int64) ([]gen.ProcurementAnswerPatch, error) {
	out := make([]gen.ProcurementAnswerPatch, 0, len(f.patches))
	for i, p := range f.patches {
		payload, err := json.Marshal(fakeAnswerPatchPayload{
			Message: p.message, Reply: p.reply, Reasoning: p.reasoning, Steps: p.steps,
			RecordedThisTurn: p.recordedThisTurn,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, gen.ProcurementAnswerPatch{
			ID:          int64(i) + 1,
			PayloadJson: payload,
			CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		})
	}
	return out, nil
}

func (f *fakeConversationRepo) RenameConversation(_ context.Context, _ int64, _ identity.BuyerID, title, _ string) (bool, bool, repository.RenameRecord, error) {
	return true, false, repository.RenameRecord{Op: "rename", Title: title}, nil
}

func (f *fakeConversationRepo) DeleteConversation(_ context.Context, _ int64, _ identity.BuyerID, _ string) (bool, bool, error) {
	return true, false, nil
}

func TestConversationUseCase_List_ClampsPageSize(t *testing.T) {
	repo := newFakeConversationRepo()
	uc := NewConversationUseCase(repo)

	_, err := uc.List(context.Background(), 1, "", 500) // 超過上限
	require.NoError(t, err)
	require.Equal(t, 51, repo.lastLimitPlusOne, "page_size 應該被 clamp 到 50，repository 拿到 limit+1")
}

func TestConversationUseCase_List_TrimsOverfetchedRecord(t *testing.T) {
	repo := newFakeConversationRepo()
	t2 := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)
	// 模擬 limit+1=3 筆回來，page_size 實為 2
	repo.records = []fakeConvRow{
		{id: 3, updatedAt: t2.Add(2 * time.Hour)},
		{id: 2, updatedAt: t2},
		{id: 1, updatedAt: t2.Add(-2 * time.Hour)},
	}
	uc := NewConversationUseCase(repo)

	res, err := uc.List(context.Background(), 1, "", 2)
	require.NoError(t, err)
	require.Len(t, res.Records, 2, "多抓的那筆要被 trim 掉")
	require.True(t, res.HasMore)

	// cursor 是回傳的最後一筆（trim 後）的 (updated_at, id) 複合值，且是不透明
	// token，不是裸 id——用 decode 反查而不是硬編字串，避免測試綁死編碼格式。
	require.NotEmpty(t, res.NextCursor)
	require.NotEqual(t, "2", res.NextCursor, "nextCursor 不再是裸 conversation id")
	cur, ok := decodeConversationCursor(res.NextCursor)
	require.True(t, ok)
	require.Equal(t, int64(2), cur.ID)
	require.True(t, t2.Equal(cur.UpdatedAt), "cursor 帶的是那一列的 updated_at")
}

// TestConversationUseCase_List_PassesDecodedCursorToStore 驗證 List 真的把
// client 傳回來的 token 解回 (updated_at, id) 再交給 repository——這是「翻第二頁」
// 在 usecase 層唯一的接縫，錯了會靜默地每次都回第一頁。
func TestConversationUseCase_List_PassesDecodedCursorToStore(t *testing.T) {
	repo := newFakeConversationRepo()
	uc := NewConversationUseCase(repo)

	updatedAt := time.Date(2026, 8, 11, 10, 0, 0, 123456000, time.UTC)
	token := encodeConversationCursor(updatedAt, 777)

	_, err := uc.List(context.Background(), 1, token, 10)
	require.NoError(t, err)
	require.True(t, repo.lastCursorSeen)
	require.NotNil(t, repo.lastCursor, "帶 token 時必須傳一個非 nil 的 cursor 給 store")
	require.Equal(t, int64(777), repo.lastCursor.ID)
	require.True(t, updatedAt.Equal(repo.lastCursor.UpdatedAt), "微秒精度要能 round-trip")
}

// TestConversationUseCase_List_TolerantCursor 驗證空 token（第一頁）與壞掉的
// token 都走「不帶 cursor」路徑，不噴錯——與 page_size 非法時 clamp 而非 400
// 的既有寬容行為一致。
func TestConversationUseCase_List_TolerantCursor(t *testing.T) {
	for _, tc := range []struct {
		name  string
		token string
	}{
		{"空 token＝第一頁", ""},
		{"非 base64", "!!!not-base64!!!"},
		{"base64 但沒有分隔點", encodeRawForTest("nodot")},
		{"分隔點兩側不是數字", encodeRawForTest("abc.def")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeConversationRepo()
			uc := NewConversationUseCase(repo)

			_, err := uc.List(context.Background(), 1, tc.token, 10)
			require.NoError(t, err)
			require.True(t, repo.lastCursorSeen)
			require.Nil(t, repo.lastCursor, "無效／空 token 一律當第一頁，不帶 cursor")
		})
	}
}

func encodeRawForTest(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// TestConversationUseCase_Messages_LegacyReasoningPayloadIgnored 釘住 #2447
// BE-01A 的相容行為：BE-01C 清理前，DB 舊 payload 仍含 reasoning key——解碼必須
// 成功（不是錯誤），但該內容不得再出現在任何 replay 出去的記錄裡（MessageRecord
// 已無 Reasoning 欄位，這裡以 JSON 化零命中驗證，防未來欄位改名重新帶出）。
func TestConversationUseCase_Messages_LegacyReasoningPayloadIgnored(t *testing.T) {
	repo := newFakeConversationRepo()
	repo.patches = []fakePatchRow{{
		message:   "我要訂雞蛋",
		reply:     "好的，還缺配送城市",
		reasoning: "REASONING-abc123",
	}}
	uc := NewConversationUseCase(repo)

	res, err := uc.Messages(context.Background(), 1, 1)
	require.NoError(t, err)
	require.Len(t, res.Records, 2, "一個 answer_patch 展成 BUYER+ASSISTANT 兩筆")
	require.Equal(t, "buyer", res.Records[0].Role)
	require.Equal(t, "我要訂雞蛋", res.Records[0].Text)
	require.Equal(t, "assistant", res.Records[1].Role)
	require.Equal(t, "好的，還缺配送城市", res.Records[1].Text)

	b, err := json.Marshal(res.Records)
	require.NoError(t, err)
	require.NotContains(t, string(b), "reasoning")
	require.NotContains(t, string(b), "REASONING-abc123")
}

// TestConversationUseCase_Messages_IncludesRecordedThisTurn 驗證 transcript
// replay 把 payload_json 裡的 recordedThisTurn 帶回 assistant 那筆 MessageRecord
// ——這是 Steps 已有的雙寫模式（見 turnPayloadForReplay／turnPayload 對稱）延伸到
// recordedThisTurn：少了這段，重新整理頁面或切換對話會讓過去輪次的①確認段
// chip 消失，即使 DB 裡資料還在。buyer 那筆刻意不驗證 RecordedThisTurn（該
// 概念只屬於 assistant 側，跟 Steps 現有行為一致）。
func TestConversationUseCase_Messages_IncludesRecordedThisTurn(t *testing.T) {
	repo := newFakeConversationRepo()
	repo.patches = []fakePatchRow{{
		message: "我要訂雞蛋",
		reply:   "好的，還缺配送城市",
		recordedThisTurn: []domain.RecordedField{
			{Key: "item", Label: "品項", Value: "雞蛋"},
		},
	}}
	uc := NewConversationUseCase(repo)

	res, err := uc.Messages(context.Background(), 1, 1)
	require.NoError(t, err)
	require.Len(t, res.Records, 2)
	require.Equal(t, "assistant", res.Records[1].Role)
	require.Equal(t, []domain.RecordedField{{Key: "item", Label: "品項", Value: "雞蛋"}}, res.Records[1].RecordedThisTurn,
		"assistant 那筆要帶回 payload_json 裡記下的 recordedThisTurn")
	require.Empty(t, res.Records[0].RecordedThisTurn, "buyer 那筆不該有 recordedThisTurn")
}
