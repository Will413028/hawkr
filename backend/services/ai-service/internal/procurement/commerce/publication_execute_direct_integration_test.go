//go:build integration

package commerce

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
)

// pubExecDirectClientRequestID 是本檔兩支測試共用的 execute 冪等鍵。每支測試都有自己的
// database（dbtest.Clone），所以共用同一個字面值不會讓兩支互相干擾。
const pubExecDirectClientRequestID = "pubexec-direct"

// recipientSpec 是一位收件人在測試裡的宣告式寫法：只寫 lane 與它那一側的 id，
// 由 recipient() 轉成 domain 的建構入口，避免測試自己填 PublicationRecipient
// struct literal（domain 那邊明講了不要這樣做）。
type recipientSpec struct {
	lane       string
	storeID    int64
	prospectID int64
}

func (s recipientSpec) recipient(t *testing.T) domain.PublicationRecipient {
	t.Helper()
	switch domain.QuoteLane(s.lane) {
	case domain.QuoteLanePlatform:
		return domain.PlatformRecipient(s.storeID)
	case domain.QuoteLaneExternal:
		return domain.ExternalRecipient(s.prospectID)
	default:
		t.Fatalf("recipientSpec: 未知的 lane %q", s.lane)
		return domain.PublicationRecipient{}
	}
}

// directExecFixture 是一組「可以成功 execute 的 DIRECT approval」＋ 指向它的 usecase
// 與輸入。刻意只留這兩個欄位：本檔的斷言全部直接查表，不經 repository。
type directExecFixture struct {
	uc    *PublicationExecuteUseCase
	input ExecuteRfqPublicationInput
}

// seedExecutableDirectApproval 用指定的收件人名單種一份 DIRECT_RECIPIENTS approval。
//
// 名單由呼叫端決定是這支存在的唯一理由：audience_hash 用
// domain.DirectRecipientsAudience(...).Hash() 算，所以「換一份名單」必須連同 hash 一起
// 換，否則 execute 的重驗 gate 會擋下——這也是它沿用 seedExecutablePublicationApproval
// 而不是自己再種一條鏈的理由（那條鏈的每一個 id 與 hash 都要是真的從上游產出來的）。
func seedExecutableDirectApproval(ctx context.Context, t *testing.T, pool *pgxpool.Pool, specs []recipientSpec) directExecFixture {
	t.Helper()

	recipients := make([]domain.PublicationRecipient, 0, len(specs))
	for _, spec := range specs {
		recipients = append(recipients, spec.recipient(t))
	}

	f := seedExecutablePublicationApproval(ctx, t, pool, recipients)
	return directExecFixture{
		uc:    f.newUseCase(nil, nil, nil),
		input: f.input(pubExecDirectClientRequestID),
	}
}

