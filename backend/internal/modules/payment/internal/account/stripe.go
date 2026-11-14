package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/stripe"
)

// StripeName 是 Stripe adapter 的 provider 代碼。
const StripeName = "stripe"

// Stripe 是 Provider 的 Stripe 實作（Accounts v2）。收件方式決定外部帳戶的 dashboard：provider 收件用 express，平台收件
// 用 none（計畫 D2、D7，spike Q9）；兩種都可以開託管的 Account Link，平台收件的帳戶在呼叫端宣告內嵌時另可開內嵌的 Account Session。建立外部帳戶的收件方式取自帳戶列保存的值（EnsureInput 的 Collection），
// 開通 session 的形狀由 ChooseSession 決定（SessionInput 的 Kind），行程設定的 collection 只決定新列寫入的值（Capabilities）。
type Stripe struct {
	client         *stripe.Client
	collection     model.RequirementCollection
	publishableKey string
}

// NewStripe 建立 Stripe adapter；collection 只接受 CollectionProvider 與 CollectionPlatform。publishableKey 是內嵌元件初始化用的公開金鑰，
// 一律必填：行程設定的收件方式可以和既有帳戶列保存的不同，既有的平台收件帳戶仍會開內嵌 session。
func NewStripe(client *stripe.Client, collection model.RequirementCollection, publishableKey string) (*Stripe, error) {
	if client == nil {
		return nil, errors.New("payment：Stripe client 必填")
	}
	if collection != model.CollectionProvider && collection != model.CollectionPlatform {
		return nil, fmt.Errorf("payment：Stripe 不支援收件方式 %q", collection)
	}
	if publishableKey == "" {
		return nil, errors.New("payment：Stripe 需要公開金鑰")
	}
	return &Stripe{client: client, collection: collection, publishableKey: publishableKey}, nil
}

// Name 實作 Provider。
func (s *Stripe) Name() string { return StripeName }

// Capabilities 實作 Provider。
func (s *Stripe) Capabilities() Capabilities {
	embedded := []model.EmbeddedRuntime{model.RuntimeStripeConnectJS}
	if s.collection == model.CollectionPlatform {
		return Capabilities{RequirementCollection: model.CollectionPlatform, EmbeddedRuntimes: embedded}
	}
	return Capabilities{RequirementCollection: model.CollectionProvider, EmbeddedRuntimes: embedded}
}

// AcquireAccount 實作 Provider：建立的第一步（本文固定，同一把鍵 30 天內回同一個帳戶）。
func (s *Stripe) AcquireAccount(ctx context.Context, in AcquireInput) (string, error) {
	if in.IdempotencyKey == "" {
		return "", fmt.Errorf("%w：缺少冪等鍵", ErrPermanent)
	}
	id, err := s.client.CreateAccountShell(ctx, in.IdempotencyKey, in.StoreID)
	return id, classify(err)
}

// ConfigureAccount 實作 Provider：建立的第二步，收件方式決定 dashboard。
func (s *Stripe) ConfigureAccount(ctx context.Context, in ConfigureInput) error {
	if in.ExternalID == "" {
		return fmt.Errorf("%w：缺少外部帳戶 ID", ErrPermanent)
	}
	var dashboard stripe.Dashboard
	switch in.Collection {
	case model.CollectionProvider:
		dashboard = stripe.DashboardExpress
	case model.CollectionPlatform:
		dashboard = stripe.DashboardNone
	default:
		return fmt.Errorf("%w：不支援的收件方式 %q", ErrPermanent, in.Collection)
	}
	if in.ContactEmail == "" {
		return fmt.Errorf("%w：缺少聯絡信箱", ErrPermanent)
	}
	return classify(s.client.UpdateAccount(ctx, in.ExternalID, dashboard, in.ContactEmail))
}

// ResolveAccount 實作 Provider：列出沒有關閉的帳戶，以 metadata.store_id 比對（第一步已經寫入）。同一家店有多個符合時（不該發生）回第一個，
// 其餘留在金流商那邊；沒有符合的才回 found 為 false。
func (s *Stripe) ResolveAccount(ctx context.Context, in ResolveInput) (string, bool, error) {
	ids, err := s.client.ListAccountIDsByStore(ctx, in.StoreID)
	if err != nil {
		return "", false, classify(err)
	}
	if len(ids) == 0 {
		return "", false, nil
	}
	return ids[0], true, nil
}

// CreateOnboardingSession 實作 Provider。
func (s *Stripe) CreateOnboardingSession(ctx context.Context, in SessionInput) (model.OnboardingSession, error) {
	if in.ExternalAccountID == "" {
		return nil, fmt.Errorf("%w：缺少外部帳戶 ID", ErrPermanent)
	}
	switch in.Kind {
	case SessionEmbedded:
		sess, err := s.client.CreateAccountSession(ctx, in.ExternalAccountID)
		if err != nil {
			return nil, classify(err)
		}
		return model.StripeEmbeddedSession{ClientSecret: sess.ClientSecret, PublishableKey: s.publishableKey}, nil
	case SessionHosted:
		link, err := s.client.CreateAccountLink(ctx, in.ExternalAccountID, in.ReturnURL, in.RefreshURL)
		if err != nil {
			return nil, classify(err)
		}
		return model.HostedSession{URL: link.URL}, nil
	}
	return nil, fmt.Errorf("%w：不支援的 session 形狀 %d", ErrPermanent, in.Kind)
}

// FetchAccount 實作 Provider：取轉帳能力（狀態與細節）、撥款能力是否 active、requirements 原始 JSON 與外部銀行帳號的 fingerprint，其餘欄位丟棄；
// 讀 fingerprint 失敗整個呼叫失敗（job 重試）。
func (s *Stripe) FetchAccount(ctx context.Context, externalID string) (Snapshot, error) {
	if externalID == "" {
		return Snapshot{}, fmt.Errorf("%w：缺少外部帳戶 ID", ErrPermanent)
	}
	acct, err := s.client.RetrieveAccount(ctx, externalID)
	if err != nil {
		return Snapshot{}, classify(err)
	}
	fingerprints, err := s.client.ListBankAccountFingerprints(ctx, externalID)
	if err != nil {
		return Snapshot{}, classify(err)
	}
	snap := Snapshot{
		BankFingerprints: fingerprints,
		Transfers:        capabilityState(acct.Transfers),
		PayoutsActive:    capabilityState(acct.Payouts) == CapabilityActive,
		Requirements:     requirementsObject(acct.Requirements),
	}
	for _, d := range acct.Transfers.Details {
		snap.Details = append(snap.Details, StatusDetail{Resolution: resolutionOf(d.Resolution), Reason: d.Code})
	}
	return snap, nil
}

// CloseAccount 實作 Provider。
func (s *Stripe) CloseAccount(ctx context.Context, externalID string) error {
	if externalID == "" {
		return fmt.Errorf("%w：缺少外部帳戶 ID", ErrPermanent)
	}
	return classify(s.client.CloseAccount(ctx, externalID))
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
	if s := CapabilityState(c.Status); s.Valid() {
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
