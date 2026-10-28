package middleware_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/router"
	"github.com/gin-gonic/gin"
)

// aiRoutePrefixes are the three route trees #2505 took off exemptPrefixes. Every
// route mounted under them must appear in aiRouteTiers below — that is what makes
// "a new AI endpoint is rate limited unless someone writes down otherwise" a
// CI-enforced property instead of a hope.
var aiRoutePrefixes = []string{
	"/api/v1/app/sellers/me/assistant/",
	"/api/v1/app/buyers/me/assistant/",
	"/api/v1/app/buyers/me/procurement/",
}

// costClass records what one request to a route actually spends downstream. It is
// not an input to classifyTier — today every route below lands in the same bucket
// — but it is the column that makes this table a decision rather than a
// transcription, and it is the shopping list for the per-account meter in 57 號 §7
// (which bills answers, not requests).
type costClass string

const (
	// costModel: this request runs an LLM turn (plus, for candidate search, the
	// two search lanes' tool and external calls on top).
	costModel costClass = "model"
	// costData: DB read/write only. Cheap per call — and unbounded while exempt.
	costData costClass = "data"
)

type aiRoute struct {
	method string
	path   string
	tier   string
	cost   costClass
}

// aiRouteTiers is the authoritative rate-limit classification for the AI assistant
// and Hawkr 幫找 trees, in the same spirit as expectedSellerRoutes in package router:
// the test asserts the LIVE route table equals this set exactly, so drift fails CI
// in both directions — a new mount missing from the table, or a row whose route
// was deleted.
//
// Everything is "default" today, and that is a decision rather than an omission:
// the exemptPrefixes comment in ratelimit.go records why nothing here reconnects
// (so nothing here needs what exemptExact exists to give) and why this change
// invents no AI-specific ceiling.
//
// ⚠️ A costModel row is NOT made safe by its tier. tierDefault counts requests per
// subject — and the subject is an address, not an account. It does not count
// tokens, tool calls, concurrent streams or money. Adding a costModel route here
// silences this test; it does not meter that route. 57 號 §7 is where a model call
// gets metered.
//
// ⚠️ Highest-frequency row, worth knowing before retuning RATE_LIMIT_DEFAULT_PER_
// WINDOW: GET .../candidate-searches/:runId is polled every 1.5s for the whole
// duration of a candidate search (hawkr-website
// features/procurement-assistant/useProcurementSession.ts), i.e. ~40 req/min per
// searching buyer against a 300/60s bucket. One buyer is comfortable; a single
// shared egress address with several buyers searching at once is not, and the
// website's poller treats the resulting 429 as a network failure (three backoffs,
// then a manual retry button). If that shows up, the levers are POLL_INTERVAL_MS
// on the website side and the tier quota on this side — not a new exemption.
var aiRouteTiers = []aiRoute{
	// 賣家 AI 助手 — six SSE lanes, one JSON autofill, one JSON session read.
	{"POST", "/api/v1/app/sellers/me/assistant/chat", "default", costModel},
	{"POST", "/api/v1/app/sellers/me/assistant/orders", "default", costModel},
	{"POST", "/api/v1/app/sellers/me/assistant/inventory", "default", costModel},
	{"POST", "/api/v1/app/sellers/me/assistant/payment", "default", costModel},
	{"POST", "/api/v1/app/sellers/me/assistant/product-listing/chat", "default", costModel},
	// Not SSE (one-shot JSON) but still a model call — 「AI 解析並回傳結構化商品草稿」.
	{"POST", "/api/v1/app/sellers/me/assistant/product-autofill", "default", costModel},
	{"GET", "/api/v1/app/sellers/me/assistant/sessions/latest", "default", costData},

	// 買家 AI 助手 — six SSE lanes, two JSON reads.
	{"POST", "/api/v1/app/buyers/me/assistant/chat", "default", costModel},
	{"POST", "/api/v1/app/buyers/me/assistant/orders", "default", costModel},
	{"POST", "/api/v1/app/buyers/me/assistant/tracking", "default", costModel},
	{"POST", "/api/v1/app/buyers/me/assistant/return-policy", "default", costModel},
	{"POST", "/api/v1/app/buyers/me/assistant/seller-match", "default", costModel},
	{"GET", "/api/v1/app/buyers/me/assistant/sessions/latest", "default", costData},

	// Hawkr 幫找 — /turns and /turns/stream are the same LLM turn, one unary and one
	// streamed; candidate-searches POST starts the two search lanes.
	{"POST", "/api/v1/app/buyers/me/procurement/turns", "default", costModel},
	{"POST", "/api/v1/app/buyers/me/procurement/turns/stream", "default", costModel},
	{"POST", "/api/v1/app/buyers/me/procurement/requests/:id/candidate-searches", "default", costModel},
	// answers and rewinds look like plain state edits and are not: both run
	// usecase.ReplyComposer, which is an llmagent (usecase/reply_composer.go).
	{"POST", "/api/v1/app/buyers/me/procurement/requests/:id/answers", "default", costModel},
	{"GET", "/api/v1/app/buyers/me/procurement/requests/:id/candidate-searches/:runId", "default", costData},
	{"GET", "/api/v1/app/buyers/me/procurement/requests/:id/snapshot", "default", costData},
	// RFQ price preview (issue #2403 Slice 2): PreviewRfqPriceUseCase.PreviewRfqPrice
	// reads DB, no LLM — same cost
	// shape as quote-batches/preview below, not the ReplyComposer-driven rows above.
	{"POST", "/api/v1/app/buyers/me/procurement/requests/:id/rfq-price-previews", "default", costData},
	// RFQ confirmation (issue #2403 Slice 3): ConfirmRfqSnapshotUseCase.ConfirmRfqSnapshot
	// is DB idempotency-key lookup + a price-preview CAS check + one insert, no LLM and no
	// downstream gRPC call — same cost shape as the row above.
	{"POST", "/api/v1/app/buyers/me/procurement/requests/:id/rfq-confirmations", "default", costData},
	// RFQ publication preview (issue #2450 Slice 1): PublicationPreviewUseCase.
	// PreviewRfqPublication reads DB (revision / confirmed snapshot / price preview /
	// candidate set), compiles two hashes in-process and does one insert — no LLM and
	// no downstream service call. Same cost shape as the two rows above.
	//
	// ⛔ It creates the approval only: no publication, invite, outbox or projection,
	// so it carries no outbound fan-out cost either.
	{"POST", "/api/v1/app/buyers/me/procurement/requests/:id/rfq-publications/preview", "default", costData},
	// Execute (#2450 Scope 4): CAS-checked write of the approval into a publication
	// row. Same reasoning as preview above — no synchronous LLM call, and this
	// slice performs zero outward delivery (no invite/outbox/projection fan-out),
	// so it carries no outbound cost either.
	{"POST", "/api/v1/app/buyers/me/procurement/requests/:id/rfq-publications", "default", costData},
	{"POST", "/api/v1/app/buyers/me/procurement/requests/:id/selections", "default", costData},
	{"POST", "/api/v1/app/buyers/me/procurement/requests/:id/rewinds", "default", costModel}, // ReplyComposer, see above
	{"GET", "/api/v1/app/buyers/me/procurement/conversations", "default", costData},
	{"GET", "/api/v1/app/buyers/me/procurement/conversations/:id/messages", "default", costData},
	{"GET", "/api/v1/app/buyers/me/procurement/conversations/:id/runtime-snapshot", "default", costData},
	// #2538 買家面四支：比較清單是 DB 讀，shortlist 是單列 upsert／delete，
	// acceptance 是一次帶 CAS 的 insert ＋（撞唯一鍵時）一次交易外回查。都沒有
	// 同步 LLM 呼叫，同 costData。
	{"GET", "/api/v1/app/buyers/me/procurement/requests/:id/seller-responses", "default", costData},
	{"PUT", "/api/v1/app/buyers/me/procurement/requests/:id/shortlist/:seller_response_id/versions/:seller_response_version", "default", costData},
	{"DELETE", "/api/v1/app/buyers/me/procurement/requests/:id/shortlist/:seller_response_id/versions/:seller_response_version", "default", costData},
	{"POST", "/api/v1/app/buyers/me/procurement/requests/:id/buyer-quote-acceptances", "default", costData},
	{"PATCH", "/api/v1/app/buyers/me/procurement/conversations/:id", "default", costData},
	{"DELETE", "/api/v1/app/buyers/me/procurement/conversations/:id", "default", costData},
	// Task 12: run ledger replay/subscribe (WatchRunEvents). Not costModel — the
	// LLM turn already happened on whichever POST created this run (turns/stream,
	// answers, rewinds); this route only replays already-written ledger rows plus
	// a DB LISTEN/wake subscription (repository.WatchRun), same cost shape as the
	// other GET rows above. Its per-minute request rate is lower than the polled
	// candidate-searches row too — one connection stays open up to
	// streamTurnDeadline (120s) instead of firing every 1.5s.
	{"GET", "/api/v1/app/buyers/me/procurement/runs/:runId/events", "default", costData},
	// Task 14: cancel/resume. CancelRun is a version-CAS'd status update plus one
	// ledger event append; ResumeRun resolves an interrupt and creates one child
	// run row (QUEUED) — neither calls an LLM synchronously (the resumed child
	// run's own pipeline execution, if/when dispatched, would be billed under
	// whichever future entry point actually runs it, not this command endpoint),
	// same cost shape as selections/quote-batches above.
	{"POST", "/api/v1/app/buyers/me/procurement/runs/:runId/cancel", "default", costData},
	{"POST", "/api/v1/app/buyers/me/procurement/runs/:runId/resume", "default", costData},
}

