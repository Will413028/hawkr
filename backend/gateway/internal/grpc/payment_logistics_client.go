package grpc

import (
	"context"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/payment"
)

// ==================== 物流財務（線 B，LogisticsFinanceService）====================
//
// 🔴 這些 RPC 走 **payment-service 的同一條 gRPC channel**：LogisticsFinanceService 與
// PaymentService 是同一個 .proto、同一個 Java 服務、同一個 port。獨立 service name 只是
// 為了讓授權規則分得開（見 payment.proto 的 service 註解），不是另一個部署單元。
//
// ⚠️ 方法掛在 *PaymentClient 上而不是另開一個 client struct：另開會多撥一條連線去打同一個
// host:port，而那條連線的健康狀態、斷路器與既有那條各算各的——同一個下游有兩套熔斷狀態，
// 是一個沒有人會想到要去查的不一致來源。
//
// ⚠️ 每支都自帶 deadline（CLAUDE.md §12：禁止無 deadline 的跨服務呼叫）。這裡全部是唯讀
// 聚合查詢，給 15s；帳單詳情會展開全部明細列，給 30s。

func (c *PaymentClient) logisticsFinance() pb.LogisticsFinanceServiceClient {
	return pb.NewLogisticsFinanceServiceClient(c.conn)
}

// GetSellerLogisticsFinanceSummary 賣家物流財務總覽（十一個維度）。
func (c *PaymentClient) GetSellerLogisticsFinanceSummary(ctx context.Context, req *pb.GetSellerLogisticsFinanceSummaryRequest) (*pb.GetSellerLogisticsFinanceSummaryResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.logisticsFinance().GetSellerLogisticsFinanceSummary(ctx, req)
}

// ListSellerLogisticsReceivables 賣家自己的應收明細。
func (c *PaymentClient) ListSellerLogisticsReceivables(ctx context.Context, req *pb.ListSellerLogisticsReceivablesRequest) (*pb.ListSellerLogisticsReceivablesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.logisticsFinance().ListSellerLogisticsReceivables(ctx, req)
}

// ListSellerLogisticsLedgerEntries 賣家自己的物流帳本流水。
func (c *PaymentClient) ListSellerLogisticsLedgerEntries(ctx context.Context, req *pb.ListSellerLogisticsLedgerEntriesRequest) (*pb.ListSellerLogisticsLedgerEntriesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.logisticsFinance().ListSellerLogisticsLedgerEntries(ctx, req)
}

// ListLogisticsReceivablesForAdmin Admin 應收列表。
func (c *PaymentClient) ListLogisticsReceivablesForAdmin(ctx context.Context, req *pb.ListLogisticsReceivablesForAdminRequest) (*pb.ListLogisticsReceivablesForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.logisticsFinance().ListLogisticsReceivablesForAdmin(ctx, req)
}

// GetLogisticsReceivableForAdmin Admin 應收詳情。
func (c *PaymentClient) GetLogisticsReceivableForAdmin(ctx context.Context, req *pb.GetLogisticsReceivableForAdminRequest) (*pb.GetLogisticsReceivableForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.logisticsFinance().GetLogisticsReceivableForAdmin(ctx, req)
}

// ListLogisticsCollectionsForAdmin Admin PayUni 收款嘗試列表。
func (c *PaymentClient) ListLogisticsCollectionsForAdmin(ctx context.Context, req *pb.ListLogisticsCollectionsForAdminRequest) (*pb.ListLogisticsCollectionsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.logisticsFinance().ListLogisticsCollectionsForAdmin(ctx, req)
}

// ListLogisticsAdjustmentsForAdmin Admin 責任／對帳調整單列表。
func (c *PaymentClient) ListLogisticsAdjustmentsForAdmin(ctx context.Context, req *pb.ListLogisticsAdjustmentsForAdminRequest) (*pb.ListLogisticsAdjustmentsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.logisticsFinance().ListLogisticsAdjustmentsForAdmin(ctx, req)
}

// ListProviderStatementsForAdmin Admin provider 帳單列表。
func (c *PaymentClient) ListProviderStatementsForAdmin(ctx context.Context, req *pb.ListProviderStatementsForAdminRequest) (*pb.ListProviderStatementsForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.logisticsFinance().ListProviderStatementsForAdmin(ctx, req)
}

// GetProviderStatementForAdmin Admin 單份帳單詳情（展開全部明細列）。
func (c *PaymentClient) GetProviderStatementForAdmin(ctx context.Context, req *pb.GetProviderStatementForAdminRequest) (*pb.GetProviderStatementForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.logisticsFinance().GetProviderStatementForAdmin(ctx, req)
}

// ListFulfillmentExceptionCasesForAdmin Admin 維運例外案件佇列。
func (c *PaymentClient) ListFulfillmentExceptionCasesForAdmin(ctx context.Context, req *pb.ListFulfillmentExceptionCasesForAdminRequest) (*pb.ListFulfillmentExceptionCasesForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.logisticsFinance().ListFulfillmentExceptionCasesForAdmin(ctx, req)
}

// GetFulfillmentExceptionCaseForAdmin Admin 單一例外案件詳情（65 §6.1.1 A-2）。
//
// 15s 而非 30s：它展開的是單一案件的提案列（個位數），屬檔頭說的「唯讀聚合查詢」，
// 不是帳單詳情那種會攤開全部明細列的查詢。
func (c *PaymentClient) GetFulfillmentExceptionCaseForAdmin(ctx context.Context, req *pb.GetFulfillmentExceptionCaseForAdminRequest) (*pb.GetFulfillmentExceptionCaseForAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.logisticsFinance().GetFulfillmentExceptionCaseForAdmin(ctx, req)
}