// 這支守的是設計書 §3：DIRECT execute 在同一交易內為每位收件人建 invite，
// Vendor Portal E1 起兩個 lane 都建 outbox 列。
//
// ⚠️ 兩段都用正向對照（斷言 count 的實際值），不是「沒看到就算過」：一個永遠不寫
// 任何 outbox 列的實作也會讓「沒看到某個 lane 的列」成立。
func TestExecuteDirect_CreatesOneInvitePerRecipientAndOutboxForBothLanes(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	fx := seedExecutableDirectApproval(ctx, t, pool, []recipientSpec{
		{lane: "PLATFORM", storeID: 5301},
		{lane: "EXTERNAL", prospectID: 6301},
		{lane: "PLATFORM", storeID: 5302},
	})

	res, err := fx.uc.ExecuteRfqPublication(ctx, fx.input)
	require.NoError(t, err)

	var inviteCount, platformOutbox, externalOutbox, queuedExternal int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_rfq_publication_invites WHERE publication_id=$1`,
		res.PublicationID).Scan(&inviteCount))
	require.Equal(t, 3, inviteCount, "每位收件人一列")

	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM procurement_rfq_publication_outbox o
		JOIN procurement_rfq_publication_invites i ON i.id = o.invite_id
		WHERE i.publication_id=$1 AND i.lane='PLATFORM'`, res.PublicationID).Scan(&platformOutbox))
	require.Equal(t, 2, platformOutbox, "PLATFORM 有真實投遞管道，每位一個 command")

	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM procurement_rfq_publication_outbox o
		JOIN procurement_rfq_publication_invites i ON i.id = o.invite_id
		WHERE i.publication_id=$1 AND i.lane='EXTERNAL'`, res.PublicationID).Scan(&externalOutbox))
	require.Equal(t, 1, externalOutbox,
		"Vendor Portal E1 起 EXTERNAL 也有真實投遞管道，每位一個 command")

	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM procurement_rfq_publication_invites
		WHERE publication_id=$1 AND lane='EXTERNAL' AND delivery_state='QUEUED'`,
		res.PublicationID).Scan(&queuedExternal))
	require.Equal(t, 1, queuedExternal, "EXTERNAL 的起始狀態現在也是 QUEUED")

	// outbox 的 payload 是**跨服務線材**：Task 6 的 notification-service template 用
	// inviteId 綁 reference_id，storeId 決定這張 RFQ 送到哪一家店。這裡讀的是真的
	// jsonb 欄位，釘住那三個 key 真的以這個形狀落到了 worker 讀得到的地方。
	//
	// ⚠️ 相鄰的 legacy publisher 發的是 {"inviteId":…,"store_id":…}（store key 是
	// snake_case）。**這一支不是那個形狀。**
	var platformInviteID int64
	var wireJSON []byte
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT i.id, o.payload FROM procurement_rfq_publication_outbox o
		JOIN procurement_rfq_publication_invites i ON i.id = o.invite_id
		WHERE i.publication_id=$1 AND i.store_id=5301`, res.PublicationID).Scan(&platformInviteID, &wireJSON))

	var wire map[string]any
	require.NoError(t, json.Unmarshal(wireJSON, &wire))
	// 整個 map 一起比對：多一個 key 也會紅（allowlist），兩個 id 對調也會紅。
	require.Equal(t, map[string]any{
		"inviteId":      strconv.FormatInt(platformInviteID, 10),
		"publicationId": strconv.FormatInt(res.PublicationID, 10),
		"storeId":       "5301",
	}, wire, "outbox payload 恰好這三個 camelCase key，值都是字串化的 id（避免 JavaScript 精度問題）")

	var externalInviteID int64
	var externalWireJSON []byte
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT i.id, o.payload FROM procurement_rfq_publication_outbox o
		JOIN procurement_rfq_publication_invites i ON i.id = o.invite_id
		WHERE i.publication_id=$1 AND i.prospect_id=6301`, res.PublicationID).Scan(&externalInviteID, &externalWireJSON))

	var externalWire map[string]any
	require.NoError(t, json.Unmarshal(externalWireJSON, &externalWire))
	require.Equal(t, map[string]any{
		"inviteId":      strconv.FormatInt(externalInviteID, 10),
		"publicationId": strconv.FormatInt(res.PublicationID, 10),
		"prospectId":    "6301",
		"expiresAt":     pubExecNow.Add(30 * 24 * time.Hour).Format(time.RFC3339),
	}, externalWire, "EXTERNAL outbox payload 恰好這四個 camelCase key，沒有 storeId")
}

func TestExecuteDirect_DuplicateExecuteCreatesNoSecondInviteOrOutbox(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	fx := seedExecutableDirectApproval(ctx, t, pool, []recipientSpec{
		{lane: "PLATFORM", storeID: 5401},
	})

	first, err := fx.uc.ExecuteRfqPublication(ctx, fx.input)
	require.NoError(t, err)
	second, err := fx.uc.ExecuteRfqPublication(ctx, fx.input)
	require.NoError(t, err)
	require.Equal(t, first.PublicationID, second.PublicationID, "同 key 重送回放同一張")

	var invites, outbox int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_rfq_publication_invites WHERE publication_id=$1`,
		first.PublicationID).Scan(&invites))
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM procurement_rfq_publication_outbox o
		JOIN procurement_rfq_publication_invites i ON i.id = o.invite_id
		WHERE i.publication_id=$1`, first.PublicationID).Scan(&outbox))
	require.Equal(t, 1, invites, "回放路徑不得產生第二張 invite")
	require.Equal(t, 1, outbox, "回放路徑不得產生第二個 outbox command")
}

// RFQ_PUBLICATION_RESULT artifact 把 PLATFORM／EXTERNAL 的分布變成買家看得到的三個數字
// （#2450 Slice 2b Task 5）。Vendor Portal E1 起兩個 lane 起始都是 QUEUED，
// notSentCount 因此恆為 0（現存的舊 NOT_SENT 列不受影響，見 InsertPublicationInvites）。
func TestExecuteDirect_ArtifactCarriesRecipientCounts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()

	fx := seedExecutableDirectApproval(ctx, t, pool, []recipientSpec{
		{lane: "PLATFORM", storeID: 5501},
		{lane: "EXTERNAL", prospectID: 6501},
	})
	res, err := fx.uc.ExecuteRfqPublication(ctx, fx.input)
	require.NoError(t, err)

	var payload []byte
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT public_data_json FROM procurement_agent_run_artifacts
		WHERE domain_ref_id = $1 AND artifact_type = 'RFQ_PUBLICATION_RESULT'`,
		strconv.FormatInt(res.PublicationID, 10)).Scan(&payload))

	var got map[string]any
	require.NoError(t, json.Unmarshal(payload, &got))
	require.Equal(t, float64(2), got["recipientCount"])
	require.Equal(t, float64(2), got["queuedCount"], "兩個 lane 起始都是 QUEUED")
	require.Equal(t, float64(0), got["notSentCount"])
	require.NotContains(t, got, "sentCount",
		"artifact 有 checksum 且不隨 worker 進度重算，放一個永遠是 0 的欄位會誤導")
}
