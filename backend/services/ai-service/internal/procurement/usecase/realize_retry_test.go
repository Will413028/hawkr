package usecase

import (
	"context"
	"errors"
	"testing"

	aidomain "github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/stretchr/testify/require"
)

// scriptedSurfaceRealizer 依序回傳固定的 (blocks, err)，呼叫次數超過腳本長度時
// 重複回傳最後一筆——同 extract_retry_test.go 的 scriptedExtractor，用來讓
// retryingSurfaceRealizer 的重試邏輯可以被計數驗證。
type scriptedSurfaceRealizer struct {
	results []scriptedRealizeResult
	calls   int
}

type scriptedRealizeResult struct {
	blocks []RealizedBlock
	err    error
}

func (s *scriptedSurfaceRealizer) Realize(_ context.Context, _ SurfaceContext) ([]RealizedBlock, error) {
	i := s.calls
	if i >= len(s.results) {
		i = len(s.results) - 1
	}
	s.calls++
	r := s.results[i]
	return r.blocks, r.err
}

func TestRetryingSurfaceRealizer_SucceedsFirstTry_NoRetry(t *testing.T) {
	inner := &scriptedSurfaceRealizer{results: []scriptedRealizeResult{
		{blocks: []RealizedBlock{{Kind: "narrative"}}},
	}}
	re := &retryingSurfaceRealizer{inner: inner}

	blocks, err := re.Realize(context.Background(), SurfaceContext{})

	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, 1, inner.calls, "第一次就成功不該多打一次")
}

func TestRetryingSurfaceRealizer_RetriesOnProviderUnavailableError_ThenSucceeds(t *testing.T) {
	inner := &scriptedSurfaceRealizer{results: []scriptedRealizeResult{
		{err: &aidomain.ProviderUnavailableError{Err: errors.New("upstream 503")}},
		{blocks: []RealizedBlock{{Kind: "narrative"}}},
	}}
	re := &retryingSurfaceRealizer{inner: inner}

	blocks, err := re.Realize(context.Background(), SurfaceContext{})

	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, 2, inner.calls, "基礎設施性失敗要補打一次")
}

func TestRetryingSurfaceRealizer_DoesNotRetryNonRetryableError(t *testing.T) {
	inner := &scriptedSurfaceRealizer{results: []scriptedRealizeResult{
		{err: errors.New("400 invalid argument")},
	}}
	re := &retryingSurfaceRealizer{inner: inner}

	_, err := re.Realize(context.Background(), SurfaceContext{})

	require.Error(t, err)
	require.Equal(t, 1, inner.calls, "非可重試錯誤不該補打，浪費使用者等待時間")
}

// TestRetryingSurfaceRealizer_CallCountIsBoundByMaxRealizeAttempts 是final
// review Finding 4 的迴歸測試：修復前 maxRealizeAttempts 只是一個沒人讀的常數，
// Realize 的重試是寫死的單一 if 判斷——把這個常數改成任何值都不會改變
// Realize 實際打幾次。這裡直接斷言呼叫次數等於 maxRealizeAttempts 本身（而不是
// 寫死的字面值 2），這樣如果未來有人把迴圈又改回寫死判斷、或改壞邊界條件，
// 這個常數與實際行為對不上時，這支測試會先紅，而不是只有 histogram 鏡射
// 常數的相等性測試（那支測試只會在兩個常數不一致時紅，不會檢查常數是否真的
// 驅動了行為）。
func TestRetryingSurfaceRealizer_CallCountIsBoundByMaxRealizeAttempts(t *testing.T) {
	results := make([]scriptedRealizeResult, 0, maxRealizeAttempts+1)
	for i := 0; i < maxRealizeAttempts+1; i++ {
		results = append(results, scriptedRealizeResult{
			err: &aidomain.ProviderUnavailableError{Err: errors.New("still unavailable")},
		})
	}
	inner := &scriptedSurfaceRealizer{results: results}
	re := &retryingSurfaceRealizer{inner: inner}

	_, err := re.Realize(context.Background(), SurfaceContext{})

	require.Error(t, err)
	require.Equal(t, maxRealizeAttempts, inner.calls,
		"持續失敗時，Realize 的呼叫次數必須恰好等於 maxRealizeAttempts——"+
			"這個常數現在是迴圈的真正上限，不是一個沒人讀的標籤")
}
