// Package eventwire 持有出向 MQ 事件上「路由識別碼」兩個軸的**不透明型別**。
//
// 為什麼要一個獨立 package：這兩個軸的缺陷（P0-3 與它在 buyer_id 上的孿生）都不是
// 筆誤，是**兩個看起來都對的欄位之間選錯**——
//
//	invoice_issuers.id             TSID，invoice_issuers 自己的 PK
//	invoice_issuers.issuer_ref_id  SELLER ⇒ sellers.store_id；PLATFORM ⇒ 0
//
//	invoice_documents.recipient_ref_id
//	    recipient_type = BUYER    ⇒ buyers.buyer_id      ← 只有這一種是 buyer_id
//	    recipient_type = SELLER   ⇒ sellers.store_id     ← 填進 buyer_id 就是裝錯軸
//	    recipient_type = EXTERNAL ⇒ 恆為 NULL
//
// 兩者填錯的後果都不是「欄位錯」而是**通知靜默投遞到錯的人或無人**，全程零錯誤、
// 零 DLQ、零 metric。
//
// 🔴 保證的形狀是**編譯期**，不是檢查：StoreID／BuyerID 的底層是**帶未匯出欄位的
// struct**，所以在本 package 之外：
//
//	ev.StoreID = "123"                       // 編不過（型別不符）
//	ev.StoreID = StoreID("123")              // 編不過（string 不能轉成 struct）
//	ev.StoreID = StoreID{v: "123"}           // 編不過（v 未匯出）
//	ev.StoreID = eventwire.StoreIDOf(issuer) // 唯一能編過的寫法
//
// 底層若用 `type StoreID string`，第二行會編得過——那是本設計刻意不採 named string
// 的唯一理由。
//
// 編譯期擋不住的有兩種，兩種都不在型別系統的守備範圍內：
//
//	① **整個欄位不填**（零值）。由 MarshalJSON 回 error 擋下——事件在業務交易內
//	   marshal，錯誤直接讓該交易失敗，不會產出一則裝著空字串的事件。AST 層另有一道
//	   「複合字面量必須顯式設定該欄」的檢查（R4）讓它在寫 code 時就紅。
//
//	② **引數是就地捏出來的**：
//	     eventwire.StoreIDOf(domain.Issuer{ID: doc.IssuerID, IssuerRefID: doc.IssuerID})
//	   🔴 型別系統對這一種**完全無能為力**，理由見下面 StoreIDOf 的紅字。
//	   它由 internal/usecase/event_store_id_contract_test.go 的 R8（引數溯源）與
//	   R9（本 package 不得就地組出 domain 聚合）擋下。
//
// 🔴 所以本 package 的保證是「**值的形狀**」不是「**值的來源**」：
// 外面組不出一個 StoreID，但外面可以決定要把哪個 issuer 餵進來。來源那一半住在
// AST gate，不住在這裡。把這兩件事講成同一件，正是本檔頭 2026-08-31 之前的錯誤。
package eventwire

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// ErrUnsetStoreID／ErrUnsetBuyerID 是「欄位被漏掉」的訊號。
//
// 🔴 刻意不是「marshal 成空字串」：空字串會讓 outbox dispatcher 的 withNotifyRouting
// 解不出賣家 ⇒ notify_seller_account_id 恆 "0" ⇒ 該則通知被下游 skip，而那正是要防的
// 靜默失效。回 error 會讓開票交易當場失敗、留在 log 與 metric 上。
var (
	ErrUnsetStoreID = errors.New("eventwire: store_id 未設定（事件 payload 漏填或 issuer 未載入）")
	ErrUnsetBuyerID = errors.New("eventwire: buyer_id 未設定（事件 payload 漏填或 document/complaint 未載入）")
)

// StoreID 是出向事件 `store_id` 欄位的唯一載體。
//
// 唯一建構點是 StoreIDOf。零值不可 marshal（見 ErrUnsetStoreID）。
type StoreID struct {
	v   string
	set bool
}

