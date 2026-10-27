package cropidentity

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type scriptedResolver struct {
	errs  []error // 第 i 次呼叫回 errs[i]；超出長度視為成功
	calls int
	got   DisambiguationResult
}

func (s *scriptedResolver) Resolve(context.Context, string) (DisambiguationResult, error) {
	i := s.calls
	s.calls++
	if i < len(s.errs) && s.errs[i] != nil {
		return DisambiguationResult{}, s.errs[i]
	}
	return s.got, nil
}

// TestWithTransientRetry_RecoversOnSecondAttempt：Pricebook 抖一下就好的情況，
// 買家完全無感——這一輪照樣拿到行情，不會掉進「自己填價格區間」。
func TestWithTransientRetry_RecoversOnSecondAttempt(t *testing.T) {
	code := "CROP-GUAVA"
	inner := &scriptedResolver{
		errs: []error{errors.New("pricebook unavailable")},
		got:  DisambiguationResult{CropCode: &code, Label: "芭樂"},
	}

	got, err := WithTransientRetry(inner).Resolve(context.Background(), "芭樂")

	require.NoError(t, err)
	require.Equal(t, 2, inner.calls, "第一次失敗要當場再試一次")
	require.Equal(t, "CROP-GUAVA", *got.CropCode)
}

// TestWithTransientRetry_StopsAtOneRetry：連兩次都失敗就放手，交給跨輪重試。
// ⛔ 不得變成退避迴圈：買家正在等這一輪。
func TestWithTransientRetry_StopsAtOneRetry(t *testing.T) {
	inner := &scriptedResolver{errs: []error{
		errors.New("boom"), errors.New("boom"), errors.New("boom"),
	}}

	_, err := WithTransientRetry(inner).Resolve(context.Background(), "芭樂")

	require.Error(t, err)
	require.Equal(t, 2, inner.calls, "最多兩次呼叫（原始一次＋重試一次）")
}

// TestWithTransientRetry_DoesNotRetryWhenContextIsDone：請求本身已經沒有時間了，
// 重試只會讓逾時更晚被發現。
func TestWithTransientRetry_DoesNotRetryWhenContextIsDone(t *testing.T) {
	inner := &scriptedResolver{errs: []error{context.Canceled}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := WithTransientRetry(inner).Resolve(ctx, "芭樂")

	require.Error(t, err)
	require.Equal(t, 1, inner.calls, "context 已結束不重試")
}

// TestWithTransientRetry_SuccessIsNotRetried 是負控制：成功（含「查無此品項」與
// 「需要消歧」這兩種**成功回傳的結果**）不得重試。
func TestWithTransientRetry_SuccessIsNotRetried(t *testing.T) {
	for name, result := range map[string]DisambiguationResult{
		"unavailable":  {Unavailable: true},
		"needs_choice": {NeedsDisambiguation: true},
	} {
		t.Run(name, func(t *testing.T) {
			inner := &scriptedResolver{got: result}
			_, err := WithTransientRetry(inner).Resolve(context.Background(), "芭樂")
			require.NoError(t, err)
			require.Equal(t, 1, inner.calls, "這是答案不是故障，重試只是多等一次")
		})
	}
}

func TestWithTransientRetry_NilInnerStaysNil(t *testing.T) {
	require.Nil(t, WithTransientRetry(nil))
}
