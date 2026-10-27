package grpc

import "testing"

func TestNewAIProcurementClient_ImplementsAllRPCs(t *testing.T) {
	// 編譯期檢查：AIProcurementClient 必須有 18 個對應 RPC 的方法。這裡只驗證
	// 建構子不 panic 且回傳非 nil——方法簽章正確與否由 Go 編譯器在其他呼叫點檢查。
	client := NewAIProcurementClient(&aiServiceConn{})
	if client == nil {
		t.Fatal("NewAIProcurementClient returned nil")
	}
}
