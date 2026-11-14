package account

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// callCounter 數 Stripe 兩種開通呼叫（Account Session 與 Account Link）各被打了幾次。
type callCounter struct {
	mu                    sync.Mutex
	sessions, accountLink int
}

func (c *callCounter) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.mu.Lock()
		defer c.mu.Unlock()
		if r.URL.Path == "/v1/account_sessions" {
			c.sessions++
			fixtureResponder(t, "account_session_platform")(w, r)
			return
		}
		c.accountLink++
		_, _ = w.Write([]byte(`{"object":"v2.core.account_link","url":"https://connect.stripe.com/x","expires_at":"2026-11-04T01:02:03Z"}`))
	}
}

// 經 Service 的開通入口（只差讀列）：平台收件的 Stripe 帳戶，呼叫端不宣告內嵌就只建 Account Link，不建 Account Session；宣告了才建 Account Session。
func TestOpenSessionNeverCreatesAnAccountSessionForACallerThatDoesNotDeclareEmbedding(t *testing.T) {
	ext := "acct_1"
	row := store.PaymentAccount{StoreID: 1, Provider: StripeName, ExternalAccountID: &ext, RequirementCollection: string(model.CollectionPlatform), Status: string(model.PaymentActionRequired)}
	for name, tc := range map[string]struct {
		runtimes                []model.EmbeddedRuntime
		wantSessions, wantLinks int
		wantEmbedded            bool
	}{
		"空清單":           {nil, 0, 1, false},
		"只有不認得的值":       {[]model.EmbeddedRuntime{"UNKNOWN"}, 0, 1, false},
		"宣告 Connect.js": {[]model.EmbeddedRuntime{model.RuntimeStripeConnectJS}, 1, 0, true},
	} {
		var calls callCounter
		svc := &Service{d: Deps{Provider: stripeProvider(t, model.CollectionPlatform, calls.handler(t))}}
		got, err := svc.openSession(context.Background(), row, "https://a.test/r", "https://a.test/f", tc.runtimes)
		if err != nil {
			t.Fatalf("%s：%v", name, err)
		}
		_, embedded := got.(model.StripeEmbeddedSession)
		if embedded != tc.wantEmbedded || calls.sessions != tc.wantSessions || calls.accountLink != tc.wantLinks {
			t.Errorf("%s：session=%T Account Session %d 次（要 %d）、Account Link %d 次（要 %d）", name, got, calls.sessions, tc.wantSessions, calls.accountLink, tc.wantLinks)
		}
	}
}

// 形狀只看帳戶列保存的收件方式，不看 provider 目前的設定：adapter 設定成金流商收件，平台收件的列仍開內嵌 session。
func TestOpenSessionReadsTheStoredCollectionNotTheCurrentConfiguration(t *testing.T) {
	ext := "acct_1"
	row := store.PaymentAccount{StoreID: 1, Provider: StripeName, ExternalAccountID: &ext, RequirementCollection: string(model.CollectionPlatform), Status: string(model.PaymentActionRequired)}
	var calls callCounter
	svc := &Service{d: Deps{Provider: stripeProvider(t, model.CollectionProvider, calls.handler(t))}}
	got, err := svc.openSession(context.Background(), row, "https://a.test/r", "https://a.test/f", []model.EmbeddedRuntime{model.RuntimeStripeConnectJS})
	if _, ok := got.(model.StripeEmbeddedSession); err != nil || !ok || calls.sessions != 1 {
		t.Fatalf("平台收件的列：%T %v session=%d", got, err, calls.sessions)
	}
	row.RequirementCollection = string(model.CollectionProvider)
	svc = &Service{d: Deps{Provider: stripeProvider(t, model.CollectionPlatform, calls.handler(t))}}
	got, err = svc.openSession(context.Background(), row, "https://a.test/r", "https://a.test/f", []model.EmbeddedRuntime{model.RuntimeStripeConnectJS})
	if _, ok := got.(model.HostedSession); err != nil || !ok || calls.accountLink != 1 {
		t.Fatalf("金流商收件的列：%T %v link=%d", got, err, calls.accountLink)
	}
}

// shapeProvider 照 shape 回 session，不管 in.Kind：模擬回傳形狀與要求不一致的 provider。
type shapeProvider struct {
	*Fake
	shape model.OnboardingSession
}

func (p shapeProvider) Capabilities() Capabilities {
	return Capabilities{RequirementCollection: model.CollectionPlatform, EmbeddedRuntimes: []model.EmbeddedRuntime{model.RuntimeStripeConnectJS}}
}

func (p shapeProvider) CreateOnboardingSession(context.Context, SessionInput) (model.OnboardingSession, error) {
	return p.shape, nil
}

// provider 回來的形狀與要求的 Kind 不一致是伺服器的錯：不把 client_secret 交給沒有宣告內嵌的呼叫端。
func TestOpenSessionRejectsAShapeThatDiffersFromTheRequestedKind(t *testing.T) {
	ext := "acct_1"
	fake := NewFake()
	row := store.PaymentAccount{StoreID: 1, Provider: fake.Name(), ExternalAccountID: &ext, RequirementCollection: string(model.CollectionPlatform), Status: string(model.PaymentActionRequired)}
	embedded := model.StripeEmbeddedSession{ClientSecret: "cs_secret", PublishableKey: "pk"}
	for name, tc := range map[string]struct {
		shape    model.OnboardingSession
		declared []model.EmbeddedRuntime
	}{
		"要託管、回內嵌": {embedded, nil},
		"要內嵌、回託管": {model.HostedSession{URL: "https://x.test"}, []model.EmbeddedRuntime{model.RuntimeStripeConnectJS}},
	} {
		svc := &Service{d: Deps{Provider: shapeProvider{Fake: fake, shape: tc.shape}}}
		got, err := svc.openSession(context.Background(), row, "https://a.test/r", "https://a.test/f", tc.declared)
		if err == nil || got != nil || apperr.KindOf(err) != apperr.KindInternal {
			t.Errorf("%s：got=%v err=%v", name, got, err)
		}
	}
}

// Fake 不支援內嵌：被要求內嵌時回錯，不假裝成託管連結。
func TestFakeRefusesAnEmbeddedSession(t *testing.T) {
	if _, err := NewFake().CreateOnboardingSession(context.Background(), SessionInput{ExternalAccountID: "acct_1", Kind: SessionEmbedded}); !errors.Is(err, ErrPermanent) {
		t.Fatalf("err = %v", err)
	}
}
