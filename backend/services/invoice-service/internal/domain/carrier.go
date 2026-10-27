package domain

import "github.com/Will413028/hawkr/go-pkg/codemeta"

// AMEGO 的載具原生詞彙。
//
// InvoiceCarrierType 的 Value 刻意不用這些字面值：4 個值裡有 2 個（TAX_ID／DONATION）在 AMEGO
// 端根本沒有對應的 CarrierType 字面值，而 CodeMeta 的 Value 是必填欄 ⇒ 一律 UPPER_SNAKE，
// AMEGO 字面值寫在 Description 並由本檔的純函式對映（§C.3）。
//
// 連帶：invoice_documents.carrier_type varchar(6) 存的是 AMEGO 原生詞彙，它不是本 enum 的 Value。
const (
	AmegoCarrierMobileBarcode = "3J0002" // 手機條碼
	AmegoCarrierCitizenCert   = "CQ0001" // 自然人憑證條碼
	AmegoCarrierNone          = ""       // 打統編／捐贈：AMEGO 端不帶 CarrierType
)

// CarrierTypeToAmego 把 CodeMeta 的載具類型換成要落 invoice_documents.carrier_type、
// 也要送給 AMEGO 的原生詞彙。第二個回傳值為 false 表示該碼不在值域內。
//
// TAX_ID 與 DONATION 都回空字串——那是正確的（AMEGO 對這兩種不帶 CarrierType），不是「查不到」。
func CarrierTypeToAmego(ct codemeta.InvoiceCarrierType) (string, bool) {
	switch ct.Code {
	case codemeta.InvoiceCarrierTypeMobileBarcode.Code:
		return AmegoCarrierMobileBarcode, true
	case codemeta.InvoiceCarrierTypeCitizenCert.Code:
		return AmegoCarrierCitizenCert, true
	case codemeta.InvoiceCarrierTypeTaxId.Code, codemeta.InvoiceCarrierTypeDonation.Code:
		return AmegoCarrierNone, true
	default:
		return "", false
	}
}

// CarrierTypeFromAmego 把 AMEGO 原生詞彙換回 CodeMeta 的載具類型。
//
// 空字串刻意回 false：TAX_ID 與 DONATION 兩者的 AMEGO 詞彙都是空字串 ⇒ 反向不是函式。
// 要區分那兩者必須看 recipient_tax_id／donation_code 哪一個非空，而那是 usecase 手上的資料，
// 不是本層能推導的。回 true 加上任一個猜測值，就是在憑證上寫錯買受人身分。
func CarrierTypeFromAmego(s string) (codemeta.InvoiceCarrierType, bool) {
	switch s {
	case AmegoCarrierMobileBarcode:
		return codemeta.InvoiceCarrierTypeMobileBarcode, true
	case AmegoCarrierCitizenCert:
		return codemeta.InvoiceCarrierTypeCitizenCert, true
	default:
		return codemeta.InvoiceCarrierType{}, false
	}
}

// CarrierUsesAmegoCarrierField 回報該載具類型是否會在 AMEGO request 上實際帶出 CarrierType。
func CarrierUsesAmegoCarrierField(ct codemeta.InvoiceCarrierType) bool {
	v, ok := CarrierTypeToAmego(ct)
	return ok && v != AmegoCarrierNone
}
