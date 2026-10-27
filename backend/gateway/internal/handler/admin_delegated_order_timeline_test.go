package handler

import (
	"strconv"
	"testing"

	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
)

// 代行訂單詳情的時間軸曾經與訂單狀態完全無關：它逐欄判「這個時間戳非空就加一個節點」，
// 於是一筆 `status=80`（已取消）、`pay_time IS NULL`、`ship_time` 有值的真實訂單被渲染成
// 「待付款 → 配送中」——既宣告了它從來沒到過的狀態，也完全不提它已經取消。
//
// 這支測試把兩條不變式釘住：
//  1. 時間軸的最後一個節點恆為訂單**當下**的狀態
//  2. 沒有任何節點的狀態超過當下狀態能證明的進度（正向流程上）
//
// 鑑別力：把 buildOrderTimeline 改回「時間戳非空就加」，cancelledOrderWithStrayShipTime
// 與 pendingShipmentWithStrayShipTime 兩例立刻轉紅（前者尾節點變 30、後者多出 30）。

func timelineStatuses(entries []DelegatedOrderTimelineEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Status)
	}
	return out
}

func TestBuildOrderTimeline_LastNodeIsAlwaysCurrentStatus(t *testing.T) {
	const (
		created = "2026-08-05T07:59:56Z"
		paid    = "2026-08-05T08:02:00Z"
		shipped = "2026-08-05T08:00:10Z"
	)

	cases := []struct {
		name   string
		view   *orderpb.OrderDetailView
		want   []string
		reason string
	}{
		{
			name: "cancelledOrderWithStrayShipTime",
			view: &orderpb.OrderDetailView{
				Status:     strconv.Itoa(80),
				CreateTime: created,
				ShipTime:   shipped, // 未付款卻有出貨時間 = 資料雜訊
			},
			want:   []string{"0", "80"},
			reason: "未付款的訂單不可能出貨；已取消必須自己現身，不能被略過",
		},
		{
			name: "pendingShipmentWithStrayShipTime",
			view: &orderpb.OrderDetailView{
				Status:     strconv.Itoa(20),
				CreateTime: created,
				PayTime:    paid,
				ShipTime:   shipped, // 待出貨卻有出貨時間 = 資料雜訊
			},
			want:   []string{"0", "10", "20"},
			reason: "當下狀態是待出貨，時間軸不得出現配送中",
		},
		{
			name: "inTransit",
			view: &orderpb.OrderDetailView{
				Status:     strconv.Itoa(30),
				CreateTime: created,
				PayTime:    paid,
				ShipTime:   shipped,
			},
			want:   []string{"0", "10", "30"},
			reason: "當下狀態就是最後一個里程碑時不重複補一個節點",
		},
		{
			name: "completed",
			view: &orderpb.OrderDetailView{
				Status:     strconv.Itoa(50),
				CreateTime: created,
				PayTime:    paid,
				ShipTime:   shipped,
			},
			want:   []string{"0", "10", "30", "50"},
			reason: "已完成必須是最後一個節點",
		},
		{
			name: "pendingPaymentOnly",
			view: &orderpb.OrderDetailView{
				Status:     strconv.Itoa(0),
				CreateTime: created,
			},
			want:   []string{"0"},
			reason: "只有一個節點時不得補出第二個",
		},
		{
			name: "cancelledAfterShipping",
			view: &orderpb.OrderDetailView{
				Status:     strconv.Itoa(70),
				CreateTime: created,
				PayTime:    paid,
				ShipTime:   shipped,
			},
			want:   []string{"0", "10", "30", "70"},
			reason: "離開正向流程的狀態推不出走到哪，此時時間戳是唯一證據",
		},
		{
			name: "missingStatus",
			view: &orderpb.OrderDetailView{
				CreateTime: created,
			},
			want:   []string{"0"},
			reason: "上游沒填 status 時不補一個沒有標籤的節點",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := timelineStatuses(buildOrderTimeline(tc.view))
			if len(got) != len(tc.want) {
				t.Fatalf("節點 = %v, want %v（%s）", got, tc.want, tc.reason)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("節點 = %v, want %v（%s）", got, tc.want, tc.reason)
				}
			}
			// 每個節點都要有可顯示的標籤，否則畫面上會出現一個裸碼值。
			for _, e := range buildOrderTimeline(tc.view) {
				if e.StatusLabel == "" {
					t.Errorf("節點 %q 沒有 statusLabel", e.Status)
				}
			}
		})
	}
}

