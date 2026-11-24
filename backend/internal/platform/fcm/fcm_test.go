package fcm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"golang.org/x/oauth2"
)

func TestChunksNeverExceedMaxBatch(t *testing.T) {
	tokens := make([]string, 1203)
	for i := range tokens {
		tokens[i] = strconv.Itoa(i)
	}
	chunks := Chunks(tokens)
	if len(chunks) != 3 {
		t.Fatalf("%d 批，預期 3", len(chunks))
	}
	total := 0
	for _, c := range chunks {
		if len(c) > MaxBatch {
			t.Fatalf("一批 %d 個 token，超過 %d", len(c), MaxBatch)
		}
		total += len(c)
	}
	if total != len(tokens) {
		t.Fatalf("共 %d 個 token，預期 %d", total, len(tokens))
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   Outcome
	}{
		{"unregistered", 404, `{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`, OutcomeUnregistered},
		{"sender mismatch", 403, `{"error":{"status":"PERMISSION_DENIED","details":[{"errorCode":"SENDER_ID_MISMATCH"}]}}`, OutcomeUnregistered},
		{"invalid argument does not deactivate", 400, `{"error":{"status":"INVALID_ARGUMENT","details":[{"errorCode":"INVALID_ARGUMENT"}]}}`, OutcomePermanent},
		{"third party auth", 401, `{"error":{"status":"UNAUTHENTICATED","details":[{"errorCode":"THIRD_PARTY_AUTH_ERROR"}]}}`, OutcomeAuth},
		{"credential rejected", 401, `{"error":{"status":"UNAUTHENTICATED"}}`, OutcomeAuth},
		{"quota", 429, `{"error":{"status":"RESOURCE_EXHAUSTED","details":[{"errorCode":"QUOTA_EXCEEDED"}]}}`, OutcomeTransient},
		{"unavailable", 503, `{"error":{"status":"UNAVAILABLE"}}`, OutcomeTransient},
		{"internal", 500, `not json`, OutcomeTransient},
		{"unknown 4xx", 400, `{}`, OutcomePermanent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classify("tok", c.status, []byte(c.body)).Outcome; got != c.want {
				t.Fatalf("outcome = %d，預期 %d", got, c.want)
			}
		})
	}
}

func TestSendReportsEachTokenAndSendsTheMessage(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/projects/proj/messages:send" || r.Header.Get("Authorization") != "Bearer at" {
			t.Errorf("path=%q auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			Message struct {
				Token        string            `json:"token"`
				Notification map[string]string `json:"notification"`
				Data         map[string]string `json:"data"`
			} `json:"message"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("body: %v", err)
		}
		if body.Message.Notification["title"] != "T" || body.Message.Data["k"] != "v" {
			t.Errorf("message = %s", raw)
		}
		switch body.Message.Token {
		case "gone":
			w.WriteHeader(404)
			_, _ = io.WriteString(w, `{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`)
		case "slow":
			w.WriteHeader(503)
		default:
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()
	c, err := New(context.Background(), Config{ProjectID: "proj", BaseURL: srv.URL, TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "at"})})
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.Send(context.Background(), []string{"ok", "gone", "slow"}, Message{Title: "T", Body: "B", Data: map[string]string{"k": "v"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []Outcome{OutcomeSent, OutcomeUnregistered, OutcomeTransient}
	for i, w := range want {
		if res[i].Outcome != w {
			t.Errorf("token %d：%d，預期 %d", i, res[i].Outcome, w)
		}
	}
	if calls.Load() != 3 {
		t.Fatalf("%d 次呼叫，預期 3", calls.Load())
	}
}

func TestSendOverTheBatchLimitAnswersEveryToken(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer srv.Close()
	c, err := New(context.Background(), Config{ProjectID: "p", BaseURL: srv.URL, TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "at"})})
	if err != nil {
		t.Fatal(err)
	}
	tokens := make([]string, MaxBatch+7)
	for i := range tokens {
		tokens[i] = strconv.Itoa(i)
	}
	res, err := c.Send(context.Background(), tokens, Message{Title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != len(tokens) || int(calls.Load()) != len(tokens) {
		t.Fatalf("results=%d calls=%d，預期 %d", len(res), calls.Load(), len(tokens))
	}
	for i, r := range res {
		if r.Token != tokens[i] || r.Outcome != OutcomeSent {
			t.Fatalf("result %d = %+v", i, r)
		}
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	if _, err := New(context.Background(), Config{ProjectID: "p"}); err == nil {
		t.Fatal("沒有金鑰也建立了")
	}
	if _, err := New(context.Background(), Config{ServiceAccountJSON: []byte("{}")}); err == nil {
		t.Fatal("沒有 ProjectID 也建立了")
	}
	if _, err := New(context.Background(), Config{ProjectID: "p", ServiceAccountJSON: []byte("not json")}); err == nil {
		t.Fatal("壞掉的金鑰也建立了")
	}
}

func TestPayloadGolden(t *testing.T) {
	full, err := json.Marshal(Payload("tok", Message{
		Title: "標題", Body: "內文", ImageURL: "https://img.example/a.jpg",
		Data: map[string]string{"deep_link": "/orders/1", "state_refresh_hints": "STATE_REFRESH_HINT_ORDER", "inbox_item_id": "0190a000-0000-7000-8000-0000000000a1"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	const wantFull = `{"message":{"apns":{"fcm_options":{"image":"https://img.example/a.jpg"},"payload":{"aps":{"mutable-content":1}}},` +
		`"data":{"deep_link":"/orders/1","inbox_item_id":"0190a000-0000-7000-8000-0000000000a1","state_refresh_hints":"STATE_REFRESH_HINT_ORDER"},` +
		`"notification":{"body":"內文","title":"標題"},"token":"tok"}}`
	if string(full) != wantFull {
		t.Fatalf("payload = %s\nwant    = %s", full, wantFull)
	}
	bare, _ := json.Marshal(Payload("tok", Message{Title: "t", Body: "b"}))
	const wantBare = `{"message":{"apns":{"payload":{"aps":{"mutable-content":1}}},"notification":{"body":"b","title":"t"},"token":"tok"}}`
	if string(bare) != wantBare {
		t.Fatalf("payload = %s\nwant    = %s", bare, wantBare)
	}
}
