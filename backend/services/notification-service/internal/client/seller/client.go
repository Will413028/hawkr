package seller

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/notification-service/internal/config"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
	"google.golang.org/grpc"
)

type Client struct {
	client pb.SellerServiceClient
}

func NewClient(cfg *config.Config) (usecase.SellerClient, error) {
	conn, err := grpcclient.Dial(cfg.SellerServiceGRPCAddr,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("seller grpc.NewClient: %w", err)
	}
	return &Client{client: pb.NewSellerServiceClient(conn)}, nil
}

// ListIDsByAccount returns every store_id ever assigned to accountID
// (including closed stints), used solely by the chat-notification PII-purge
// path (buyer.deleted/seller.deleted/account.deleted). Mirrors
// im-service/internal/client/seller.Client.ListIDsByAccount exactly (same
// RPC). Unlike buyer-service's equivalent, seller.proto's StoreIds field is
// a native repeated int64 — no string parsing needed.
func (c *Client) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.ListStoreIDsByAccount(ctx, &pb.ListStoreIDsByAccountRequest{AccountId: accountID})
	if err != nil {
		return nil, fmt.Errorf("list store ids by account: %w", err)
	}
	return resp.GetStoreIds(), nil
}

// EnsureInvite proxies EnsureVendorPortalInvite (#2539 E1). seller-service owns
// seller_prospects.public_email and resolves it server-side, returning it as
// invited_email so this consumer can address the outbound mail. Status codes
// are passed through unwrapped so callers can classify via status.Code
// (codes.AlreadyExists / codes.FailedPrecondition) and ErrorInfo.reason.
func (c *Client) EnsureInvite(ctx context.Context, prospectID, rfqRecipientID, expiresAt string) (inviteID, secret, invitedEmail string, err error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.EnsureVendorPortalInvite(ctx, &pb.EnsureVendorPortalInviteRequest{
		ProspectId:     prospectID,
		RfqRecipientId: rfqRecipientID,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		return "", "", "", err
	}
	return resp.GetInviteId(), resp.GetSecret(), resp.GetInvitedEmail(), nil
}

// MarkInviteSecretDelivered proxies MarkVendorPortalInviteSecretDelivered.
// Called after a successful Send, with the secret that was sent, so a later
// EnsureInvite for the same recipient returns AlreadyExists instead of rotating
// and resending. A secret that a newer EnsureInvite already replaced comes back
// as codes.Aborted with ErrorInfo.reason INVITE_SECRET_SUPERSEDED (unwrapped).
func (c *Client) MarkInviteSecretDelivered(ctx context.Context, inviteID, secret string) error {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	_, err := c.client.MarkVendorPortalInviteSecretDelivered(ctx,
		&pb.MarkVendorPortalInviteSecretDeliveredRequest{InviteId: inviteID, Secret: secret})
	return err
}
