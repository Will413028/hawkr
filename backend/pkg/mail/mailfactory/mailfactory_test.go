package mailfactory_test

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/mail/guard"
	"github.com/Will413028/hawkr/go-pkg/mail/logmail"
	"github.com/Will413028/hawkr/go-pkg/mail/mailfactory"
	"github.com/Will413028/hawkr/go-pkg/mail/resend"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewSelectsProvider(t *testing.T) {
	log := zap.NewNop()

	s, err := mailfactory.New(mailfactory.Config{Provider: "log"}, log)
	require.NoError(t, err)
	require.IsType(t, &logmail.Sender{}, s)

	s, err = mailfactory.New(mailfactory.Config{
		Provider: "resend", APIKey: "re_x",
		RecipientPolicy: string(guard.ModeUnrestricted),
	}, log)
	require.NoError(t, err)
	require.IsType(t, &guard.Sender{}, s)
}

// 🔴 會真的投遞的 provider 一定被 guard 包住，沒有例外路徑。少了這條，
// 「resend 直接回傳未包裝的 sender」這種改動不會有任何測試轉紅。
func TestNewAlwaysGuardsADeliveringProvider(t *testing.T) {
	for _, mode := range []guard.Mode{guard.ModeUnrestricted, guard.ModeAllowlist, guard.ModeRedirect} {
		t.Run(string(mode), func(t *testing.T) {
			s, err := mailfactory.New(mailfactory.Config{
				Provider: "resend", APIKey: "re_x",
				RecipientPolicy:   string(mode),
				AllowedRecipients: []string{"@example.com"},
				RedirectTo:        "willwu@example.com",
			}, zap.NewNop())
			require.NoError(t, err)
			require.IsType(t, &guard.Sender{}, s)
			_, bare := s.(*resend.Sender)
			require.False(t, bare, "未包裝的 resend sender 直通代表守衛被繞過")
		})
	}
}

// 🔴 resend 少了收件人政策必須在啟動時就失敗。這一格是整個 guard 的存在理由：
// 有憑證、有真實名單、卻沒有收件人限制的部署，必須開不起來。
func TestNewRejectsDeliveringProviderWithoutRecipientPolicy(t *testing.T) {
	_, err := mailfactory.New(mailfactory.Config{Provider: "resend", APIKey: "re_x"}, zap.NewNop())
	require.ErrorContains(t, err, "recipient policy mode is required")
}

// 對照組：logmail 寄不到任何人，不強制政策——既有的「本機與測試免設定即可開機」
// 契約仍然成立。
func TestNewAllowsMissingPolicyForNonDeliveringProvider(t *testing.T) {
	for _, p := range []string{"", "log"} {
		s, err := mailfactory.New(mailfactory.Config{Provider: p}, zap.NewNop())
		require.NoError(t, err)
		require.IsType(t, &logmail.Sender{}, s)
	}
}

// 政策設錯值一樣要在啟動時失敗，不得退回任何預設。
func TestNewRejectsUnknownRecipientPolicy(t *testing.T) {
	_, err := mailfactory.New(mailfactory.Config{
		Provider: "resend", APIKey: "re_x", RecipientPolicy: "production",
	}, zap.NewNop())
	require.ErrorContains(t, err, `unknown recipient policy mode "production"`)
}

// 空的 provider 走 log：本機與測試不必設環境變數就能跑起來。
func TestNewDefaultsToLog(t *testing.T) {
	s, err := mailfactory.New(mailfactory.Config{}, zap.NewNop())
	require.NoError(t, err)
	require.IsType(t, &logmail.Sender{}, s)
}

// resend 少了 API key 要在啟動時就失敗，不要等到第一封信才發現。
func TestNewRejectsResendWithoutAPIKey(t *testing.T) {
	_, err := mailfactory.New(mailfactory.Config{Provider: "resend"}, zap.NewNop())
	require.ErrorContains(t, err, "MAIL_PROVIDER=resend requires an API key")
}

func TestNewRejectsUnknownProvider(t *testing.T) {
	_, err := mailfactory.New(mailfactory.Config{Provider: "sendgrid"}, zap.NewNop())
	require.ErrorContains(t, err, `unknown mail provider "sendgrid"`)
}

// 大小寫與前後空白要容錯，因為它來自人手填的 env var。
func TestNewTrimsAndLowercasesProvider(t *testing.T) {
	s, err := mailfactory.New(mailfactory.Config{
		Provider: "  Resend  ", APIKey: "re_x",
		RecipientPolicy: "  Unrestricted  ",
	}, zap.NewNop())
	require.NoError(t, err)
	require.IsType(t, &guard.Sender{}, s)
}

// 空的 MAIL_PROVIDER 是設定遺失會落地的地方，要警告；明確選 "log" 是刻意的
// 選擇，不該被當成事故一樣吵。
func TestNewWarnsOnlyWhenProviderIsUnset(t *testing.T) {
	t.Run("empty provider warns", func(t *testing.T) {
		core, logs := observer.New(zapcore.WarnLevel)
		_, err := mailfactory.New(mailfactory.Config{}, zap.New(core))
		require.NoError(t, err)
		require.Equal(t, 1, logs.Len())
		require.Contains(t, logs.All()[0].Message, "MAIL_PROVIDER is unset")
	})

	t.Run("explicit log does not warn", func(t *testing.T) {
		core, logs := observer.New(zapcore.WarnLevel)
		_, err := mailfactory.New(mailfactory.Config{Provider: "log"}, zap.New(core))
		require.NoError(t, err)
		require.Zero(t, logs.Len())
	})
}
