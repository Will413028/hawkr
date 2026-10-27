package guard_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/mail"
	"github.com/Will413028/hawkr/go-pkg/mail/fake"
	"github.com/Will413028/hawkr/go-pkg/mail/guard"
)

func msg(to string) mail.Message {
	return mail.Message{
		From:    mail.Address{Email: "supports@allowed.example", Name: "範例生鮮"},
		To:      mail.Address{Email: to},
		Subject: "您有一筆新的採購詢價",
		Text:    "body",
	}
}

func TestNew_FailsClosed(t *testing.T) {
	cases := []struct {
		name   string
		policy guard.Policy
	}{
		{"mode 未設", guard.Policy{}},
		{"未知 mode", guard.Policy{Mode: "production"}},
		{"allowlist 但清單空", guard.Policy{Mode: guard.ModeAllowlist}},
		{"redirect 但無目標", guard.Policy{Mode: guard.ModeRedirect}},
		{"redirect 目標只有空白", guard.Policy{Mode: guard.ModeRedirect, RedirectTo: "   "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := guard.New(&fake.Sender{}, tc.policy); err == nil {
				t.Fatal("預期 New 失敗（fail-closed），卻回 nil error")
			}
		})
	}
}

func TestNew_NilInner(t *testing.T) {
	if _, err := guard.New(nil, guard.Policy{Mode: guard.ModeUnrestricted}); err == nil {
		t.Fatal("inner 為 nil 時必須失敗")
	}
}

func TestUnrestricted_PassesThrough(t *testing.T) {
	f := &fake.Sender{}
	g, err := guard.New(f, guard.Policy{Mode: guard.ModeUnrestricted})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Send(context.Background(), msg("vendor@example.com")); err != nil {
		t.Fatalf("unrestricted 不該擋: %v", err)
	}
	if got := f.Sent[0].To.Email; got != "vendor@example.com" {
		t.Errorf("收件人被改寫成 %q", got)
	}
	if strings.HasPrefix(f.Sent[0].Subject, "[→") {
		t.Error("unrestricted 不該改寫主旨")
	}
}

func TestAllowlist(t *testing.T) {
	policy := guard.Policy{
		Mode:    guard.ModeAllowlist,
		Allowed: []string{"@allowed.example", "delivered@resend.dev"},
	}
	cases := []struct {
		to     string
		passes bool
		why    string
	}{
		{"willwu@allowed.example", true, "網域相符"},
		{"WillWu@Allowed.Example", true, "比對不分大小寫"},
		{"delivered@resend.dev", true, "整串位址相符"},
		{"bounced@resend.dev", false, "整串比對不得退化成網域比對"},
		{"vendor@example.com", false, "不在清單"},
		{"attacker@evil.com@allowed.example.evil.com", false, "尾綴比對不得被拼接位址騙過"},
		{"", false, "空位址一律擋"},
	}
	for _, tc := range cases {
		t.Run(tc.to+"/"+tc.why, func(t *testing.T) {
			f := &fake.Sender{}
			g, err := guard.New(f, policy)
			if err != nil {
				t.Fatal(err)
			}
			_, sendErr := g.Send(context.Background(), msg(tc.to))
			if tc.passes {
				if sendErr != nil {
					t.Fatalf("預期放行（%s），卻被擋: %v", tc.why, sendErr)
				}
				if len(f.Sent) != 1 {
					t.Fatalf("預期送出 1 封，實際 %d", len(f.Sent))
				}
				return
			}
			if sendErr == nil {
				t.Fatalf("預期擋下（%s），卻放行了", tc.why)
			}
			if !errors.Is(sendErr, mail.ErrPermanent) {
				t.Errorf("擋下時必須是 ErrPermanent（否則 outbox 會無限重試），實際 %v", sendErr)
			}
			if len(f.Sent) != 0 {
				t.Errorf("被擋的訊息不得抵達 provider，實際送出 %d 封", len(f.Sent))
			}
			if strings.Contains(sendErr.Error(), tc.to) && tc.to != "" {
				t.Errorf("錯誤訊息不得包含未遮罩的位址: %v", sendErr)
			}
		})
	}
}

func TestRedirect_RewritesAndKeepsOriginal(t *testing.T) {
	f := &fake.Sender{}
	g, err := guard.New(f, guard.Policy{Mode: guard.ModeRedirect, RedirectTo: "willwu@allowed.example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Send(context.Background(), msg("yenyu@example.net")); err != nil {
		t.Fatal(err)
	}
	sent := f.Sent[0]
	if sent.To.Email != "willwu@allowed.example" {
		t.Fatalf("收件人未被改寫，實際 %q —— 這一格失敗代表真實業者會收到信", sent.To.Email)
	}
	if !strings.Contains(sent.Subject, "yenyu@example.net") {
		t.Errorf("主旨必須留下原收件人，否則轉寄信箱看不出這封是要寄給誰: %q", sent.Subject)
	}
	if !strings.Contains(sent.Subject, "您有一筆新的採購詢價") {
		t.Errorf("原主旨不得被吃掉: %q", sent.Subject)
	}
	if sent.From.Email != "supports@allowed.example" {
		t.Errorf("寄件人不該被動到: %q", sent.From.Email)
	}
	if sent.Text != "body" {
		t.Errorf("內文不該被動到")
	}
}

func TestRedirect_AppliesToEveryRecipient(t *testing.T) {
	f := &fake.Sender{}
	g, err := guard.New(f, guard.Policy{Mode: guard.ModeRedirect, RedirectTo: "willwu@allowed.example"})
	if err != nil {
		t.Fatal(err)
	}
	for _, to := range []string{"a@x.com", "b@y.com", "willwu@allowed.example"} {
		if _, err := g.Send(context.Background(), msg(to)); err != nil {
			t.Fatal(err)
		}
	}
	for i, s := range f.Sent {
		if s.To.Email != "willwu@allowed.example" {
			t.Errorf("第 %d 封逃出了 redirect: %q", i, s.To.Email)
		}
	}
}

func TestMaskEmail(t *testing.T) {
	cases := map[string]string{
		"yenyu@example.net": "y***@example.net",
		"a@b.c":             "a***@b.c",
		"@nodomain":         "***",
		"notanemail":        "***",
		"":                  "***",
	}
	for in, want := range cases {
		if got := guard.MaskEmail(in); got != want {
			t.Errorf("MaskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseAllowed(t *testing.T) {
	got := guard.ParseAllowed(" @allowed.example , delivered@resend.dev ,, ")
	want := []string{"@allowed.example", "delivered@resend.dev"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}
