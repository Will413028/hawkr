package grpc

import (
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
)

// 本檔是 B07 申訴案的**買家視角投影**（#2911）。
//
// # 保證
//
// 58A §B07 明訂「買家面不出現『違規』二字」（狀態表 `VIOLATION_CONFIRMED` 列逐字、
// §E3 逐字：「那是平台對賣家的治理結論，向買家揭露一家店的違規紀錄既無必要也有法律
// 風險」）。
//
// # 為什麼保證在後端，不在前端
//
// 58A §0.4 ⑤ 逐字：「CodeMeta：**中文由後端下發，前端只寫「值 → 顏色」**」。
// 買家畫面上每一句狀態名／動作名都必須查 CodeMeta 的生成對照表，而 CodeMeta 裡
// **有兩個** label 含「違規」（`InvoiceComplaintAction.DECIDED_VIOLATION` 逐字
// 「平台判定賣家違規」、`InvoiceComplaintStatus.VIOLATION_CONFIRMED` 逐字
// 「判定賣家違規」）⇒ 只要那兩個**值**送到買家手上，前端照規矩查表就會印出那兩個字。
// 前端沒有合法手段擋下它。
//
// 而 58A §0.5.2 對同型問題（發票號碼遮罩）給了通則，逐字：「前端一旦補遮，萬一後端
// 真的把該遮的東西洩漏出來，**也沒有任何人會發現**」——把可見性保證放在前端，等於
// 同時關掉偵測它失效的能力。
//
// ⇒ 兩個值都在買家面的型別邊界投影掉，投影點只有這一處。
//
// # 投影三件（缺一都不算修好）
//
//	① responses[].action：DECIDED_VIOLATION → DECIDED_RESOLVED
//	② responses[].note（僅 ADM 的列）→ 清空
//	③ status：VIOLATION_CONFIRMED → RESOLVED
//
// **① 投影而不是整列不下發**，依據是 §B07 自己的三段：
//
//	(a) 版面結構逐字「<ol> 回覆歷程（responses[]）：角色（賣家／**平台**）· 動作 label ·
//	    時間 · 說明」⇒ 平台的列本來就是買家畫面的一部分。
//	(b) 文案表「歷程 action label」列的是 CodeMeta 的**全部六句**（含 ADM 的三句），
//	    ⇒ 買家本來就預期看得到平台做了什麼，缺的只是那一句不能出現。
//	(c) 狀態表 RESOLVED ／ VIOLATION_CONFIRMED「兩者用完全相同的兩句」⇒ 規格處理這件事的
//	    手法一貫是「讓兩者對買家等價」，不是「把其中一個藏起來」。整列不下發還會讓案件從
//	    ESCALATED_TO_ADM 的「案件已交由平台客服處理，有結果會通知你」直接跳到「申訴已完成」
//	    而歷程上沒有任何平台的紀錄——那句承諾在畫面上會落空。
//
// 🔴 **② note 也要處理，只改 action 不算修好**：ADM 的 `reason` 是端點 60 的必填自由文字，
// 原樣落進 invoice_complaint_events.message（admin_complaint.go 的 appendTimeline
// `Message: in.Reason`）⇒ 一句「查證屬實，判定賣家違規」照樣把那兩個字送到買家眼前，
// 而它是**裁定違規的理由**，實務上幾乎必然含有那兩個字。只投影 action 的保證會退化成
// 「取決於 ADM 怎麼打字」，那不是保證。
//
// ⇒ 買家視角一律不下發 ADM 的自由文字。這不是資訊被藏起來：買家該讀到的那兩句由 §B07
// 的狀態橫幅文案負責（RESOLVED／VIOLATION_CONFIRMED 共用「文件已補開或補寄…」），而
// A08 的決策理由欄在 ADM 介面上沒有任何「買家看得到」的提示（對照端點 63 明寫「這會直接
// 給客服看」）⇒ 它從來就不是寫給買家的。賣家與 ADM 的視角不套用本投影，仍讀得到全文。
//
// 🔴 **③ status 曾被刻意留著，那是本檔的第一版判斷錯誤（#2911 覆核推翻）**。
// 當時的理由是「前端對 VIOLATION_CONFIRMED 有自己的分支與買家面文案」——那句話只對了
// 一半：`foldInvoiceComplaintStatus()` 確實把 RESOLVED／VIOLATION_CONFIRMED 折成相同的
// **橫幅**文案，但同一個元件另外渲染了一個獨立的狀態 `<Chip>`
// （hawkr-website `features/invoices/components/InvoiceComplaintSection.tsx` 的
// `statusChipLabel`），而它直接查 `INVOICE_COMPLAINT_STATUS_LABEL` ⇒ 買家實際看到的
// 就是「判定賣家違規」四個字。那個 Chip 不是前端自己加的，58A §B07 的版面結構逐字寫著
// 「狀態橫幅：案件編號 · <Chip status> · <SlaCountdown>」。
//
// 投影目標選 RESOLVED 的依據：§B07 狀態表 `VIOLATION_CONFIRMED` 列的「買家面語意」欄
// 逐字就是「已解決」，而 `INVOICE_COMPLAINT_STATUS_LABEL[RESOLVED]` 逐字是「已解決」。
// 投影之後 Chip 印出來的正是規格要的那兩個字，且中文仍然來自 CodeMeta（§0.4 ⑤ 成立）。
//
// # ③ 對前端三個讀者的影響（投影前逐一實查過，不是推論）
//
//	橫幅  `foldInvoiceComplaintStatus`：RESOLVED 與 VIOLATION_CONFIRMED 本來就 fallthrough
//	      到同一個 banner ⇒ 投影後零變化。
//	Chip  `statusChipLabel`：「判定賣家違規」→「已解決」⇒ 這就是本次要修的東西。
//	動作鍵 `complaintActions`：`canReject`／`canWithdraw` 對兩個值都是 false ⇒ 不受影響；
//	      **`showReopen` 原本是 `status === RESOLVED`，投影後會對終態案件冒出一顆重開鍵**。
//
// 最後那一項是投影唯一的真實代價，配套修法在 website 端把 `showReopen` 補成
// 「`status === RESOLVED` **且** `reopenAllowedAfter !== ''`」——那不是為了配合本投影而
// 發明的條件，**58A §B07 的版面結構本來就是這樣寫的**，逐字：
//
//	「重新提出」 ← 65；status === 'RESOLVED' 且 Number(reopenCount) < 2
//	              且 reopenAllowedAfter !== '' 且 now >= reopenAllowedAfter
//
// 那是「動作列（渲染條件互斥）」底下的**渲染**條件。前端把它拆成 showReopen／canReopen
// 兩半（因為文案表另外要求「冷卻中不隱藏按鈕」），拆的時候把 `reopenAllowedAfter !== ''`
// 整條放進了 canReopen 那一半 ⇒ 規格要求「不渲染」的那一格變成了「渲染成 disabled」。
// 本次只是把它放回渲染側，與共通規則第 4 條（「重開鍵讀 `reopenCount` ＋
// `reopenAllowedAfter` 兩欄自行判斷」）也一致。
//
// 那個條件成立的依據在 SQL 而不是慣例：`invoice_complaints.sql` 的 K13 述詞含
// `reopen_allowed_after IS NOT NULL`，而三條進 RESOLVED 的邊（K06／K08／K12）全部寫
// `now + 24h`、K09（VIOLATION_CONFIRMED）逐字「傳 NULL（終態，沒有重開邊）」
// ⇒ `reopenAllowedAfter` 是否為空，就是後端對「有沒有重開邊」的權威回答。
// 前端補上它之後兩種部署順序都安全，而**前端先上零窗口**（今天 status 還沒被投影，
// 真正的 RESOLVED 一律帶得到那一欄 ⇒ 該條件恆真 ⇒ 行為完全不變）。
//
// ⚠️ **投影必須發生在 toComplaintInfo 之後**：那支 mapper 用**真實** status 決定
// `buyer_review_due_at` 與 `reopen_allowed_after` 該不該填（`c.Status == complaintStatusResolved`）。
// 先投影再組裝會讓終態案件被填上一個不存在的重開時間，而那正好會把上面那條配套條件廢掉。
//
// # 值域收窄不改 proto
//
// 買家面收到的 `status` 因此只有 6 個值、`action` 只有 5 個。proto 的欄位註解仍列
// 完整值域（它描述的是契約值域，不是某一個視角的投影後值域），與 `action` 在 #2911
// 的處理方式一致——收窄一律登記在 58A §B07 的文案表與狀態表。

