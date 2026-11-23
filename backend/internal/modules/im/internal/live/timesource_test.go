package live

import (
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/backend/internal/modules/im/internal/store"
)

// 跨進程共享的時間只來自資料庫：presence 的 expires_at 與配發 seq 時寫的 last_message_at 都只由 SQL 的 now() 產生，
// 所以這些查詢的參數型別裡不能有時間。
func TestSharedTimestampsAreNeverParameters(t *testing.T) {
	t.Parallel()
	params := []any{
		store.AllocateSeqParams{},
		store.InsertPresenceStreamParams{},
		store.ExtendPresenceStreamsParams{},
		store.CountLivePartyStreamsParams{},
		store.PresenceForPartyParams{},
		store.LockPartyStreamsParams{},
	}
	timeTypes := []reflect.Type{
		reflect.TypeFor[time.Time](), reflect.TypeFor[*time.Time](),
		reflect.TypeFor[pgtype.Timestamptz](), reflect.TypeFor[pgtype.Timestamp](), reflect.TypeFor[pgtype.Date](),
	}
	for _, p := range params {
		rt := reflect.TypeOf(p)
		for i := range rt.NumField() {
			f := rt.Field(i)
			for _, tt := range timeTypes {
				if f.Type == tt {
					t.Errorf("%s.%s 是時間型別 %v：時間要由 SQL 的 now() 產生", rt.Name(), f.Name, f.Type)
				}
			}
		}
	}
}
