// Package resend is an anti-corruption layer over the Resend email API
// (https://api.resend.com). It translates Resend's HTTP status codes and error
// names into the pkg/mail taxonomy so callers never see a Resend-shaped error.
//
// Idempotency: this adapter DOES forward mail.Message.IdempotencyKey as
// Resend's Idempotency-Key header, which Resend honours for 24 hours.
//
// ⛔ The adapter never retries. One Send is at most one HTTP request; retry is
// the caller's outbox worker's job (pkg/outbox), same division as pkg/sms.
//
// To.Name: Resend's API documents the "Name <email>" display-name form only
// for `from`, not for `to`. Guessing wrong turns every send into a 422, which
// this package classifies permanent — not worth risking for a display name
// nobody needs yet. This adapter sends only mail.Message.To.Email and ignores
// To.Name; verify against Resend's API before changing that.
//
// Tags: Resend restricts tag names and values to ASCII letters, digits, `_`
// and `-`. That constraint is Resend's, not validated here — a non-ASCII tag
// would make Send return a 422, which classify treats as permanent.
package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/Will413028/hawkr/go-pkg/mail"
)

const (
	defaultBaseURL = "https://api.resend.com"
	defaultTimeout = 10 * time.Second
	// errBodyLimit caps how much of an error body we read. Resend errors are
	// small JSON; an unbounded read on a misbehaving proxy would not be.
	errBodyLimit = 8 << 10
)

// Sender submits messages to Resend.
type Sender struct {
	apiKey  string
	baseURL string
	http    *http.Client
	now     func() time.Time
}

// Option customises a Sender.
type Option func(*Sender)

// WithBaseURL overrides the API host. Tests point this at an httptest server.
func WithBaseURL(u string) Option { return func(s *Sender) { s.baseURL = u } }

// WithHTTPClient supplies the http.Client, e.g. one with tracing installed.
func WithHTTPClient(c *http.Client) Option { return func(s *Sender) { s.http = c } }

// New builds a Sender for the given API key.
func New(apiKey string, opts ...Option) *Sender {
	s := &Sender{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    &http.Client{Timeout: defaultTimeout},
		now:     time.Now,
	}
	for _, o := range opts {
		o(s)
	}
	// WithHTTPClient replaces the whole client, timeout included. The planned
	// first consumer of this package is an outbox worker (pkg/outbox), whose
	// process loop runs Send with no context deadline, so a client left with
	// a zero timeout could block that worker indefinitely while it holds a
	// claim lease. pkg/sms's client guards the same trap explicitly; do the
	// same here rather than trust every future caller to remember.
	//
	// Copy before adjusting rather than mutate s.http in place: WithHTTPClient's
	// own doc invites passing in a client another part of the process also
	// holds (e.g. one with tracing installed), and http.Client is a plain
	// struct that is safe to shallow-copy. Writing the timeout directly onto
	// that client would leak our default onto an object we do not own.
	if s.http.Timeout == 0 {
		httpClient := *s.http
		httpClient.Timeout = defaultTimeout
		s.http = &httpClient
	}
	return s
}

type tag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type sendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html,omitempty"`
	Text    string   `json:"text,omitempty"`
	Tags    []tag    `json:"tags,omitempty"`
}

type sendResponse struct {
	ID string `json:"id"`
}

type errorResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// Send submits one message. See package doc for the retry contract.
func (s *Sender) Send(ctx context.Context, m mail.Message) (mail.Receipt, error) {
	if err := m.Validate(); err != nil {
		return mail.Receipt{}, err
	}

	body := sendRequest{
		From: m.From.String(), To: []string{m.To.Email},
		Subject: m.Subject, HTML: m.HTML, Text: m.Text,
	}
	// Deterministic tag order: map iteration is random, and a stable request
	// body is what makes the golden assertions in the tests meaningful.
	for _, k := range sortedKeys(m.Tags) {
		body.Tags = append(body.Tags, tag{Name: k, Value: m.Tags[k]})
	}

	raw, err := json.Marshal(body)
	if err != nil {
		// %s + err.Error(), not %w: a second %w would let the underlying error
		// (here, later a provider/transport error) into the returned tree,
		// breaking the anti-corruption contract in the package doc above.
		// errorlint's errorf check is what enforces this — do not "fix" it to %w.
		return mail.Receipt{}, fmt.Errorf("%w: marshal request: %s", mail.ErrPermanent, err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/emails", bytes.NewReader(raw))
	if err != nil {
		return mail.Receipt{}, fmt.Errorf("%w: build request: %s", mail.ErrPermanent, err.Error())
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	// ⛔ Only set when non-empty: an empty Idempotency-Key is rejected by
	// Resend with invalid_idempotency_key, which classify treats as permanent —
	// so a caller that simply does not use idempotency would lose every send.
	if m.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", m.IdempotencyKey)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		// Transport failure: DNS, dial, TLS, timeout, context cancellation.
		// All are worth another attempt later.
		return mail.Receipt{}, fmt.Errorf("%w: resend request: %s", mail.ErrTransient, err.Error())
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var er errorResponse
		// Best effort: the error body shape is not part of Resend's documented
		// contract, so classify must stay correct with an empty name.
		if b, readErr := io.ReadAll(io.LimitReader(resp.Body, errBodyLimit)); readErr == nil {
			_ = json.Unmarshal(b, &er)
		}
		return mail.Receipt{}, classify(resp.StatusCode, er.Name, er.Message)
	}

	var out sendResponse
	// Same cap as the error path: unbounded reads on a misbehaving proxy would
	// not stay small JSON just because the status happened to be 2xx.
	if err := json.NewDecoder(io.LimitReader(resp.Body, errBodyLimit)).Decode(&out); err != nil {
		return mail.Receipt{}, fmt.Errorf("%w: decode resend response: %s", mail.ErrTransient, err.Error())
	}
	if out.ID == "" {
		// 2xx without an id: we cannot prove anything was accepted and have no
		// handle for a later webhook. Transient so the caller tries again
		// rather than recording a receipt it cannot trace.
		return mail.Receipt{}, fmt.Errorf("%w: resend returned 2xx with no message id", mail.ErrTransient)
	}
	return mail.Receipt{ProviderMessageID: out.ID, AcceptedAt: s.now()}, nil
}

func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
