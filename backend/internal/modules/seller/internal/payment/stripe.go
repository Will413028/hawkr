package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/stripe"
)

// StripeName 是 Stripe adapter 的 provider 代碼。
const StripeName = "stripe"

// Stripe 是 Provider 的 Stripe 實作（Accounts v2）。收件方式決定 dashboard 與開通介面：provider 收件用 express 加託管的 Account Link，平台收件
// 用 none 加內嵌的 Account Session（計畫 D2、D7，spike Q9）。
type Stripe struct {
	client     *stripe.Client
	collection model.RequirementCollection
}

// NewStripe 建立 Stripe adapter；collection 只接受 CollectionProvider 與 CollectionPlatform。
func NewStripe(client *stripe.Client, collection model.RequirementCollection) (*Stripe, error) {
	if client == nil {
		return nil, errors.New("payment：Stripe client 必填")
	}
	if collection != model.CollectionProvider && collection != model.CollectionPlatform {
		return nil, fmt.Errorf("payment：Stripe 不支援收件方式 %q", collection)
	}
	return &Stripe{client: client, collection: collection}, nil
}

// Name 實作 Provider。
func (s *Stripe) Name() string { return StripeName }

// Capabilities 實作 Provider。
func (s *Stripe) Capabilities() Capabilities {
	if s.collection == model.CollectionPlatform {
		return Capabilities{RequirementCollection: model.CollectionPlatform, OnboardingSurface: model.SurfaceEmbedded}
	}
	return Capabilities{RequirementCollection: model.CollectionProvider, OnboardingSurface: model.SurfaceHosted}
}

// EnsureAccount 實作 Provider。
func (s *Stripe) EnsureAccount(ctx context.Context, in EnsureInput) (string, error) {
	if in.IdempotencyKey == "" {
		return "", fmt.Errorf("%w：缺少冪等鍵", ErrPermanent)
	}
	dashboard := stripe.DashboardExpress
	if s.collection == model.CollectionPlatform {
		dashboard = stripe.DashboardNone
	}
	id, err := s.client.CreateAccount(ctx, in.IdempotencyKey, dashboard)
	return id, classify(err)
}

// CreateOnboardingSession 實作 Provider。
func (s *Stripe) CreateOnboardingSession(ctx context.Context, in SessionInput) (model.OnboardingSession, error) {
	if in.ExternalAccountID == "" {
		return model.OnboardingSession{}, fmt.Errorf("%w：缺少外部帳戶 ID", ErrPermanent)
	}
	if s.collection == model.CollectionPlatform {
		sess, err := s.client.CreateAccountSession(ctx, in.ExternalAccountID)
		if err != nil {
			return model.OnboardingSession{}, classify(err)
		}
		return model.OnboardingSession{Surface: model.SurfaceEmbedded, ClientSecret: sess.ClientSecret, ExpiresAt: sess.ExpiresAt}, nil
	}
	link, err := s.client.CreateAccountLink(ctx, in.ExternalAccountID, in.ReturnURL, in.RefreshURL)
	if err != nil {
		return model.OnboardingSession{}, classify(err)
	}
	return model.OnboardingSession{Surface: model.SurfaceHosted, URL: link.URL, ExpiresAt: link.ExpiresAt}, nil
}

// FetchAccount 實作 Provider：只取轉帳能力（狀態與細節）、撥款能力是否 active 與 requirements 原始 JSON，其餘欄位丟棄。
func (s *Stripe) FetchAccount(ctx context.Context, externalID string) (Snapshot, error) {
	if externalID == "" {
		return Snapshot{}, fmt.Errorf("%w：缺少外部帳戶 ID", ErrPermanent)
	}
	acct, err := s.client.RetrieveAccount(ctx, externalID)
	if err != nil {
		return Snapshot{}, classify(err)
	}
	snap := Snapshot{
		Transfers:     capabilityState(acct.Transfers),
		PayoutsActive: capabilityState(acct.Payouts) == CapabilityActive,
		Requirements:  requirementsObject(acct.Requirements),
	}
	for _, d := range acct.Transfers.Details {
		snap.Details = append(snap.Details, StatusDetail{Resolution: resolutionOf(d.Resolution), Reason: d.Code})
	}
	return snap, nil
}

// ParseEvent 實作 Provider。
func (s *Stripe) ParseEvent(header http.Header, body []byte) (Event, error) {
	ev, err := s.client.ParseEvent(header, body)
	switch {
	case errors.Is(err, stripe.ErrNotHandled):
		return Event{}, errors.Join(ErrIgnoredEvent, err)
	case errors.Is(err, stripe.ErrSignature), errors.Is(err, stripe.ErrMalformed):
		return Event{}, errors.Join(ErrInvalidEvent, err)
	case err != nil:
		return Event{}, err
	}
	return Event{ID: ev.ID, ExternalAccountID: ev.AccountID}, nil
}

func classify(err error) error {
	var api *stripe.APIError
	if errors.As(err, &api) && api.Permanent() {
		return fmt.Errorf("%w：%w", ErrPermanent, err)
	}
	return err
}

func capabilityState(c stripe.Capability) CapabilityState {
	if s := CapabilityState(c.Status); slices.Contains([]CapabilityState{CapabilityActive, CapabilityPending, CapabilityRestricted, CapabilityRejected, CapabilityUnsupported}, s) {
		return s
	}
	return CapabilityUnknown
}

func resolutionOf(r string) Resolution {
	switch r {
	case "provide_info":
		return ResolutionSellerAction
	case "contact_stripe":
		return ResolutionPlatformAction
	case "no_resolution":
		return ResolutionNone
	}
	return ResolutionUnknown
}

func requirementsObject(raw json.RawMessage) json.RawMessage {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return json.RawMessage(`{}`)
	}
	return raw
}