// complaintActorAdmin 是 invoice_complaint_events.actor_type 的 ADM 值
// （chk_invcompev_actor 四值之一）。無對應 CodeMeta 碼表，與 usecase 側同名常數同源。
const complaintActorAdmin = "ADMIN"

// toBuyerComplaintInfo 是**買家面唯一**的申訴 mapper（端點 13／14／15／63／64／65）。
//
// 賣家面（端點 34／35／36）與 ADM 面（端點 58／59）一律用 toComplaintInfo，不套投影。
func toBuyerComplaintInfo(v usecase.ComplaintView) *pb.InvoiceComplaintInfo {
	return projectComplaintForBuyer(toComplaintInfo(v))
}

// projectComplaintForBuyer 就地投影（toComplaintInfo 每次回新的 message，不會影響別人）。
func projectComplaintForBuyer(info *pb.InvoiceComplaintInfo) *pb.InvoiceComplaintInfo {
	if info == nil {
		return nil
	}
	// ③ 狀態值本身。買家面語意等同 RESOLVED（§B07 狀態表），而 CodeMeta 對
	// VIOLATION_CONFIRMED 的 label 是「判定賣家違規」⇒ 值不能出去。
	if info.GetStatus() == codemeta.InvoiceComplaintStatusViolationConfirmed.Value {
		info.Status = codemeta.InvoiceComplaintStatusResolved.Value
	}
	for _, r := range info.GetResponses() {
		if r == nil || r.GetActor() != complaintActorAdmin {
			continue
		}
		// ① 動作值。
		if r.GetAction() == codemeta.InvoiceComplaintActionDecidedViolation.Value {
			r.Action = codemeta.InvoiceComplaintActionDecidedResolved.Value
		}
		// ② ADM 的裁決理由是寫給賣家與稽核的，不下發給買家（見檔頭）。
		r.Note = ""
	}
	return info
}
