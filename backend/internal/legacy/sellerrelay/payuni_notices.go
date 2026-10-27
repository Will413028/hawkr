package sellerrelay

// payuniUnknown 是目錄沒有的狀態碼的 fail-closed 文案的 key。
const payuniUnknown = "PAYUNI_UNKNOWN"

// payuniNotices 是通知賣家審核結果的文案：來源是 CodeMeta 的 PayuniSellerFacingError（pkg/codemeta/labels.go，另一個 Go
// module，單體不 import），只放 PayUni 狀態碼（payuniStatusCode 可能回的 MER_x、MEMBER_x，錯誤目錄裡有登記的那些）、
// 待補件的預設文案（MEMBER_5）與 fail-closed 的通用文案。TestNoticesMatchCodeMeta 逐字比對 labels.go。CodeMeta 凍結，
// 隨舊契約退場。
var payuniNotices = map[string]string{
	"MER_1":          "收款功能已開通",
	"MER_2":          "這家商店的金流服務已關閉，請聯繫客服協助確認",
	"MER_8":          "金流申請未通過審核，請確認資料後重新提交",
	"MER_9":          "金流申請審核中",
	"MER_91":         "金流審核需要補件，請依指示補齊資料後重新提交",
	"MER_92":         "金流申請審核未通過，請確認資料後重新提交",
	"MER_99":         "這家商店的金流服務已停用，請聯繫客服協助確認",
	"MEMBER_1":       "金流會員資料已通過",
	"MEMBER_5":       "請查收 PayUni 寄送的通知信，並依指示至 PayUni 會員後台填寫會員資料",
	"MEMBER_8":       "金流會員審核未通過，請確認個人或公司資料後重新提交",
	"MEMBER_9":       "金流會員資料審核中",
	"MEMBER_92":      "金流會員審核未通過，請確認個人或公司資料後重新提交",
	"MEMBER_99":      "金流會員帳號已停權，請聯繫客服協助確認",
	"PAYUNI_UNKNOWN": "申請未通過，請聯繫客服協助確認",
}
