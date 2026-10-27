package resend

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/mail"
	"github.com/stretchr/testify/require"
)

// 這張表逐條對應 Resend 官方錯誤清單（2026-09-11 查核）。
// 新增一列時要同時回答：重試這個錯誤有沒有可能成功？沒有就是 permanent。
func TestClassify(t *testing.T) {
	transient := []struct {
		status int
		name   string
	}{
		{429, "rate_limit_exceeded"},
		{429, "daily_quota_exceeded"},
		{429, "monthly_quota_exceeded"},
		// body 不是預期的 JSON 形狀時（例如 CDN/WAF/proxy 回 HTML）也要分類正確——
		// Resend 的 error body 形狀不在它的文件契約裡，所以這裡要靠狀態碼本身，
		// 一條規則需要涵蓋這兩個 name 為空的情況。
		{429, ""},
		// Resend 本身不會回 408，但我們與 Resend 之間的 proxy 可能會；HTTP 語意
		// 本身就把重試當作 408 的標準處理方式。
		{408, ""},
		{500, "application_error"},
		{503, "service_unavailable"},
		// 同一把 idempotency key 正在處理中——稍後重試會拿到那一次的結果。
		{409, "concurrent_idempotent_requests"},
		{409, "resource_locked"},
		// 未列出的 5xx 一律當可重試。
		{502, ""},
		{504, "unknown_future_name"},
	}
	for _, tc := range transient {
		t.Run("transient/"+tc.name, func(t *testing.T) {
			err := classify(tc.status, tc.name, "msg")
			require.ErrorIs(t, err, mail.ErrTransient)
			require.NotErrorIs(t, err, mail.ErrPermanent)
			require.Contains(t, err.Error(), "msg")
		})
	}

	permanent := []struct {
		status int
		name   string
	}{
		{400, "validation_error"},
		{400, "invalid_idempotency_key"},
		{401, "missing_api_key"},
		{401, "restricted_api_key"},
		{403, "invalid_permission"},
		{403, "restricted_api_key"},
		{403, "suspended_api_key"},
		{403, "email_above_quota"},
		{403, "validation_error"},
		{404, "not_found"},
		{405, "method_not_allowed"},
		// 同一把 key、不同內容：重送同樣的東西只會再被拒。
		{409, "invalid_idempotent_request"},
		{422, "invalid_parameter"},
		{422, "missing_required_field"},
		{422, "missing_required_parameter"},
		{422, "invalid_attachment"},
		// 未列出的 4xx 一律當不可重試。
		{418, ""},
	}
	for _, tc := range permanent {
		t.Run("permanent/"+tc.name, func(t *testing.T) {
			err := classify(tc.status, tc.name, "msg")
			require.ErrorIs(t, err, mail.ErrPermanent)
			require.NotErrorIs(t, err, mail.ErrTransient)
		})
	}
}

// 409 is the only status that splits between transient and permanent, so the name
// cannot be ignored. 403 is uniformly permanent (each code requires human action).
func TestClassifySplitsWithinSameStatus(t *testing.T) {
	require.ErrorIs(t, classify(409, "concurrent_idempotent_requests", ""), mail.ErrTransient)
	require.ErrorIs(t, classify(409, "invalid_idempotent_request", ""), mail.ErrPermanent)
}

// 2xx 不該進到這裡；真的進來了要吵，不要靜默當成功。
func TestClassifyRejectsSuccessStatus(t *testing.T) {
	err := classify(200, "", "")
	require.ErrorIs(t, err, mail.ErrPermanent)
}
