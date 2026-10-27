package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubVendorPortalRepo struct {
	ensureErr    error
	markErr      error
	markInviteID int64
	invite       domain.VendorPortalInvite
	secret       string
}

func (s *stubVendorPortalRepo) EnsureInvite(context.Context, domain.EnsureInviteInput) (domain.VendorPortalInvite, string, error) {
	return s.invite, s.secret, s.ensureErr
}
func (s *stubVendorPortalRepo) MarkSecretDelivered(_ context.Context, inviteID int64, _ time.Time) error {
	s.markInviteID = inviteID
	return s.markErr
}

func TestEnsureVendorPortalInvite_AlreadyDelivered(t *testing.T) {
	h := newVendorPortalHandler(&stubVendorPortalRepo{ensureErr: domain.ErrInviteSecretAlreadyDelivered})
	_, err := h.EnsureVendorPortalInvite(context.Background(), &pb.EnsureVendorPortalInviteRequest{
		ProspectId: "1", RfqRecipientId: "2",
		ExpiresAt: time.Now().UTC().Format(time.RFC3339),
	})
	require.Equal(t, codes.AlreadyExists, status.Code(err))
}

func TestEnsureVendorPortalInvite_ReturnsInvitedEmail(t *testing.T) {
	h := newVendorPortalHandler(&stubVendorPortalRepo{
		invite: domain.VendorPortalInvite{ID: 555, Email: "vendor@example.com"},
		secret: "abc123",
	})
	got, err := h.EnsureVendorPortalInvite(context.Background(), &pb.EnsureVendorPortalInviteRequest{
		ProspectId: "1", RfqRecipientId: "2",
		ExpiresAt: time.Now().UTC().Format(time.RFC3339),
	})
	require.NoError(t, err)
	require.Equal(t, "555", got.InviteId)
	require.Equal(t, "abc123", got.Secret)
	require.Equal(t, "vendor@example.com", got.InvitedEmail)
}

func TestMarkVendorPortalInviteSecretDelivered_Success(t *testing.T) {
	repo := &stubVendorPortalRepo{}
	h := newVendorPortalHandler(repo)
	got, err := h.MarkVendorPortalInviteSecretDelivered(context.Background(),
		&pb.MarkVendorPortalInviteSecretDeliveredRequest{InviteId: "555"})
	require.NoError(t, err)
	require.Equal(t, int32(0), got.Code)
	require.Equal(t, int64(555), repo.markInviteID)
}

func TestMarkVendorPortalInviteSecretDelivered_RepoErrorIsInternal(t *testing.T) {
	h := newVendorPortalHandler(&stubVendorPortalRepo{markErr: errors.New("db down")})
	_, err := h.MarkVendorPortalInviteSecretDelivered(context.Background(),
		&pb.MarkVendorPortalInviteSecretDeliveredRequest{InviteId: "555"})
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestEnsureVendorPortalInvite_NoEmailIsFailedPrecondition(t *testing.T) {
	h := newVendorPortalHandler(&stubVendorPortalRepo{ensureErr: domain.ErrProspectHasNoEmail})
	_, err := h.EnsureVendorPortalInvite(context.Background(), &pb.EnsureVendorPortalInviteRequest{
		ProspectId: "1", RfqRecipientId: "2",
		ExpiresAt: time.Now().UTC().Format(time.RFC3339),
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// step-up 成功時回升級後的 session，且**不換發 token**。
// 🔴 升級失敗一律 PermissionDenied 且訊息不分成因。
//
// 這一支守的是「別把成因告訴對端」：session 不可用與 email 對不上若回不同訊息，
// 拿到邀請信但不是本人的人就有了一個逐項試探的介面。
// 缺任一必填參數要在打 repository 之前就擋下。
