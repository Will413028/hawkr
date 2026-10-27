package usecase

import (
	"regexp"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
)

// payuniAgentCodeRe matches PayUni error codes like "AGENT04017".
var payuniAgentCodeRe = regexp.MustCompile(`AGENT\d{5}`)

// parsePayuniErrorCode extracts a PayUni error code from a response message.
// Returns empty string if no AGENT code is found.
//
// Examples:
//
//	"AGENT04017 商店代號已存在"                  -> "AGENT04017"
//	"錯誤 AGENT03009：會員電子信箱格式錯誤"      -> "AGENT03009"
//	"商店代號已存在"                              -> ""
func parsePayuniErrorCode(message string) string {
	return payuniAgentCodeRe.FindString(message)
}

// PayuniBindError represents a failure from PayUni bind_merchant API.
// It carries the original AGENT error code for observability and implements
// pkgerrors.DomainCoder so the gRPC error interceptor can map it to the
// appropriate gRPC status code.
type PayuniBindError struct {
	// Cause is the underlying error (network, HTTP status, etc.) if any.
	Cause error

	// Code is the PayUni AGENT error code (e.g. "AGENT04028"). Empty for
	// network / serialization failures with no business error code.
	Code string

	// Message is the user-facing error message (already localized by PayUni).
	Message string
}

// Error implements the error interface.
func (e *PayuniBindError) Error() string {
	if e.Code != "" {
		return "payuni bind failed [" + e.Code + "]: " + e.Message
	}
	if e.Cause != nil {
		return "payuni bind failed: " + e.Cause.Error()
	}
	return "payuni bind failed: " + e.Message
}

// Unwrap returns the underlying cause for errors.Is/errors.As support.
func (e *PayuniBindError) Unwrap() error {
	return e.Cause
}

// DomainCode maps the PayUni error code to a pkgerrors.ErrorCode so the gRPC
// error interceptor can translate it to the appropriate gRPC status code.
//
// Mapping rules:
//   - Network / unknown error (no Code) -> Unavailable (503)
//   - AGENT03019                        -> Conflict (409 AlreadyExists)
//   - AGENT03031                        -> PreconditionFailed (409)
//   - Everything else defers to the payuni_error_catalog.go remedy:
//     RemedyRetryLater -> Unavailable (503), RemedyContactSupport -> Unknown (500),
//     RemedySellerFix (or unknown remedy) -> InvalidInput (400)
func (e *PayuniBindError) DomainCode() int {
	if e.Code == "" {
		return int(pkgerrors.CodeUnavailable)
	}
	switch e.Code {
	case "AGENT03019":
		return int(pkgerrors.CodeConflict)
	case "AGENT03031":
		return int(pkgerrors.CodePreconditionFailed)
	}
	switch ResolvePayuniError(e.Code, "").Remedy {
	case RemedyRetryLater:
		return int(pkgerrors.CodeUnavailable)
	case RemedyContactSupport:
		return int(pkgerrors.CodeUnknown)
	default:
		return int(pkgerrors.CodeInvalidInput)
	}
}

// DomainMessage returns the user-facing message (without internal prefix).
func (e *PayuniBindError) DomainMessage() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return "PayUni 服務暫時無法使用，請稍後再試"
	}
	return "PayUni 註冊失敗"
}

// newPayuniBindErrorFromNetwork wraps a network / serialization error.
func newPayuniBindErrorFromNetwork(cause error) *PayuniBindError {
	return &PayuniBindError{
		Message: "PayUni 服務暫時無法使用，請稍後再試",
		Cause:   cause,
	}
}

// newPayuniBindErrorFromBusiness wraps a PayUni business error response.
// errorCode is PayUni's error code from the Status field (e.g. "AGENT04017");
// rawMessage is the localized human-readable description from the Message field.
// If errorCode is empty the parser falls back to scanning the message for an
// AGENT-prefixed token so legacy callers (and any accidentally-stripped
// responses) still land on the right classification.
func newPayuniBindErrorFromBusiness(errorCode, rawMessage string) *PayuniBindError {
	code := errorCode
	if code == "" {
		code = parsePayuniErrorCode(rawMessage)
	}
	return &PayuniBindError{
		Code:    code,
		Message: rawMessage,
	}
}
