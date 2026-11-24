package resend

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestSendClassifiesStatuses(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{200, nil}, {429, ErrTransient}, {500, ErrTransient}, {503, ErrTransient}, {409, ErrTransient},
		{400, ErrPermanent}, {422, ErrPermanent},
	}
	for _, c := range cases {
		t.Run(strconv.Itoa(c.status), func(t *testing.T) {
			var key, auth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key, auth = r.Header.Get("Idempotency-Key"), r.Header.Get("Authorization")
				_, _ = io.Copy(io.Discard, r.Body)
				w.WriteHeader(c.status)
			}))
			defer srv.Close()
			cl, err := New(Config{APIKey: "k", From: "A <a@b.example>", BaseURL: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			err = cl.Send(context.Background(), Email{To: "u@x.example", Subject: "s", Text: "t", IdempotencyKey: "d1"})
			if c.want == nil {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, c.want) {
				t.Fatalf("err = %v，預期 %v", err, c.want)
			}
			if key != "d1" || auth != "Bearer k" {
				t.Fatalf("key=%q auth=%q", key, auth)
			}
		})
	}
}

// 401、403 是金鑰設定問題：暫時性（修好設定後重試可能成功）並帶 ErrAuth 讓呼叫端告警。
func TestSendRejectedKeyIsTransientAndFlagged(t *testing.T) {
	for _, status := range []int{401, 403} {
		err := classify(status, "restricted_api_key")
		if !errors.Is(err, ErrTransient) || !errors.Is(err, ErrAuth) || errors.Is(err, ErrPermanent) {
			t.Errorf("%d：err = %v", status, err)
		}
	}
}

// 409 以錯誤名稱區分：同鍵請求還在處理是暫時性，同鍵卻帶不同內容是永久性。
func TestClassifyIdempotencyConflicts(t *testing.T) {
	if err := classify(409, "concurrent_idempotent_requests"); !errors.Is(err, ErrTransient) {
		t.Errorf("進行中：%v", err)
	}
	if err := classify(409, "invalid_idempotent_request"); !errors.Is(err, ErrPermanent) || errors.Is(err, ErrTransient) {
		t.Errorf("同鍵不同內容：%v", err)
	}
}

func TestSendNetworkFailureIsTransient(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	cl, err := New(Config{APIKey: "k", From: "a@b.example", BaseURL: url})
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.Send(context.Background(), Email{To: "u@x.example"}); !errors.Is(err, ErrTransient) {
		t.Fatalf("err = %v", err)
	}
}

func sign(secret, id string, ts time.Time, body string) http.Header {
	key, _ := base64.StdEncoding.DecodeString(secret[len("whsec_"):])
	m := hmac.New(sha256.New, key)
	tsText := strconv.FormatInt(ts.Unix(), 10)
	m.Write([]byte(id + "." + tsText + "." + body))
	h := http.Header{}
	h.Set("svix-id", id)
	h.Set("svix-timestamp", tsText)
	h.Set("svix-signature", "v1,AAAA v1,"+base64.StdEncoding.EncodeToString(m.Sum(nil)))
	return h
}

func TestVerify(t *testing.T) {
	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	now := time.Unix(1_800_000_000, 0)
	body := `{"type":"email.bounced","data":{"to":["a@x.example"],"bounce":{"type":"Permanent"}}}`
	ev, err := Verify(secret, sign(secret, "msg_1", now, body), []byte(body), now)
	if err != nil {
		t.Fatal(err)
	}
	if ev.ID != "msg_1" || ev.Type != EventBounced || len(ev.To) != 1 || !ev.Suppresses() {
		t.Fatalf("event = %+v", ev)
	}
	t.Run("tampered body", func(t *testing.T) {
		if _, err := Verify(secret, sign(secret, "msg_1", now, body), []byte(body+" "), now); !errors.Is(err, ErrSignature) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("stale timestamp", func(t *testing.T) {
		if _, err := Verify(secret, sign(secret, "msg_1", now.Add(-SignatureTolerance-time.Second), body), []byte(body), now); !errors.Is(err, ErrSignature) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("missing headers", func(t *testing.T) {
		if _, err := Verify(secret, http.Header{}, []byte(body), now); !errors.Is(err, ErrSignature) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("wrong secret", func(t *testing.T) {
		other := "whsec_" + base64.StdEncoding.EncodeToString([]byte("another-secret!!"))
		if _, err := Verify(other, sign(secret, "msg_1", now, body), []byte(body), now); !errors.Is(err, ErrSignature) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestSuppresses(t *testing.T) {
	cases := []struct {
		ev   Event
		want bool
	}{
		{Event{Type: EventComplained}, true},
		{Event{Type: EventBounced, BounceType: "Permanent"}, true},
		{Event{Type: EventBounced, BounceType: "Transient"}, false},
		{Event{Type: EventBounced, BounceType: "Undetermined"}, false},
		{Event{Type: "email.delivered"}, false},
	}
	for _, c := range cases {
		if got := c.ev.Suppresses(); got != c.want {
			t.Errorf("%+v：%v，預期 %v", c.ev, got, c.want)
		}
	}
}

func TestParseEventRejectsGarbage(t *testing.T) {
	for _, body := range []string{"nope", `{}`} {
		if _, err := ParseEvent([]byte(body)); !errors.Is(err, ErrMalformed) {
			t.Errorf("%q：%v", body, err)
		}
	}
}
