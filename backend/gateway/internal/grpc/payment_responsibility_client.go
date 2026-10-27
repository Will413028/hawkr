package grpc

import (
	"context"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/payment"
)

// ==================== 責任判定 maker-checker（FulfillmentResponsibilityService）====================
//
// 🔴 這兩支 RPC 走 **payment-service 的同一條 gRPC channel**：FulfillmentResponsibilityService
// 與 PaymentService／LogisticsFinanceService 是同一個 .proto、同一個 Java 服務、同一個
// port（9925）。獨立 service name 是為了讓「唯讀」與「寫入」的授權邊界看一眼就成立，
// 不是另一個部署單元。
//
// ⚠️ **deadline 給 30s 而不是唯讀那批的 15s**：這兩支都會寫 DB，而且各自帶一個跨服務
// 的同步輸入——Propose 要向 order-service 取 orders.total_actual_amount 才算得出影響
// 預覽，Review 的 APPROVE 要在同一個交易內推進案件狀態並排入 N16 outbox。deadline 必須
// 大於這條鏈的最壞耗時，否則 gateway 這端超時放棄時 payment-service 那端可能已經寫完
// ——而呼叫端會把它讀成「沒做到」而重送，那正是 RequireIdempotencyKey 在路由層守的東西。
//
// 🔴 **操作者身分不在 request 裡**：proto 刻意沒有 proposed_by_account_id／
// reviewed_by_account_id，兩者都取自 Java 側的 GrpcAuthContext，而那個 context 由
// DefaultDialOpts 的 AuthMetadataInterceptor 從 gateway 的 x-account-id 轉發過去。
// 由 request 帶等於讓呼叫端宣稱自己是誰，chk_fcr_two_person（覆核者不得為提案者）
// 就形同虛設。

// ProposeResponsibilityResolutionForAdmin A-4 maker：提出責任判定提案。
func (c *PaymentClient) ProposeResponsibilityResolutionForAdmin(ctx context.Context, req *pb.ProposeResponsibilityResolutionForAdminRequest) (*pb.ProposeResponsibilityResolutionForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.responsibility.ProposeResponsibilityResolutionForAdmin(ctx, req)
}

// ReviewResponsibilityResolutionForAdmin A-5 checker：覆核提案（APPROVE／REJECT）。
func (c *PaymentClient) ReviewResponsibilityResolutionForAdmin(ctx context.Context, req *pb.ReviewResponsibilityResolutionForAdminRequest) (*pb.ReviewResponsibilityResolutionForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.responsibility.ReviewResponsibilityResolutionForAdmin(ctx, req)
}
