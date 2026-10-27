package resend

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// s.http is unexported, so WithHTTPClient's timeout guard (New, above) can
// only be observed from inside this package — hence an internal test file,
// same split as classify_test.go already uses in this directory.

// 呼叫端傳進來的 client 若 Timeout 是零值，New 要補上 defaultTimeout；同時不能
// 動到呼叫端自己手上那個 *http.Client（它可能是 process 裡其他地方也在用的共用
// 物件，例如裝了 tracing 的那個）。
func TestNewWithHTTPClientZeroTimeoutGetsDefault(t *testing.T) {
	caller := &http.Client{}

	s := New("k", WithHTTPClient(caller))

	require.Equal(t, defaultTimeout, s.http.Timeout, "零 timeout 的 client 要補上 defaultTimeout")
	require.Zero(t, caller.Timeout, "New 不可以動到呼叫端自己的 client 物件")
}

// 呼叫端自帶非零 timeout 時，New 不能覆蓋掉它。
func TestNewWithHTTPClientKeepsNonZeroTimeout(t *testing.T) {
	caller := &http.Client{Timeout: 3 * time.Second}

	s := New("k", WithHTTPClient(caller))

	require.Equal(t, 3*time.Second, s.http.Timeout)
	require.Equal(t, 3*time.Second, caller.Timeout)
}
