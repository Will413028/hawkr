package delegationservice

import "strings"

// 舊 gRPC 與事件的契約把這些代碼當小寫字串傳（api/proto/account/delegation.proto、gateway 與 admin 原樣比對），
// 模組儲存與內部型別是 UPPER_SNAKE（ADR 2026-09-25-codemeta-retirement D3）。轉換只在這一層：模組搬完、
// 舊 gateway 退場後，連同這個檔案一起刪除，對外改走 proto enum。

// toWire 把模組的代碼轉成舊契約的小寫字串。
func toWire[T ~string](v T) string { return strings.ToLower(string(v)) }

// fromWire 把舊契約的小寫字串轉成模組的代碼。只收小寫（大寫與混合大小寫回空字串）；不在模組列舉內的值仍原樣轉大寫，由模組當作不認得的值拒絕。
func fromWire[T ~string](s string) T {
	v := T(strings.ToUpper(s))
	if toWire(v) != s {
		return ""
	}
	return v
}