// StoreIDOf 由**開票人本體**取出 store_id。
//
// 參數是 domain.Issuer 而不是 int64：收 int64 的話 `StoreIDOf(doc.IssuerID)` 會編譯過，
// 而那正是 P0-3 的那一行。換成 domain.Issuer 之後，那個**手滑**的形狀編不過了。
//
// 🔴 **但這不等於「值一定來自 repository」，而本檔在 2026-08-31 之前正是這樣宣稱的**
// （原文：「取得 domain.Issuer 的唯一路徑是 IssuerRepository」）。那句話是假的：
// domain.Issuer 是同一個 module 裡的普通 struct、欄位全部匯出，所以
//
//	eventwire.StoreIDOf(domain.Issuer{ID: doc.IssuerID, IssuerRefID: doc.IssuerID})
//
// 就地捏得出來、編譯得過，而且填進 store_id 的是 invoice_issuers 的 PK ——
// P0-3 原樣復活。實測：`go build` exit 0、AST gate 連跑 8 次全 PASS、
// `go test -tags=integration` exit 0、check-mq-wire-contract.py exit 0，**零訊號**。
//
// 為什麼型別系統補不上這個洞：能設那個未匯出見證欄位的只有 package domain 自己，
// 而真正組出 Issuer 的是 internal/repository/postgres 的 mapper（**另一個 package**）
// ⇒ 見證欄位得靠一支 `domain.NewIssuer(...)` 來設，而那支誰都能呼叫 ⇒ 憑證可偽造，
// 只是多繞一層。（把欄位改成未匯出更不可行：整個讀路徑都在 domain 之外。）
//
// ⇒ 「來源正確」這件事由 internal/usecase/event_store_id_contract_test.go 的兩條
// AST 規則守，不由本 package 守：
//
//	R8 建構函式的引數必須追得回一個真的從 repository 取得的物件（追不到即報錯）
//	R9 internal/usecase 內不得就地組出 domain.Issuer／Document／Complaint
//
// 讀到這裡要記得的一句話：**本函式的參數型別擋的是手滑，不是刻意繞道。**
//
// issuer 是零值（沒有真的去載）時回不可 marshal 的值：真實 issuer 的 PK 是 TSID，
// 恆不為 0。這一條擋的是「忘了 GetByID 就直接組事件」。
func StoreIDOf(issuer domain.Issuer) StoreID {
	if issuer.ID == 0 {
		return StoreID{}
	}
	// PLATFORM 開票人的 issuer_ref_id 恆為 0 ⇒ store_id "0" 是正確結果（沒有賣家要通知），
	// 不是缺漏。
	return StoreID{v: strconv.FormatInt(issuer.IssuerRefID, 10), set: true}
}

// String 只給 log／錯誤訊息用。
func (s StoreID) String() string { return s.v }

// MarshalJSON 讓 StoreID 在 wire 上是一個**字串**。
//
// TSID 以 JSON number 進 generic-map consumer（notification-service）會被解成 float64
// 而截斷 ⇒ 這個型別永遠不吐數字。
func (s StoreID) MarshalJSON() ([]byte, error) {
	if !s.set {
		return nil, ErrUnsetStoreID
	}
	return json.Marshal(s.v)
}

// BuyerID 是出向事件 `buyer_id` 欄位的唯一載體。
//
// 「沒有買家」與「忘了填」是**兩件事**：前者是 BuyerIDOfDocument 對 SELLER／EXTERNAL
// 收件人回的值（set 且為空字串，wire 上是 ""），後者是零值（marshal 失敗）。
type BuyerID struct {
	v   string
	set bool
}

// BuyerIDOfDocument 由 document 取出 buyer_id，**先判 recipient_type**。
//
// 🔴 recipient_ref_id 是一個多義欄位（見 package doc）：只有 recipient_type = BUYER
// 時它才是 buyers.buyer_id。SELLER 時它是 sellers.store_id、EXTERNAL 時恆為 NULL。
// 不判就填 ⇒ 方向二／四的事件會把一個 store_id 裝進 buyer_id ⇒ 58 §6.3.9 模板的
// target_user_field 正是它 ⇒ 通知投遞到錯的人或無人。
//
// ⚠️ 這道守衛守的是「這份 document 說自己是什麼」，不是「這份 document 是真的」。
// 就地捏一個 `domain.Document{RecipientType: "BUYER", RecipientRefID: <store_id>}`
// 可以把守衛整個繞過去 —— 同 StoreIDOf 的紅字，那一半由 R8／R9 擋。
func BuyerIDOfDocument(doc domain.Document) BuyerID {
	if doc.ID == 0 {
		return BuyerID{}
	}
	if doc.RecipientType != codemeta.InvoiceRecipientTypeBuyer.Value || doc.RecipientRefID == nil {
		return BuyerID{set: true}
	}
	return BuyerID{v: strconv.FormatInt(*doc.RecipientRefID, 10), set: true}
}

// BuyerIDOfComplaint 由申訴案件取出申訴人 buyer_id。
//
// 這個軸沒有多義性：invoice_complaints.complainant_buyer_id 依 schema 恆為 buyers 的
// 業務鍵（申訴只有買家能開），所以不需要 recipient_type 判斷。它獨立成一個建構函式而
// 不是共用 BuyerIDOfDocument，是為了讓「為什麼這裡不用判」在型別層就有答案。
func BuyerIDOfComplaint(c domain.Complaint) BuyerID {
	if c.ID == 0 {
		return BuyerID{}
	}
	return BuyerID{v: strconv.FormatInt(c.ComplainantBuyerID, 10), set: true}
}

// String 只給 log／錯誤訊息用。
func (b BuyerID) String() string { return b.v }

// MarshalJSON 同 StoreID：恆為字串，零值回 error。
func (b BuyerID) MarshalJSON() ([]byte, error) {
	if !b.set {
		return nil, ErrUnsetBuyerID
	}
	return json.Marshal(b.v)
}
