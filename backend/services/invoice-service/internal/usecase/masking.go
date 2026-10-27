package usecase

import (
	"strings"
	"unicode/utf8"
)

// 遮罩函式（58 號 §8.1.3(d)）。純函式、無 IO，由 usecase 在組回應時呼叫。
//
// ⚠️ **落點與設計書不同**：§8.1.3(d) 指定 `internal/domain/masking.go`。本波的可寫範圍
// 只含 `internal/domain/repository.go`，故暫置於本層。兩者的約束相同（純函式、無 IO、
// 由組回應的那一層呼叫），搬家時連同 masking_test.go 一起移動即可，呼叫端不受影響。
//
// 🔴 遮罩由**後端**產出，前端不做（§8.1.3(c) 裁定 3）：前端遮罩時完整值仍在 response
// body 裡 ⇒ 會出現在 devtools、Gateway access log 的 resp_body 與前端 Sentry breadcrumb。
//
// 🔴 遮罩不適用於 O 類（發票號碼、買受人統編、賣家統編）——那三個值三端皆完整
// （§8.1.3(b)＋(c) 裁定 1）。看到「號碼」兩個字就順手遮，會直接消滅對獎與核對統編的功能。

// maskFill 是全遮時的固定輸出：不洩漏長度、也不洩漏任何一碼。
const maskFill = "••••"

// MaskEmail 遮罩發票通知 email（P 類，非資料主體視角）。
//
// 規則：保留 local part 前 4 碼（不足 4 碼保留 1 碼）＋ 完整 domain，遮罩字元**固定 3 個
// `*`、不隨長度變化**。
//
//	acmestore@example.com → acme***@example.com
//	ab@example.com        → a***@example.com
//
// 🔴 這裡刻意**不**沿用 MaskCarrierID 的「len-4」規則（§8.1.3(d) 末段）：email local part
// 是 1~64 的任意值且本身就是可辨識資訊，len-4 會產出 60 個 `*` 的荒謬輸出，而且長度照樣洩漏。
func MaskEmail(s string) string {
	if s == "" {
		return ""
	}
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		// 不是可辨識的 email 形狀 ⇒ 全遮。回原值等於「格式怪的那些人不受保護」。
		return maskFill
	}
	local, domain := s[:at], s[at:]
	keep := 4
	if utf8.RuneCountInString(local) < keep {
		keep = 1
	}
	r := []rune(local)
	return string(r[:keep]) + "***" + domain
}

// MaskPhone 遮罩買家手機（P 類，非資料主體視角）。保留前 2 與後 4（repo 既有慣例）。
//
//	0912341234 → 09••••1234
//
// 短於 7 碼時全遮：保留前 2 後 4 會露出 6 碼中的 6 碼，等於沒遮。
func MaskPhone(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) < 7 {
		return maskFill
	}
	return string(r[:2]) + strings.Repeat("•", len(r)-6) + string(r[len(r)-4:])
}

// AMEGO CarrierType 原值（DB 的 buyer_invoice_preferences.carrier_type 與
// invoice_documents.carrier_type 存的就是這兩個字面值，見 db/schema/schema.sql
// 的 chk_bip_kind_shape）。CodeMeta 的 MOBILE_BARCODE／CITIZEN_CERT 是**另一組值**
// （preference_kind／API 值域），不要拿來比對本函式的第一個參數。
const (
	amegoCarrierMobileBarcode = "3J0002"
	amegoCarrierCitizenCert   = "CQ0001"
)

// MaskCarrierID 遮罩載具識別碼（P 類，非資料主體視角）。
//
// 🔴 **兩個參數，禁止只傳值後靠長度猜型別**（§8.1.3(d) 表格與 §8.6 T-09）：手機條碼
// 8 碼、自然人憑證 16 碼，猜錯時要嘛遮過頭要嘛露太多，而兩者都不會報錯。
//
// 遮罩字元數 = len(value)-4，**刻意隨長度變化**：載具長度由 carrier_type 決定、是公開
// 資訊，露出長度沒有代價；固定字元數反而會讓 16 碼的憑證條碼看起來像 8 碼的手機條碼。
//
//	3J0002 /T7R3U+P         → /T••••+P
//	CQ0001 AB12345678901234 → AB••••••••••••34
//	其他／未知／len < 6      → ••••（全遮，不得回顯任何一碼）
//
// 新增載具型別 ＝ 在本 switch 加一個 case 並補測試，不是讓函式去猜。
func MaskCarrierID(carrierType, value string) string {
	if value == "" {
		return ""
	}
	r := []rune(value)
	switch carrierType {
	case amegoCarrierMobileBarcode, amegoCarrierCitizenCert:
		if len(r) < 6 {
			return maskFill
		}
		return string(r[:2]) + strings.Repeat("•", len(r)-4) + string(r[len(r)-2:])
	default:
		return maskFill
	}
}

// MaskDisplayName 遮罩買家顯示名（P 類，非資料主體視角；#3017 的後半）。
//
// 值的來源是 `buyers.display_name`（暱稱），**不是自然人真實姓名**——本服務從不持有
// 買家真名（§5.1.5：`recipient_name` 在 B2C 恆為常數「消費者」，I-45 有測試守著）。
// 但暱稱仍是使用者自選的可辨識字串，而讀它的人是**賣家與平台管理員**，不是資料主體本人
// ⇒ 依 58A §0.4 ③ 走遮罩。
//
// 規則：保留第 1 個字元 ＋ 固定 3 個 `•`。**刻意不隨長度變化**：暱稱長度沒有任何
// 業務用途，露出長度只是白給一個比對維度。
//
//	陳小明        → 陳•••
//	alexis       → a•••
//	單一字元      → 原樣（遮了等於全遮，而保留一個字元本來就是規則的底線）
//
// 🔴 **墓碑字串一律原樣回傳**：已註銷 stint 的 display_name 是 buyer-service 產生的
// 固定文案（不是任何人的個資），遮罩它會得到「已•••」這種讀不懂的東西，而它存在的
// 唯一理由就是要讓人讀懂。判斷「是不是墓碑」必須讀 `BuyerDisplay.deleted`，
// **不得拿 display_name 去跟墓碑字串做字串比對**（那會在每個顯示端硬編碼一份文案，
// 正是 BatchGetBuyerDisplayByBuyerIDs 的 proto 註解明令禁止的事）⇒ 由呼叫端傳入 deleted。
func MaskDisplayName(s string, deleted bool) string {
	if s == "" {
		return ""
	}
	if deleted {
		return s
	}
	r := []rune(s)
	if len(r) == 1 {
		return s
	}
	return string(r[:1]) + "•••"
}
