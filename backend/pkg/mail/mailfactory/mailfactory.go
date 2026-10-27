// Package mailfactory builds the configured mail.Sender.
//
// It lives outside package mail because mail is imported by every provider
// sub-package; a factory inside mail would import them back and form a cycle.
package mailfactory

import (
	"fmt"
	"strings"

	"github.com/Will413028/hawkr/go-pkg/mail"
	"github.com/Will413028/hawkr/go-pkg/mail/guard"
	"github.com/Will413028/hawkr/go-pkg/mail/logmail"
	"github.com/Will413028/hawkr/go-pkg/mail/resend"
	"go.uber.org/zap"
)

// Provider values accepted by New.
const (
	ProviderLog    = "log"
	ProviderResend = "resend"
)

// Config is the deployment-time mail configuration, sourced from
// MAIL_PROVIDER, the provider's credential env var, and the recipient policy.
type Config struct {
	Provider string
	APIKey   string

	// RecipientPolicy, AllowedRecipients and RedirectTo configure the guard
	// that every provider is wrapped in. See package mail/guard — the mode is
	// required in every environment, including local ones.
	RecipientPolicy   string
	AllowedRecipients []string
	RedirectTo        string
}

// New returns the Sender for cfg.
//
// An empty Provider means "log": local runs and unit tests must not require
// mail configuration to boot. Anything misconfigured fails here, at startup,
// rather than on the first message — a service that boots and then silently
// cannot send is the failure this rejects.
func New(cfg Config, logger *zap.Logger) (mail.Sender, error) {
	provider, delivers, err := newProvider(cfg, logger)
	if err != nil {
		return nil, err
	}

	mode := guard.ParseMode(cfg.RecipientPolicy)

	// The seatbelt is required exactly when the provider can actually reach a
	// person. logmail cannot, so an unset policy there stays bootable — that is
	// the "local runs and unit tests need no mail configuration" contract this
	// package already carries, and demanding a policy for a sender that reaches
	// nobody buys no safety. Flipping MAIL_PROVIDER to a delivering one re-runs
	// this function, so the permissive case cannot be carried over by accident.
	if mode == "" && !delivers {
		logger.Info("no mail recipient policy set; the configured provider cannot deliver, so none is required",
			zap.String("provider", cfg.Provider))
		return provider, nil
	}

	g, err := guard.New(provider, guard.Policy{
		Mode:       mode,
		Allowed:    cfg.AllowedRecipients,
		RedirectTo: cfg.RedirectTo,
	})
	if err != nil {
		return nil, fmt.Errorf("mailfactory: %w", err)
	}
	if mode == guard.ModeUnrestricted {
		logger.Info("mail recipient policy is unrestricted — every address the caller asks for will be mailed")
	}
	return g, nil
}

// newProvider returns the raw provider and whether it can actually deliver to a
// recipient. The second value drives whether a recipient policy is mandatory.
func newProvider(cfg Config, logger *zap.Logger) (mail.Sender, bool, error) {
	switch p := strings.ToLower(strings.TrimSpace(cfg.Provider)); p {
	case "":
		// Empty is indistinguishable at runtime from an explicit "log", but the
		// causes differ: this branch is where a lost/misspelled k8s env var
		// lands. logmail's own package doc says it must never be the production
		// default and names this factory as the place that choice is made
		// explicit — so warn here. An explicit "log" below stays silent: that is
		// a deliberate choice, not an accident.
		logger.Warn("MAIL_PROVIDER is unset — falling back to the log sender; no mail will actually be delivered")
		return logmail.New(logger), false, nil
	case ProviderLog:
		return logmail.New(logger), false, nil
	case ProviderResend:
		if cfg.APIKey == "" {
			return nil, false, fmt.Errorf("mailfactory: MAIL_PROVIDER=resend requires an API key")
		}
		return resend.New(cfg.APIKey), true, nil
	default:
		return nil, false, fmt.Errorf("mailfactory: unknown mail provider %q", cfg.Provider)
	}
}
