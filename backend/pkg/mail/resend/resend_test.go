package resend_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/mail"
	"github.com/Will413028/hawkr/go-pkg/mail/resend"
	"github.com/stretchr/testify/require"
)

func msg() mail.Message {
	return mail.Message{
		From:           mail.Address{Email: "supports@example.com", Name: "範例生鮮"},
		To:             mail.Address{Email: "vendor@example.com"},
		Subject:        "詢價邀請",
		HTML:           "<p>內文</p>",
		Text:           "內文",
		IdempotencyKey: "rfq-invite:123",
		// Two tags: sortedKeys' ordering is not exercised with only one — map
		// iteration order is random, so a single-tag test passes whether or not
		// sorting actually works.
		Tags: map[string]string{"rfq_invite_id": "123", "vendor_id": "456"},
	}
}

func TestSendSuccess(t *testing.T) {
	var gotBody map[string]any
	var gotAuth, gotIdem string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/emails", r.URL.Path)
		gotAuth = r.Header.Get("Authorization")
		gotIdem = r.Header.Get("Idempotency-Key")
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &gotBody))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"49a3999c-0ce1-4ea6-ab68-afcd6dc2e794"}`))
	}))
	defer srv.Close()

	s := resend.New("re_test_key", resend.WithBaseURL(srv.URL))
	got, err := s.Send(context.Background(), msg())
	require.NoError(t, err)
	require.Equal(t, "49a3999c-0ce1-4ea6-ab68-afcd6dc2e794", got.ProviderMessageID)
	require.False(t, got.AcceptedAt.IsZero())

	require.Equal(t, "Bearer re_test_key", gotAuth)
	require.Equal(t, "rfq-invite:123", gotIdem)
	require.Equal(t, `"範例生鮮" <supports@example.com>`, gotBody["from"])
	require.Equal(t, []any{"vendor@example.com"}, gotBody["to"])
	require.Equal(t, "詢價邀請", gotBody["subject"])
	require.Equal(t, "<p>內文</p>", gotBody["html"])
	require.Equal(t, "內文", gotBody["text"])
	// Order, not just membership: this is what actually exercises sortedKeys.
	require.Equal(t, []any{
		map[string]any{"name": "rfq_invite_id", "value": "123"},
		map[string]any{"name": "vendor_id", "value": "456"},
	}, gotBody["tags"])
}

// IdempotencyKey 空字串時不得送出空的 header——Resend 會回 invalid_idempotency_key。
func TestSendOmitsEmptyIdempotencyHeader(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Idempotency-Key"]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()

	m := msg()
	m.IdempotencyKey = ""
	_, err := resend.New("k", resend.WithBaseURL(srv.URL)).Send(context.Background(), m)
	require.NoError(t, err)
	require.False(t, present)
}

func TestSendClassifiesErrors(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		wantErr error
	}{
		{"rate limit", 429, `{"name":"rate_limit_exceeded","message":"too many"}`, mail.ErrTransient},
		{"server error", 500, `{"name":"application_error","message":"boom"}`, mail.ErrTransient},
		{"bad address", 422, `{"name":"invalid_parameter","message":"to is invalid"}`, mail.ErrPermanent},
		{"unverified domain", 403, `{"name":"validation_error","message":"domain not verified"}`, mail.ErrPermanent},
		// 錯誤 body 不是 JSON 時仍要分類——Resend 的 body 形狀不在它的契約裡。
		{"unparseable 503", 503, `<html>gateway</html>`, mail.ErrTransient},
		{"unparseable 400", 400, `nope`, mail.ErrPermanent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			_, err := resend.New("k", resend.WithBaseURL(srv.URL)).Send(context.Background(), msg())
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// 2xx 但 body 不是合法 JSON：transient（不能排除是短暫的爛回應），而且不能讓
// *json.SyntaxError 漏進回傳的 error tree——那正是這個套件存在的理由。
func TestSendDecodeFailureDoesNotLeakProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	_, err := resend.New("k", resend.WithBaseURL(srv.URL)).Send(context.Background(), msg())
	require.ErrorIs(t, err, mail.ErrTransient)

	var se *json.SyntaxError
	require.False(t, errors.As(err, &se), "decode error must not leak into the returned error tree")
}

// 成功但沒有 id ⇒ 不能回 nil error：呼叫端會把它當「已受理」記下來，
// 而我們其實沒有任何可追的憑據。
func TestSendRejectsSuccessWithoutID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, err := resend.New("k", resend.WithBaseURL(srv.URL)).Send(context.Background(), msg())
	require.ErrorIs(t, err, mail.ErrTransient)
}

func TestSendValidatesBeforeCall(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()

	m := msg()
	m.To = mail.Address{}
	_, err := resend.New("k", resend.WithBaseURL(srv.URL)).Send(context.Background(), m)
	require.ErrorIs(t, err, mail.ErrPermanent)
	require.False(t, called, "不合法的信不該打到 provider")
}

// 連不上（context 取消／網路失敗）是 transient。
func TestSendNetworkFailureIsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close() // 立刻關掉，讓連線失敗

	_, err := resend.New("k", resend.WithBaseURL(srv.URL)).Send(context.Background(), msg())
	require.ErrorIs(t, err, mail.ErrTransient)

	// This is the package's primary invariant: the transport error type must
	// not escape, or a caller's retry logic can grow a dependency on net/url.
	var ue *url.Error
	require.False(t, errors.As(err, &ue), "transport error must not leak into the returned error tree")
}

var _ mail.Sender = (*resend.Sender)(nil)
