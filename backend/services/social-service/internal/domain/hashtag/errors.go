package hashtag

import "errors"

// Error sentinels mapped to gateway error codes (sharing product namespace —
// see docs/代碼管理（CodeMeta）/錯誤碼.md).
var (
	// ErrInvalidChar — code 3015
	ErrInvalidChar = errors.New("hashtag 含不允許字元（僅接受中文/英文/數字，不可含空白、#、emoji 或標點）")

	// ErrLengthExceeded — code 3016
	ErrLengthExceeded = errors.New("hashtag 長度超出範圍（每個 hashtag 須為 1–40 字元）")

	// ErrCountExceeded — code 3017
	ErrCountExceeded = errors.New("hashtag 數量超出上限（每篇貼文最多 10 個）")

	// ErrEmptyAfterNormalize — code 3018
	ErrEmptyAfterNormalize = errors.New("hashtag 經規範化後為空，請重新輸入")
)

// Numeric codes for use by the gRPC handler when wrapping for the gateway
// envelope ([NNNN] message format).
const (
	CodeInvalidChar         = 3015
	CodeLengthExceeded      = 3016
	CodeCountExceeded       = 3017
	CodeEmptyAfterNormalize = 3018
)
