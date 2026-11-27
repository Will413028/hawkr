//go:build ruleguard
// +build ruleguard

package gateway

import "github.com/quasilyte/go-ruleguard/dsl"

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


func banDirectProtoPayload(m dsl.Matcher) {
	m.Match(
		`response.OK($c, $payload)`,
		`response.GrpcResult($c, $code, $message, $payload)`,
	).
		Where(m.File().PkgPath.Matches(`gateway/internal/handler`) &&
			m["payload"].Type.Implements(`google.golang.org/protobuf/proto.Message`)).
		Report(`Gateway 禁止直接輸出 proto message；請建立 endpoint DTO 並逐欄轉換`)

	m.Match(
		`gin.H{$*_, $key: $payload, $*_}`,
		`map[string]interface{}{$*_, $key: $payload, $*_}`,
		`map[string]any{$*_, $key: $payload, $*_}`,
	).
		Where(m.File().PkgPath.Matches(`gateway/internal/handler`) &&
			m["payload"].Text.Matches(`^resp\.(Reports|Violations|Rows|SellerReport|FoodSafety|Report|Violation|Blocks|Suspensions|Suspension)$`)).
		Report(`Gateway response map 禁止嵌入 proto 欄位；請先轉成 endpoint DTO`)
}

func banClientIPForQuota(m dsl.Matcher) {
	m.Match(`$c.ClientIP()`).
		Where(m.File().PkgPath.Matches(`gateway/internal/`) &&
			!m.File().Name.Matches(`^(logger|tracing|ratelimit_subject|ratelimit_test)\.go$`)).
		Report(`c.ClientIP() 不是配額身分；用 middleware.RateLimitSubject(c)（唯一問答點，見 01 設計書 §3.2.4）`)
}

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
