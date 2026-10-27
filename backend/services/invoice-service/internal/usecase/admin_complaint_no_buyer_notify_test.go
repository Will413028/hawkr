package usecase

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestAdminComplaintEmitsNoEventsUntilProjectionCoversNotifications 釘住的是一個
// **前提**，不是一個決定。
//
// ## 為什麼需要它
//
// 58A §B07 明訂買家面不出現「違規」二字。#2911 的投影（`toBuyerComplaintInfo`）
// 關掉了三條 RPC 路徑（`responses[].action`／ADM 列的 `note`／`status`），
// 而 `complaint_buyer_view_test.go` 的閘門是從 proto 描述子推導買家面 RPC 全集
// —— **它的射程只到 RPC**。
//
// 🔴 通知路徑完全在射程外：
//   - `services/notification-service/db/migrations/20260830160000_invoice_notification_templates.sql`
//     的 `invoice.complaint.responded.buyer`（target = buyer）在 `data_fields` 明文宣告
//     `action` 與 `status` 兩個欄位
//   - `notification-service` 的 inbox 以 `"data": item.Data` **整包下發**
//     ⇒ 只要有買家向的事件帶著這兩個軸，那兩個字就會繞過投影直達買家的站內信／推播。
//
// 今天不外洩的唯一理由是：**ADM 裁決路徑一個事件都不發**（本測試斷言的就是這件事）。
// 唯一的 producer 是賣家回覆路徑，而它的 `nextStatus` 只可能是
// `SELLER_RESPONDED` / `ESCALATED_TO_ADM`，兩者的 label 都不含「違規」。
//
// ## 這條紅燈出現時該做什麼（不是把測試改掉）
//
// 58A §B07 對 `ESCALATED_TO_ADM` 的買家文案逐字是「案件已交由平台客服處理，
// **有結果會通知你**」——所以補上 K08／K09 的裁決通知是**遲早要做的事**，
// 不是違規。要做的是**先讓通知 payload 走同一份投影**，再發事件：
//
//  1. 在 `emit` 之前對 `action` / `status` 套 `projectComplaintForBuyer` 的同一套規則
//     （或抽成共用函式，讓 RPC 與事件兩條路只有一份實作）
//  2. 把該事件加進 `complaint_buyer_view_test.go` 的覆蓋表
//  3. 才移除本測試
//
// ⚠️ **不要只把本測試刪掉**。它守的不是「不准發事件」，是「發事件之前投影要先覆蓋它」。
func TestAdminComplaintEmitsNoEventsUntilProjectionCoversNotifications(t *testing.T) {
	const path = "admin_complaint.go"

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("讀不到 %s：%v —— 檔案被改名或搬走時本測試必須 FAIL 而不是靜默跳過", path, err)
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 %s：%v", path, err)
	}

	// 錨在**識別字**上而不是字面字串：outbox 的用法一律經由這些名字，
	// 而字面字串會被註解與說明文字誤命中（本檔頭就寫了好幾次 outbox）。
	forbidden := map[string]string{
		"outbox":         "欄位或變數名為 outbox",
		"Outbox":         "型別或方法名含 Outbox",
		"NewOutboxEvent": "直接建構 outbox 事件",
		"Insert":         "outbox 寫入的慣用入口",
	}

	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Ident:
			if why, bad := forbidden[v.Name]; bad {
				pos := token.NewFileSet().Position(v.Pos())
				hits = append(hits, v.Name+"（"+why+"）"+pos.String())
			}
		case *ast.SelectorExpr:
			if why, bad := forbidden[v.Sel.Name]; bad {
				hits = append(hits, v.Sel.Name+"（"+why+"）")
			}
		}
		return true
	})

	if len(hits) > 0 {
		t.Fatalf(`🔴 admin_complaint.go 出現了事件發布的跡象：%s

ADM 裁決一旦發出買家向事件，「違規」二字會繞過 toBuyerComplaintInfo 的投影，
經由 invoice.complaint.*.buyer 模板的 data_fields（明文宣告 action 與 status）
直達買家的站內信／推播 —— 而 complaint_buyer_view_test.go 的閘門只掃 RPC，看不到它。

要做的不是刪掉本測試，是先讓通知 payload 走同一份投影（見本測試的檔頭註解）。`,
			strings.Join(hits, "、"))
	}
}
