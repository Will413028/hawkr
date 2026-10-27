//go:build integration

package sellerserver

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/agent"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

// inviteProspects 是 seller 的 ProspectContact port 的替身。
type inviteProspects map[int64]seller.ProspectContact

func (p inviteProspects) ProspectContact(_ context.Context, id int64) (seller.ProspectContact, error) {
	c, ok := p[id]
	if !ok {
		return seller.ProspectContact{}, apperr.NotFound("業者 %d 不存在", id)
	}
	return c, nil
}

// 邀請的兩支 RPC 讀寫真的 agent 模組：Ensure 建立、Mark 以回傳的 secret 標記、再 Ensure 回 AlreadyExists、過時的 secret 回
// Aborted 與 INVITE_SECRET_SUPERSEDED、不能寄的 prospect 各回自己的 reason。
func TestVendorPortalInviteRPCsUseTheAgentModule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Migrations(migrate.Module{Name: "agent", Dir: agent.Migrations()}))
	email := "vendor@example.com"
	prospects := inviteProspects{
		101: {ProspectID: 101, DisplayName: "業者", Email: &email, Status: seller.ProspectStatusActive},
		102: {ProspectID: 102, DisplayName: "封存", Email: &email, Status: seller.ProspectStatusArchived},
		103: {ProspectID: 103, DisplayName: "沒有 email", Status: seller.ProspectStatusActive},
	}
	now := time.Date(2026, 10, 5, 4, 0, 0, 0, time.UTC)
	module := agent.New(agent.Deps{DB: d, Prospects: prospects, Now: func() time.Time { return now }})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(&seller.Module{}, fakeRatings{}, fakeFollowers{}, module, log)
	expires := now.Add(72 * time.Hour).Format(time.RFC3339)

	ensure := func(prospect, recipient string) (*pb.EnsureVendorPortalInviteResponse, error) {
		r, err := s.EnsureVendorPortalInvite(ctx, connect.NewRequest(&pb.EnsureVendorPortalInviteRequest{ProspectId: prospect, RfqRecipientId: recipient, ExpiresAt: expires}))
		if err != nil {
			return nil, err
		}
		return r.Msg, nil
	}
	mark := func(inviteID, secret string) error {
		_, err := s.MarkVendorPortalInviteSecretDelivered(ctx, connect.NewRequest(&pb.MarkVendorPortalInviteSecretDeliveredRequest{InviteId: inviteID, Secret: secret}))
		return err
	}
	wantFailure := func(t *testing.T, err error, code connect.Code, reason string) {
		t.Helper()
		if err == nil || connect.CodeOf(err) != code {
			t.Fatalf("err = %v，預期 %v", err, code)
		}
		got, _ := reasonOf(t, err)
		if got != reason {
			t.Fatalf("reason = %q，預期 %q", got, reason)
		}
	}

	t.Run("Ensure 建立、Mark 標記、再 Ensure 回 AlreadyExists", func(t *testing.T) {
		first, err := ensure("101", "201")
		if err != nil {
			t.Fatal(err)
		}
		if _, perr := uuid.Parse(first.GetInviteId()); perr != nil || first.GetSecret() == "" || first.GetInvitedEmail() != email {
			t.Fatalf("response = %v", first)
		}
		if err := mark(first.GetInviteId(), first.GetSecret()); err != nil {
			t.Fatal(err)
		}
		if err := mark(first.GetInviteId(), first.GetSecret()); err != nil {
			t.Fatalf("同一把 secret 重複標記應冪等：%v", err)
		}
		_, err = ensure("101", "201")
		wantFailure(t, err, connect.CodeAlreadyExists, "")
	})

	t.Run("再 Ensure 換 secret 之後，過時的 Mark 回 Aborted 與 superseded", func(t *testing.T) {
		first, err := ensure("101", "202")
		if err != nil {
			t.Fatal(err)
		}
		second, err := ensure("101", "202")
		if err != nil {
			t.Fatal(err)
		}
		if first.GetInviteId() != second.GetInviteId() || first.GetSecret() == second.GetSecret() {
			t.Fatalf("同一張邀請應換 secret：%v %v", first, second)
		}
		wantFailure(t, mark(first.GetInviteId(), first.GetSecret()), connect.CodeAborted, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_INVITE_SECRET_SUPERSEDED.String())
		if err := mark(second.GetInviteId(), second.GetSecret()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("不能寄的 prospect 各回自己的 reason", func(t *testing.T) {
		_, err := ensure("102", "203")
		wantFailure(t, err, connect.CodeFailedPrecondition, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_PROSPECT_NOT_ACTIVE.String())
		_, err = ensure("103", "204")
		wantFailure(t, err, connect.CodeFailedPrecondition, pb.VendorPortalInviteErrorReason_VENDOR_PORTAL_INVITE_ERROR_REASON_NO_DELIVERABLE_EMAIL.String())
	})

	t.Run("不存在的業者與邀請回 NotFound", func(t *testing.T) {
		_, err := ensure("999", "205")
		wantFailure(t, err, connect.CodeNotFound, "")
		issued, err := ensure("101", "206")
		if err != nil {
			t.Fatal(err)
		}
		wantFailure(t, mark(uuid.New().String(), issued.GetSecret()), connect.CodeNotFound, "")
	})
}
