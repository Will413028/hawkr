package media

import "testing"

// FinalizeUnlockEvidence 的 refType 必須是 InvoiceDocumentRefType，不得借用
// RefType（＝ CERTIFICATION，稅籍佐證）。
//
// 🔴 借用的後果不是「分類不精確」，是**資料被別人的回收打掉**：
// ReleaseReferences 依 (ref_type, ref_id) 批次釋放，而 CERTIFICATION 的 ref_id
// 空間與 seller-service 的商家認證共用（見 client.go 的 FinalizeEvidence 紅字）。
// 兩者撞在一起時，某次稅籍佐證的回收會連帶釋放掉解鎖證據的引用，
// 而症狀是「證據檔在某天之後下載不到了」——沒有任何錯誤訊息。
func TestUnlockEvidenceUsesInvoiceDocumentRefType(t *testing.T) {
	if InvoiceDocumentRefType == RefType {
		t.Fatal("解鎖證據不得與稅籍佐證共用 refType：ReleaseReferences 依 (ref_type, ref_id) 批次釋放")
	}
	if InvoiceDocumentRefType == "" {
		t.Fatal("InvoiceDocumentRefType 不得為空——空值會落進別人的 ref 空間")
	}
}

// 空 token ⇒ 回空字串且不打 media：證據檔是選填的（kind ＋ 查證說明已構成 §6.4 的證據）。
// 這一格若改成回錯誤，端點 54 的「不附檔」路徑會整條壞掉。
func TestFinalizeUnlockEvidenceEmptyTokenIsNoop(t *testing.T) {
	c := &Client{} // 刻意不給 stub：真的打出去就會 nil panic，那正是本測試要抓的
	key, err := c.FinalizeUnlockEvidence(t.Context(), "", 7001)
	if err != nil {
		t.Fatalf("空 token 不是錯誤：%v", err)
	}
	if key != "" {
		t.Fatalf("空 token 應回空字串，實得 %q", key)
	}
}
