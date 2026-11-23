package announce

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 永久性錯誤（重試不會好）只有 apperr 的輸入、找不到、狀態、權限類，與資料庫的資料例外、完整性違反；連線、取消、死結與未分類的都是暫時性。
// mutation：permanent 對任何錯誤都回 true → 暫時性錯誤也被略過、公告悄悄少發，這個測試紅。
func TestPermanentOnlyCoversErrorsThatRetryingCannotFix(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"找不到":      {fmt.Errorf("寫入：%w", apperr.NotFound("x")), true},
		"輸入不合法":    {apperr.InvalidInput("x"), true},
		"CHECK 違反": {fmt.Errorf("寫入：%w", &pgconn.PgError{Code: "23514"}), true},
		"唯一鍵違反":    {&pgconn.PgError{Code: "23505"}, true},
		"資料例外":     {&pgconn.PgError{Code: "22001"}, true},
		"死結":       {&pgconn.PgError{Code: "40P01"}, false},
		"序列化失敗":    {&pgconn.PgError{Code: "40001"}, false},
		"取消":       {context.Canceled, false},
		"逾時":       {context.DeadlineExceeded, false},
		"未分類":      {fmt.Errorf("連線中斷"), false},
		"暫時不可用":    {apperr.Unavailable("x"), false},
	}
	for name, c := range cases {
		if got := permanent(c.err); got != c.want {
			t.Errorf("%s：permanent = %v，預期 %v", name, got, c.want)
		}
	}
}

// Chunk 沒給用預設，超過名單來源一頁的上限就夾回 MaxChunk，否則 Directory 會回 InvalidInput 而派送永遠失敗。
// mutation：New 拿掉 MaxChunk 的夾制 → 5000 原樣留下，失敗。
func TestNewClampsChunk(t *testing.T) {
	for chunk, want := range map[int]int{-1: ChunkSize, 0: ChunkSize, 7: 7, MaxChunk: MaxChunk, MaxChunk + 1: MaxChunk, 5000: MaxChunk} {
		if got := New(Deps{Chunk: chunk}).d.Chunk; got != want {
			t.Errorf("Chunk %d → %d，預期 %d", chunk, got, want)
		}
	}
}