// 結案節點（送達／完成／取消）有專屬時間欄，時間軸最後一格必須帶上它。
//
// 這一格正是時間軸最重要的資訊——「這筆訂單什麼時候完成／取消的」。曾經三者一律送空
// 字串，前端因此完全不渲染那一行。
//
// 鑑別力：把 buildOrderTimeline 規則 2 改回 `entry(current, "")`，三個子測全部轉紅。
func TestBuildOrderTimeline_TerminalNodeCarriesItsOwnTimestamp(t *testing.T) {
	const (
		created   = "2026-08-05T07:59:56Z"
		paid      = "2026-08-05T08:02:00Z"
		shipped   = "2026-08-05T08:10:00Z"
		delivered = "2026-08-06T01:00:00Z"
		completed = "2026-08-09T01:00:00Z"
		cancelled = "2026-08-05T09:30:00Z"
	)

	cases := []struct {
		name   string
		view   *orderpb.OrderDetailView
		status string
		want   string
	}{
		{
			name: "cancelled",
			view: &orderpb.OrderDetailView{
				Status: strconv.Itoa(80), CreateTime: created, CancelledAt: cancelled,
			},
			status: "80", want: "2026-08-05T09:30:00Z",
		},
		{
			name: "completed",
			view: &orderpb.OrderDetailView{
				Status: strconv.Itoa(50), CreateTime: created, PayTime: paid,
				ShipTime: shipped, CompletedAt: completed,
			},
			status: "50", want: "2026-08-09T01:00:00Z",
		},
		{
			name: "delivered",
			view: &orderpb.OrderDetailView{
				Status: strconv.Itoa(40), CreateTime: created, PayTime: paid,
				ShipTime: shipped, DeliveredAt: delivered,
			},
			status: "40", want: "2026-08-06T01:00:00Z",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entries := buildOrderTimeline(tc.view)
			last := entries[len(entries)-1]
			if last.Status != tc.status {
				t.Fatalf("最後節點 = %q, want %q", last.Status, tc.status)
			}
			if last.OccurredAt != tc.want {
				t.Errorf("occurredAt = %q, want %q", last.OccurredAt, tc.want)
			}
			if entries[0].OccurredAt == "" {
				t.Error("建立節點必須帶 createTime")
			}
		})
	}
}

// 沒有專屬時間欄的狀態仍送空字串，前端據此不渲染時間那一行。
//
// 這是與上一支對稱的另一半：規則 2 不得為了「每個節點都有時間」而拿 updated_at 之類的
// 欄位充數——那一欄會被任何後續寫入推走（dev 實測最大漂 22.7 天）。
func TestBuildOrderTimeline_StatusWithoutOwnColumnHasNoFabricatedTime(t *testing.T) {
	const (
		created = "2026-08-05T07:59:56Z"
		paid    = "2026-08-05T08:02:00Z"
		shipped = "2026-08-05T08:10:00Z"
	)

	for _, tc := range []struct {
		name   string
		view   *orderpb.OrderDetailView
		status string
	}{
		{
			name: "pendingShipment",
			view: &orderpb.OrderDetailView{
				Status: strconv.Itoa(20), CreateTime: created, PayTime: paid,
			},
			status: "20",
		},
		{
			name: "fullyRefunded",
			view: &orderpb.OrderDetailView{
				Status: strconv.Itoa(70), CreateTime: created, PayTime: paid, ShipTime: shipped,
			},
			status: "70",
		},
		{
			// 已取消但取消時間為空（V11 之前的舊訂單無法回填）——仍不得回退到別的欄位。
			name: "cancelledWithoutCancelledAt",
			view: &orderpb.OrderDetailView{
				Status: strconv.Itoa(80), CreateTime: created,
			},
			status: "80",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := buildOrderTimeline(tc.view)
			last := entries[len(entries)-1]
			if last.Status != tc.status {
				t.Fatalf("最後節點 = %q, want %q", last.Status, tc.status)
			}
			if last.OccurredAt != "" {
				t.Errorf("occurredAt = %q, want 空字串（該狀態沒有專屬時間欄）", last.OccurredAt)
			}
		})
	}
}
