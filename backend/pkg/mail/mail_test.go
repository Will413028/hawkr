package mail_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/mail"
	"github.com/Will413028/hawkr/go-pkg/mail/fake"
	"github.com/stretchr/testify/require"
)

func TestAddressString(t *testing.T) {
	require.Equal(t, "supports@example.com",
		mail.Address{Email: "supports@example.com"}.String())
	require.Equal(t, `"範例生鮮" <supports@example.com>`,
		mail.Address{Email: "supports@example.com", Name: "範例生鮮"}.String())
}

func TestMessageValidate(t *testing.T) {
	valid := mail.Message{
		From:    mail.Address{Email: "supports@example.com"},
		To:      mail.Address{Email: "vendor@example.com"},
		Subject: "詢價邀請",
		Text:    "內文",
	}
	require.NoError(t, valid.Validate())

	// 缺欄位一律是 ErrPermanent：呼叫端組錯了信，重試多少次都一樣。
	for name, mutate := range map[string]func(m *mail.Message){
		"no from":    func(m *mail.Message) { m.From = mail.Address{} },
		"no to":      func(m *mail.Message) { m.To = mail.Address{} },
		"no subject": func(m *mail.Message) { m.Subject = "" },
		"no body":    func(m *mail.Message) { m.Text, m.HTML = "", "" },
	} {
		t.Run(name, func(t *testing.T) {
			m := valid
			mutate(&m)
			err := m.Validate()
			require.Error(t, err)
			require.ErrorIs(t, err, mail.ErrPermanent)
			require.NotErrorIs(t, err, mail.ErrTransient)
		})
	}
}

// HTML 或 Text 有一個就夠——Resend 缺 text 時會自己從 html 生。
func TestMessageValidateHTMLOnly(t *testing.T) {
	m := mail.Message{
		From:    mail.Address{Email: "supports@example.com"},
		To:      mail.Address{Email: "vendor@example.com"},
		Subject: "詢價邀請",
		HTML:    "<p>內文</p>",
	}
	require.NoError(t, m.Validate())
}

func TestFakeSenderRecordsAndReplays(t *testing.T) {
	f := &fake.Sender{Receipt: mail.Receipt{ProviderMessageID: "fake-1"}}
	m := mail.Message{
		From:    mail.Address{Email: "supports@example.com"},
		To:      mail.Address{Email: "vendor@example.com"},
		Subject: "詢價邀請",
		Text:    "內文",
	}

	got, err := f.Send(context.Background(), m)
	require.NoError(t, err)
	require.Equal(t, "fake-1", got.ProviderMessageID)
	require.Len(t, f.Sent, 1)
	require.Equal(t, "vendor@example.com", f.Sent[0].To.Email)

	f.Err = errors.New("boom")
	_, err = f.Send(context.Background(), m)
	require.EqualError(t, err, "boom")
	require.Len(t, f.Sent, 2, "失敗的呼叫也要記錄，測試才驗得到「試過寄什麼」")
}

// fake 必須滿足介面——編譯期斷言。
var _ mail.Sender = (*fake.Sender)(nil)
