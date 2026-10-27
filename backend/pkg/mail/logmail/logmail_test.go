package logmail_test

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/mail"
	"github.com/Will413028/hawkr/go-pkg/mail/logmail"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func msg() mail.Message {
	return mail.Message{
		From:    mail.Address{Email: "supports@example.com"},
		To:      mail.Address{Email: "vendor@example.com"},
		Subject: "詢價邀請",
		Text:    "https://www.example.com/vendor/rfq#invite=abc",
	}
}

func TestSendLogsWholeMessageAndReturnsStableID(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	s := logmail.New(zap.New(core))

	got, err := s.Send(context.Background(), msg())
	require.NoError(t, err)
	require.NotEmpty(t, got.ProviderMessageID)
	require.False(t, got.AcceptedAt.IsZero())

	// 同一封信兩次要得到同一個 id：dev e2e 靠它把 log 行對回那一封信。
	again, err := s.Send(context.Background(), msg())
	require.NoError(t, err)
	require.Equal(t, got.ProviderMessageID, again.ProviderMessageID)

	// 整封信要進 log——dev 沒有真的收件匣，log 就是收件匣。
	require.Equal(t, 2, logs.Len())
	entry := logs.All()[0]
	fields := entry.ContextMap()
	require.Equal(t, "vendor@example.com", fields["to"])
	require.Equal(t, "詢價邀請", fields["subject"])
	require.Contains(t, fields["text"], "invite=abc")
}

// id 是對整封信 hash 出來的，不是只對「收件人＋主旨」——這裡逐一改動一個欄位，
// 釘住每個欄位都真的進了 hash。少了任何一個都會讓這個測試失敗，例如 hash 退化回
// "to|subject|idempotencyKey" 就會讓 Text 不同的兩封信撞成同一個 id（正是
// TestSendLogsWholeMessageAndReturnsStableID 上面那段註解在修的碰撞問題）。
func TestSendIDDiffersWhenBodyOrFromDiffers(t *testing.T) {
	core, _ := observer.New(zapcore.DebugLevel)
	s := logmail.New(zap.New(core))

	base := msg()
	base.HTML = "<p>base</p>"
	got, err := s.Send(context.Background(), base)
	require.NoError(t, err)

	diffText := base
	diffText.Text = base.Text + "-diff"
	gotDiffText, err := s.Send(context.Background(), diffText)
	require.NoError(t, err)
	require.NotEqual(t, got.ProviderMessageID, gotDiffText.ProviderMessageID,
		"Text 不同時 id 也要不同")

	diffHTML := base
	diffHTML.HTML = "<p>different</p>"
	gotDiffHTML, err := s.Send(context.Background(), diffHTML)
	require.NoError(t, err)
	require.NotEqual(t, got.ProviderMessageID, gotDiffHTML.ProviderMessageID,
		"HTML 不同時 id 也要不同")

	diffFrom := base
	diffFrom.From = mail.Address{Email: "other-sender@example.com"}
	gotDiffFrom, err := s.Send(context.Background(), diffFrom)
	require.NoError(t, err)
	require.NotEqual(t, got.ProviderMessageID, gotDiffFrom.ProviderMessageID,
		"From 不同時 id 也要不同")
}

// 驗證失敗要在送出前擋下，而且是 ErrPermanent。
func TestSendRejectsInvalidMessage(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	s := logmail.New(zap.New(core))

	m := msg()
	m.Subject = ""
	_, err := s.Send(context.Background(), m)
	require.ErrorIs(t, err, mail.ErrPermanent)
	require.Zero(t, logs.Len(), "不合法的信不該留下「已寄出」的痕跡")
}

var _ mail.Sender = (*logmail.Sender)(nil)
