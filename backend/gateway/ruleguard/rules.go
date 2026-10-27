//go:build ruleguard
// +build ruleguard

package gateway

import "github.com/quasilyte/go-ruleguard/dsl"

// banDirectJSON forbids direct gin JSON emitters in handler/middleware/router/proxy;
// use response.* helpers (gateway/internal/response/response.go) instead.
func banDirectJSON(m dsl.Matcher) {
	m.Match(
		`$c.JSON($*_)`,
		`$c.IndentedJSON($*_)`,
		`$c.PureJSON($*_)`,
		`$c.SecureJSON($*_)`,
		`$c.AbortWithStatusJSON($*_)`,
	).
		Where(m.File().PkgPath.Matches(`gateway/internal/(handler|middleware|router|proxy)`)).
		Report(`direct gin JSON emitter banned; use response.* helpers (see response/response.go)`)
}

// banLiteralOKMsg 已於 2026-08-22（#2704）移除：它守的 response.OKMsg 本身已刪除。
//
// 那個函式的 message 參數在 pkg/envelope 抽取時就被丟棄（函式體硬寫 "success"），所以
// 「唯一合法用途是透傳下游訊息」從實作上就不存在，而 ruleguard 只擋字面值、擋不住這個。
// 現在成功回應唯一入口是 response.OK / Page / GrpcResult，三者的 message 都由 envelope
// 固定成 "success"；要傳語意就放進 data。自造訊息的其餘出口由 banDirectJSON 守住，
// 而「再引入一個 OKMsg」已由編譯器擋下——那比 lint 規則強。

// banDirectProtoPayload keeps generated protobuf structs behind endpoint DTOs.
// protoc-gen-go JSON tags are snake_case + omitempty, which violates the public
// HTTP contract even when the Go value looks structurally compatible.
func banDirectProtoPayload(m dsl.Matcher) {
	m.Match(
		`response.OK($c, $payload)`,
		`response.GrpcResult($c, $code, $message, $payload)`,
	).
		Where(m.File().PkgPath.Matches(`gateway/internal/handler`) &&
			m["payload"].Type.Implements(`google.golang.org/protobuf/proto.Message`)).
		Report(`Gateway 禁止直接輸出 proto message；請建立 endpoint DTO 並逐欄轉換`)

	// Composite response payloads hide proto messages one level down. These are
	// the repeated/singular governance and moderation fields that triggered
	// #2267; matching the selector at the gin.H boundary also covers slices,
	// whose type itself does not implement proto.Message.
	m.Match(
		`gin.H{$*_, $key: $payload, $*_}`,
		`map[string]interface{}{$*_, $key: $payload, $*_}`,
		`map[string]any{$*_, $key: $payload, $*_}`,
	).
		Where(m.File().PkgPath.Matches(`gateway/internal/handler`) &&
			m["payload"].Text.Matches(`^resp\.(Reports|Violations|Rows|SellerReport|FoodSafety|Report|Violation|Blocks|Suspensions|Suspension)$`)).
		Report(`Gateway response map 禁止嵌入 proto 欄位；請先轉成 endpoint DTO`)
}

// banClientIPForQuota keeps "who is this request charged to" a single question
// point (#2264).
//
// c.ClientIP() answers "which machine sent this", which is NOT the same thing:
// the website's SSR worker, the admin BFF and the CI runner each present one
// address for an unbounded number of people. Before this rule the four per-IP
// mechanisms (tier quotas, auth-failure axis, OTP counts, SSE connection caps)
// each called it themselves and each got that wrong independently. Use
// middleware.RateLimitSubject(c).
//
// Allowlist, each with its own reason:
//   - logger.go        — forensic access log; the real peer is the point
//   - tracing.go       — http.client_ip is an OTel semconv attribute name; a
//     non-IP value there makes the span lie
//   - ratelimit_subject.go — RateLimitSubject and the vendor portal's address key
//   - ratelimit_test.go    — asserts gin's own IP resolution, orthogonal to the
//     subject layer. .golangci.yml's _test.go exemption is
//     source-scoped and does not cover ruleguard, so without
//     naming it here this gate would red-flag our own test on
//     day one.
func banClientIPForQuota(m dsl.Matcher) {
	m.Match(`$c.ClientIP()`).
		Where(m.File().PkgPath.Matches(`gateway/internal/`) &&
			!m.File().Name.Matches(`^(logger|tracing|ratelimit_subject|ratelimit_test)\.go$`)).
		Report(`c.ClientIP() 不是配額身分；用 middleware.RateLimitSubject(c)（唯一問答點，見 01 設計書 §3.2.4）`)
}

// banEnvelopeSuccessEmitters keeps every success envelope behind
// internal/response, because that is where bizcarrier is consulted.
//
// pkg/envelope's OK/OKProto/Page/GrpcResult write `code: 0` with no knowledge of
// whether the upstream reply carried a preserveData business code (9331, 4001,
// …). internal/response's wrappers take the carrier first and rewrite the
// response to Biz(code, message, data) when one is pending. Calling the pkg
// helpers directly from a handler therefore reintroduces exactly the silent
// success this mechanism exists to make unwritable — and it does so while
// looking like a harmless import shortcut.
//
// Scoped to the four packages that produce HTTP responses. internal/response
// itself must keep calling them (it is the wrapper), and non-gateway code is
// out of scope. Existing violations at introduction: zero — handler's two
// current pkg/envelope imports use ExtractTraceID (procurement_errors.go) and
// ParseBizMessage (delegated_capability.go), neither of which is an emitter.
func banEnvelopeSuccessEmitters(m dsl.Matcher) {
	m.Match(
		`envelope.OK($*_)`,
		`envelope.OKProto($*_)`,
		`envelope.Page($*_)`,
		`envelope.GrpcResult($*_)`,
	).
		Where(m.File().PkgPath.Matches(`gateway/internal/(handler|middleware|router|proxy)`)).
		Report(`成功 envelope 只能經 response.*（它會先取 bizcarrier）；直呼 pkg/envelope 會繞過業務碼保護`)
}
