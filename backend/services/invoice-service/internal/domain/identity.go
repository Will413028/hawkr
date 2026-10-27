package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// 衍生識別字（§5.3.2）。三個共同性質：**可重建**、**逾時重試不換號**、**純函式**。
//
// 需求書建議的 `issue:{store_id}:{order_no}:{snapshot_version}` 在作廢重開情境會撞鍵
// （買家只改通知 Email ⇒ snapshot_version 不變 ⇒ 算出同一個 OrderId ⇒ AMEGO 回 3040171
// 而我方以為「已成功」⇒ 原票已作廢、新票沒開出來，買家手上一張發票都沒有）。
// ⇒ 兩個正交的版本軸：內容軸 snapshot_version（order-service 寫）＋ 實體軸
// issue_attempt_seq（本服務寫，作廢重開必 +1）。

// ProviderRequestNoForOrder 組出 ORDER 來源的 AMEGO OrderId（＝ provider_request_no）。
//
// 長度上界：order_no ＝ "O" + %09d ＝ 10 字，加 "-999-999" ＝ 18 字 ≪ 40
// （chk_invdoc_request_no_len／chk_invop_request_no_len 兜底）。
// 用帶訂單編號的單號而非純 TSID，是因為 ADM 向光貿查證時是人在 AMEGO 後台用 OrderId 找。
func ProviderRequestNoForOrder(orderNo string, snapshotVersion, issueAttemptSeq int32) string {
	return fmt.Sprintf("%s-%d-%d", orderNo, snapshotVersion, issueAttemptSeq)
}

// ProviderRequestNoForManual 組出 MANUAL 來源的 AMEGO OrderId。
func ProviderRequestNoForManual(documentID int64) string {
	return fmt.Sprintf("MAN-%d", documentID)
}

// OperationKeyFor 組出 invoice_operations.operation_key（全域 UNIQUE，§5.3.2）。
//
// 六種 operation_type 各有一條推導規則，一條都不能少：少一條的症狀不是編譯錯，是那一類
// operation 沒有 operation_key 可寫（NOT NULL）⇒ 第一次建立就 500。
// ISSUE／REISSUE／QUERY 掛在 document 上，三種調整類掛在 adjustment 上；VOID_ALLOWANCE 與
// VOID／ALLOWANCE 用同一個 adjustment_id 但**前綴不同**，三者可並存。
//
// 第二個回傳值為 false 代表 operationType 不在值域內——呼叫端必須當錯誤處理，
// 不得 fallback 成任一前綴（撞到別人的鍵就是把兩件事當成同一件）。
func OperationKeyFor(operationType string, documentID, adjustmentID int64, issueAttemptSeq int32, queryRound int32) (string, bool) {
	switch operationType {
	case codemeta.InvoiceOperationTypeIssue.Value:
		return fmt.Sprintf("issue:%d:%d", documentID, issueAttemptSeq), true
	case codemeta.InvoiceOperationTypeReissue.Value:
		return fmt.Sprintf("reissue:%d:%d", documentID, issueAttemptSeq), true
	case codemeta.InvoiceOperationTypeQuery.Value:
		return fmt.Sprintf("query:%d:%d:%d", documentID, issueAttemptSeq, queryRound), true
	case codemeta.InvoiceOperationTypeVoid.Value:
		return fmt.Sprintf("void:%d", adjustmentID), true
	case codemeta.InvoiceOperationTypeAllowance.Value:
		return fmt.Sprintf("allowance:%d", adjustmentID), true
	case codemeta.InvoiceOperationTypeVoidAllowance.Value:
		return fmt.Sprintf("void_allowance:%d", adjustmentID), true
	default:
		return "", false
	}
}

// RequestSnapshotHash 是 invoice_operations.request_snapshot_hash：送出內容的雜湊。
//
// 用途是「不一樣代表有人改了快照」——它**不是**冪等鍵（那是 operation_key），也不參與
// 任何 CAS 述詞。因此它只需要 ① 決定性 ② 涵蓋所有會影響 AMEGO request 的欄位。
//
// 決定性靠逐欄手寫的字串而非 struct 反射：encoding/json 對 map 會排序、對 struct 依欄位
// 宣告序輸出，但欄位重排是無聲的 —— 手寫序列讓「改了欄位順序就換 hash」變成 code review
// 看得見的事。
func RequestSnapshotHash(d *IssueDraft) string {
	if d == nil {
		return ""
	}
	type hashLine struct {
		Seq  int32  `json:"seq"`
		Type string `json:"type"`
		Desc string `json:"desc"`
		Qty  int32  `json:"qty"`
		Unit int64  `json:"unit"`
		Net  int64  `json:"net"`
		Tax  int16  `json:"tax"`
	}
	payload := struct {
		Buyer    string     `json:"buyer"`
		Carrier  string     `json:"carrier"`
		CarrierV string     `json:"carrier_value"`
		Donation string     `json:"donation"`
		Sales    int64      `json:"sales"`
		Free     int64      `json:"free"`
		Zero     int64      `json:"zero"`
		Tax      int64      `json:"tax"`
		Total    int64      `json:"total"`
		TaxType  int16      `json:"tax_type"`
		Items    []hashLine `json:"items"`
	}{
		Buyer:    d.BuyerIdentifier,
		Carrier:  d.CarrierType,
		CarrierV: d.CarrierValue,
		Donation: d.DonationCode,
		Sales:    d.Header.SalesAmount,
		Free:     d.Header.FreeTaxSalesAmount,
		Zero:     d.Header.ZeroTaxSalesAmount,
		Tax:      d.Header.TaxAmount,
		Total:    d.Header.TotalAmount,
		TaxType:  int16(d.Header.TaxType),
	}
	for _, l := range d.Items {
		payload.Items = append(payload.Items, hashLine{
			Seq: l.LineSeq, Type: l.LineType, Desc: l.Description,
			Qty: l.Quantity, Unit: l.UnitPrice, Net: l.NetAmount, Tax: int16(l.TaxCategory),
		})
	}
	b, err := json.Marshal(payload)
	if err != nil {
		// payload 只含 string／整數／slice，Marshal 不可能失敗；真的失敗時回一個
		// 恆不相等的值比回空字串好——空字串會讓「沒算過」與「算出來是空的」混為一談。
		return "unmarshalable"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// IsBusinessTaxID 回報這串字是不是可用於 B2B 發票的統一編號。
//
// 只驗形狀（8 位數字），不驗檢查碼：檢查碼規則由財政部定義且有歷史版本差異，
// 誤判會把一張合法的 B2B 發票降級成 B2C；真正的驗證是 AMEGO 的 ban_query（§4.5.5）。
// 🔴 8 碼**不可**左補零成 10 碼（會被 3040122 拒），chk_invdoc_b2b 也會擋。
func IsBusinessTaxID(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
