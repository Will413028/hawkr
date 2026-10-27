package buyer

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"google.golang.org/grpc"
)

// Entitlement is ai-service's view of a buyer's current membership plan and
// monthly answer quota, as reported by buyer-service's GetBuyerEntitlement RPC.
type Entitlement struct {
	PlanCode           string
	MonthlyAnswerLimit int
	PeriodStart        time.Time
	PeriodEnd          time.Time
	Source             string
}

// Client is ai-service's gRPC client to buyer-service's membership/quota RPCs.
type Client interface {
	GetBuyerEntitlement(ctx context.Context, buyerID identity.BuyerID) (Entitlement, error)
}

type client struct {
	client pb.BuyerServiceClient
}

// NewClient creates a gRPC client to buyer-service.
func NewClient(cfg *config.Config) (Client, error) {
	target := fmt.Sprintf("%s:%s", cfg.BuyerServiceHost, cfg.BuyerServicePort)
	conn, err := grpcclient.Dial(target,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
		// 內部身分 HMAC 簽章（#1366）；此 RPC 目前不需要轉發呼叫端 buyer_id 作 outgoing
		// x-account-id（buyerID 已直接作為 request 欄位傳遞），故 forwardAccountHeaders=false。
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("ai-service", false)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to buyer-service at %s: %w", target, err)
	}
	return &client{client: pb.NewBuyerServiceClient(conn)}, nil
}

// GetBuyerEntitlement queries a buyer's current membership plan and monthly
// answer quota. buyer-service reports business errors (invalid buyer_id, not
// found, etc.) via a 200-shaped response with a non-zero Code and no other
// fields populated — never via the gRPC error channel — so Code must be
// checked explicitly (mirrors internal/client/order/adapter.go's handling of
// order-service's identical response contract).
func (c *client) GetBuyerEntitlement(ctx context.Context, buyerID identity.BuyerID) (Entitlement, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.GetBuyerEntitlement(ctx, &pb.GetBuyerEntitlementRequest{
		BuyerId: strconv.FormatInt(int64(buyerID), 10),
	})
	if err != nil {
		return Entitlement{}, err
	}
	if resp.GetCode() != 0 {
		return Entitlement{}, fmt.Errorf("get buyer entitlement failed: [%d] %s", resp.GetCode(), resp.GetMessage())
	}

	limit, err := strconv.Atoi(resp.GetMonthlyAnswerLimit())
	if err != nil {
		return Entitlement{}, fmt.Errorf("buyer-service returned non-numeric monthly_answer_limit %q: %w", resp.GetMonthlyAnswerLimit(), err)
	}
	start, err := time.Parse(time.RFC3339, resp.GetPeriodStart())
	if err != nil {
		return Entitlement{}, fmt.Errorf("buyer-service returned invalid period_start %q: %w", resp.GetPeriodStart(), err)
	}
	end, err := time.Parse(time.RFC3339, resp.GetPeriodEnd())
	if err != nil {
		return Entitlement{}, fmt.Errorf("buyer-service returned invalid period_end %q: %w", resp.GetPeriodEnd(), err)
	}

	return Entitlement{
		PlanCode: resp.GetPlanCode(), MonthlyAnswerLimit: limit,
		PeriodStart: start.UTC(), PeriodEnd: end.UTC(), Source: resp.GetSource(),
	}, nil
}
