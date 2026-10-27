package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type stubInviteViewRepo struct {
	view repository.InviteSellerView
	err  error
}

func (s *stubInviteViewRepo) GetPublicationInviteSellerView(ctx context.Context, inviteID int64) (repository.InviteSellerView, error) {
	if s.err != nil {
		return repository.InviteSellerView{}, s.err
	}
	return s.view, nil
}

func TestGetRfqPublicationInviteSellerView_ReturnsWhitelistedFieldsOnly(t *testing.T) {
	h := newInviteSellerViewHandler(&stubInviteViewRepo{
		view: repository.InviteSellerView{
			Item: "台灣蘋果", ItemLabel: "蘋果", Quantity: "50", Unit: "BOX",
			Packaging: "紙箱", DeliveryCity: "桃園市", DeliveryDate: "2026-09-16",
			ResponseDeadline: "2026-09-14", QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
		},
	})

	got, err := h.GetRfqPublicationInviteSellerView(context.Background(),
		&pb.GetRfqPublicationInviteSellerViewRequest{InviteId: "123"})
	require.NoError(t, err)
	require.Equal(t, "台灣蘋果", got.Item)
	require.Equal(t, "桃園市", got.DeliveryCity)
	require.Equal(t, "2026-09-14", got.ResponseDeadline)
	require.Equal(t, "BUYER_QUALITY_UNKNOWN", got.QualityKnowledgeState)
}

// 舊 approval 讀回空字串，RPC 必須原樣傳遞，不得補成 BUYER_QUALITY_UNKNOWN。
func TestGetRfqPublicationInviteSellerView_KeepsLegacyEmptyQualityState(t *testing.T) {
	h := newInviteSellerViewHandler(&stubInviteViewRepo{
		view: repository.InviteSellerView{Item: "台灣蘋果", DeliveryCity: "桃園市"},
	})

	got, err := h.GetRfqPublicationInviteSellerView(context.Background(),
		&pb.GetRfqPublicationInviteSellerViewRequest{InviteId: "123"})
	require.NoError(t, err)
	require.Empty(t, got.QualityKnowledgeState)
}

func TestGetRfqPublicationInviteSellerView_UnknownInviteIsNotFound(t *testing.T) {
	h := newInviteSellerViewHandler(&stubInviteViewRepo{err: repository.ErrInviteNotFound})
	_, err := h.GetRfqPublicationInviteSellerView(context.Background(),
		&pb.GetRfqPublicationInviteSellerViewRequest{InviteId: "999"})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetRfqPublicationInviteSellerView_RejectsNonNumericID(t *testing.T) {
	h := newInviteSellerViewHandler(&stubInviteViewRepo{})
	_, err := h.GetRfqPublicationInviteSellerView(context.Background(),
		&pb.GetRfqPublicationInviteSellerViewRequest{InviteId: "abc"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestGetRfqPublicationInviteSellerView_ReachableOnRegisteredServer 證明這支 RPC
// 真的掛在對外服務的 *AIProcurementGrpcServer 上、經過真正的 method-policy
// interceptor 鏈路仍可呼叫，而不是只在 inviteSellerViewHandler 這個內部型別上
// 可呼叫。不帶任何 actor metadata：ActorNone 的登記若漏掉或方法本身沒有登記進
// methodPolicies，UnaryMethodPolicyInterceptor 會 fail-closed 回
// PermissionDenied，跟真正的業務錯誤（NotFound）分不開。
func TestGetRfqPublicationInviteSellerView_ReachableOnRegisteredServer(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	server := NewGRPCServer(&AIGrpcServer{}, &AIAssistantGrpcServer{}, zap.NewNop())
	registerAIProcurementService(server, &AIProcurementGrpcServer{
		inviteSellerView: newInviteSellerViewHandler(&stubInviteViewRepo{
			view: repository.InviteSellerView{Item: "台灣蘋果", DeliveryCity: "桃園市"},
		}),
	})
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	dialer := func(ctx context.Context, _ string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := pb.NewAIProcurementServiceClient(conn)
	resp, err := client.GetRfqPublicationInviteSellerView(ctx,
		&pb.GetRfqPublicationInviteSellerViewRequest{InviteId: "123"})
	require.NoError(t, err)
	require.Equal(t, "台灣蘋果", resp.GetItem())
	require.Equal(t, "桃園市", resp.GetDeliveryCity())
}

func TestGetRfqPublicationInviteSellerView_CarriesBuyerRequestedRange(t *testing.T) {
	h := newInviteSellerViewHandler(&stubInviteViewRepo{
		view: repository.InviteSellerView{
			Item: "甘藍-初秋",
			BuyerRequestedRange: &domain.SellerFacingRequestedRange{
				Currency: "TWD", PriceComponent: "GOODS_ONLY",
				BasisQuantity: "1", BasisUnit: "BOX",
				LowerBoundAmount: "38000", UpperBoundAmount: "43000",
			},
		},
	})

	got, err := h.GetRfqPublicationInviteSellerView(context.Background(),
		&pb.GetRfqPublicationInviteSellerViewRequest{InviteId: "123"})
	require.NoError(t, err)
	require.Equal(t, "TWD", got.GetBuyerRequestedRange().GetCurrency())
	require.Equal(t, "38000", got.GetBuyerRequestedRange().GetLowerBoundAmount())
	require.Equal(t, "43000", got.GetBuyerRequestedRange().GetUpperBoundAmount())
}

func TestGetRfqPublicationInviteSellerView_KeepsAbsentBuyerRequestedRangeNil(t *testing.T) {
	h := newInviteSellerViewHandler(&stubInviteViewRepo{
		view: repository.InviteSellerView{Item: "台灣蘋果", DeliveryCity: "桃園市"},
	})

	got, err := h.GetRfqPublicationInviteSellerView(context.Background(),
		&pb.GetRfqPublicationInviteSellerViewRequest{InviteId: "123"})
	require.NoError(t, err)
	require.Nil(t, got.GetBuyerRequestedRange())
}