func aiRouteKey(method, path string) string { return method + " " + path }

// liveAIRoutes returns every route the REAL router mounts under aiRoutePrefixes.
// This test package is middleware_test rather than middleware precisely so it may
// import router (router imports middleware, so the internal test package cannot).
func liveAIRoutes(t *testing.T) map[string]struct{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	router.SetupRoutes(r, &handler.Holder{})

	live := map[string]struct{}{}
	for _, ri := range r.Routes() {
		for _, p := range aiRoutePrefixes {
			if strings.HasPrefix(ri.Path, p) {
				live[aiRouteKey(ri.Method, ri.Path)] = struct{}{}
				break
			}
		}
	}
	return live
}

// TestAIRouteTierTableMatchesLiveRouter is the drift gate. Mounting a new
// assistant/procurement endpoint without classifying it fails here — the property
// a prefix rule made structurally unreachable, since under one a new route
// inherited "unlimited" and nothing anywhere had to be edited.
func TestAIRouteTierTableMatchesLiveRouter(t *testing.T) {
	live := liveAIRoutes(t)

	declared := map[string]struct{}{}
	for _, r := range aiRouteTiers {
		k := aiRouteKey(r.method, r.path)
		if _, dup := declared[k]; dup {
			t.Errorf("aiRouteTiers lists %s twice", k)
		}
		declared[k] = struct{}{}
	}

	var missing, stale []string
	for k := range live {
		if _, ok := declared[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range declared {
		if _, ok := live[k]; !ok {
			stale = append(stale, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)

	for _, k := range missing {
		t.Errorf("route %s is mounted but has no rate-limit classification: add it to aiRouteTiers (read 57 號 §7 before marking it costModel)", k)
	}
	for _, k := range stale {
		t.Errorf("aiRouteTiers declares %s but no such route is mounted: delete the row", k)
	}
}

// TestAIRoutesAreNotRateLimitExempt pins the actual #2505 fix: no route under the
// three AI trees may classify as exempt, whatever the table happens to say.
//
// Kept separate from the table comparison on purpose. The table is data and a row
// may legitimately be edited; "exempt" is the one value that must never reappear
// in it, and why is the whole issue.
func TestAIRoutesAreNotRateLimitExempt(t *testing.T) {
	for _, r := range aiRouteTiers {
		got := middleware.RateLimitTierNameForTest(r.path)
		if got == "exempt" {
			t.Errorf("%s %s classifies as exempt — #2505 removed the prefix exemption because it handed every JSON/CRUD route under it an unlimited per-subject budget; a route that genuinely needs exemption goes in exemptExact with its reason written next to it", r.method, r.path)
			continue
		}
		if got != r.tier {
			t.Errorf("%s %s: classifyTier = %q, aiRouteTiers says %q", r.method, r.path, got, r.tier)
		}
	}
}

// TestAIRoutePrefixesHaveNoExemptSurvivor guards the shape rather than the roster:
// no path under these trees may resolve to exempt, including one no route uses
// yet. It catches an exemptPrefixes entry reinstated by a merge before any new
// endpoint is mounted behind it.
func TestAIRoutePrefixesHaveNoExemptSurvivor(t *testing.T) {
	for _, p := range aiRoutePrefixes {
		probe := p + "some-new-endpoint"
		if got := middleware.RateLimitTierNameForTest(probe); got == "exempt" {
			t.Errorf("prefix %s still grants exemption (probe %s → %q)", p, probe, got)
		}
	}
}
