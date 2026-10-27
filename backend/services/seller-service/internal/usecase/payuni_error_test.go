package usecase

import (
	"errors"
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
)

func TestParsePayuniErrorCode(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "code with description (standard)",
			message: "AGENT04017 商店代號已存在",
			want:    "AGENT04017",
		},
		{
			name:    "code in middle",
			message: "錯誤 AGENT03009：會員電子信箱格式錯誤",
			want:    "AGENT03009",
		},
		{
			name:    "description only (no code)",
			message: "商店代號已存在",
			want:    "",
		},
		{
			name:    "empty message",
			message: "",
			want:    "",
		},
		{
			name:    "code with punctuation",
			message: "[AGENT07003] 系統錯誤，請稍後再試",
			want:    "AGENT07003",
		},
		{
			name:    "malformed code 4 digits - no match",
			message: "AGENT0401 bad code",
			want:    "",
		},
		{
			name:    "malformed code 6 digits - matches first 5",
			message: "AGENT040171 too many digits",
			want:    "AGENT04017",
		},
		{
			name:    "multiple codes - returns first",
			message: "AGENT03009 and AGENT04017",
			want:    "AGENT03009",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePayuniErrorCode(tt.message)
			if got != tt.want {
				t.Errorf("parsePayuniErrorCode(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

func TestPayuniBindError_DomainCode(t *testing.T) {
	tests := []struct {
		err      *PayuniBindError
		name     string
		wantCode pkgerrors.ErrorCode
	}{
		{
			name:     "network error (no code)",
			err:      &PayuniBindError{Cause: errors.New("connection refused")},
			wantCode: pkgerrors.CodeUnavailable,
		},
		{
			name:     "AGENT01003 config error (hash mismatch)",
			err:      &PayuniBindError{Code: "AGENT01003", Message: "資料 HASH 比對不符合"},
			wantCode: pkgerrors.CodeUnknown,
		},
		{
			name:     "AGENT05001 agent not enabled",
			err:      &PayuniBindError{Code: "AGENT05001", Message: "代理商未啟用"},
			wantCode: pkgerrors.CodeUnknown,
		},
		{
			name:     "AGENT03009 email format",
			err:      &PayuniBindError{Code: "AGENT03009", Message: "會員電子信箱格式錯誤"},
			wantCode: pkgerrors.CodeInvalidInput,
		},
		{
			name:     "AGENT03019 member duplicated",
			err:      &PayuniBindError{Code: "AGENT03019", Message: "會員帳號或手機或 Email 資料重複"},
			wantCode: pkgerrors.CodeConflict,
		},
		{
			name:     "AGENT03031 member suspended",
			err:      &PayuniBindError{Code: "AGENT03031", Message: "此會員已停權"},
			wantCode: pkgerrors.CodePreconditionFailed,
		},
		{
			name:     "AGENT04028 english name charset",
			err:      &PayuniBindError{Code: "AGENT04028", Message: "英文帳單名稱字元集錯誤"},
			wantCode: pkgerrors.CodeInvalidInput,
		},
		{
			name:     "AGENT07003 system error",
			err:      &PayuniBindError{Code: "AGENT07003", Message: "系統錯誤，請稍後再試"},
			wantCode: pkgerrors.CodeUnavailable,
		},
		{
			name:     "AGENT07006 system error",
			err:      &PayuniBindError{Code: "AGENT07006", Message: "系統錯誤"},
			wantCode: pkgerrors.CodeUnavailable,
		},
		{
			// AGENT07021 是平台送出的費率設定被拒，非賣家可填欄位，見
			// payuni_error_catalog.go 的 payuniErrorGroups「平台送出的固定參數或
			// 平台生成值被拒」分組（remedy: RemedyContactSupport -> CodeUnknown）。
			name:     "AGENT07021 platform fee config (contact support, not seller field)",
			err:      &PayuniBindError{Code: "AGENT07021", Message: "手續費率設定錯誤"},
			wantCode: pkgerrors.CodeUnknown,
		},
		{
			name:     "AGENT08001 person in charge",
			err:      &PayuniBindError{Code: "AGENT08001", Message: "請輸入負責人姓名"},
			wantCode: pkgerrors.CodeInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pkgerrors.ErrorCode(tt.err.DomainCode())
			if got != tt.wantCode {
				t.Errorf("DomainCode() = %d, want %d", got, tt.wantCode)
			}
		})
	}
}

func TestPayuniBindError_ErrorsAs(t *testing.T) {
	// Ensure errors.As works with PayuniBindError so the gRPC interceptor
	// can detect it via DomainCoder.
	var err error = &PayuniBindError{Code: "AGENT04028", Message: "test"}

	var pe *PayuniBindError
	if !errors.As(err, &pe) {
		t.Fatal("errors.As failed to unwrap *PayuniBindError")
	}
	if pe.Code != "AGENT04028" {
		t.Errorf("got Code=%q, want AGENT04028", pe.Code)
	}

	var dc pkgerrors.DomainCoder
	if !errors.As(err, &dc) {
		t.Fatal("errors.As failed to detect DomainCoder implementation")
	}
}

func TestIsPayuniAlreadyExistsError(t *testing.T) {
	// Input is PayUni's error code (from Status field), not the message.
	tests := []struct {
		code string
		want bool
	}{
		{"AGENT04017", true},
		{"AGENT04018", false},
		{"AGENT03019", false},
		{"", false},
		{"SUCCESS", false},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := IsPayuniAlreadyExistsError(tt.code)
			if got != tt.want {
				t.Errorf("IsPayuniAlreadyExistsError(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}
